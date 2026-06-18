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
	for _, t := range s.Tracks {
		if t.Name == name {
			return false
		}
	}
	s.Tracks = append(s.Tracks, Track{Name: name})
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
	// TODO
	return nil
}

func (t *Tracks) Load(p string) error {
	// TODO
	return nil
}
