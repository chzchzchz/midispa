package main

import (
	"context"
	"io"
	"slices"

	"github.com/xthexder/go-jack"

	"github.com/chzchzchz/midispa/wav"
)

type TrackPlayer struct {
	*player
	track   *Track
	readers map[string]wav.Reader

	// Used by goroutine
	inBuf  []int
	endPos SampleTick

	// Cache for segment lookup
	lastSegIdx int
}

func NewTrackPlayer(t *Track, ps PoolStream) (*TrackPlayer, error) {
	return newTrackPlayer(t, ps, wav.OpenReader)
}

func newTrackPlayer(
	t *Track,
	ps PoolStream,
	opener func(string) (wav.Reader, error)) (*TrackPlayer, error) {

	tp := &TrackPlayer{
		player:  newPlayer(ps),
		track:   t,
		readers: make(map[string]wav.Reader),
	}
	for _, ts := range t.Segments {
		path := ts.Segment.Path
		if _, ok := tp.readers[path]; ok {
			continue
		}
		r, err := opener(path)
		if err != nil {
			tp.Close()
			return nil, err
		}
		tp.readers[path] = r
	}
	return tp, nil
}

func (tp *TrackPlayer) Play(ctx context.Context, w SampleWindow) {
	tp.play(ctx, w, func() {
		outBuf := tp.ps.Buffer(tp.ctx)
		outc := tp.ps.Chan()
		tp.lastSegIdx = 0
		tp.inBuf = make([]int, len(outBuf))
		tp.endPos = w.start + SampleTick(w.samples)
		pos := w.start
		for outBuf != nil {
			err := tp.read(outBuf, pos)
			select {
			case outc <- outBuf:
			case <-tp.ctx.Done():
				outc <- outBuf
				return
			}
			pos += SampleTick(len(outBuf))
			if pos > tp.endPos || err != nil {
				break
			}
			outBuf = tp.ps.Buffer(tp.ctx)
		}
	})
}

func (tp *TrackPlayer) read(outBuf []jack.AudioSample, pos SampleTick) error {
	if tp.track.Mute {
		clearBuffer(outBuf)
		return nil
	}

	// This function is kind of complicated, it might be better to build up
	// a list of wavs and gaps and run through that instead of the binsearch stuff.
	remaining := int(tp.endPos - pos)
	segs := tp.track.Segments
	outIdx := 0

	gain := tp.track.Gain / float32(1 << 15)
	for remaining > 0 && outIdx < len(outBuf) {
		segIdx := tp.firstSegmentAtOrAfter(pos)
		tp.lastSegIdx = segIdx

		var holdsPosTs *TrackSegment
		// Check if previous segment contains pos
		if segIdx > 0 {
			if prevTs := &segs[segIdx-1]; prevTs.Contains(pos) {
				holdsPosTs = prevTs
			}
		}
		// Check if no more segments to read.
		if segIdx < 0 || (holdsPosTs == nil && segIdx >= len(segs)) {
			n := min(len(outBuf)-outIdx, remaining)
			clearBuffer(outBuf[outIdx:outIdx+n])
			outIdx += n
			pos += SampleTick(n)
			remaining -= n
			return nil
		}
		// Check if current segment (at segIdx) contains pos
		if holdsPosTs == nil {
			if curTs := &segs[segIdx]; curTs.Contains(pos) {
				holdsPosTs = curTs
			}
		}
		// Check if could not find segment that holds 'pos'.
		if holdsPosTs == nil {
			// NOTE: by binary search, know pos < ts.SegmentWindow.Start
			// pos is before next segment
			// Gap before segment starts
			gap := int(segs[segIdx].Start - pos)
			n := min(len(outBuf)-outIdx, min(remaining, gap))
			clearBuffer(outBuf[outIdx:outIdx+n])
			outIdx += n
			pos += SampleTick(n)
			remaining -= n
			continue
		}

		// pos is within segment; copy it
		ts := holdsPosTs
		segEnd := ts.Start + ts.Length
		segRemaining := int(segEnd - pos)
		n := min(len(outBuf)-outIdx, min(remaining, segRemaining))

		segOffset := (pos - ts.Start) + ts.Offset
		r := tp.readers[ts.Segment.Path]
		r.Seek(int(segOffset), io.SeekStart)
		if _, err := r.Read(tp.inBuf[:n]); err != nil && err != io.EOF {
			return err
		}

		for i := 0; i < n; i++ {
			outBuf[outIdx+i] = jack.AudioSample(float32(tp.inBuf[i]) * gain)
		}

		outIdx += n
		pos += SampleTick(n)
		remaining -= n
	}

	return nil
}

// firstSegmentAtOrAfter returns the index of the first segment that starts at or after pos.
func (tp *TrackPlayer) firstSegmentAtOrAfter(pos SampleTick) int {
	segments := tp.track.Segments
	if tp.lastSegIdx >= len(segments) {
		return tp.lastSegIdx
	}
	if segments[tp.lastSegIdx].Contains(pos) {
		return tp.lastSegIdx
	}
	idx, _ := slices.BinarySearchFunc(segments[tp.lastSegIdx:], pos, func(ts TrackSegment, v SampleTick) int {
		return int(ts.Start - v)
	})
	if idx == -1 {
		return -1
	}
	return idx + tp.lastSegIdx
}

func (tp *TrackPlayer) Close() {
	tp.Stop()
	for _, r := range tp.readers {
		r.Close()
	}
}
