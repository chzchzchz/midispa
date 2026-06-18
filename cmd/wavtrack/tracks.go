package main

import (
	"log"
	"os"
	"path/filepath"
)

type Tracks struct {
	baseDir string
	Tracks  []Track `json:"tracks"`
}

func (s *Tracks) add(name string) bool {
	for _, t := range s.tracks {
		if t.name == name {
			return false
		}
	}
	s.tracks = append(s.tracks, Track{name: name})
	return true
}

func (s *Tracks) dir(t *Track) string {
	return filepath.Join(s.baseDir, t.name)
}

func (s *Tracks) rename(oldName, newName string) bool {
	idx := -1
	for i, t := range s.tracks {
		if t.name == newName {
			return false
		}
		if t.name == oldName {
			idx = i
		}
	}
	if idx == -1 {
		return false
	}

	if oldName != newName {
		t := &s.tracks[idx]
		oldDir := s.dir(t)
		t.name = newName
		newDir := s.dir(t)
		if err := os.Rename(oldDir, newDir); err != nil {
			log.Printf("error renaming track directory: %v", err)
		}
	}
	return true
}

func (t *Tracks) Save(p string) error {
	// TODO
	return nil
}

func (t *Tracks) Load(p string) error {
	// TODO
	return nil
}
