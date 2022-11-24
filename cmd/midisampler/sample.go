package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"

	"github.com/chzchzchz/midispa/ladspa"
)

type Sample struct {
	Name  string
	*ADSR `json:"ADSR",omitempty`

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
	for i := range newData {
		fi, fj := float64(i-1)/ratio, float64(i)/ratio
		if fi < 0 {
			fi = 0
		}
		ii, ij := int(fi), int(fj)
		alpha := float32(fi - float64(ii))
		newData[i] = (alpha*s.data[ii] + (1.0-alpha)*s.data[ij]) / 2.0
	}

	p, err := ladspa.LowPassFilter(sampleHz)
	if err != nil {
		log.Printf("resampled %s from %d to %d", s.Name, s.rate, sampleHz)
		s.rate, s.data = sampleHz, newData
		return
	}
	defer p.Close()

	hz := float32(s.rate) / 2.0
	log.Printf("resampled %s from %d to %d; lpf %dHz", s.Name, s.rate, sampleHz, int(hz))
	p.Connect("Input", &newData[0])
	p.Connect("Output", &newData[0])
	p.Connect("Cutoff Frequency (Hz)", &hz)
	p.Run(len(newData))

	s.rate, s.data = sampleHz, newData
}

func (s *Sample) Normalize() {
	min, max := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, v := range s.data {
		if v > max {
			max = v
		} else if v < min {
			min = v
		}
	}
	for i := range s.data {
		s.data[i] = 2.0 * (((s.data[i] - min) / (max - min)) - 0.5)
	}
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
	format := dec.Format()
	if format.NumChannels == 0 {
		return nil, fmt.Errorf("bad format %+v", *format)
	}
	pcm, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	pcmf32 := pcm.AsFloat32Buffer()
	mono := pcmf32.Data
	if format.NumChannels > 1 {
		log.Printf("downmixing %q to single channel", name)
		mono = make([]float32, len(pcmf32.Data)/format.NumChannels)
		for i := range mono {
			v := float32(0)
			for j := 0; j < format.NumChannels; j++ {
				v += pcmf32.Data[i*format.NumChannels+j]
			}
			mono[i] = v / float32(format.NumChannels)
		}
	}
	s := &Sample{
		Duration: dur,
		data:     mono,
		rate:     pcm.Format.SampleRate,
		Name:     name,
	}
	return s, nil
}

func NewSample(name string, rate int, data []float32) *Sample {
	seconds := float64(len(data)) / float64(rate)
	dur := time.Duration(seconds * float64(time.Second))
	return &Sample{
		Name:     name,
		Duration: dur,
		data:     data,
		rate:     rate,
	}
}

func (s *Sample) Save(path string) error {
	w, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer w.Close()
	enc := wav.NewEncoder(w, s.rate, 16, 1 /* chans */, 1 /* fmt */)

	// wav encoder will normalize ints to [-1.0,1.0] but won't expand back.
	renormalizedData := make([]float32, len(s.data))
	for i := range s.data {
		renormalizedData[i] = s.data[i] * float32((1<<15)-1)
	}

	buf := audio.PCMBuffer{
		Format:         &audio.Format{NumChannels: 1, SampleRate: s.rate},
		F32:            renormalizedData,
		DataType:       audio.DataTypeF32,
		SourceBitDepth: 2,
	}
	if err := enc.Write(buf.AsIntBuffer()); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return nil
}

func (s *Sample) Copy() *Sample {
	data := make([]float32, len(s.data))
	copy(data, s.data)
	return &Sample{
		Name:     s.Name,
		Duration: s.Duration,
		data:     data,
		rate:     s.rate,
	}
}

func (s *Sample) Chop(start, end time.Duration) {
	startIdx := int(float64(s.rate) * start.Seconds())
	endIdx := int(float64(s.rate) * end.Seconds())
	if end == 0 || endIdx > len(s.data) {
		endIdx = len(s.data)
	}
	if startIdx >= endIdx {
		return
	}
	s.data = s.data[startIdx:endIdx]
	seconds := float64(len(s.data)) / float64(s.rate)
	s.Duration = time.Duration(seconds * float64(time.Second))
}
