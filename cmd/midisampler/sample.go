package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-audio/wav"

	"github.com/chzchzchz/midispa/ladspa"
	"github.com/chzchzchz/midispa/util"
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

func (sb *SampleBank) ByPrefix(pfx string) (ret []*Sample) {
	for _, v := range sb.slice {
		if strings.HasPrefix(v.Name, pfx) {
			ret = append(ret, v)
		}
	}
	sort.Sort(sampleSlice(ret))
	return ret
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
