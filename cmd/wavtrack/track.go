package main

type Track struct {
	Name         string         `json:"name"`
	Segments     []TrackSegment `json:"segments"`
	Mute         bool           `json:"mute"`
	SegmentCount int            `json:"segment_count"`
	segmentStore []*Segment
}

type SegmentWindow struct {
	Start  SampleTick `json:"start"`
	Offset SampleTick `json:"offset"`
	Length SampleTick `json:"length"`
}

type TrackSegment struct {
	Segment       *Segment `json:"segment"`
	SegmentWindow `json:",inline"`
}

func (t *Track) AddSegment(s *Segment) {
	t.segmentStore = append(t.segmentStore, s)
	t.SegmentCount++
}
