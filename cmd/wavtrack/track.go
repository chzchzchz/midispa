package main

type Track struct {
	name     string
	segments []TrackSegment
}

type TrackSegment struct {
	segment *Segment
	Start   SampleTick
	Offset  SampleTick
	Length  SampleTick
}
