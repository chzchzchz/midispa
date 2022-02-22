package main

import (
	"sync"
)

type Song struct {
	Patterns []*Pattern
	mu       sync.RWMutex
}

func (s *Song) GetPattern(ndx int) (p *Pattern) {
	if ndx < 0 {
		return nil
	}
	s.mu.RLock()
	if ndx < len(s.Patterns) {
		p = s.Patterns[ndx]
	}
	s.mu.RUnlock()
	return p
}

func (s *Song) BeatToPattern(beat float32) (p *Pattern, idx int) {
	// TODO: logn lookup
	curBeat := float32(0)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for idx, sp := range s.Patterns {
		if sp == nil {
			continue
		}
		if curBeat += sp.Beats(); curBeat > beat {
			return sp, idx
		}
	}
	return nil, -1
}

func (s *Song) SetPattern(p *Pattern, ndx int) {
	if ndx < 0 {
		panic("negative song pattern index")
	}
	s.mu.Lock()
	if l := len(s.Patterns); ndx >= l {
		s.Patterns = append(s.Patterns, make([]*Pattern, 1+ndx-l)...)
	}
	if s.Patterns[ndx] = p; s.Patterns[ndx] == nil {
		isEmptyTail := true
		for i := ndx; i < len(s.Patterns); i++ {
			if isEmptyTail = s.Patterns[i] == nil; !isEmptyTail {
				break
			}
		}
		if isEmptyTail {
			s.Patterns = s.Patterns[:ndx]
		}
	}
	s.mu.Unlock()
}

// IndexToBeat counts beats and measures leading up to an index.
func (s *Song) IndexToBeat(idx int) (ret float32) {
	s.mu.RLock()
	for i := 0; i < idx && i < len(s.Patterns); i++ {
		if p := s.Patterns[i]; p != nil {
			ret += p.Beats()
		}
	}
	s.mu.RUnlock()
	return ret
}
