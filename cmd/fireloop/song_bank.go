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
	// screen is where the arrangement text goes, for the same reason as PatternBank.screen:
	// the Fire by default and a recorder in a test, so a test reads what a row says instead
	// of decoding pixels.
	screen textScreen
}

func NewSongBank(f *Fire, pb *PatternBank) *SongBank {
	sb := &SongBank{
		Songs:      make(map[int]*Song),
		selSongIdx: 1,
		pb:         pb,
		f:          f,
		screen:     f,
	}
	sb.Songs[sb.selSongIdx] = &Song{}
	sb.resetArrangementView()
	return sb
}

// kit is the voice bank the song's patterns are written against, which is also where the
// patches sent when playback starts are read from.
func (sb *SongBank) kit() *VoiceBank {
	return sb.pb.vb
}

func (sb *SongBank) CurrentSong() *Song {
	return sb.Songs[sb.selSongIdx]
}

// installSongs moves in songs that were already built against the restored patterns, so
// the measures and the pattern bank keep pointing at one object each. The song slots and
// the arrangement window are left where they were: where the user was scrolling is the
// display's business, not part of a set.
//
// The selected song is the one slot a load has to supply, because a song holding no
// measures is not written to a file. Restoring the bank onto a song the file does not
// carry would leave CurrentSong nil, and the arrangement view reads it on every pad press.
func (s *SongBank) installSongs(songs map[int]*Song) {
	if _, ok := songs[s.selSongIdx]; !ok {
		songs[s.selSongIdx] = &Song{}
	}
	s.Songs = songs
}

// askStop stops whatever is playing on behalf of an edit that rewrites a measure. The
// controller is the pattern bank's, which is the same one: an arrangement is always built
// over a pattern bank, so asking through it is asking the owner rather than a copy of it.
func (s *SongBank) askStop() error {
	return s.pb.controller.stopPlayback()
}

func (s *SongBank) Jump(n int) error {
	if n != 0 {
		if err := s.askStop(); err != nil {
			return err
		}
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

	if err := s.PrintSong(); err != nil {
		return err
	}
	if err := s.PrintPattern(); err != nil {
		return err
	}
	if err := s.PrintTempo(); err != nil {
		return err
	}
	// The rows below the header belong to the playback worker, which reports the measure it
	// reached. They are cleared rather than left holding the last song's reading, because a
	// stale measure looks like a jump the music did not make.
	for row := 3; row <= 5; row++ {
		if err := s.printRow(row, " "); err != nil {
			return err
		}
	}
	if err := s.printView(); err != nil {
		return err
	}
	if err := s.DrawPadMeasures(); err != nil {
		return err
	}
	return s.DrawPadPatterns()
}

// JumpMeasure seeks the running set to the measure under the grid. A seek with nothing
// playing has nothing to schedule, so the gesture does nothing at all rather than reporting
// a position it did not move to.
func (s *SongBank) JumpMeasure(x, y int) error {
	idx, ok := s.measureIndex(x, y)
	if !ok || !s.pb.controller.playing() {
		return nil
	}
	// Determine beat from grid position.
	b := s.CurrentSong().IndexToBeat(idx)
	lastSongBeat := s.pb.controller.seekSongBeat(b)
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
	if sp := song.GetPattern(idx); sp == p {
		song.SetPattern(nil, idx)
		p = nil
	} else {
		song.SetPattern(p, idx)
	}
	color := oledBlack
	if p != nil {
		color = dimColor(patternColor(sb.pb.selPatIdx))
	}
	return sb.f.LightPadSlice([]akai.Pad{makePad(x, y, color)})
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
		color := dimColor(p2c[pattern])
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
	if n <= 0 || n > maxPatternIndex {
		return nil
	}
	if err := s.askStop(); err != nil {
		return err
	}
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
	return s.printRow(2, fmt.Sprintf("Tempo %03d", currentBPM()))
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
			color = dimColor(p2c[pattern])
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
			color = patternColor(index)
			if index != s.pb.selPatIdx {
				color = dimColor(color)
			}
		}
		// rightmost bank of 16 pads
		x, y := (i%4)+4*3, i/4
		pads[i] = makePad(x, y, color)
	}
	return s.f.LightPadSlice(pads)
}

// patternColor is the colour one pattern is drawn in, taken from its bank index. Neighbouring
// patterns sit three steps apart in the table so that the selected one is never beside a
// similar colour. A pattern the bank has no slot for has no colour, which is the black an
// empty measure and an unwritten slot are drawn in.
func patternColor(index int) [3]int {
	if index < 1 || index > maxPatternIndex {
		return oledBlack
	}
	return oledColorTable[(3*(index-1))%len(oledColorTable)]
}

func (s *SongBank) patternsToColors() map[*Pattern][3]int {
	ret := make(map[*Pattern][3]int)
	for index, pattern := range s.pb.Patterns {
		if pattern == nil {
			continue
		}
		ret[pattern] = patternColor(index)
	}
	return ret
}

// printText writes to the song bank's text layer.
func (s *SongBank) printText(row, col int, text string, inverted bool) error {
	return displayPrint(s.screen, row, col, text, inverted)
}

// clearTextRows blanks rows on the song bank's text layer.
func (s *SongBank) clearTextRows(row, n int) error {
	return displayClear(s.screen, row, n)
}

// printRow replaces one row of the arrangement with text.
func (s *SongBank) printRow(row int, text string) error {
	return replaceRow(s.screen, row, text, false)
}
