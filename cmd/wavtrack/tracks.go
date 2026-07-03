package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

type Tracks struct {
	baseDir string
	Tracks  []Track `json:"tracks"`
}

func (s *Tracks) add(name string) bool {
	for _, t := range s.Tracks {
		if t.Name == name {
			return false
		}
	}
	s.Tracks = append(s.Tracks, Track{Name: name})
	// Create the track directory after appending
	trackDir := filepath.Join(s.baseDir, name)
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		log.Printf("error creating track directory: %v", err)
	}
	return true
}

func (s *Tracks) dir(t *Track) string {
	return filepath.Join(s.baseDir, t.Name)
}

func (s *Tracks) rename(oldName, newName string) bool {
	idx := -1
	for i, t := range s.Tracks {
		if t.Name == newName {
			return false
		}
		if t.Name == oldName {
			idx = i
		}
	}
	if idx == -1 {
		return false
	}

	if oldName != newName {
		t := &s.Tracks[idx]
		oldDir := s.dir(t)
		t.Name = newName
		newDir := s.dir(t)
		if err := os.Rename(oldDir, newDir); err != nil {
			log.Printf("error renaming track directory: %v", err)
		}
	}
	return true
}

func (t *Tracks) Save(p string) error {
	cfgBytes, err := json.MarshalIndent(t, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, cfgBytes, 0644)
}

func (t *Tracks) loadUniqueSegments() {
	// Ensure unique Segment pointers per unique Segment path
	// For each track, ensure there is one unique Segment pointer per unique Segment path
	// in its segmentStore, referenced by track segments
	for i := range t.Tracks {
		track := &t.Tracks[i]
		// Build a map of path -> *Segment for this track
		segmentMap := make(map[string]*Segment)

		// First pass: collect all unique segments from TrackSegments
		for j := range track.Segments {
			seg := &track.Segments[j]
			if seg.Segment == nil {
				continue
			}
			// Check if we already have a segment with this path in this track's map
			if existingSeg, exists := segmentMap[seg.Segment.Path]; exists {
				// Use the existing segment pointer
				seg.Segment = existingSeg
			} else {
				// Add to map
				segmentMap[seg.Segment.Path] = seg.Segment
			}
		}

		// Rebuild segmentStore with unique segments
		track.segmentStore = nil
		for _, seg := range segmentMap {
			track.segmentStore = append(track.segmentStore, seg)
		}
	}
}

func (t *Tracks) Load(p string) error {
	cfgBytes, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(cfgBytes, t); err != nil {
		return err
	}
	t.loadUniqueSegments()
	return nil
}

func (t *Tracks) Length() SampleTick {
	var maxEnd SampleTick
	for _, track := range t.Tracks {
		for _, ts := range track.Segments {
			maxEnd = max(maxEnd, ts.Start+ts.Length)
		}
	}
	return maxEnd
}
