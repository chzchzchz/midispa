package main

import (
	"fmt"
	"os"
	"time"

	"github.com/go-audio/wav"
)

type Segment struct {
	Path       string        `json:"path"`
	Duration   time.Duration `json:"duration"`
	Samples    SampleTick    `json:"samples"`
	SampleRate int           `json:"sample_rate"`
}

func NewSegment(path string) (*Segment, error) {
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
	format := dec.Format()
	if format.NumChannels == 0 {
		return nil, fmt.Errorf("bad format %+v", *format)
	}
	if format.NumChannels > 1 {
		return nil, fmt.Errorf("expected mono channel")
	}
	sampleRate := format.SampleRate
	s := &Segment{
		Path:       path,
		Duration:   dur,
		Samples:    SampleTick(dur.Seconds() * float64(sampleRate)),
		SampleRate: sampleRate,
	}
	return s, nil
}

func (s *Segment) Delete() error { return os.Remove(s.Path) }
