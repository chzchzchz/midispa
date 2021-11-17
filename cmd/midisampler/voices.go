package main

import (
	"sync"
)

type voicedSample struct {
	*Sample
	adsrState *adsrCycleState
	remaining []float32 // slice from sample
	velocity  float32

	mu sync.RWMutex
}

var lastWas0 = false
var soundOff = false
var masterVolume = float32(1.0)

func playVoices(s []float32) {
	// Copy voices to avoid threading problems.
	// TODO: move outside loop, copy when voice is sounded.
	voicesCopy := copyVoices()
	// Check if only writing out zeroes.
	if len(voicesCopy) > 0 && !soundOff {
		lastWas0 = false
	}
	if lastWas0 {
		return
	}
	for i := 0; i < len(s); i++ {
		s[i] = 0
	}
	if len(voicesCopy) == 0 || soundOff {
		lastWas0 = true
		return
	}
	// Apply all voices to sample buffer.
	for _, vs := range voicesCopy {
		copyLen := len(s)
		if copyLen > len(vs.remaining) {
			copyLen = len(vs.remaining)
		}
		if vs.adsrState == nil {
			for i := 0; i < copyLen; i++ {
				s[i] += vs.velocity * vs.remaining[i]
			}
		} else {
			for i := 0; i < copyLen; i++ {
				s[i] += vs.adsrState.Apply(vs.remaining[i])
			}
		}

		if len(vs.remaining) <= len(s) {
			vs.remaining = nil
		} else {
			vs.remaining = vs.remaining[len(s):]
		}
	}
	for i := range s {
		s[i] *= masterVolume
	}
}

var voicesMu sync.Mutex
var voices map[*voicedSample]struct{} = make(map[*voicedSample]struct{})

func addVoice(s *Sample, vel float32) {
	var as *adsrCycleState
	if s.ADSR != nil {
		ac := s.Cycles(float64(s.rate))
		a := ac.Press(vel)
		as = &a
	}
	vs := &voicedSample{
		Sample:    s,
		adsrState: as,
		remaining: s.data,
		velocity:  vel,
	}
	voicesMu.Lock()
	voices[vs] = struct{}{}
	voicesMu.Unlock()
}

func stopVoice(s *Sample) {
	voicesMu.Lock()
	for vs := range voices {
		if vs.Sample == s {
			if vs.adsrState != nil {
				vs.adsrState.Off()
			} else {
				delete(voices, vs)
			}
		}
	}
	voicesMu.Unlock()
}

func stopVoices() {
	voicesMu.Lock()
	for vs := range voices {
		if vs.adsrState != nil {
			vs.adsrState.Off()
		} else {
			delete(voices, vs)
		}
	}
	voicesMu.Unlock()
}

func copyVoices() (ret []*voicedSample) {
	voicesMu.Lock()
	voicesCopy := make([]*voicedSample, 0, len(voices))
	for vs := range voices {
		if vs.remaining == nil {
			delete(voices, vs)
			continue
		}
		voicesCopy = append(voicesCopy, vs)
	}
	voicesMu.Unlock()
	return voicesCopy
}
