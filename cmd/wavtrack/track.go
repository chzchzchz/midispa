package main

type Track struct {
	name     string
	segments []TrackSegment
	mute     bool
}

type TrackSegment struct {
	segment *Segment
	Start   SampleTick
	Offset  SampleTick
	Length  SampleTick
}
