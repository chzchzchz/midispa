package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chzchzchz/midispa/wav"
)

var wavA = []int{1, 2, 3, 4}
var wavB = []int{5, 6, 7, 8}

func scaleWav(w []int) []int {
	out := make([]int, len(w))
	for i, v := range w {
		out[i] = v * (1 << 15)
	}
	return out
}

func wavReaderOpener(p string) (wav.Reader, error) {
	switch p {
	case "a":
		return wav.NewSliceReader(scaleWav(wavA)), nil
	case "b":
		return wav.NewSliceReader(scaleWav(wavB)), nil
	default:
		return nil, fmt.Errorf("bad")
	}
}

func TestTrackPlayerSideBySide(t *testing.T) {
	sA := &Segment{Path: "a", Samples: 4}
	sB := &Segment{Path: "b", Samples: 4}
	swA := SegmentWindow{Start: 0, Offset: 0, Length: 4}
	swB := SegmentWindow{Start: 4, Offset: 0, Length: 4}
	tr := Track{Segments: []TrackSegment{{sA, swA}, {sB, swB}}}

	swAOff := SegmentWindow{Start: 1, Offset: 0, Length: 4}
	swBOff := SegmentWindow{Start: 5, Offset: 0, Length: 4}
	trOff := Track{Segments: []TrackSegment{{sA, swAOff}, {sB, swBOff}}}
	tts := []struct {
		t      Track
		sw     SampleWindow
		expect []float32
	}{
		{
			t:      tr,
			sw:     SampleWindow{0, 8},
			expect: []float32{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{

			t:      tr,
			sw:     SampleWindow{0, 7},
			expect: []float32{1, 2, 3, 4, 5, 6, 7, 0},
		},
		{

			t:      tr,
			sw:     SampleWindow{1, 8},
			expect: []float32{2, 3, 4, 5, 6, 7, 8, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{1, 7},
			expect: []float32{2, 3, 4, 5, 6, 7, 8, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{1, 6},
			expect: []float32{2, 3, 4, 5, 6, 7, 0, 0},
		},
		{
			t:      trOff,
			sw:     SampleWindow{0, 8},
			expect: []float32{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			t:      trOff,
			sw:     SampleWindow{1, 8},
			expect: []float32{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			t:      trOff,
			sw:     SampleWindow{2, 8},
			expect: []float32{2, 3, 4, 5, 6, 7, 8, 0},
		},
		{
			t:      trOff,
			sw:     SampleWindow{8, 8},
			expect: []float32{8, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i, tt := range tts {
		t.Run(fmt.Sprintf("t%d", i), func(t *testing.T) {
			ps := newTestPoolStream(8, 1)
			defer ps.Close()
			tp, err := newTrackPlayer(&tt.t, ps, wavReaderOpener)
			require.Equal(t, nil, err)
			tp.Play(ctx, tt.sw)
			assert.Equal(t, tt.expect, audioSampleToFloat32(ps.Data(ctx)))
		})
	}
}

func TestTrackPlayerGap(t *testing.T) {
	sA, swA := &Segment{Path: "a", Samples: 4}, SegmentWindow{Start: 0, Offset: 0, Length: 4}
	sB, swB := &Segment{Path: "b", Samples: 4}, SegmentWindow{Start: 5, Offset: 0, Length: 4}

	tr := Track{Segments: []TrackSegment{{sA, swA}, {sB, swB}}}
	tts := []struct {
		t      Track
		sw     SampleWindow
		expect []float32
	}{
		{
			t:      tr,
			sw:     SampleWindow{0, 8},
			expect: []float32{1, 2, 3, 4, 0, 5, 6, 7},
		},
		{

			t:      tr,
			sw:     SampleWindow{1, 8},
			expect: []float32{2, 3, 4, 0, 5, 6, 7, 8},
		},
		{
			t:      tr,
			sw:     SampleWindow{3, 8},
			expect: []float32{4, 0, 5, 6, 7, 8, 0, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{4, 8},
			expect: []float32{0, 5, 6, 7, 8, 0, 0, 0},
		},

		{
			t:      tr,
			sw:     SampleWindow{5, 8},
			expect: []float32{5, 6, 7, 8, 0, 0, 0, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{6, 8},
			expect: []float32{6, 7, 8, 0, 0, 0, 0, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{8, 8},
			expect: []float32{8, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			t:      tr,
			sw:     SampleWindow{9, 8},
			expect: []float32{0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i, tt := range tts {
		t.Run(fmt.Sprintf("t%d", i), func(t *testing.T) {
			ps := newTestPoolStream(8, 1)
			defer ps.Close()
			tp, err := newTrackPlayer(&tt.t, ps, wavReaderOpener)
			require.Equal(t, nil, err)
			tp.Play(ctx, tt.sw)
			assert.Equal(t, tt.expect, audioSampleToFloat32(ps.Data(ctx)))
		})
	}
}

func TestTrackPlayerOffset(t *testing.T) {
	sA, swA := &Segment{Path: "a", Samples: 4}, SegmentWindow{Start: 0, Offset: 1, Length: 3}
	sB, swB := &Segment{Path: "b", Samples: 4}, SegmentWindow{Start: 5, Offset: 2, Length: 2}

	swA2 := SegmentWindow{Start: 1, Offset: 1, Length: 3}
	swB2 := SegmentWindow{Start: 5, Offset: 2, Length: 1}

	tts := []struct {
		t      Track
		sw     SampleWindow
		expect []float32
	}{
		{
			t:      Track{Segments: []TrackSegment{{sA, swA}, {sB, swB}}},
			sw:     SampleWindow{0, 8},
			expect: []float32{2, 3, 4, 0, 0, 7, 8, 0},
		},
		{
			t:      Track{Segments: []TrackSegment{{sA, swA2}, {sB, swB2}}},
			sw:     SampleWindow{0, 8},
			expect: []float32{0, 2, 3, 4, 0, 7, 0, 0},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i, tt := range tts {
		t.Run(fmt.Sprintf("t%d", i), func(t *testing.T) {
			ps := newTestPoolStream(8, 1)
			defer ps.Close()
			tp, err := newTrackPlayer(&tt.t, ps, wavReaderOpener)
			require.Equal(t, nil, err)
			tp.Play(ctx, tt.sw)
			assert.Equal(t, tt.expect, audioSampleToFloat32(ps.Data(ctx)))
		})
	}
}
