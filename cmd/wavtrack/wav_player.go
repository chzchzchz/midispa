package main

import (
	"context"
	"io"
	"sync"

	"github.com/xthexder/go-jack"

	mwav "github.com/chzchzchz/midispa/wav"
)

type WavPlayer struct {
	play   PoolStream
	reader *mwav.WavReader
	ctx    context.Context
	cancel context.CancelFunc
	stopc  chan struct{}
	mu     sync.Mutex
}

func NewWavPlayer(p string, play PoolStream) (*WavPlayer, error) {
	reader, err := mwav.OpenReader(p)
	if err != nil {
		return nil, err
	}
	wp := &WavPlayer{
		play:   play,
		reader: reader,
	}
	return wp, nil
}

func (wp *WavPlayer) Play(ctx context.Context, w SampleWindow) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.cancel != nil {
		return
	}
	wp.ctx, wp.cancel = context.WithCancel(ctx)
	wp.stopc = make(chan struct{})
	wp.reader.Seek(int(w.start), io.SeekStart)

	go func() {
		defer func() {
			wp.mu.Lock()
			defer wp.mu.Unlock()
			wp.cancel = nil
			close(wp.stopc)
		}()
		outBuf := wp.play.Buffer(ctx)
		outc := wp.play.Chan()
		inBuf := make([]int, len(outBuf))
		samples := 0
		for outBuf != nil {
			sz, _ := wp.reader.Read(inBuf)
			for i := 0; i < len(inBuf); i++ {
				outBuf[i] = jack.AudioSample(float32(inBuf[i]) / float32(1<<15))
			}
			samples += sz
			select {
			case outc <- outBuf:
			case <-ctx.Done():
				// must return buffer to Play
				// NOTE: play must be stopped *after* wavplayer
				outc <- outBuf
				return
			}
			if sz < len(inBuf) || samples >= w.samples {
				// No more samples to process.
				return
			}
			outBuf = wp.play.Buffer(ctx)
		}
	}()
}

func (wp *WavPlayer) Running() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.cancel != nil
}

func (wp *WavPlayer) Stop() {
	wp.mu.Lock()
	stopc := wp.stopc
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.mu.Unlock()
	<-stopc
}

func (wp *WavPlayer) Close() {
	wp.Stop()
	wp.reader.Close()
}
