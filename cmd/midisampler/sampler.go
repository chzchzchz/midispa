package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Sampler is responsible for collecting samples from live input.
type Sampler struct {
	recording atomic.Bool
	mu        sync.Mutex

	buffersIdx int
	bufferIdx  int
	buffers    [][]float32

	rate int

	// originalSample is the sample that was first recorded; for undo.
	originalSample *Sample

	// samp is the currently chopped sample.
	samp *Sample
}

func NewSampler(rate int) *Sampler {
	// TODO: possibly make the first buffer a circular buffer
	// and always recording so that if a record event is too late,
	// it'll still pick up everything.
	buffers := make([][]float32, 1)
	buffers[0] = make([]float32, 1024*256)
	return &Sampler{
		buffers: buffers,
		rate:    rate,
	}
}

func (s *Sampler) growBuffer() {
	newBuffer := make([]float32, 1024*256)
	s.buffers = append(s.buffers, newBuffer)
	s.buffersIdx, s.bufferIdx = len(s.buffers)-1, 0
}

func (s *Sampler) startRecord() {
	s.mu.Lock()
	s.buffersIdx, s.bufferIdx = 0, 0
	s.recording.Store(true)
	s.mu.Unlock()
}

func (s *Sampler) stopRecord() {
	s.recording.Store(false)
	s.mu.Lock()
	var data []float32
	for i := 0; i < s.buffersIdx; i++ {
		data = append(data, s.buffers[i]...)
	}
	data = append(data, s.buffers[s.buffersIdx][:s.bufferIdx]...)
	s.mu.Unlock()

	name := fmt.Sprintf("capture/sample-%v.wav", time.Now().UnixMicro())
	s.samp = NewSample(name, s.rate, data)
	// Assuming record rate is playback rate, so don't resample.
	s.samp.Normalize()
	s.originalSample = s.samp.Copy()
}

func (s *Sampler) currentSample() *Sample {
	return s.samp
}

func (s *Sampler) chop(start, end time.Duration) {
	if s.samp == nil {
		return
	}
	s.samp.Chop(start, end)
	s.samp.Normalize()
}

func (s *Sampler) reset() {
	s.samp = s.originalSample.Copy()
}

func (s *Sampler) record(samples []float32) {
	if !s.recording.Load() {
		return
	}
	s.mu.Lock()
	if !s.recording.Load() {
		s.mu.Unlock()
		return
	}
	copy(s.buffers[s.buffersIdx][s.bufferIdx:], samples)
	s.bufferIdx += len(samples)
	if s.bufferIdx == len(s.buffers[s.buffersIdx]) {
		s.bufferIdx = 0
		s.buffersIdx++
	}
	if s.buffersIdx == len(s.buffers) {
		s.growBuffer()
	}
	s.mu.Unlock()
}
