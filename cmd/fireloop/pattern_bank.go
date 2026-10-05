package main

import (
	"fmt"
	"sync"
)

// padRows is the number of hardware pad rows. The track window shows one track per row.
const padRows = 4

// noPlayheadStep means the note-edit playhead is not lighting a strip cell.
const noPlayheadStep = -1

// noHeldStep means no step cell is held, so a press cannot be the second half of a tie.
const noHeldStep = -1

type PatternBank struct {
	Patterns          map[int]*Pattern
	selPatIdx         int
	selTrackRow       int // pad row [1,padRows], or 0 when no row is selected
	trackOffset       int // zero-based track shown on the first pad row
	editingLength     bool
	editingNote       bool
	stepCursor        int
	chromaticVelocity int
	pressedPads       uint64
	rowPadMasks       [padRows]uint16
	trackVoices       []int
	f                 *Fire
	// screen is where text goes. It is the Fire by default and a recorder in tests, so a
	// test reads what a row says instead of decoding pixels.
	screen   textScreen
	vb       *VoiceBank
	playback *Playback
	// controller is the owner of this bank, set when the controller takes it. A bank built
	// on its own, as a test builds one, has none: nothing is playing, so there is nothing
	// to ask and no event to route.
	controller *Controller
	// playheadStep is the strip cell the note-edit playhead is lighting, or
	// noPlayheadStep when it is not lighting one.
	playheadStep int
	// noteChosen records that a note has been picked at the current step. Until then the
	// pad standing for that step is guarded, because in note-edit mode a grid pad is a
	// pitch pad and pressing the one the user means as "this step" would rewrite the note
	// they were trying to reach.
	noteChosen bool
	// noteEditHeldStep is the step cell held while note editing. A second cell pressed
	// before that one is released ties the two steps, which is the tie gesture step mode
	// uses on the step grid. noHeldStep when nothing is held.
	noteEditHeldStep int
	// paletteOctave is how many octaves the SELECT knob has moved the note palette. It is a
	// view setting: it decides which pitches a pad press can reach and leaves the notes
	// already written into a pattern exactly where they are.
	paletteOctave int
	// trackMu guards the track window. The playback worker draws the visible tracks
	// from its own goroutine, so scrolling must not race with it.
	trackMu sync.RWMutex
}

func NewPatternBank(f *Fire, vb *VoiceBank) *PatternBank {
	if len(vb.voices) == 0 {
		panic("no voices")
	}
	ret := &PatternBank{
		Patterns:          make(map[int]*Pattern),
		chromaticVelocity: defaultStepVelocity,
		playheadStep:      noPlayheadStep,
		noteEditHeldStep:  noHeldStep,
		f:                 f,
		screen:            f,
		vb:                vb,
	}
	ret.trackVoices = make([]int, padRows)
	for i := range ret.trackVoices {
		ret.trackVoices[i] = i % len(vb.voices)
	}
	return ret
}

// TrackCount is the number of tracks available in the current pattern.
func (p *PatternBank) TrackCount() int {
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	return len(p.trackVoices)
}

// TrackOffset is the zero-based track shown on the first pad row.
func (p *PatternBank) TrackOffset() int {
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	return p.trackOffset
}

// trackForPadRow maps a one-based pad row to its one-based track, or zero when the row holds none.
func (p *PatternBank) trackForPadRow(row int) int {
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	return p.trackForPadRowLocked(row)
}

func (p *PatternBank) trackForPadRowLocked(row int) int {
	if row < 1 || row > padRows {
		return 0
	}
	track := p.trackOffset + row - 1
	if track < 0 || track >= len(p.trackVoices) {
		return 0
	}
	return track + 1
}

// trackVoice resolves the voice assigned to the track behind a pad row.
func (p *PatternBank) trackVoice(row int) *Voice {
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	return p.trackVoiceLocked(row)
}

func (p *PatternBank) trackVoiceLocked(row int) *Voice {
	track := p.trackForPadRowLocked(row)
	if track == 0 {
		return nil
	}
	return p.vb.voices[p.trackVoices[track-1]]
}

// visibleTrackVoices returns the voice on each pad row in row order, taking the window
// lock once. A row with no track comes back nil, and one voice can sit on several rows.
func (p *PatternBank) visibleTrackVoices() [padRows]*Voice {
	var voices [padRows]*Voice
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	for row := 1; row <= padRows; row++ {
		voices[row-1] = p.trackVoiceLocked(row)
	}
	return voices
}

// maxTrackCount caps the window at one track per kit voice, so every track can be
// given a real voice. A kit smaller than the pad grid keeps all four rows filled.
func maxTrackCount(voices int) int {
	if voices < padRows {
		return padRows
	}
	return voices
}

// growTracksToLocked appends empty tracks so the window can reach them. Tracks past the
// end of the kit wrap its voices, the same way a small kit fills every row.
func (p *PatternBank) growTracksToLocked(count int) {
	if limit := maxTrackCount(len(p.vb.voices)); count > limit {
		count = limit
	}
	for len(p.trackVoices) < count {
		p.trackVoices = append(p.trackVoices, len(p.trackVoices)%len(p.vb.voices))
	}
}

// maxTrackOffsetLocked stops the window with the last track on the bottom row, so
// scrolling never parks the pads on rows that have no track behind them.
func (p *PatternBank) maxTrackOffsetLocked() int {
	if len(p.trackVoices) <= padRows {
		return 0
	}
	return len(p.trackVoices) - padRows
}

// snapshotSession copies the track window and hands back the pattern map under the lock
// the playback worker reads the window through, so a save taken while a pattern plays
// cannot race the redraw, and cannot see the bank half-way through a load.
//
// The map itself comes back by reference rather than copied. The patterns in it carry their
// own locks and a save reads each one through them, so what needs protecting is the map,
// and copying it would be a shallow copy that costs the walk twice.
func (p *PatternBank) snapshotSession() ([]int, map[int]*Pattern) {
	p.trackMu.RLock()
	defer p.trackMu.RUnlock()
	return append([]int(nil), p.trackVoices...), p.Patterns
}

// restoreState installs a loaded session. The pattern map is replaced whole rather than
// merged, so a pattern that is gone from the file is gone from the session. The selection
// is left where it was: where the user was looking is the display's business, not part of
// a set, so a load only needs to make sure that pattern exists.
func (p *PatternBank) restoreState(state stateFile, patterns map[int]*Pattern) {
	p.restoreTracks(state.TrackVoices)
	p.trackMu.Lock()
	if _, ok := patterns[p.selPatIdx]; !ok {
		patterns[p.selPatIdx] = &Pattern{}
	}
	p.Patterns = patterns
	p.trackMu.Unlock()
}

// restoreTracks clamps the loaded voices into what the kit can supply. A track holding a
// voice that no longer exists takes the last one rather than being dropped, because a row
// with no voice behind it cannot be edited at all.
func (p *PatternBank) restoreTracks(voices []int) {
	if len(p.vb.voices) == 0 {
		return
	}
	last := len(p.vb.voices) - 1
	restored := make([]int, 0, len(voices))
	for _, voice := range voices {
		if voice < 0 {
			voice = 0
		}
		if voice > last {
			voice = last
		}
		restored = append(restored, voice)
	}
	p.trackMu.Lock()
	p.trackVoices = restored
	if len(p.trackVoices) < padRows {
		p.growTracksToLocked(padRows)
	}
	if limit := maxTrackCount(len(p.vb.voices)); len(p.trackVoices) > limit {
		p.trackVoices = p.trackVoices[:limit]
	}
	// A window scrolled by a longer track list would park the pads on rows with no track
	// behind them, so it comes back to where this list can show.
	p.trackOffset = clampIndex(p.trackOffset, 0, p.maxTrackOffsetLocked())
	p.trackMu.Unlock()
}

// forEachRowWithVoice calls fn with each zero-based pad row the voice stands on. One voice
// can sit on several rows, and a change to the voice applies to the notes on every one of
// them, so the walk over those rows is the same wherever a voice is being changed.
func (p *PatternBank) forEachRowWithVoice(voice *Voice, fn func(row int) error) error {
	for row, rowVoice := range p.visibleTrackVoices() {
		if rowVoice != voice {
			continue
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	return nil
}

// leaveNoteEdit closes note editing and puts the Mode light out again. It draws nothing:
// every caller is about to redraw what note editing owned, so a redraw here would only be
// thrown away by the one that follows.
func (p *PatternBank) leaveNoteEdit() error {
	p.editingNote = false
	p.clearPadState()
	return p.f.SetLed(NoteMode, LEDOff)
}

// resetEditState returns the view to where a pattern starts being worked on: nothing being
// edited, no step chosen, and no guard held on a step. The cursor belongs to the pattern
// being left rather than the one arriving, which is why choosing a pattern clears it.
func (p *PatternBank) resetEditState() error {
	p.stepCursor = 0
	p.noteChosen = false
	return p.leaveNoteEdit()
}

// ScrollTracks moves the visible track window. Scrolling past the last track adds
// tracks instead of stopping, so a pattern only takes on the tracks the user reaches
// and a large kit does not open on a track list too long to work with. Navigation
// changes no notes, so playback keeps running.
func (p *PatternBank) ScrollTracks(delta int) error {
	p.trackMu.Lock()
	offset := p.trackOffset + delta
	if offset > p.maxTrackOffsetLocked() {
		p.growTracksToLocked(offset + padRows)
		offset = p.maxTrackOffsetLocked()
	}
	if offset < 0 {
		offset = 0
	}
	if offset == p.trackOffset {
		p.trackMu.Unlock()
		return nil
	}
	p.trackOffset = offset
	// Released before the redraw, which reads the window again through trackVoice.
	p.trackMu.Unlock()
	if err := p.leaveNoteEdit(); err != nil {
		return err
	}
	return p.redraw()
}

// CurrentPattern is the pattern the bank is showing. The map is guarded by trackMu for a
// save, which reads it by reference; the read here does not take that lock, because the
// playback worker reaches this on every drawn column. What makes that safe is that every
// path which changes the map stops playback first, so the worker has been joined by the
// time the map moves. A new path that writes the map has to do the same.
func (p *PatternBank) CurrentPattern() *Pattern {
	return p.Patterns[p.selPatIdx]
}

// PatternIdxMap maps each pattern back to the bank index it sits at.
func (pb *PatternBank) PatternIdxMap() map[*Pattern]int {
	return patternIndexMap(pb.Patterns)
}

// patternIndexMap does the mapping for a bank the caller has already snapshotted, so a save
// reads the map it took once rather than walking the live one a second time.
func patternIndexMap(patterns map[int]*Pattern) map[*Pattern]int {
	ret := make(map[*Pattern]int)
	for i, p := range patterns {
		ret[p] = i
	}
	return ret
}

// askStop stops whatever is playing on behalf of an edit that rewrites what the worker
// reads. The controller is always there because it is what builds the bank.
func (p *PatternBank) askStop() error {
	return p.controller.stopPlayback()
}

func (p *PatternBank) SetPattern(pat *Pattern) error {
	// Don't swap out pointer since song may already be using it.
	if pat == nil {
		return nil
	}
	if err := p.askStop(); err != nil {
		return err
	}
	oldPat, ok := p.Patterns[p.selPatIdx]
	if !ok || oldPat == nil {
		oldPat = &Pattern{}
		p.Patterns[p.selPatIdx] = oldPat
	}
	events, lengthSteps := pat.snapshot()
	oldPat.mu.Lock()
	oldPat.Events = events
	oldPat.lengthSteps = lengthSteps
	oldPat.normalizeLocked()
	oldPat.mu.Unlock()
	if err := p.resetEditState(); err != nil {
		return err
	}
	return p.Jump(0)
}

func (p *PatternBank) Jump(n int) error {
	newIdx := p.selPatIdx + n
	if newIdx <= 0 || newIdx > 999 {
		return nil
	}
	changed := newIdx != p.selPatIdx
	if changed {
		if err := p.askStop(); err != nil {
			return err
		}
	}
	p.selPatIdx = newIdx
	if _, ok := p.Patterns[p.selPatIdx]; !ok {
		p.Patterns[p.selPatIdx] = &Pattern{}
	}
	if changed {
		if err := p.resetEditState(); err != nil {
			return err
		}
	}
	p.clampStepCursor()
	return p.redraw()
}

// headerText carries the track window so a scrolled pattern stays readable on the display.
func (p *PatternBank) headerText() string {
	return fmt.Sprintf("Pattern %03d %2d/%d", p.selPatIdx, p.TrackOffset()+1, p.TrackCount())
}

func (p *PatternBank) redraw() error {
	if err := p.printText(0, 0, p.headerText(), false); err != nil {
		return err
	}
	if err := p.printText(1, 0, "-----------", false); err != nil {
		return err
	}
	for row := 1; row <= padRows; row++ {
		if err := p.redrawTrackPads(row); err != nil {
			return err
		}
		if err := p.printTrackRow(row, row == p.selTrackRow); err != nil {
			return err
		}
	}
	if p.editingLength {
		return p.printLength()
	}
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	}
	return p.printStepStatus()
}

func (p *PatternBank) ClearTrackRow(row int) error {
	if p.trackForPadRow(row) == 0 {
		return nil
	}
	if err := p.askStop(); err != nil {
		return err
	}
	voice := p.trackVoice(row)
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	pattern.ClearVoice(voice)
	if err := p.leaveNoteEdit(); err != nil {
		return err
	}
	if err := p.forEachRowWithVoice(voice, func(row int) error {
		return p.redrawTrackPads(row + 1)
	}); err != nil {
		return err
	}
	return p.printStepStatus()
}

// SelectTrackRow moves the selection to a pad row and keeps the pattern playing. Choosing
// a track to edit is a view change rather than an edit, so stopping to do it would cut a
// performance short every time the row moved. The selection is not something the playback
// worker reads, so this stays safe while a pattern runs.
//
// Note editing is left alone, which is also what keeps it safe: the worker reads whether
// note editing owns the pad grid, and a playing pattern makes that a shared read. The
// palette simply follows the selection, editing the voice now on the selected row; the
// status row and the step guard already report that voice. Press Mode to leave.
func (p *PatternBank) SelectTrackRow(row int) error {
	if p.trackForPadRow(row) == 0 {
		return nil
	}
	// Deselect currently selected row, if any.
	if p.selTrackRow > 0 {
		if err := p.f.SetLed(CCMuteLED1+(p.selTrackRow-1), 0); err != nil {
			return err
		}
		if err := p.printTrackRow(p.selTrackRow, false); err != nil {
			return err
		}
	}
	// A pad held for the previous selection would complete a gesture against the wrong
	// voice, so the held pads go; the note itself is untouched.
	p.clearPadState()
	if p.selTrackRow == row {
		p.selTrackRow = 0
		return p.printStepStatus()
	}
	// Select new row.
	p.selTrackRow = row
	if err := p.printTrackRow(row, true); err != nil {
		return err
	}
	if err := p.f.SetLed(CCMuteLED1+(row-1), LEDGreen); err != nil {
		return err
	}
	if err := p.redrawTrackPads(row); err != nil {
		return err
	}
	return p.printStepStatus()
}

// printTrackRow names the track on a pad row; the header shows which track the row starts at.
func (p *PatternBank) printTrackRow(row int, inv bool) error {
	return replaceRow(p.screen, row+1, voiceDisplayName(p.trackVoice(row)), inv)
}

// textScreen is the text layer of the display. The Fire rasterizes what it is given; a
// recorder keeps the strings so a test can read a row.
type textScreen interface {
	Print(x, y int, s string) error
	PrintInvert(x, y int, s string) error
	ClearOLEDRows(y, n int) error
}

// displayPrint writes text at a row and column, inverted when asked. Both banks draw
// through their screen, so each of them reaches the display the same way and a test can
// put a recorder in front of either.
func displayPrint(screen textScreen, row, col int, text string, inverted bool) error {
	if inverted {
		return screen.PrintInvert(col, row, text)
	}
	return screen.Print(col, row, text)
}

// displayClear blanks rows on the text layer.
func displayClear(screen textScreen, row, n int) error {
	return screen.ClearOLEDRows(row, n)
}

// replaceRow blanks a row and writes one line into it, which is how every readout reaches
// the display: new text has to overwrite what was there, because a shorter line drawn over
// a longer one leaves the tail of the old one showing.
func replaceRow(screen textScreen, row int, text string, inverted bool) error {
	if err := displayClear(screen, row, 1); err != nil {
		return err
	}
	return displayPrint(screen, row, 0, text, inverted)
}

// printText writes to the pattern bank's text layer.
func (p *PatternBank) printText(row, col int, text string, inverted bool) error {
	return displayPrint(p.screen, row, col, text, inverted)
}

// clearTextRows blanks rows on the pattern bank's text layer.
func (p *PatternBank) clearTextRows(row, n int) error {
	return displayClear(p.screen, row, n)
}

func (p *PatternBank) JogSelect(n int) error {
	if p.selTrackRow == 0 || len(p.vb.voices) == 0 {
		return nil
	}
	track := p.trackForPadRow(p.selTrackRow)
	if track == 0 {
		return nil
	}
	if err := p.askStop(); err != nil {
		return err
	}
	if err := p.leaveNoteEdit(); err != nil {
		return err
	}
	p.trackMu.Lock()
	voice := p.trackVoices[track-1] + n
	if voice >= len(p.vb.voices) {
		voice = 0
	} else if voice < 0 {
		voice = len(p.vb.voices) - 1
	}
	p.trackVoices[track-1] = voice
	// Released before the redraw, which reads the window again through trackVoice.
	p.trackMu.Unlock()
	if err := p.printTrackRow(p.selTrackRow, true); err != nil {
		return err
	}
	if err := p.redrawTrackPads(p.selTrackRow); err != nil {
		return err
	}
	return p.printStepStatus()
}

func (p *PatternBank) redrawTrackPads(row int) error {
	// One read of the window answers both whether the row holds a track and which voice
	// stands behind it. Asking twice took the window lock twice for a row, and a cursor
	// move repaints all four.
	tv := p.trackVoice(row)
	if tv == nil {
		return nil
	}
	pat := p.CurrentPattern()
	if pat == nil {
		return nil
	}
	evs := pat.FindBeat(0)
	var rgb [16][3]int
	for _, ev := range evs {
		if ev.Voice != tv {
			continue
		}
		idx := eventStep(ev)
		if idx < 0 || idx >= len(rgb) {
			continue
		}
		rgb[idx] = chromaticEventColor(ev)
		if ev.Tie {
			rgb[idx] = markTieColor(rgb[idx], false)
		}
	}
	if !p.editingNote {
		if p.stepCursor >= 0 && p.stepCursor < len(rgb) {
			rgb[p.stepCursor] = markCursorColor(rgb[p.stepCursor])
		}
	}
	return p.f.LightPadRow(row-1, rgb)
}

// drawPadColumn repaints a column the playhead has moved off, so each step goes back to
// its own colour.
func (p *PatternBank) drawPadColumn(col int) error {
	return p.drawPadColumnColor(col, false)
}

// drawPadColumnInvert repaints the column the playhead is on, inverting each step's own
// colour so a pitch still reads while it is being played.
func (p *PatternBank) drawPadColumnInvert(col int) error {
	return p.drawPadColumnColor(col, true)
}

// playheadEventColor is a step under the playhead. A chromatic step keeps its pitch colour
// either way, so the playhead does not paint it green and the column behind it restores
// the real colour rather than a flat one.
func playheadEventColor(event Event, invert bool) [3]int {
	color := chromaticEventColor(event)
	if invert {
		return invertColor(color)
	}
	return color
}

// invertColor is per channel, so an inverted step reads as its own colour played hot.
func invertColor(color [3]int) [3]int {
	return eachChannel(color, func(value int) int {
		return midiNoteMax - clampMidiDataValue(value)
	})
}

// emptyStepColor is how a column with no note reads: black behind the playhead, grey under
// it. An empty cell is not inverted, because inverting black would be blinding.
func emptyStepColor(invert bool) [3]int {
	if invert {
		return [3]int{50, 50, 50}
	}
	return [3]int{}
}

func (p *PatternBank) drawPadColumnColor(col int, invert bool) error {
	if col < 0 || col >= padColumns {
		return errOutOfRange
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	var rgb [padRows][3]int
	for row := range rgb {
		rgb[row] = emptyStepColor(invert)
	}
	evs := pattern.FindBeat(stepBeat(col))
	// The window is read once for the whole column. The loop below breaks at the first
	// event past the column, so it runs at most padRows times, but every pass used to
	// take the window lock again for a mapping that cannot change under it.
	rowVoices := p.visibleTrackVoices()
	for _, ev := range evs {
		idx := eventStep(ev)
		if idx < col {
			continue
		}
		if idx >= col+1 {
			break
		}
		for row, rowVoice := range rowVoices {
			if ev.Voice == rowVoice {
				rgb[row] = playheadEventColor(ev, invert)
				if ev.Tie {
					rgb[row] = markTieColor(rgb[row], invert)
				}
			}
		}
	}
	return p.f.LightPadColumn(col, rgb)
}

// ToggleEvent edits the track shown on a zero-based pad row.
func (p *PatternBank) ToggleEvent(row, col, v int) (Event, error) {
	voice := p.trackVoice(row + 1)
	if voice == nil {
		return Event{}, nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil || col < 0 || col >= pattern.LengthSteps() {
		return Event{}, nil
	}
	if voice.IsChromatic() {
		if event, ok := pattern.EventAtStep(col, voice); ok {
			pattern.RemoveEventAtStep(col, voice)
			event.Velocity = 0
			return event, nil
		}
		return Event{}, nil
	}
	ev := Event{
		Voice:    voice,
		Beat:     stepBeat(col),
		Velocity: clampStepVelocity(voice, v),
	}
	added := pattern.ToggleEvent(ev)
	if !added {
		ev.Velocity = 0
	}
	// The press lights the step it placed and puts out the one it removed, so the pad
	// says what the press did without waiting for a redraw of the whole row.
	color := percussionStepColor
	if !added {
		color = [3]int{}
	}
	if err := p.forEachRowWithVoice(voice, func(row int) error {
		return p.f.LightPadColor(col, row, color)
	}); err != nil {
		return ev, err
	}
	return ev, nil
}
