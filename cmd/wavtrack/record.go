package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/xthexder/go-jack"

	mjack "github.com/chzchzchz/midispa/jack"
	mwav "github.com/chzchzchz/midispa/wav"
)

type Record struct {
	Port        *mjack.Port
	running     atomic.Bool
	buffers     sync.Pool
	bufc        chan []jack.AudioSample
	bufDuration time.Duration
	sampleRate  int
	mu          sync.Mutex

	recCtx    context.Context
	recCancel context.CancelFunc
	writer    mwav.WriteFunc
	werr      error
	donec     chan struct{}
}

const BufferPoolSize = 16

func NewRecord(portName string) (*Record, error) {
	r := &Record{
		bufc:  make(chan []jack.AudioSample, BufferPoolSize),
		donec: make(chan struct{}, 1),
	}
	pc := mjack.PortConfig{
		ClientName:    "wavtrack",
		PortName:      portName,
		AudioCallback: r.callback,
	}
	var err error
	if r.Port, err = mjack.NewReadPort(pc); err != nil {
		return nil, err
	}

	bufSize := r.Port.Client.GetBufferSize()
	r.sampleRate = int(r.Port.Client.GetSampleRate())
	for i := 0; i < BufferPoolSize; i++ {
		r.buffers.Put(make([]jack.AudioSample, bufSize))
	}

	r.bufDuration = time.Duration((float32(bufSize) / float32(r.sampleRate)) * float32(time.Second))
	return r, nil
}

func (r *Record) callback(in []jack.AudioSample) int {
	if !r.running.Load() {
		return 0
	}
	if buf := r.buffers.Get(); buf != nil {
		copy(buf.([]jack.AudioSample), in)
		r.bufc <- buf.([]jack.AudioSample)
	}
	return 0
}

func (r *Record) Start(f string) (err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recCancel != nil {
		return nil
	}
	if r.writer, err = mwav.OpenWriter(f, r.sampleRate); err != nil {
		return err
	}
	r.werr = nil
	r.running.Store(true)
	r.recCtx, r.recCancel = context.WithCancel(context.Background())
	go r.record()
	return nil
}

func (r *Record) record() {
	defer func() {
		r.writer(nil)
		r.donec <- struct{}{}
	}()
	for {
		select {
		case <-r.recCtx.Done():
			return
		case buf := <-r.bufc:
			bb := (*[]float32)(unsafe.Pointer(&buf))
			r.werr = r.writer(*bb)
			min, max := float32(-100000), float32(100000.0)
			for _, v := range *bb {
				if v > max {
					max = v
				}
				if v < min {
					min = v
				}
			}
			fmt.Println(min, max)
			r.buffers.Put(buf)
			if r.werr != nil {
				r.running.Store(false)
				return
			}
		}
	}
}

func (r *Record) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop callback.
	r.running.Store(false)
	if r.recCancel == nil {
		return nil
	}
	// Wait for record()
	r.recCancel()
	r.recCancel = nil
	<-r.donec
	// Drain.
	after := time.After(r.bufDuration * 2)
	begin := time.Now()
	for time.Since(begin) < (2 * r.bufDuration) {
		select {
		case buf := <-r.bufc:
			r.buffers.Put(buf)
		case <-after:
		}
	}
	return r.werr
}

func (r *Record) Close() {
	r.Stop()
	close(r.bufc)
	close(r.donec)
	r.Port.Close()
}
