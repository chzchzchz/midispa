package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/util"
	"github.com/go-audio/wav"
)

type Sample struct {
	Name  string
	*ADSR `json:"ADSR",omitempty`

	time.Duration
	data []float32
	rate int
}

type SampleBank struct {
	samples map[string]*Sample
	slice   sampleSlice
}

func MustLoadSampleBank(libPath string) *SampleBank {
	sb := &SampleBank{samples: make(map[string]*Sample)}
	files := util.Walk(libPath)
	adsr := ADSR{
		Attack:  time.Millisecond,
		Decay:   100 * time.Millisecond,
		Sustain: 0.7,
		Release: 200 * time.Millisecond,
	}
	var wg sync.WaitGroup
	outc := make(chan *Sample, len(files))
	for i := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			path := filepath.Join(libPath, f)
			s, err := LoadSample(f, path)
			if err != nil {
				log.Printf("error loading %q: %v", f, err)
			} else {
				s.ADSR = &adsr
				s.Normalize()
				outc <- s
			}
		}(files[i])
	}
	wg.Wait()
	close(outc)
	for s := range outc {
		sb.samples[s.Name], sb.slice = s, append(sb.slice, s)
	}
	sort.Sort(sb.slice)
	log.Printf("loaded %d of %d samples from %q", len(sb.samples), len(files), libPath)

	return sb
}

type sampleSlice []*Sample

func (ss sampleSlice) Len() int           { return len(ss) }
func (ss sampleSlice) Less(i, j int) bool { return ss[i].Name < ss[j].Name }
func (ss sampleSlice) Swap(i, j int)      { ss[i], ss[j] = ss[j], ss[i] }

func (s *Sample) Resample(sampleHz int) {
	if s.rate == sampleHz {
		return
	}
	ratio := float64(sampleHz) / float64(s.rate)
	newSamples := int(float64(len(s.data)) * ratio)
	newData := make([]float32, newSamples)
	newData[0] = s.data[0]
	for i := 0; i < newSamples; i++ {
		fi, fj := float64(i-1)*1.0/ratio, float64(i)*1.0/ratio
		ii, ij := int(fi), int(fj)
		alpha := float32(fi - float64(ii))
		newData[i] = (alpha*s.data[ii] + (1.0-alpha)*s.data[ij]) / 2.0
	}
	s.rate, s.data = sampleHz, s.data
}

func (s *Sample) Normalize() {
	var min float32
	var max float32
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
	if format := dec.Format(); format.NumChannels != 1 {
		return nil, fmt.Errorf("bad format %+v", *format)
	}
	pcm, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	}
	pcmf32 := pcm.AsFloat32Buffer()
	s := &Sample{
		Duration: dur,
		data:     pcmf32.Data,
		rate:     pcm.Format.SampleRate,
		Name:     name,
	}
	return s, nil
}
