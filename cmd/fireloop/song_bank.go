package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/sysex/akai"
)

type SongBank struct {
	Songs      map[int]*Song
	selSongIdx int // [1,999]
	// Viewport offsets stay independent from the selected pattern and measure.
	patternStart int
	measureStart int
	pb           *PatternBank
	f            *Fire
	playback     *Playback
}

func NewSongBank(f *Fire, pb *PatternBank) *SongBank {
	sb := &SongBank{
		Songs:      make(map[int]*Song),
		selSongIdx: 1,
		pb:         pb,
		f:          f,
	}
	sb.Songs[sb.selSongIdx] = &Song{}
	sb.resetArrangementView()
	return sb
}

func (sb *SongBank) CurrentSong() *Song {
	return sb.Songs[sb.selSongIdx]
}

func (s *SongBank) Jump(n int) error {
	if n != 0 {
		stopPlayback()
	}
	newIdx := s.selSongIdx + n
	if newIdx <= 0 || newIdx > maxPatternIndex {
		return nil
	} else if _, ok := s.Songs[newIdx]; !ok {
		s.Songs[newIdx] = &Song{}
	}
	s.selSongIdx = newIdx
	if n != 0 || s.patternStart == 0 {
		s.resetArrangementView()
	}

	must(s.PrintSong())
	must(s.PrintPattern())
	must(s.PrintTempo())
	must(s.printRow(3, " "))
	must(s.printRow(4, " "))
	must(s.printRow(5, " "))
	must(s.printView())

	must(s.DrawPadMeasures())
	must(s.DrawPadPatterns())
	return nil
}

func (s *SongBank) JumpMeasure(x, y int) error {
	idx, ok := s.measureIndex(x, y)
	if !ok || s.playback == nil {
		return nil
	}
	// Determine beat from grid position.
	b := s.CurrentSong().IndexToBeat(idx)
	lastSongBeat := s.playback.JumpSongBeat(b)
	lastMeasure := -1
	if lastSongBeat >= 0 {
		_, lastMeasure = s.CurrentSong().BeatToPattern(lastSongBeat)
	}
	return s.printRow(4, fmt.Sprintf("Measure %03d->%03d", lastMeasure+1, idx+1))
}

func (sb *SongBank) ToggleMeasure(x, y int) error {
	idx, ok := sb.measureIndex(x, y)
	if !ok {
		return nil
	}
	song := sb.CurrentSong()
	p := sb.pb.Patterns[sb.pb.selPatIdx]
	if p == nil {
		return nil
	}
	if sp := song.GetPattern(idx); sp == p {
		song.SetPattern(nil, idx)
		p = nil
	} else {
		song.SetPattern(p, idx)
	}
	p2c := sb.patternsToColors()
	return sb.f.LightPadSlice([]akai.Pad{makePad(x, y, Dim(p2c[p], 16))})
}

func (sb *SongBank) ToggleMeasureBrightness(lastMeasure, nextMeasure int) error {
	p2c := sb.patternsToColors()
	pads := make([]akai.Pad, 0, 2)
	for _, item := range []struct {
		measure int
		bright  bool
	}{
		{measure: lastMeasure},
		{measure: nextMeasure, bright: true},
	} {
		x, y, ok := sb.measurePadPosition(item.measure)
		if !ok {
			continue
		}
		pattern := sb.CurrentSong().GetPattern(item.measure)
		color := Dim(p2c[pattern], 16)
		if item.bright {
			color = p2c[pattern]
		}
		pads = append(pads, makePad(x, y, color))
	}
	if len(pads) == 0 {
		return nil
	}
	return sb.f.LightPadSlice(pads)
}

func (s *SongBank) SelectPattern(n int) error {
	if s.pb == nil || n <= 0 || n > maxPatternIndex {
		return nil
	}
	stopPlayback()
	s.pb.editingNote = false
	s.pb.clearPadState()
	if s.pb.Patterns == nil {
		s.pb.Patterns = make(map[int]*Pattern)
	}
	if _, ok := s.pb.Patterns[n]; !ok {
		s.pb.Patterns[n] = &Pattern{}
	}
	s.pb.selPatIdx = n
	s.ensurePatternVisible(n)
	if err := s.DrawPadPatterns(); err != nil {
		return err
	}
	if err := s.PrintPattern(); err != nil {
		return err
	}
	return s.printView()
}

func (s *SongBank) PrintSong() error {
	return s.printRow(0, fmt.Sprintf("Song %03d", s.selSongIdx))
}

func (s *SongBank) PrintPattern() error {
	steps := defaultPatternSteps
	if pattern := s.pb.CurrentPattern(); pattern != nil {
		steps = pattern.LengthSteps()
	}
	return s.printRow(1, fmt.Sprintf("Pattern %03d L%02d", s.pb.selPatIdx, steps))
}

func (s *SongBank) PrintTempo() error {
	return s.printRow(2, fmt.Sprintf("Tempo %03d", bpm))
}

func (s *SongBank) DrawPadMeasures() error {
	p2c := s.patternsToColors()
	song := s.CurrentSong()
	start, _ := s.measureWindow()
	pads := make([]akai.Pad, measureViewSize)
	for i := range pads {
		col := i%4 + 4*(i/16)
		row := (i / 4) % 4
		color := oledBlack
		if pattern := song.GetPattern(start + i); pattern != nil {
			color = Dim(p2c[pattern], 16)
		}
		pads[i] = makePad(col, row, color)
	}
	return s.f.LightPadSlice(pads)
}

func (s *SongBank) DrawPadPatterns() error {
	start, _ := s.patternWindow()
	pads := make([]akai.Pad, patternViewSize)
	for i := range pads {
		index := start + i
		color := oledBlack
		if pattern := s.pb.Patterns[index]; pattern != nil {
			color = oledColorTable[(3*(index-1))%len(oledColorTable)]
			if index != s.pb.selPatIdx {
				color = Dim(color, 16)
			}
		}
		// rightmost bank of 16 pads
		x, y := (i%4)+4*3, i/4
		pads[i] = makePad(x, y, color)
	}
	return s.f.LightPadSlice(pads)
}

func (s *SongBank) patternsToColors() map[*Pattern][3]int {
	ret := make(map[*Pattern][3]int)
	for index, pattern := range s.pb.Patterns {
		if pattern == nil || index < 1 || index > maxPatternIndex {
			continue
		}
		ret[pattern] = oledColorTable[(3*(index-1))%len(oledColorTable)]
	}
	return ret
}

func (sb *SongBank) printRow(row int, s string) error {
	if err := sb.f.ClearOLEDRows(row, 1); err != nil {
		return err
	}
	return sb.f.Print(0, row, s)
}
