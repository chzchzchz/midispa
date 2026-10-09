package main

import (
	"fmt"
	"sync"

	"github.com/chzchzchz/midispa/sysex/akai"
)

// padRows is the number of hardware pad rows. The track window shows one track per row.
const padRows = 4

// noPlayheadStep means the note-edit playhead is not lighting a strip cell.
const noPlayheadStep = -1

// editMode is what a grid pad means while a pattern is being edited. It is one value
// rather than two flags because the modes are exclusive: Overview takes the grid for the
// pattern's length, Mode takes it for the pitch palette, and a press meant for one while
// the other was on would change something the user never asked to change. Two booleans
// made that a convention spread over four places; one value makes it a property of the
// type, so a bank cannot be in two of them at once even by accident.
type editMode int

const (
	// stepEdit is the ordinary mode: a grid pad places or removes a step.
	stepEdit editMode = iota
	// noteEdit hands the grid to the pitch palette, so a pad chooses a pitch for the step
	// being edited rather than being a step.
	noteEdit
	// lengthEdit hands the SELECT knob to the pattern's length.
	lengthEdit
	// swingEdit hands the SELECT knob and the pads as a keypad to the swing amount.
	swingEdit
)

// noHeldStep means no step cell is held, so a press cannot be the second half of a tie.
const noHeldStep = -1

type PatternBank struct {
	Patterns          map[int]*Pattern
	selPatIdx         int
	selTrackRow       int // pad row [1,padRows], or 0 when no row is selected
	trackOffset       int // zero-based track shown on the first pad row
	mode              editMode
	stepCursor        int
	chromaticVelocity int
	pressedPads       uint64
	rowPadMasks       [padRows]uint16
	trackVoices       []int
	// screen is where text goes. It is the Fire by default and a recorder in tests, so a
	// test reads what a row says instead of decoding pixels.
	screen textScreen
	// pads is where everything the unit shows rather than says goes. It is the Fire in
	// production and a recorder in a test, so a pad or a light can be asserted without
	// running an event through the handler that set it.
	pads unitScreen
	vb   *VoiceBank
	// controller is the owner of this bank, set when the controller takes it. A bank built
	// on its own, as a test builds one, has none: nothing is playing, so there is nothing
	// to ask and no event to route.
	controller *Controller
	// playheadStep is the strip cell the note-edit playhead is lighting, or
	// noPlayheadStep when it is not lighting one.
	playheadStep int
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
		screen:            f,
		pads:              f,
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
	p.claimMode(false, noteEdit)
	p.clearPadState()
	return p.pads.SetLed(NoteMode, LEDOff)
}

// resetEditState returns the view to where a pattern starts being worked on: nothing
// being edited and no step chosen. The cursor belongs to the pattern being left
// rather than the one arriving, which is why choosing a pattern clears it.
func (p *PatternBank) resetEditState() error {
	p.stepCursor = 0
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
	oldPat.install(events, lengthSteps)
	if err := p.resetEditState(); err != nil {
		return err
	}
	return p.redraw()
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
	// printSwing rebuilds this row from the separator and the swing, so it has to come after
	// the separator rather than instead of it.
	if err := p.printSwing(); err != nil {
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
	if p.lengthEditActive() {
		return p.printLength()
	}
	if p.swingEditActive() {
		return p.printSwingEntry()
	}
	if p.noteEditActive() {
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

// SelectTrackRow moves the selection to a pad row, or clears it when the row is pressed
// again, and keeps the pattern playing. Choosing a track to edit is a view change rather
// than an edit, so stopping to do it would cut a performance short every time the row
// moved. The selection is not something the playback worker reads, so this stays safe
// while a pattern runs.
//
// Note editing is left alone, which is also what keeps it safe: the worker reads whether
// note editing owns the pad grid, and a playing pattern makes that a shared read. The
// palette simply follows the selection, editing the voice now on the selected row; the
// status row and the step guard already report that voice. Press Mode to leave.
//
// Only the selection changes here. What the unit shows afterwards is the render's job, so
// the state can be set and read without drawing and the drawing fails on its own.
func (p *PatternBank) SelectTrackRow(row int) error {
	if p.trackForPadRow(row) == 0 {
		return nil
	}
	// A pad held for the previous selection would complete a gesture against the wrong
	// voice, so the held pads go; the notes themselves are untouched.
	p.clearPadState()
	previous := p.selTrackRow
	if previous == row {
		p.selTrackRow = 0
	} else {
		p.selTrackRow = row
	}
	return p.renderTrackSelection(previous)
}

// renderTrackSelection repaints the display for a move from one selected row to another,
// reading the row being arrived at from the selection rather than being told, so the two
// cannot disagree about where the highlight is. The row being left goes dark and the row
// being arrived at goes green and has its pads repainted, so the highlight follows the
// selection rather than being left wherever it was last drawn.
func (p *PatternBank) renderTrackSelection(previous int) error {
	if previous > 0 {
		if err := p.pads.SetLed(CCMuteLED1+(previous-1), LEDOff); err != nil {
			return err
		}
		if err := p.printTrackRow(previous, false); err != nil {
			return err
		}
	}
	if current := p.selTrackRow; current > 0 {
		if err := p.printTrackRow(current, true); err != nil {
			return err
		}
		if err := p.pads.SetLed(CCMuteLED1+(current-1), LEDGreen); err != nil {
			return err
		}
		if err := p.redrawTrackPads(current); err != nil {
			return err
		}
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

// unitScreen is everything about how the unit looks rather than what it says: the pads,
// the button lights, and the blackout that suppresses both until a press wakes them.
//
// It is a seam of its own rather than more of textScreen because the two are wanted for
// opposite things. A test that reads the screen wants the pads and lights left where the
// Fire put them, so swapping the text layer alone must not redirect them; a test that
// wants to know what a method painted swaps this instead. Keeping them apart is what lets
// one recorder read text while the hardware keeps the rest.
//
// *Fire is the whole thing in production. A bank holds one of these and nothing else that
// reaches the unit, so there is a single record of where a pad went and a test stands in
// front of exactly one field.
type unitScreen interface {
	LightPadSlice(pads []akai.Pad) error
	LightPadColor(x, y int, color [3]int) error
	LightPadRow(row int, vals [16][3]int) error
	LightPadColumn(col int, vals [4][3]int) error
	SetLed(n, v int) error
	Blackout() error
	Wake() bool
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

// printReadout replaces the bottom row, which whichever mode or message owns it shares with
// all the others. The row is cleared first because there is no partial-row clear, so a
// shorter message would otherwise leave the tail of the one before it.
//
// Only this row is cleared. Wiping the display looked like a blackout while the pads stayed
// lit, which is the whole reason this is not just printText over a wider clear.
func (p *PatternBank) printReadout(col int, text string) error {
	if err := p.clearTextRows(readoutRow, 1); err != nil {
		return err
	}
	return p.printText(readoutRow, col, text, false)
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
	// The groups come from the same walk that fills the row, because
	// the flag that names one rides on its first note. A group's first
	// note can sit in an earlier column than the cell it shades, so
	// the walk's own colours cannot supply the shade: the group is
	// recorded here and drawn after.
	var groups []tripletGroup
	groupColors := map[int][3]int{}
	for _, ev := range evs {
		if ev.Voice != tv {
			continue
		}
		idx := eventStep(ev)
		if idx < 0 || idx >= len(rgb) {
			continue
		}
		rgb[idx] = stepColor(ev)
		if ev.Tie {
			rgb[idx] = markTieColor(rgb[idx], false)
		}
		if ev.Triplet != tripletNone {
			groups = append(groups, tripletGroup{start: idx, cells: int(ev.Triplet)})
			groupColors[idx] = rgb[idx]
		}
	}
	for _, group := range groups {
		if cell := group.start + group.cells - 1; group.consumed(cell) && cell < len(rgb) {
			rgb[cell] = tripletShadeColor(groupColors[group.start], false)
		}
	}
	if !p.noteEditActive() {
		if p.stepCursor >= 0 && p.stepCursor < len(rgb) {
			rgb[p.stepCursor] = markCursorColor(rgb[p.stepCursor])
		}
	}
	return p.pads.LightPadRow(row-1, rgb)
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
	color := stepColor(event)
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
	// The column's own events cannot supply a group's shade, because a
	// group's first note sits in an earlier column than the cell it
	// shades. One walk resolves every row's shade group at once.
	rowGroups := pattern.tripletGroupsConsuming(col, rowVoices)
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
	// A consumed cell is drawn from its group's first note, which the
	// column walk above did not see: the first note sits in an earlier
	// column.
	for row, group := range rowGroups {
		if group.consumed(col) {
			rgb[row] = tripletConsumedColor(pattern, group, rowVoices[row], invert)
		}
	}
	return p.pads.LightPadColumn(col, rgb)
}

// toggleTripletGroup is the triplet gesture's one entry point from the pad
// grid: it sets or clears the group the pressed step belongs to, on the voice
// of the row the press came from. The gesture is a claim about a beat rather
// than a pad being held, so the held-pad state is left alone and the cursor
// stays where it was.
func (p *PatternBank) toggleTripletGroup(step int, voice *Voice, kind tripletKind) error {
	// The guard is the readout row's own, which every editor that writes a
	// line holds: a gesture that wrote its line over a number being entered
	// would be the failure the guard exists to prevent. Swing entry never
	// reaches this far, because its keypad branch types ahead of both
	// gesture branches.
	if p.readoutIsOwned() {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil || voice == nil {
		return nil
	}
	start := tripletGroupStart(step, int(kind))
	set, err := pattern.ToggleTripletGroup(step, voice, kind)
	if err != nil {
		logger.Debug("triplet refused",
			"start", start,
			"cells", int(kind),
			"voice", voiceLabel(voice),
			"error", err,
		)
		return p.repaintTripletEdit(tripletReadout(kind, start, err, false))
	}
	logger.Debug("triplet",
		"start", start,
		"cells", int(kind),
		"voice", voiceLabel(voice),
		"set", set,
	)
	return p.repaintTripletEdit(tripletReadout(kind, start, nil, set))
}

// repaintTripletEdit repaints the view the gesture edited and then writes the
// gesture's own line on the readout row. It is repaintEditView with the
// readout left out, because the gesture's line is the status for this press:
// the gesture can be aimed at a row the selection is not on, so the step
// status would say nothing about the beat that just changed.
func (p *PatternBank) repaintTripletEdit(line string) error {
	if p.noteEditActive() {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	} else if err := p.redrawStepRows(); err != nil {
		return err
	}
	return p.printReadout(0, fitOLEDText(line))
}

// redrawRowsWithVoice repaints every row a voice is shown on, which is what a
// change that can move notes across a row needs.
func (p *PatternBank) redrawRowsWithVoice(voice *Voice) error {
	return p.forEachRowWithVoice(voice, func(row int) error {
		return p.redrawTrackPads(row + 1)
	})
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
	// A press on a cell a triplet group owns can take the group apart, and
	// the repair then moves notes onto cells the press never touched, so the
	// row is repainted whole rather than pad by pad.
	_, groupCovered := pattern.TripletGroupAt(col, voice)
	if voice.IsChromatic() {
		if event, ok := pattern.EventAtStep(col, voice); ok {
			pattern.RemoveEventAtStep(col, voice)
			event.Velocity = 0
			if groupCovered {
				return event, p.redrawRowsWithVoice(voice)
			}
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
	if groupCovered {
		return ev, p.redrawRowsWithVoice(voice)
	}
	// The press lights the step it placed and puts out the one it removed, so the pad
	// says what the press did without waiting for a redraw of the whole row.
	color := stepColor(ev)
	if !added {
		color = [3]int{}
	}
	if err := p.forEachRowWithVoice(voice, func(row int) error {
		return p.pads.LightPadColor(col, row, color)
	}); err != nil {
		return ev, err
	}
	return ev, nil
}
