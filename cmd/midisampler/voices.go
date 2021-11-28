package main

import (
	"sync"
)

type Voices struct {
	lastWas0 bool
	soundOff bool

	mu     sync.Mutex
	voices map[*voicedSample]struct{}

	voicesCopy []*voicedSample

	fx *Fx
}

func newVoices(sampleRate int) *Voices {
	fx, err := NewFx(sampleRate)
	if err != nil {
		panic(err)
	}
	return &Voices{
		voices: make(map[*voicedSample]struct{}),
		fx:     fx,
	}
}

var masterVolume = float32(1.0)
var bufferSize = 1024

func (vv *Voices) play(s []float32) {
	// Copy voices to avoid threading conflicts.
	vv.mu.Lock()
	voicesCopy := vv.voicesCopy
	vv.mu.Unlock()

	// Check if only writing out zeroes.
	if len(voicesCopy) > 0 && !vv.soundOff {
		vv.lastWas0 = false
	}
	if vv.lastWas0 {
		return
	}
	for i := 0; i < len(s); i++ {
		s[i] = 0
		vv.fx.revBuffer[i], vv.fx.choBuffer[i] = 0, 0
	}
	if len(voicesCopy) == 0 || vv.soundOff {
		vv.lastWas0 = true
		return
	}

	// Apply all voices to sample buffer.
	for _, vs := range voicesCopy {
		for i, v := range vs.step() {
			s[i] += v
			vv.fx.revBuffer[i] += vs.ReverbSendLevel * v
			vv.fx.choBuffer[i] += vs.ChorusSendLevel * v
		}
	}

	// Apply chorus and reverb effects.
	vv.fx.Run(s)

	// Apply master volume.
	for i := range s {
		s[i] *= masterVolume
	}
}

func (vv *Voices) add(s *Sample, vel float32, fxlvl *FxLevel) {
	var as *adsrCycleState
	if s.ADSR != nil {
		ac := s.Cycles(float64(s.rate))
		a := ac.Press(vel)
		as = &a
	}
	vs := &voicedSample{
		Sample:       s,
		adsrState:    as,
		remaining:    s.data,
		velocity:     vel,
		directBuffer: make([]float32, bufferSize),
		FxLevel:      fxlvl,
	}
	vv.mu.Lock()
	vv.voices[vs] = struct{}{}
	vv.mu.Unlock()

	vv.copyVoices()
}

func (vv *Voices) stop(s *Sample) {
	vv.mu.Lock()
	for vs := range vv.voices {
		if vs.Sample == s {
			if vs.adsrState != nil {
				vs.adsrState.Off()
			} else {
				delete(vv.voices, vs)
			}
		}
	}
	vv.mu.Unlock()
	vv.copyVoices()
}

func (vv *Voices) stopAll() {
	vv.mu.Lock()
	for vs := range vv.voices {
		if vs.adsrState != nil {
			vs.adsrState.Off()
		} else {
			delete(vv.voices, vs)
		}
	}
	vv.mu.Unlock()
	vv.copyVoices()
}

func (vv *Voices) copyVoices() {
	vv.mu.Lock()
	vv.voicesCopy = make([]*voicedSample, 0, len(vv.voices))
	for vs := range vv.voices {
		if vs.remaining == nil {
			delete(vv.voices, vs)
			continue
		}
		vv.voicesCopy = append(vv.voicesCopy, vs)
	}
	vv.mu.Unlock()
}

type voicedSample struct {
	*Sample
	adsrState    *adsrCycleState
	remaining    []float32 // slice from sample
	directBuffer []float32 // holds sample data before fx

	velocity float32
	*FxLevel

	mu sync.RWMutex
}

func (vs *voicedSample) step() []float32 {
	copyLen := len(vs.directBuffer)
	if copyLen > len(vs.remaining) {
		copyLen = len(vs.remaining)
	}
	if vs.adsrState == nil {
		for i := 0; i < copyLen; i++ {
			vs.directBuffer[i] = vs.velocity * vs.remaining[i]
		}
	} else {
		for i := 0; i < copyLen; i++ {
			vs.directBuffer[i] = vs.adsrState.Apply(vs.remaining[i])
		}
	}
	if len(vs.remaining) <= len(vs.directBuffer) {
		vs.remaining = nil
	} else {
		vs.remaining = vs.remaining[len(vs.directBuffer):]
	}
	return vs.directBuffer[:copyLen]
}
