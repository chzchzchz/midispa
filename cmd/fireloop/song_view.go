package main

import "fmt"

const (
	patternViewSize     = 16
	measureViewSize     = 48
	maxPatternIndex     = 999
	maxMeasureIndex     = 999
	patternPageSize     = patternViewSize
	measurePageSize     = 16
	measureFinePageSize = 4
)

// Where each viewport may start. A window has to begin on a whole page from the top of the
// index, because one that began part-way through a page would have fewer entries on it
// than the grid has pads, and the pads it does fill would name the wrong slots. These are
// the bounds every scroll and every reposition clamps against, so a window cannot be
// scrolled to a place the view cannot show.
const (
	patternWindowFirst = 1
	patternWindowLast  = maxPatternIndex - patternViewSize + 1
	measureWindowFirst = 0
	measureWindowLast  = maxMeasureIndex - measureViewSize + 1
)

// clampIndex keeps a viewport position inside the range the view can show.
func clampIndex(value, low, high int) int {
	return min(max(value, low), high)
}

// clampPatternWindow keeps a pattern window start inside its page-aligned range.
func clampPatternWindow(start int) int {
	return clampIndex(start, patternWindowFirst, patternWindowLast)
}

// clampMeasureWindow keeps a measure window start inside the range the arrangement holds.
func clampMeasureWindow(start int) int {
	return clampIndex(start, measureWindowFirst, measureWindowLast)
}

func patternPageStart(index int) int {
	index = max(index, 1)
	return ((index-1)/patternViewSize)*patternViewSize + 1
}

func (sb *SongBank) resetArrangementView() {
	index := 1
	if sb.pb != nil && sb.pb.selPatIdx > 0 {
		index = sb.pb.selPatIdx
	}
	sb.patternStart = clampPatternWindow(patternPageStart(index))
	sb.measureStart = 0
}

func (sb *SongBank) patternWindow() (int, int) {
	start := clampPatternWindow(sb.patternStart)
	return start, start + patternViewSize - 1
}

func (sb *SongBank) measureWindow() (int, int) {
	start := clampMeasureWindow(sb.measureStart)
	return start, start + measureViewSize - 1
}

func (sb *SongBank) ensurePatternVisible(index int) {
	start, end := sb.patternWindow()
	if index < start {
		start = index
	} else if index > end {
		start = index - patternViewSize + 1
	}
	sb.patternStart = clampPatternWindow(start)
}

// Keep scrolling separate from editing so browsing a long arrangement cannot change a measure.
func (sb *SongBank) ScrollPatterns(delta int) error {
	start, _ := sb.patternWindow()
	sb.patternStart = clampPatternWindow(start + delta*patternPageSize)
	if err := sb.DrawPadPatterns(); err != nil {
		return err
	}
	return sb.printView()
}

func (sb *SongBank) SelectPatternSlot(slot int) error {
	if slot < 0 || slot >= patternViewSize {
		return nil
	}
	start, _ := sb.patternWindow()
	return sb.SelectPattern(start + slot)
}

func (sb *SongBank) MovePatternSelection(delta int) error {
	return sb.SelectPattern(clampIndex(sb.pb.selPatIdx+delta, 1, maxPatternIndex))
}

func (sb *SongBank) ScrollMeasures(delta int) error {
	start, _ := sb.measureWindow()
	sb.measureStart = clampMeasureWindow(start + delta)
	if err := sb.DrawPadMeasures(); err != nil {
		return err
	}
	return sb.printView()
}

// measurePad is the pad the measure at offset local of the window sits on,
// and measureAt is the offset of the measure the pad at (x, y) shows: the
// two are inverses. The measure area is three 4x4 banks, each filled left
// to right and top to bottom, so a pad's column counts banks of sixteen
// measures and its row counts groups of four. Both directions live beside
// each other so a pad press and the pad that lights for a measure cannot
// drift apart, which TestMeasurePadPositionInvertsMeasureIndex pins.
func measurePad(local int) (int, int) {
	return (local % 4) + 4*(local/16), (local / 4) % 4
}

func measureAt(x, y int) int {
	return y*4 + x%4 + 16*(x/4)
}

// measureIndex reads a pad press back to the measure it addresses.
func (sb *SongBank) measureIndex(x, y int) (int, bool) {
	if x < 0 || x >= 12 || y < 0 || y >= 4 {
		return 0, false
	}
	start, _ := sb.measureWindow()
	return start + measureAt(x, y), true
}

// measurePadPosition is the pad a measure is drawn on, which is the pad a
// press edits that measure with.
func (sb *SongBank) measurePadPosition(measure int) (int, int, bool) {
	start, end := sb.measureWindow()
	if measure < start || measure > end {
		return 0, 0, false
	}
	x, y := measurePad(measure - start)
	return x, y, true
}

func (sb *SongBank) printView() error {
	patternStart, patternEnd := sb.patternWindow()
	measureStart, measureEnd := sb.measureWindow()
	return sb.printRow(3, fmt.Sprintf(
		"P %03d-%03d M %03d-%03d",
		patternStart,
		patternEnd,
		measureStart+1,
		measureEnd+1,
	))
}
