package main

import (
	"context"
	"sync"
)

type Player interface {
	Play(ctx context.Context, w SampleWindow)
	Running() bool
	Stop()
	Close()
}

type player struct {
	ps     PoolStream
	ctx    context.Context
	cancel context.CancelFunc
	stopc  chan struct{}
	mu     sync.Mutex
}

func newPlayer(ps PoolStream) *player {
	stopc := make(chan struct{})
	close(stopc)
	return &player{
		ps:    ps,
		stopc: stopc,
	}
}

func (p *player) Stop() {
	p.mu.Lock()
	stopc := p.stopc
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()
	<-stopc
}

func (p *player) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cancel != nil
}

func (p *player) play(ctx context.Context, w SampleWindow, f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.stopc = make(chan struct{})
	go func() {
		defer func() {
			p.mu.Lock()
			p.cancel = nil
			close(p.stopc)
			p.mu.Unlock()
		}()
		f()
	}()
}
