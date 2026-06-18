package main

type Track struct {
	Name         string         `json:"name"`
	Segments     []TrackSegment `json:"segments"`
	Mute         bool           `json:"mute"`
	SegmentCount int            `json:"segment_count"`
	segmentStore []*Segment
}

type SegmentWindow struct {
	// Start tick in track.
	Start SampleTick `json:"start"`
	// Offset sample into segment.
	Offset SampleTick `json:"offset"`
	// Length past offset to use in segment.
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

func (t *Track) AddTrackSegment(ts TrackSegment) bool {
	for _, existing := range t.Segments {
		if existing.Start == ts.Start {
			return false
		}
	}
	t.Segments = append(t.Segments, ts)
	for i := len(t.Segments) - 1; i > 0; i-- {
		if t.Segments[i].Start < t.Segments[i-1].Start {
			t.Segments[i], t.Segments[i-1] = t.Segments[i-1], t.Segments[i]
		} else {
			break
		}
	}
	return true
}

func (t *Track) RemoveTrackSegment(start SampleTick) *TrackSegment {
	for i, ts := range t.Segments {
		if ts.Start == start {
			t.Segments = append(t.Segments[:i], t.Segments[i+1:]...)
			return &ts
		}
	}
	return nil
}
