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

func (s *Sample) Resample(sampleHz int) {
	if s.rate == sampleHz {
		return
	}
	ratio := float64(sampleHz) / float64(s.rate)
	newSamples := int(float64(len(s.data)) * ratio)
	newData := make([]float32, newSamples)
	newData[0] = s.data[0]
	var min float32
	var max float32
	for i := 0; i < newSamples; i++ {
		fi, fj := float64(i-1)*1.0/ratio, float64(i)*1.0/ratio
		ii, ij := int(fi), int(fj)
		alpha := float32(fi - float64(ii))
		newData[i] = (alpha*s.data[ii] + (1.0-alpha)*s.data[ij]) / 2.0
		if newData[i] > max {
			max = newData[i]
		} else if newData[i] < min {
			min = newData[i]
		}
	}
	for i := 0; i < newSamples; i++ {
		newData[i] = 2.0 * (((newData[i] - min) / (max - min)) - 0.5)
	}
	s.rate, s.data = sampleHz, newData
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
