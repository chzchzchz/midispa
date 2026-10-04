package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/midi"
)

// This file models the Fire's display so a button sequence can be replayed and read back
// as "what would the unit be showing". That answers questions a test otherwise cannot,
// such as whether a blackout really blanks the pads or something relights them afterwards.
//
// Run it with:
//
//	go test -run Trace -v ./cmd/fireloop
//
// A step is one input. vel 0 is a release, which the Fire sends for its own buttons.

// The pad light message is f0 47 7f 43 65, two length bytes, then four bytes per pad.
const (
	padLightCommand  = 0x65
	padLightPayload  = 7
	padLightPerPad   = 4
	padLightTotalLen = padLightPayload + padColumns*padLightPerPad + 1
	// screenHeaderLen is f0, maker, device, family, command, two length bytes, then the
	// first band, last band, first column and last column.
	screenHeaderLen = 11
)

// fireSim applies every display message the app sends and holds the result the way the
// hardware would, so a later write can be told apart from an earlier one.
type fireSim struct {
	leds map[int]int
	pads [padColumns * padRows][3]int
	// fullClears and rowClears separate a wiped display from a single row redrawn, which
	// is the difference between a blackout and a readout.
	fullClears int
	rowClears  int
	writes     []string
}

func newFireSim() *fireSim {
	return &fireSim{leds: make(map[int]int)}
}

func isPadLight(data []byte) bool {
	return len(data) >= padLightTotalLen && data[0] == 0xf0 && data[4] == padLightCommand
}

func (s *fireSim) write(data []byte) error {
	switch {
	case len(data) == 3 && midi.IsCC(data[0]):
		s.leds[int(data[1])] = int(data[2])
		s.writes = append(s.writes, fmt.Sprintf("LED %s = %d", ledName(int(data[1])), data[2]))
	case isPadLight(data):
		lit := 0
		for at := padLightPayload; at+padLightPerPad <= len(data); at += padLightPerPad {
			idx := int(data[at])
			if idx < 0 || idx >= len(s.pads) {
				continue
			}
			s.pads[idx] = [3]int{int(data[at+1]), int(data[at+2]), int(data[at+3])}
			if s.pads[idx] != [3]int{0, 0, 0} {
				lit++
			}
		}
		s.writes = append(s.writes, fmt.Sprintf("pads: %d in message, %d lit overall", (len(data)-padLightPayload-1)/padLightPerPad, s.litPads()))
	case len(data) > 3 && data[0] == 0xf0:
		// The screen message carries its band range, which tells a full wipe from a row.
		if len(data) > 8 {
			firstRow, rows := int(data[7]), int(data[8])-int(data[7])+1
			if firstRow == 0 && rows == 8 {
				s.fullClears++
			} else {
				s.rowClears++
			}
			s.writes = append(s.writes, fmt.Sprintf("screen %d bytes rows %d-%d", len(data), firstRow, firstRow+rows-1))
			return nil
		}
		s.writes = append(s.writes, fmt.Sprintf("screen %d bytes", len(data)))
	default:
		s.writes = append(s.writes, fmt.Sprintf("unexpected %d bytes", len(data)))
	}
	return nil
}

// litPads counts the pads the unit would be showing as on.
func (s *fireSim) litPads() int {
	lit := 0
	for _, color := range s.pads {
		if color != [3]int{0, 0, 0} {
			lit++
		}
	}
	return lit
}

// litRow counts the lit pads on one row, which is how the track display is read.
// resetLog clears the recorded writes but keeps the pads and lights the unit is showing,
// so a later check can tell "still lit" from "never lit".
func (s *fireSim) resetLog() {
	s.fullClears = 0
	s.rowClears = 0
	s.writes = nil
}

func (s *fireSim) litRow(row int) int {
	lit := 0
	for col := 0; col < padColumns; col++ {
		if s.pads[col+row*padColumns] != [3]int{0, 0, 0} {
			lit++
		}
	}
	return lit
}

func (s *fireSim) litLEDs() []string {
	var names []string
	for control, value := range s.leds {
		if value != ledOffValue(control) {
			names = append(names, fmt.Sprintf("%s=%d", ledName(control), value))
		}
	}
	return names
}

// screenRecorder is a text-only display. It stands in for the Fire so a test can read what
// a row says, without rasterizing glyphs or decoding pixels.
type screenRecorder struct {
	rows [8]string
}

func (r *screenRecorder) Print(_, row int, text string) error {
	if row >= 0 && row < len(r.rows) {
		r.rows[row] = text
	}
	return nil
}

func (r *screenRecorder) PrintInvert(col, row int, text string) error {
	return r.Print(col, row, text)
}

func (r *screenRecorder) ClearOLEDRows(row, n int) error {
	for i := row; i < row+n && i < len(r.rows); i++ {
		if i >= 0 {
			r.rows[i] = ""
		}
	}
	return nil
}

func (r *screenRecorder) row(n int) string {
	if n < 0 || n >= len(r.rows) {
		return ""
	}
	return r.rows[n]
}

// useScreenRecorder points a bank's text at a recorder for the duration of a test.
func useScreenRecorder(t *testing.T, bank *PatternBank) *screenRecorder {
	t.Helper()
	recorder := &screenRecorder{}
	previous := bank.screen
	bank.screen = recorder
	t.Cleanup(func() { bank.screen = previous })
	return recorder
}

// uiStep is one input in a scripted sequence.
type uiStep struct {
	label string
	note  int
	vel   int
}

func press(label string, note int) uiStep {
	return uiStep{label: label, note: note, vel: 100}
}

func release(label string, note int) uiStep {
	return uiStep{label: label + " (up)", note: note, vel: 0}
}

// uiSnapshot is what the display held after one step.
type uiSnapshot struct {
	label   string
	litPads int
	litRow  [padRows]int
	litLEDs []string
	writes  int
}

func (s uiSnapshot) String() string {
	leds := "none"
	if len(s.litLEDs) > 0 {
		leds = strings.Join(s.litLEDs, " ")
	}
	return fmt.Sprintf("%-26s pads %2d lit (rows %v)  LEDs: %s", s.label, s.litPads, s.litRow, leds)
}

// traceUI runs a scripted sequence through the real handler and returns what the display
// held after each step. Each step is logged, so -v prints the whole story.
func traceUI(t *testing.T, steps []uiStep) []uiSnapshot {
	t.Helper()
	sim := newFireSim()
	patternBank := NewPatternBank(NewFire(sim.write), trackWindowKit(8, -1))
	if err := patternBank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, patternBank)
	snapshots := make([]uiSnapshot, 0, len(steps))
	for _, step := range steps {
		before := len(sim.writes)
		if err := processPatternEvent(nil, padMessage(step.note, step.vel)); err != nil {
			t.Fatal(err)
		}
		snapshot := uiSnapshot{
			label:   step.label,
			litPads: sim.litPads(),
			litLEDs: sim.litLEDs(),
			writes:  len(sim.writes) - before,
		}
		for row := 0; row < padRows; row++ {
			snapshot.litRow[row] = sim.litRow(row)
		}
		snapshots = append(snapshots, snapshot)
		t.Log(snapshot)
	}
	return snapshots
}

// The pad clear goes out as row-sized messages, so the whole grid ends up dark, and a
// real press brings it back while still doing what it says.
func TestTraceBlackoutBlanksTheWholeGrid(t *testing.T) {
	steps := []uiStep{
		press("toggle a step", 54),
		press("select row 1", NoteMute1),
		press("shift", NoteShift),
		press("alt: blackout", NoteAlt),
		press("stop", NoteStop),
		press("mute 3", NoteMute3),
	}
	snapshots := traceUI(t, steps)
	if snapshots[1].litPads == 0 {
		t.Fatal("the trace does not start from a lit grid, so it cannot prove a blank")
	}
	blackout := snapshots[3]
	if blackout.litPads != 0 {
		t.Fatalf("blackout left the grid lit: %v", blackout.litRow)
	}
	if len(blackout.litLEDs) != 0 {
		t.Fatalf("blackout left buttons lit: %v", blackout.litLEDs)
	}
	// Stop is a real press, so it ends the blackout and then stops playback.
	if woken := snapshots[4]; woken.litPads == 0 || len(woken.litLEDs) == 0 {
		t.Fatalf("a press did not wake the blackout: %v", woken)
	}
	// The waking press is handled, so the mute selection follows.
	if after := snapshots[5]; !contains(after.litLEDs, "Mute3=2") {
		t.Fatalf("the waking press was not handled: %v", after.litLEDs)
	}
}

// A blackout has to survive the buttons being released, which is how the Fire reports
// that a button came back up.
func TestTraceBlackoutSurvivesButtonReleases(t *testing.T) {
	snapshots := traceUI(t, []uiStep{
		press("shift", NoteShift),
		press("alt: blackout", NoteAlt),
		release("shift", NoteShift),
		release("alt", NoteAlt),
	})
	if after := snapshots[2]; after.litPads != 0 || len(after.litLEDs) != 0 {
		t.Fatalf("a release ended the blackout: %v", after)
	}
	if after := snapshots[3]; after.litPads != 0 || len(after.litLEDs) != 0 {
		t.Fatalf("the second release ended the blackout: %v", after)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// Tempo entry writes its number on one row. Wiping the whole display made it look like a
// blackout: the screen went dark while the pads stayed lit.
func TestTraceTempoEntryOnlyClearsItsRow(t *testing.T) {
	sim := newFireSim()
	bank := NewPatternBank(NewFire(sim.write), trackWindowKit(8, -1))
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	// Light the grid first, so a later blank could be told from never having been lit.
	for _, step := range []uiStep{press("select row 1", NoteMute1), press("toggle a step", 54)} {
		if err := processPatternEvent(nil, padMessage(step.note, step.vel)); err != nil {
			t.Fatal(err)
		}
	}
	if sim.litPads() == 0 {
		t.Fatal("the grid never lit, so the check below cannot prove anything")
	}
	sim.resetLog()
	for _, step := range []uiStep{press("shift", NoteShift), press("pad", 55), press("pad", 56)} {
		if err := processPatternEvent(nil, padMessage(step.note, step.vel)); err != nil {
			t.Fatal(err)
		}
	}
	for _, write := range sim.writes {
		t.Log(write)
	}
	if sim.fullClears != 0 {
		t.Fatalf("tempo entry wiped the display %d times, want row clears only", sim.fullClears)
	}
	if sim.rowClears == 0 {
		t.Fatal("tempo entry cleared no row, so the number would be drawn over stale text")
	}
	if sim.litPads() == 0 {
		t.Fatal("tempo entry blanked the grid, which a readout should never do")
	}
}

// The status line has to show each step's own velocity. Two steps holding different
// velocities that read back the same means the display is showing the wrong one.
func TestTraceStatusShowsEachStepItsOwnVelocity(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, 0)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, bank)
	voice := voiceBank.voices[0]
	bank.CurrentPattern().SetChromaticNote(0, voice, 60, 83)
	bank.CurrentPattern().SetChromaticNote(4, voice, 62, 99)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}

	seen := map[int]string{}
	for _, step := range []int{0, 4} {
		if err := bank.setStepCursor(step); err != nil {
			t.Fatal(err)
		}
		status := recorder.row(lengthDisplayRow)
		t.Logf("cursor at step %d -> status %q", step, status)
		seen[step] = status
	}
	if seen[0] == seen[4] {
		t.Fatalf("two steps with different velocities read the same: %q", seen[0])
	}
	if !strings.Contains(seen[0], "@083") {
		t.Fatalf("status at step 1 = %q, want its own velocity 083", seen[0])
	}
	if !strings.Contains(seen[4], "@099") {
		t.Fatalf("status at step 5 = %q, want its own velocity 099", seen[4])
	}
}

// A step with no note must not display a velocity. The value a new note would inherit is
// not that step's velocity, and showing it there made two steps look equal.
func TestTraceEmptyStepShowsNoVelocity(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, 0)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, bank)
	voice := voiceBank.voices[0]
	bank.CurrentPattern().SetChromaticNote(0, voice, 60, 83)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.setStepCursor(0); err != nil {
		t.Fatal(err)
	}
	if status := recorder.row(lengthDisplayRow); !strings.Contains(status, "@083") {
		t.Fatalf("step 1 status = %q, want its velocity 083", status)
	}
	// An empty step must not carry the previous note's velocity on the display.
	if err := bank.setStepCursor(4); err != nil {
		t.Fatal(err)
	}
	empty := recorder.row(lengthDisplayRow)
	t.Logf("empty step status %q", empty)
	if strings.Contains(empty, "@") {
		t.Fatalf("empty step shows a velocity: %q", empty)
	}
	if empty != "S05 --" {
		t.Fatalf("empty step status = %q, want %q", empty, "S05 --")
	}
}

// Moving onto a step with a pad shows that step's velocity without disturbing the encoder.
func TestTracePadMoveShowsThatStepWithoutMovingTheEncoder(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, 0)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, bank)
	voice := voiceBank.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 83)
	pattern.SetChromaticNote(4, voice, 62, 99)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	// Move onto step 5 by pressing its pad, the way a pad press does.
	if handled, err := bank.handleChromaticStepPress(0, 4); !handled || err != nil {
		t.Fatalf("pad move = %v/%v", handled, err)
	}
	// The display reports the step that is now selected.
	if status := recorder.row(lengthDisplayRow); !strings.Contains(status, "@099") {
		t.Fatalf("status after a pad move = %q, want @099", status)
	}
	// Re-picking the pitch sets the dynamics from the new press.
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := bank.handleNoteEditPad(nil, 1, 2, 60); err != nil {
		t.Fatal(err)
	}
	event, ok := pattern.EventAtStep(4, voice)
	if !ok {
		t.Fatal("the pitch edit removed the note")
	}
	if event.Velocity != 60 {
		t.Fatalf("velocity after a pitch change = %d, want the new press 60", event.Velocity)
	}
	if event.ChromaticNote != chromaticPaletteNote(1, 2) {
		t.Fatalf("pitch = %d, want the palette note", event.ChromaticNote)
	}
}

// How hard the palette pad was pressed becomes the note's velocity, and the display
// reports that same number rather than a fixed one.
func TestTracePressVelocityBecomesTheNotesVelocity(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, 0)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, bank)
	voice := voiceBank.voices[0]
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	preview := &captureMidiWriter{}
	for _, pressed := range []int{40, 90} {
		if err := bank.setStepCursor(0); err != nil {
			t.Fatal(err)
		}
		if err := bank.handleNoteEditPad(preview, 0, 0, pressed); err != nil {
			t.Fatal(err)
		}
		event, ok := bank.CurrentPattern().EventAtStep(0, voice)
		if !ok || event.Velocity != pressed {
			t.Fatalf("note velocity = %d/%v, want the pad press %d", event.Velocity, ok, pressed)
		}
		if status := recorder.row(lengthDisplayRow); !strings.Contains(status, fmt.Sprintf("@%03d", pressed)) {
			t.Fatalf("status = %q, want the press velocity %03d", status, pressed)
		}
	}
	// The audition carries the press velocity too, so what is heard matches what is shown.
	events := preview.events
	if len(events) < 4 {
		t.Fatalf("audition wrote %d messages, want a pair per press", len(events))
	}
	last := events[len(events)-2:]
	if !midi.IsNoteOn(last[0].Data[0]) || last[0].Data[2] != 90 {
		t.Fatalf("audition note on = %v, want velocity 90", last[0].Data)
	}
	if !midi.IsNoteOff(last[1].Data[0]) || last[1].Data[2] != 0 {
		t.Fatalf("audition note off = %v, want velocity 0", last[1].Data)
	}
}
