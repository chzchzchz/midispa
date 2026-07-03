package main

import (
	"context"
	"io"

	"github.com/xthexder/go-jack"

	wav "github.com/chzchzchz/midispa/wav"
)

type WavPlayer struct {
	*player
	reader wav.Reader
}

func NewWavPlayer(p string, ps PoolStream) (*WavPlayer, error) {
	reader, err := wav.OpenReader(p)
	if err != nil {
		return nil, err
	}
	wp := &WavPlayer{
		player: newPlayer(ps),
		reader: reader,
	}
	return wp, nil
}

func (wp *WavPlayer) Play(ctx context.Context, w SampleWindow) {
	wp.play(ctx, w, func() {
		wp.reader.Seek(int(w.start), io.SeekStart)
		outBuf := wp.ps.Buffer(wp.ctx)
		outc := wp.ps.Chan()
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
			case <-wp.ctx.Done():
				// must return buffer to Play
				// NOTE: play must be stopped *after* wavplayer
				outc <- outBuf
				return
			}
			if sz < len(inBuf) || samples >= w.samples {
				// No more samples to process.
				return
			}
			outBuf = wp.ps.Buffer(wp.ctx)
		}
	})
}

func (wp *WavPlayer) Close() {
	wp.Stop()
	wp.reader.Close()
}
