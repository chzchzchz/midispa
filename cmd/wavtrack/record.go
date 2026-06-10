package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/jack"
)

type Record struct {
	Port        jack.Port
	running     atomic.Bool
	buffers     sync.Pool
	bufc        chan []jack.AudioSample
	bufDuration time.Duration
	sampleRate  int
	mu          sync.Mutex

	recCtx    context.Context
	recCancel context.CancelFunc
	donec     chan struct{}
}

const BufferPoolSize = 16

func NewRecord(portName string) (*Record, error) {
	r := &Record{
		bufc:  make(chan []jack.AudioSample, BufferPoolSize),
		donec: make(chan struct{}, 1),
	}
	pc := jack.PortConfig{
		ClientName:    "wavtrack",
		PortName:      portName,
		AudioCallback: r.callback,
	}
	if r.Port, err := jack.NewReadPort(pc); err != nil {
		return nil, err
	}

	bufSize := r.Port.Client.GetBufferSize()
	r.sampleRate := r.Port.Client.GetSampleRate()
	for i := 0; i < BufferPoolSize; i++ {
		r.buffers.Put(make([]jack.AudioSample, bufSize))
	}

	r.bufDuration = time.Duration((float32(bufSize) / float32(sampleRate)) * time.Second)
	return r, nil
}

func (r *Record) callback(in []jack.AudioSample) int {
	if !atomic.Load(&r.running) {
		return 0
	}
	if buf := r.buffers.Get(); buf != nil {
		copy(buf, in)
		r.bufc <- buf
	}
	return 0
}

func (r *Record) Start(f string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recCancel != nil {
		return
	}
	atomic.Store(&r.running, true)
	r.recCtx, r.recCancel := context.WithCancel(context.Background())
	go r.record()
}

func (r *Record) record() {
	defer func() {
		donec <- struct{}{}
	}()
	for {
		select {
		case <-r.recCtx.Done():
			return
		case buf := <-r.bufc:
			r.buffers.Put(buf)
		}
	}
}

func (r *Record) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop callback.
	atomic.Store(&r.running, false)
	if r.recCancel == nil {
		return
	}
	// Wait for record()
	r.recCancel()
	r.recCancel = nil
	<-r.donec
	// Drain.
	after := time.After(r.bufDuration * 2)
	wait := time.Now().Add(r.bufDuration * 2)
	for time.Now() < wait {
		select {
		case buf := <-r.bufc:
			r.buffers.Put(buf)
		case <-after:
		}
	}
	return nil
}

func (r *Record) Close() {
	r.Stop()
	close(r.bufc)
	close(r.donec)
	r.Port.Close()
}
