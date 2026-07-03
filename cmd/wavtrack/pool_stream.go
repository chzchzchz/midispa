package main

import (
	"context"

	"github.com/xthexder/go-jack"
)

type PoolStream interface {
	Chan() chan<- []jack.AudioSample
	Buffer(ctx context.Context) []jack.AudioSample
}

type testPoolStream struct {
	bufSize int
	n       int
	c       chan []jack.AudioSample
	donec   chan struct{}
	data    []jack.AudioSample
	ctx     context.Context
	cancel  context.CancelFunc
}

func newTestPoolStream(bufSize, n int) *testPoolStream {
	ctx, cancel := context.WithCancel(context.Background())
	ps := &testPoolStream{
		bufSize: bufSize,
		n:       n,
		c:       make(chan []jack.AudioSample, n),
		donec:   make(chan struct{}),
		ctx:     ctx,
		cancel:  cancel,
	}
	go func() {
		defer close(ps.donec)
		for i := 0; i < n; i++ {
			select {
			case <-ctx.Done():
			case b := <-ps.c:
				ps.data = append(ps.data, b...)
			}
		}
	}()
	return ps
}

func (ps *testPoolStream) Chan() chan<- []jack.AudioSample { return ps.c }

func (ps *testPoolStream) Buffer(ctx context.Context) []jack.AudioSample {
	if len(ps.c) >= ps.n {
		return nil
	}
	return make([]jack.AudioSample, ps.bufSize)
}

func (ps *testPoolStream) Close() {
	ps.cancel()
	<-ps.donec
}

func (ps *testPoolStream) Data(ctx context.Context) []jack.AudioSample {
	select {
	case <-ps.donec:
	case <-ctx.Done():
	}
	return ps.data
}

type BiDirPoolStream struct {
	pool *Pool[[]jack.AudioSample]
	c    chan []jack.AudioSample
}

func NewBiDirPoolStream(bufSize, n int) *BiDirPoolStream {
	p := NewPool[[]jack.AudioSample](n)
	for i := 0; i < n; i++ {
		p.Put(make([]jack.AudioSample, bufSize))
	}
	return &BiDirPoolStream{pool: p, c: make(chan []jack.AudioSample, n)}
}

func (p *BiDirPoolStream) ReadChan() <-chan []jack.AudioSample { return p.c }
func (p *BiDirPoolStream) Chan() chan<- []jack.AudioSample     { return p.c }

func (p *BiDirPoolStream) Buffer(ctx context.Context) []jack.AudioSample {
	return p.pool.Wait(ctx)
}
