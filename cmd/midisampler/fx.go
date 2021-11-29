package main

import (
	"github.com/chzchzchz/midispa/ladspa"
)

// Fx implements a General Midi 2 effects chain (section 2.9).
type Fx struct {
	reverb    *ladspa.Plugin
	chorus    *ladspa.Plugin
	revBuffer []float32
	choBuffer []float32

	SendToReverb  float32
	ChorusModRate *float32
}

type FxLevel struct {
	ChorusSendLevel float32
	ReverbSendLevel float32
}

func NewFx(rate int) (*Fx, error) {
	rev, err := ladspa.Reverb(rate)
	if err != nil {
		return nil, err
	}
	cho, err := ladspa.Chorus(rate)
	if err != nil {
		rev.Close()
		return nil, err
	}
	revBuffer := make([]float32, bufferSize)
	rev.Connect("Input", &revBuffer[0])
	rev.Connect("Output", &revBuffer[0])

	*rev.Control("Delay Time (s)") = 0.050
	*rev.Control("Dry Level (dB)") = 0.0
	*rev.Control("Wet Level (dB)") = 0.0
	*rev.Control("Feedback") = 0.05
	*rev.Control("Crossfade samples") = 64

	choBuffer := make([]float32, bufferSize)
	cho.Connect("Input", &choBuffer[0])
	cho.Connect("Output", &choBuffer[0])

	*cho.Control("Number of voices") = 3.0
	*cho.Control("Delay base (ms)") = 6.3
	*cho.Control("Voice separation (ms)") = 1.0
	*cho.Control("Detune (%)") = 1
	*cho.Control("LFO frequency (Hz)") = 1.1
	*cho.Control("Output attenuation (dB)") = -10.0

	return &Fx{
		reverb:        rev,
		chorus:        cho,
		revBuffer:     revBuffer,
		choBuffer:     choBuffer,
		ChorusModRate: cho.Control("LFO frequency (Hz)"),
		SendToReverb:  0,
	}, nil
}

func (fx *Fx) Run(directBuffer []float32) {
	// Compute chorus.
	fx.chorus.Run(len(directBuffer))
	// Mix chorus into reverb signal.
	if fx.SendToReverb != 0 {
		for i := range fx.choBuffer {
			fx.revBuffer[i] += fx.SendToReverb * fx.choBuffer[i]
		}
	}
	// Compute reverb.
	fx.reverb.Run(len(directBuffer))
	// Mix chorus and reverb into direct buffer.
	for i := range fx.revBuffer {
		directBuffer[i] += fx.revBuffer[i] + fx.choBuffer[i]
	}
}
