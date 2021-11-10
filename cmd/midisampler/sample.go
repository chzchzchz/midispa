package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-audio/wav"
)

var samples map[string]*Sample = make(map[string]*Sample)
var sampleSlice []*Sample

type Sample struct {
	time.Duration
	data []float32
	rate int
}

func LoadSample(name, path string) (*Sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := wav.NewDecoder(f)
	dec.ReadInfo()
	dur, err := dec.Duration()
	if err != nil {
		return nil, err
	}
	if format := dec.Format(); format.NumChannels != 1 {
		return nil, fmt.Errorf("bad format %+v", *format)
	}
	pcm, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	pcmf32 := pcm.AsFloat32Buffer()
	s := &Sample{Duration: dur, data: pcmf32.Data, rate: pcm.Format.SampleRate}

	samples[name] = s
	sampleSlice = append(sampleSlice, s)
	return s, nil
}
