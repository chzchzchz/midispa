package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type State struct {
	cfg          ProjectConfig
	running      bool
	tracks       Tracks
	Markers      []Marker
	RecordTrack  *Track
	rec          *Record
	clock        Clock
	Play         *Play
	TracksPlayer *TracksPlayer
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
	segmentNum := track.SegmentCount + 1
	for {
		path := filepath.Join(trackDir, fmt.Sprintf("%06d.wav", segmentNum))
		_, err := os.Stat(path)
		if err == nil {
			segmentNum++
			continue
		}
		if !os.IsNotExist(err) {
			log.Printf("error checking recording path: %v", err)
			return ""
		}
		return path
	}
}

func (s *State) tracksJSONPath() string {
	return filepath.Join(s.cfg.Dir(), "tracks.json")
}

func (s *State) Save() error {
	return s.tracks.Save(s.tracksJSONPath())
}

func (s *State) loadExtraSegments() {
	// Scan each track directory and add missing segments to segmentStore
	for i := range s.tracks.Tracks {
		track := &s.tracks.Tracks[i]
		trackDir := s.tracks.dir(track)

		existingPaths := make(map[string]struct{})
		for _, seg := range track.segmentStore {
			existingPaths[seg.Path] = struct{}{}
		}
		// Scan the track directory for .wav files
		entries, err := os.ReadDir(trackDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(trackDir, entry.Name())
			if _, ok := existingPaths[path]; ok {
				continue
			}
			if !strings.HasSuffix(path, ".wav") {
				continue
			}
			seg, err := NewSegment(path)
			if err != nil {
				log.Printf("error loading segment %s: %v", path, err)
				continue
			}
			track.segmentStore = append(track.segmentStore, seg)
		}
		if totalSegs := len(track.segmentStore); totalSegs > track.SegmentCount {
			track.SegmentCount = totalSegs
		}
	}
}

func (s *State) Load() error {
	if err := s.tracks.Load(s.tracksJSONPath()); err != nil {
		return err
	}
	s.loadExtraSegments()
	return nil
}
