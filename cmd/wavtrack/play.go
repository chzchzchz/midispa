package main

import (
	"context"
	"sync/atomic"

	"github.com/xthexder/go-jack"

	mjack "github.com/chzchzchz/midispa/jack"
)

type Play struct {
	port    *mjack.Port
	buffers *Pool[[]jack.AudioSample]
	running atomic.Bool
	stopc   chan struct{}
	c       chan []jack.AudioSample
}

const PlayBufferSize = 4

func NewPlay(extPorts []string) (*Play, error) {
	p := &Play{
		stopc:   make(chan struct{}),
		c:       make(chan []jack.AudioSample, PlayBufferSize),
		buffers: NewPool[[]jack.AudioSample](PlayBufferSize),
	}
	pc := mjack.PortConfig{
		ClientName:    "wavtrack-play",
		PortName:      "play",
		ExactName:     extPorts,
		AudioCallback: p.callback,
	}
	var err error
	if p.port, err = mjack.NewWritePort(pc); err != nil {
		return nil, err
	}
	bufSize := p.port.Client.GetBufferSize()
	for i := 0; i < PlayBufferSize; i++ {
		p.buffers.Put(make([]jack.AudioSample, bufSize))
	}
	return p, nil
}

func (p *Play) Running() bool { return p.running.Load() }

func (p *Play) Start() { p.running.Store(true) }

func (p *Play) callback(in []jack.AudioSample) int {
	if !p.running.Load() {
		// Only notify if there is a reader waiting.
		select {
		case p.stopc <- struct{}{}:
		default:
		}
		clearBuffer(in)
		return 0
	}
	select {
	case b, ok := <-p.c:
		if ok {
			copy(in, b)
			p.buffers.Put(b)
		}
	default:
		// nonblocking; underflow
		clearBuffer(in)
	}
	return 0
}

func (p *Play) Stop() {
	if !p.running.Load() {
		return
	}
	p.running.Store(false)
	// Wait for stop ack from callback.
	<-p.stopc
	p.Flush(0)
}

func (p *Play) Flush(n int) {
	for len(p.c) > n {
		p.buffers.Put(<-p.c)
	}
}

func (p *Play) Chan() chan<- []jack.AudioSample { return p.c }

func (p *Play) Buffer(ctx context.Context) []jack.AudioSample { return p.buffers.Wait(ctx) }

func (p *Play) Close() {
	p.port.Close()
	close(p.stopc)
	close(p.c)
}
