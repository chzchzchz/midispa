package main

import (
	"sync"
)

type voicedSample struct {
	*Sample
	remaining []float32 // slice from sample
	velocity  float32
}

var voicesMu sync.Mutex
var voices map[*voicedSample]struct{} = make(map[*voicedSample]struct{})

func addVoice(s *Sample, vel int) {
	vs := &voicedSample{Sample: s, remaining: s.data, velocity: float32(vel) / 127.0}
	voicesMu.Lock()
	voices[vs] = struct{}{}
	voicesMu.Unlock()
}

func stopVoice(s *Sample) {
	voicesMu.Lock()
	for vs := range voices {
		if vs.Sample == s {
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
