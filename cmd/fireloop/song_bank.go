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
	// pageIdx and pageOn are the page the arrangement is
	// showing, as plain display state: the controller, which
	// owns the page table, sets them whenever they change,
	// because the bank redraws itself from its own gestures
	// and cannot be handed them then. The bank formats the
	// two values into row 0 and knows nothing beyond that.
	pageIdx int
	pageOn  bool
	pb      *PatternBank
	// pads is this bank's own seam, so a test can redirect the arrangement's pads without
	// also redirecting the pattern's.
	pads unitScreen
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
		screen:     f,
		pads:       f,
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

// pageSongState is the song bank's share of a page: the
// songs the page holds, which one it has selected and where
// its two viewports sit. The song map moves by reference
// under the same rule as the pattern map: the bank only ever
// mutates what it currently points at.
type pageSongState struct {
	Songs        map[int]*Song
	SelSongIdx   int
	PatternStart int
	MeasureStart int
}

// savePageState snapshots the bank into a page.
func (sb *SongBank) savePageState() pageSongState {
	return pageSongState{
		Songs:        sb.Songs,
		SelSongIdx:   sb.selSongIdx,
		PatternStart: sb.patternStart,
		MeasureStart: sb.measureStart,
	}
}

// loadPageState restores a page into the bank, verbatim, UI
// fields and all: a jump must put back exactly the state the
// page was left with. The selected slot is made to exist, the
// guarantee a load performs today, because an arrangement view
// reads the selected song on every pad press and a slot the
// page does not have would leave it reading nothing.
func (sb *SongBank) loadPageState(state pageSongState) {
	if state.Songs == nil {
		state.Songs = make(map[int]*Song)
	}
	if _, ok := state.Songs[state.SelSongIdx]; !ok {
		state.Songs[state.SelSongIdx] = &Song{}
	}
	sb.Songs = state.Songs
	sb.selSongIdx = state.SelSongIdx
	sb.patternStart = state.PatternStart
	sb.measureStart = state.MeasureStart
}

// setPageIndicator records the page the arrangement is showing
// and whether that page is enabled, for row 0. The controller is
// the page table's owner and so the only caller: the bank cannot
// be handed the values when its own gestures redraw it.
func (sb *SongBank) setPageIndicator(idx int, on bool) {
	sb.pageIdx = idx
	sb.pageOn = on
}

// pageFlagText is how row 0 reads a page's enabled flag.
func pageFlagText(on bool) string {
	if on {
		return "on"
	}
	return "off"
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
	if n != 0 {
		s.resetArrangementView()
	}
	return s.redraw()
}

// redraw puts the arrangement on the display as it stands. The window follows the current
// pattern only while it is already at the top, which is the rule Jump has always had: a
// window the user scrolled to is theirs, and one at the top has not been put anywhere.
//
// It draws and settles nothing else, and in particular it does not create the selected
// song the way Jump does, so a repaint is not a write to the map the playback worker reads.
func (s *SongBank) redraw() error {
	if s.patternStart == 0 {
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
	return sb.pads.LightPadSlice([]akai.Pad{makePad(x, y, color)})
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
	return sb.pads.LightPadSlice(pads)
}

func (s *SongBank) SelectPattern(n int) error {
	if n <= 0 || n > maxPatternIndex {
		return nil
	}
	if err := s.askStop(); err != nil {
		return err
	}
	s.pb.claimMode(false, noteEdit)
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
	return s.printRow(0, fmt.Sprintf("Song %03d Pg %03d %s",
		s.selSongIdx, s.pageIdx, pageFlagText(s.pageOn)))
}

func (s *SongBank) PrintPattern() error {
	steps := defaultPatternSteps
	if pattern := s.pb.CurrentPattern(); pattern != nil {
		steps = pattern.LengthSteps()
	}
	return s.printRow(1, fmt.Sprintf("Pattern %03d L%02d", s.pb.selPatIdx, steps))
}

// PrintTempo is the arrangement's tempo row, carrying the swing beside it so a groove set
// against a song is visible from the song view. The swing is read live rather than redrawn on
// a change, so this row and the pattern view's cannot disagree.
func (s *SongBank) PrintTempo() error {
	row := fmt.Sprintf("Tempo %03d", currentBPM()) + swingText(currentSwingPct())
	return s.printRow(2, row)
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
	return s.pads.LightPadSlice(pads)
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
	return s.pads.LightPadSlice(pads)
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
