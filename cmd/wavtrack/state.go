package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type State struct {
	cfg            ProjectConfig
	running        bool
	tracks         Tracks
	Markers        []Marker
	RecordTrack    *Track
	rec            *Record
	clock          Clock
	currentRecPath string
}

func NewState(cfg *ProjectConfig) *State {
	return &State{
		cfg:    *cfg,
		tracks: Tracks{baseDir: cfg.Dir()},
		clock:  Clock{rate: float64(cfg.SampleRate)},
	}

}

func (s *State) Recording() bool { return s.rec.Running() }

func (s *State) getRecordingPath() string {
	track := s.RecordTrack
	if track == nil {
		return ""
	}
	trackDir := s.tracks.dir(track)
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		log.Printf("error creating track directory: %v", err)
		return ""
	}
	segmentNum := track.SegmentCount() + 1
	return filepath.Join(trackDir, fmt.Sprintf("%06d.wav", segmentNum))
}

func (s *State) Save() error {
	// nop
	return nil
}
