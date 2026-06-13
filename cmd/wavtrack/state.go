package main

type State struct {
	cfg         ProjectConfig
	basePath    string
	running     bool
	position    SampleTick
	Tracks      []Track
	Markers     []Marker
	RecordTrack *Track
	rec         *Record
}

func (s *State) addTrack(name string) bool {
	for _, t := range s.Tracks {
		if t.name == name {
			return false
		}
	}
	s.Tracks = append(s.Tracks, Track{name: name})
	return true
}

func (s *State) renameTrack(oldName, newName string) bool {
	idx := -1
	for i, t := range s.Tracks {
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
	s.Tracks[idx].name = newName
	return true
}
