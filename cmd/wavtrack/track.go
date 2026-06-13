package main

type Track struct {
	name         string
	segments     []TrackSegment
	mute         bool
	segmentCount int
	segmentStore []*Segment
}

type TrackSegment struct {
	segment *Segment
	Start   SampleTick
	Offset  SampleTick
	Length  SampleTick
}

func (t *Track) AddSegment(s *Segment) {
	t.segmentStore = append(t.segmentStore, s)
	t.segmentCount++
}

func (t *Track) SegmentCount() int { return t.segmentCount }
