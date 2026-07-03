package main

import (
	"context"
	"log"

	"github.com/xthexder/go-jack"
)

type TracksPlayer struct {
	*player
	tracks *Tracks
	bsz    int
	tps    []*TrackPlayer
	pools  []*BiDirPoolStream
}

const TracksPlayerBufferSize = 16

func NewTracksPlayer(t *Tracks, ps PoolStream) (*TracksPlayer, error) {
	tsp := &TracksPlayer{player: newPlayer(ps), tracks: t}
	buf := ps.Buffer(context.TODO())
	tsp.bsz = len(buf)
	ps.Chan() <- buf
	return tsp, nil
}

func (tsp *TracksPlayer) mix(ctx context.Context, outBuf []jack.AudioSample) {
	clearBuffer(outBuf)
	for i := range tsp.tps {
		var buf []jack.AudioSample
		select {
		case buf = <-tsp.pools[i].ReadChan():
		default:
			return
		}
		if !tsp.tracks.Tracks[i].Mute {
			for j := range buf {
				outBuf[j] += buf[j]
			}
		}
		tsp.pools[i].pool.Put(buf)
	}
}

func (tsp *TracksPlayer) flush() {
	for _, tp := range tsp.tps {
		tp.Stop()
	}
	for _, p := range tsp.pools {
		c := p.ReadChan()
		for len(c) > 0 {
			p.pool.Put(<-c)
		}
	}
}

func (tsp *TracksPlayer) Play(ctx context.Context, w SampleWindow) {
	tsp.play(ctx, w, func() {
		defer func() {
			tsp.flush()
			for _, tp := range tsp.tps {
				tp.Close()
			}
			tsp.tps = nil
		}()
		for i, tr := range tsp.tracks.Tracks {
			if len(tsp.pools) <= i {
				p := NewBiDirPoolStream(tsp.bsz, TracksPlayerBufferSize)
				tsp.pools = append(tsp.pools, p)
			}
			tp, err := NewTrackPlayer(&tr, tsp.pools[i])
			if err != nil {
				log.Printf("could not open player for %s", tr.Name)
				return
			}
			tsp.tps = append(tsp.tps, tp)
		}
		for _, tp := range tsp.tps {
			tp.Play(ctx, w)
		}
		outBuf := tsp.ps.Buffer(tsp.ctx)
		outc := tsp.ps.Chan()
		samples := 0
		for outBuf != nil {
			tsp.mix(ctx, outBuf)
			samples += len(outBuf)
			select {
			case outc <- outBuf:
			case <-tsp.ctx.Done():
				outc <- outBuf
				return
			}
			if samples >= w.samples {
				return
			}
			outBuf = tsp.ps.Buffer(tsp.ctx)
		}
	})
}

func (tsp *TracksPlayer) Close() {
	tsp.Stop()
}
