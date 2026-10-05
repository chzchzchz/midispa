package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	padLightCommand = 0x65
	padLightPayload = 7
	padLightPerPad  = 4
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

// isPadLight recognises the pad command by its header. The pad count is whatever the
// payload holds, so single pads, columns and whole rows are all covered.
func isPadLight(data []byte) bool {
	return len(data) > padLightPayload+1 && data[0] == 0xf0 &&
		data[1] == 0x47 && data[4] == padLightCommand
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
// a row says, without rasterizing glyphs or decoding pixels. Access is guarded because a
// save publishes off the event loop and writes its result to the same row the test is
// polling, so a test that watches for a late report reads and writes at once.
type screenRecorder struct {
	mu   sync.Mutex
	rows [8]string
}

func (r *screenRecorder) Print(_, row int, text string) error {
	if row < 0 || row >= len(r.rows) {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[row] = text
	return nil
}

func (r *screenRecorder) PrintInvert(col, row int, text string) error {
	return r.Print(col, row, text)
}

func (r *screenRecorder) ClearOLEDRows(row, n int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rows[n]
}

// useScreenRecorder points a bank's text at a recorder for the duration of a test. Both
// banks draw through a layer of the same kind, so the caller names which one it is
// redirecting, and the layer it had comes back when the test ends.
func useScreenRecorder(t *testing.T, screen *textScreen) *screenRecorder {
	t.Helper()
	recorder := &screenRecorder{}
	previous := *screen
	*screen = recorder
	t.Cleanup(func() { *screen = previous })
	return recorder
}

// uiStep is one input in a scripted sequence.
type uiStep struct {
	label string
	note  int
	vel   int
	// until is set on a step whose result arrives late. A save publishes off the event
	// loop, so the row it reports on is not written by the time the press returns, and a
	// trace that read it straight away would be describing a moment the user never sees.
	until func(recorder *screenRecorder) bool
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
	controller := useController(t, NewFire(sim.write), trackWindowKit(8, -1))
	patternBank := controller.patbank
	snapshots := make([]uiSnapshot, 0, len(steps))
	for _, step := range steps {
		before := len(sim.writes)
		require.NoError(t, dispatch(patternBank, padMessage(step.note, step.vel)))
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
	require.NotZero(t, snapshots[1].litPads, "the trace must start from a lit grid")
	blackout := snapshots[3]
	require.Zerof(t, blackout.litPads, "blackout left the grid lit: %v", blackout.litRow)
	require.Emptyf(t, blackout.litLEDs, "blackout left buttons lit: %v", blackout.litLEDs)
	// Stop is a real press, so it ends the blackout and then stops playback.
	woken := snapshots[4]
	require.NotZerof(t, woken.litPads, "a press did not wake the blackout: %v", woken)
	require.NotEmptyf(t, woken.litLEDs, "a press did not wake the blackout: %v", woken)
	// The waking press is handled, so the mute selection follows.
	assert.Contains(t, snapshots[5].litLEDs, "Mute3=2", "the waking press was not handled")
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
	for _, after := range snapshots[2:] {
		require.Zerof(t, after.litPads, "a release ended the blackout: %v", after)
		require.Emptyf(t, after.litLEDs, "a release ended the blackout: %v", after)
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
	bank, _, sim := recordedBank(t, trackWindowKit(8, -1))
	// Light the grid first, so a later blank could be told from never having been lit.
	for _, step := range []uiStep{press("select row 1", NoteMute1), press("toggle a step", 54)} {
		require.NoError(t, dispatch(bank, padMessage(step.note, step.vel)))
	}
	require.NotZero(t, sim.litPads(), "the grid never lit, so the checks below prove nothing")
	sim.resetLog()
	for _, step := range []uiStep{press("shift", NoteShift), press("pad", 55), press("pad", 56)} {
		require.NoError(t, dispatch(bank, padMessage(step.note, step.vel)))
	}
	for _, write := range sim.writes {
		t.Log(write)
	}
	require.Zerof(t, sim.fullClears, "tempo entry wiped the display, want row clears only")
	require.NotZerof(t, sim.rowClears, "tempo entry cleared no row, so the number would be drawn over stale text")
	require.NotZerof(t, sim.litPads(), "tempo entry blanked the grid, which a readout should never do")
}

// The status line has to show each step's own velocity. Two steps holding different
// velocities that read back the same means the display is showing the wrong one.
func TestTraceStatusShowsEachStepItsOwnVelocity(t *testing.T) {
	bank, voiceBank, recorder := screenBank(t, trackWindowKit(8, 0))
	voice := voiceBank.voices[0]
	bank.CurrentPattern().SetChromaticNote(0, voice, 60, 83)
	bank.CurrentPattern().SetChromaticNote(4, voice, 62, 99)
	require.NoError(t, bank.SelectTrackRow(1))

	seen := map[int]string{}
	for _, step := range []int{0, 4} {
		require.NoError(t, bank.setStepCursor(step))
		status := recorder.row(lengthDisplayRow)
		t.Logf("cursor at step %d -> status %q", step, status)
		seen[step] = status
	}
	require.NotEqualf(t, seen[4], seen[0], "two steps with different velocities read the same: %q", seen[0])
	assert.Containsf(t, seen[0], "@083", "status at step 1 should carry its own velocity")
	assert.Containsf(t, seen[4], "@099", "status at step 5 should carry its own velocity")
}

// A step with no note must not display a velocity. The value a new note would inherit is
// not that step's velocity, and showing it there made two steps look equal.
func TestTraceEmptyStepShowsNoVelocity(t *testing.T) {
	bank, voiceBank, recorder := screenBank(t, trackWindowKit(8, 0))
	voice := voiceBank.voices[0]
	bank.CurrentPattern().SetChromaticNote(0, voice, 60, 83)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(0))
	assert.Contains(t, recorder.row(lengthDisplayRow), "@083", "step 1 should show its own velocity")

	// An empty step must not carry the previous note's velocity on the display.
	require.NoError(t, bank.setStepCursor(4))
	empty := recorder.row(lengthDisplayRow)
	t.Logf("empty step status %q", empty)
	require.NotContainsf(t, empty, "@", "an empty step shows a velocity: %q", empty)
	require.Equal(t, "S05 --", empty)
}

// Moving onto a step with a pad shows that step's velocity without disturbing the encoder.
func TestTracePadMoveShowsThatStepWithoutMovingTheEncoder(t *testing.T) {
	bank, voiceBank, recorder := screenBank(t, trackWindowKit(8, 0))
	voice := voiceBank.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 83)
	pattern.SetChromaticNote(4, voice, 62, 99)
	require.NoError(t, bank.SelectTrackRow(1))
	// Move onto step 5 by pressing its pad, the way a pad press does.
	handled, err := bank.handleChromaticStepPress(0, 4)
	require.NoErrorf(t, err, "the pad press was not handled (handled=%v)", handled)
	require.True(t, handled, "the pad press was not handled")
	// The display reports the step that is now selected.
	assert.Contains(t, recorder.row(lengthDisplayRow), "@099", "status after a pad move")
	// Re-picking the pitch sets the dynamics from the new press.
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, bank.handleNoteEditPad(nil, 1, 2, 60, false))
	event, ok := pattern.EventAtStep(4, voice)
	require.True(t, ok, "the pitch edit removed the note")
	require.Equal(t, 60, event.Velocity, "velocity after a pitch change should be the new press")
	want, _ := chromaticPaletteNote(1, 2, 0)
	require.Equal(t, want, event.ChromaticNote, "the pad should choose the palette pitch")
}

// How hard the palette pad was pressed becomes the note's velocity, and the display
// reports that same number rather than a fixed one.
func TestTracePressVelocityBecomesTheNotesVelocity(t *testing.T) {
	bank, voiceBank, recorder := screenBank(t, trackWindowKit(8, 0))
	voice := voiceBank.voices[0]
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	preview := &captureMidiWriter{}
	for _, pressed := range []int{40, 90} {
		require.NoError(t, bank.setStepCursor(0))
		require.NoError(t, bank.handleNoteEditPad(preview, 0, 1, pressed, false))
		event, ok := bank.CurrentPattern().EventAtStep(0, voice)
		require.Truef(t, ok, "the pad press placed no note (velocity %d)", event.Velocity)
		require.Equalf(t, pressed, event.Velocity, "the note should carry the pad press")
		assert.Containsf(t, recorder.row(lengthDisplayRow), fmt.Sprintf("@%03d", pressed),
			"the status should show the press velocity")
	}
	// The audition carries the press velocity too, so what is heard matches what is shown.
	events := preview.events
	require.GreaterOrEqualf(t, len(events), 4, "audition wrote too few messages")
	last := events[len(events)-2:]
	require.Truef(t, midi.IsNoteOn(last[0].Data[0]) && last[0].Data[2] == 90,
		"audition note on = %v, want velocity 90", last[0].Data)
	require.Truef(t, midi.IsNoteOff(last[1].Data[0]) && last[1].Data[2] == 0,
		"audition note off = %v, want velocity 0", last[1].Data)
}

// The two unclaimed buttons reach the state file, and what they did has to be readable on
// the display rather than only in the log: during a set there is nowhere else to look.
func TestTraceStateGesturesReportOnTheReadoutRow(t *testing.T) {
	kit := trackWindowKit(6, 2)
	path := filepath.Join(t.TempDir(), "set.json")
	controller := useStateController(t, path, NewFire(func([]byte) error { return nil }), kit, []string{"kits/gm_drums.json"})
	bank := controller.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)

	rows := make([]string, 0, 5)
	for _, step := range []uiStep{
		press("select row 1", NoteMute1),
		press("browser alone", NoteBrowser),
		press("shift", NoteShift),
		{
			label: "shift+browser: save",
			note:  NoteBrowser,
			vel:   100,
			until: func(r *screenRecorder) bool {
				return strings.HasPrefix(r.row(lengthDisplayRow), "Saved")
			},
		},
		press("shift+accent: load", NoteAccent),
	} {
		require.NoError(t, dispatch(bank, padMessage(step.note, step.vel)))
		if step.until != nil {
			waitFor(t, step.label+" to report", func() bool { return step.until(recorder) })
		}
		readout := recorder.row(lengthDisplayRow)
		t.Logf("%-20s readout %q", step.label, readout)
		rows = append(rows, readout)
	}
	// A press of Browser on its own says nothing and writes nothing. What the readout row
	// already holds is the step status the selection left there, so what the press must not
	// do is change it.
	assert.Equalf(t, rows[0], rows[1], "Browser on its own changed the readout")
	assert.Truef(t, strings.HasPrefix(rows[3], "Saved"), "shift+browser reported %q, want the save", rows[3])
	assert.Truef(t, strings.HasPrefix(rows[4], "Loaded"), "shift+accent reported %q, want the load", rows[4])
	// Shift is left engaged by the trace, so the test hands the next one a clean unit.
	pressButton(t, bank, NoteShift)
}
