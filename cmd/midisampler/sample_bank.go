package main

import (
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/util"
)

type SampleBank struct {
	samples map[string]*Sample
	slice   sampleSlice

	path string
}

type sampleSlice []*Sample

func (ss sampleSlice) Len() int           { return len(ss) }
func (ss sampleSlice) Less(i, j int) bool { return ss[i].Name < ss[j].Name }
func (ss sampleSlice) Swap(i, j int)      { ss[i], ss[j] = ss[j], ss[i] }

func MustLoadSampleBank(libPath string) *SampleBank {
	sb := &SampleBank{samples: make(map[string]*Sample), path: libPath}
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
