package main

import (
	"os"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shape of a swung beat, in one table: straight is straight, the boundary falls on the
// eighth rather than on parity, and below 50 the halves swap without the total moving. The
// last case is the reason a wait can never come out negative, which would make a timer fire
// at once and spin the worker rather than crash it.
func TestSwungFraction(t *testing.T) {
	for _, c := range []struct {
		name    string
		swing   float64
		step    int
		within  float64
		want    float64
		comment string
	}{
		{"straight first step", straightSwingPct, 1, 0.25, 0.25, "half a beat in quarters"},
		{"straight last step", straightSwingPct, 4, 1.00, 1.00, "a whole beat"},
		{"swung first step", 66, 1, 0.25, 0.33, "a third of the beat"},
		{"swung on the eighth", 66, 2, 0.50, 0.66, "the long half has run out"},
		{"swung whole beat", 66, 4, 1.00, 1.00, "the beat still completes"},
		{"reverse on the eighth", 40, 2, 0.50, 0.40, "the halves have swapped"},
		{"reverse whole beat", 40, 4, 1.00, 1.00, "and still completes"},
		{"floor whole beat", minSwingPct, 4, 1.00, 1.00, "at the bound"},
		{"ceiling whole beat", maxSwingPct, 4, 1.00, 1.00, "at the other bound"},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.InDeltaf(t, c.want, swungFraction(c.within, c.swing), 1e-9,
				"step %d of a beat swung %v: %s", c.step, c.swing, c.comment)
		})
	}
}

// Never going backwards is its own assertion rather than part of the table, because it is a
// property of every position rather than of any one of them.
func TestSwungFractionNeverGoesBackwards(t *testing.T) {
	for _, swing := range []float64{minSwingPct, 40, straightSwingPct, 66, maxSwingPct} {
		previous := swungFraction(0, swing)
		for step := 1; step <= 8*patternStepsPerBeat; step++ {
			within := float64(step) / patternStepsPerBeat
			current := swungFraction(within, swing)
			assert.GreaterOrEqualf(t, current, previous,
				"the clock went backwards at step %v at swing %v", step, swing)
			previous = current
		}
	}
}

// The invariant that makes swing safe: a whole number of beats takes exactly as long
// however far the swing is pushed. This is the test that says the tempo does not drift, and
// it is the one that must never be relaxed.
//
// The beats are exact quarters rather than accumulated sums because swungBeatTime floors,
// and a float32 3.9999 would come back a hair under 4 and fail on float error rather than
// on a defect. The tolerance is there for the duration arithmetic underneath, not to paper
// over a beat that is not where it should be.
func TestSwungBeatTimeHoldsWholeBeats(t *testing.T) {
	beatDur := 500 * time.Millisecond
	for _, swing := range []float64{minSwingPct, 40, straightSwingPct, 66, maxSwingPct} {
		for _, beats := range []float32{1, 2, 4, 16} {
			want := float64(beats) * float64(beatDur)
			assert.InDeltaf(t, want, swungBeatTime(beats, beatDur, swing), float64(time.Microsecond),
				"swungBeatTime(%v, %v, %v) drifted", beats, beatDur, swing)
		}
	}
}

// beatDuration returns zero for a tempo that is not positive rather than dividing by it, and
// a span of zero for every beat would have the worker spinning instead of playing.
func TestSwungSpanIsSafeAtANonPositiveTempo(t *testing.T) {
	assert.Zero(t, swungSpan(0, 0.25, 0, 66), "no beat duration means no span")
	assert.Zero(t, swungSpan(0, 0.25, -time.Second, 66), "a negative beat duration means no span")
}

// The swing store and its formatting, in one table: a tenth survives, an untouched swing is
// straight, the clamp bounds what the controls can produce, and both readouts go through one
// formatter so they cannot disagree about a value they are showing at the same time.
func TestSwingStoreAndLabels(t *testing.T) {
	// Before anything sets it: the atomic's Go zero is zero counts, which is 0% and not
	// straight, so this is what the seeding init is for.
	require.Equal(t, straightSwingPct, currentSwingPct(), "an untouched swing should be straight")

	setSwing(t, 66.7)
	assert.InDelta(t, 66.7, currentSwingPct(), 1e-9, "a tenth should survive the store")

	assert.Equal(t, maxSwingPct, clampSwingPct(200), "above the ceiling")
	assert.Equal(t, minSwingPct, clampSwingPct(-5), "below the floor")
	assert.InDelta(t, 66.7, clampSwingPct(66.7), 1e-9, "a value inside the range is its own")

	assert.Equal(t, "66", swingLabel(66), "a whole percent drops its tenths")
	assert.Equal(t, "66.7", swingLabel(66.7))
	assert.Empty(t, swingText(straightSwingPct), "straight leaves the separator bare")
	assert.Equal(t, " Sw 66", swingText(66))
	assert.Equal(t, " Sw 66.7", swingText(66.7))
}

// The file's two halves have to agree, because one of them is what makes the key absent, and
// the other is what turns a hand-edited number back into a value the controls could produce.
func TestSwingFileRoundTrip(t *testing.T) {
	assert.Zero(t, swingPctFor(straightSwingPct), "straight is written as no key at all")
	assert.InDelta(t, 66.7, swingPctFor(66.7), 1e-9, "a real value is written as itself")
	assert.Equal(t, straightSwingPct, swingPctFrom(0), "a missing key means straight")
	assert.InDelta(t, 66.7, swingPctFrom(66.7), 1e-9, "a real value survives")
	assert.Equal(t, maxSwingPct, swingPctFrom(500), "a hand-edited file is clamped")
}

// midiclock's mapping, so the same knob means the same thing in both programs: 64 is its
// centre and 127 its top.
func TestCCToSwingPctMapping(t *testing.T) {
	for _, cc := range []int{0, 1, 32, 63, 64, 65, 96, 127} {
		got := ccSwingToSwingPct(cc)
		assert.GreaterOrEqualf(t, got, minSwingPct, "cc %d below the floor", cc)
		assert.LessOrEqualf(t, got, maxSwingPct, "cc %d above the ceiling", cc)
	}
	assert.InDelta(t, straightSwingPct, ccSwingToSwingPct(64), 1e-9, "cc 64 should be straight")
	assert.InDelta(t, 99.21875, ccSwingToSwingPct(127), 1e-9, "cc 127 should be near the ceiling")
	assert.Equal(t, minSwingPct, ccSwingToSwingPct(0), "cc 0 lands on the floor")
}

// The tempo is one 14-bit value in two halves rather than a coarse and fine pair: 120 BPM
// is 7680 counts, so its MSB is 60 and its LSB is 0.
func TestCoarseAndFineAreOneFourteenBitTempo(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)

	require.NoError(t, sendCC(bank, softwareChannel, CcBpmMsb, 0x3C))
	require.NoError(t, sendCC(bank, softwareChannel, CcBpmLsb, 0x00))
	assert.Equal(t, 120, currentBPM(), "the pair should give 120")

	// A fine message with no coarse one keeps the coarse half standing rather than
	// throwing the tempo away.
	require.NoError(t, sendCC(bank, softwareChannel, CcBpmLsb, 0x40))
	assert.Equal(t, 121, currentBPM(), "the fine half alone should move the tempo by one")
}

// Only a floor is written, because the fourteen-bit format is its own ceiling. This is the
// test that says so: the largest raw two halves can carry is below the slowest-bound's mirror
// image, so the clamp has nothing to do above and everything to do below.
func TestClampBPMRawOnlyHasAFloorToFire(t *testing.T) {
	floor := int64(stateTempoMin * bpmFixed)
	assert.Equal(t, floor, clampBPMRaw(0), "a zero coarse send lands on the floor")
	assert.Equal(t, floor, clampBPMRaw(1), "one count up is still under the slowest beat")
	assert.Equal(t, int64(120*bpmFixed), clampBPMRaw(120*bpmFixed), "a legal tempo is its own clamp")

	// The largest raw the format can carry, and the tempo it comes to. Both are inside the
	// range, which is why nothing above the floor needs a bound.
	const widest = 0x3fff
	assert.Equal(t, int64(widest), clampBPMRaw(widest), "the format's own maximum is not clamped")
	assert.Less(t, float64(widest)/bpmFixed, float64(stateTempoMax),
		"the format's ceiling is under the tempo ceiling, so an upper bound could never fire")
}

// The two sets of controls never cross. A Cc number on the device's channel does nothing,
// and a CC number off it does nothing.
func TestDeviceAndSoftwareControlsNeverCross(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)
	require.Equal(t, defaultBPM, currentBPM(), "the default tempo")

	// On the device's channel the software's numbers are ignored, which is what stops the
	// Volume knob from setting the tempo and the pan knob from setting the swing.
	require.NoError(t, sendCC(bank, deviceChannel, CcBpmMsb, 0x3C))
	require.NoError(t, sendCC(bank, deviceChannel, CcBpmLsb, 0x00))
	assert.Equal(t, defaultBPM, currentBPM(), "the software's numbers must be ignored on the device's channel")
	assert.Empty(t, screen.row(readoutRow), "an ignored control should say nothing")

	// Off it they are honoured.
	require.NoError(t, sendCC(bank, softwareChannel, CcBpmMsb, 0x3C))
	require.NoError(t, sendCC(bank, softwareChannel, CcBpmLsb, 0x00))
	assert.Equal(t, 120, currentBPM(), "the pair should set the tempo off the device's channel")

	before := bank.selTrackRow
	require.NoError(t, sendCC(bank, softwareChannel, CCSelect, EncoderRight))
	assert.Equal(t, before, bank.selTrackRow, "the device's numbers must be ignored off its channel")

	assert.NotEmpty(t, screen.row(readoutRow), "a honoured control change should say what it did")
}

// Note 48 is Shift and CC 48 is the tempo's fine half. One switch on the second byte could
// not tell them apart, which is why control changes are dispatched on the status byte.
func TestNoteAndControlChangeShareANumber(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	setBPM(120)

	pressButton(t, bank, NoteShift)
	require.True(t, bank.controller.shift, "note 48 should toggle Shift")
	pressButton(t, bank, NoteShift)
	require.False(t, bank.controller.shift, "note 48 should release Shift")

	require.NoError(t, sendCC(bank, softwareChannel, CcBpmLsb, 0x40))
	require.False(t, bank.controller.shift, "cc 48 must not toggle Shift")
	assert.Equal(t, 121, currentBPM(), "cc 48 alone moved the tempo rather than toggling Shift")
}

// A controller sets the swing in song mode as well, which is where a groove is usually set
// against something, and the Fire's own knobs still do nothing there.
func TestSoftwareControlsWorkInSongMode(t *testing.T) {
	_, bank, songs, _ := useTestBanks(t)
	recorder := useScreenRecorder(t, &songs.screen)
	bank.controller.mode = songView

	require.NoError(t, sendCC(bank, softwareChannel, CcSwing, 86))
	assert.InDelta(t, 67.2, currentSwingPct(), 1e-9, "cc 86 should be a swung 67.2")
	assert.Contains(t, recorder.row(2), "Sw 67.2", "the song view should show it")

	before := bank.selTrackRow
	require.NoError(t, sendCC(bank, deviceChannel, CCSelect, EncoderRight))
	assert.Equal(t, before, bank.selTrackRow, "the device's knobs do nothing in song mode")
}

// A swing control that moved something invisible would look like a broken knob, so both
// numbers say what they did on the row the player is looking at.
func TestSwingControlSaysWhatItDid(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)

	require.NoError(t, sendCC(bank, softwareChannel, CcSwing, 86))
	assert.Contains(t, screen.row(readoutRow), "67.2", "the swing change should be reported")
	require.NoError(t, sendCC(bank, softwareChannel, CcBpmMsb, 0x3C))
	assert.Contains(t, screen.row(readoutRow), "Tempo", "the tempo change should be reported")

	// Both swing numbers reach the same mapping.
	setSwingPct(straightSwingPct)
	require.NoError(t, sendCC(bank, softwareChannel, CcSwingAlt, 86))
	assert.InDelta(t, 67.2, currentSwingPct(), 1e-9, "the second swing number should reach it too")
}

// Snap enters swing entry, the encoder moves it, and the pads type it. The mode takes the
// readout row and the pads from everything else, and leaves playback running.
func TestSwingEntryOwnsTheRowAndThePads(t *testing.T) {
	leds, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)
	require.NoError(t, bank.SelectTrackRow(1), "the setup needs a track to report on")

	pressButton(t, bank, NoteSnap)
	require.True(t, bank.swingEditActive(), "Snap did not enter swing entry")
	assert.Equal(t, LEDRed, leds.leds[NoteSnap], "the Snap light should report the mode")
	assert.Equal(t, "Swing: 50", screen.row(readoutRow), "the readout row while the mode owns it")

	// The encoder moves it a percent a detent.
	pressEncoder(t, bank, EncoderRight)
	pressEncoder(t, bank, EncoderRight)
	assert.InDelta(t, 52.0, currentSwingPct(), 1e-9, "two detents")
	assert.Equal(t, "Swing: 52", screen.row(readoutRow), "the readout should follow")

	// The pads type it outright, with no Shift.
	pressPad(t, bank, padTyping(6))
	pressPad(t, bank, padTyping(6))
	assert.Equal(t, "Swing: 66", screen.row(readoutRow), "the keypad should fill in the number")

	// Leaving applies the number the player typed.
	pressButton(t, bank, NoteSnap)
	require.False(t, bank.swingEditActive(), "Snap did not leave swing entry")
	assert.InDelta(t, 66.0, currentSwingPct(), 1e-9, "leaving should apply what was typed")
	assert.Equal(t, LEDOff, leds.leds[NoteSnap], "the Snap light should go out")
	assert.Contains(t, screen.row(readoutRow), "S", "the step status should be back")

	// A pad inside swing entry is a digit, not a note. Without the keypad's own early return
	// above ToggleEvent a stray hit would write one into the pattern. This is its own entry
	// so that the digit does not land in the number typed above.
	pressButton(t, bank, NoteSnap)
	pressPad(t, bank, padTyping(4))
	assert.Empty(t, bank.CurrentPattern().EventsForVoice(bank.SelectedVoice()),
		"the keypad wrote a note into the pattern")
	pressButton(t, bank, NoteSnap)
}

// One stray digit is a mis-hit rather than a groove, so it is discarded rather than turned
// into a tenth of a percent.
func TestSwingEntryDiscardsASingleDigit(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)
	setSwing(t, 66)

	pressButton(t, bank, NoteSnap)
	pressPad(t, bank, padTyping(9))
	assert.Equal(t, "Swing: 09", screen.row(readoutRow), "the readout should show the digit")

	pressButton(t, bank, NoteSnap)
	assert.InDelta(t, 66.0, currentSwingPct(), 1e-9, "a single digit should be discarded")
}

// A tempo entry the player abandoned must not be applied as a swing on the way out.
//
// The entry is made through the production call rather than through a chord, because
// Shift held across a Snap press is the groove toggle now: the keys that used to spell
// "abandon the tempo and open swing entry" spell "turn the groove off" instead. The rule
// under test is the one that matters — opening swing entry must not inherit digits — and
// the chord that reached it is a different gesture now.
func TestSwingEntryClearsAnAbandonedTempoEntry(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)

	pressButton(t, bank, NoteShift)
	pressPad(t, bank, padTyping(6))
	require.Equal(t, 6, bank.controller.pending, "the tempo entry should have digits")

	require.NoError(t, bank.ToggleSwingEntry())
	require.Zero(t, bank.controller.pending, "entering swing entry should drop the digits")

	require.NoError(t, bank.ToggleSwingEntry())
	assert.InDelta(t, straightSwingPct, currentSwingPct(), 1e-9,
		"the abandoned tempo entry was applied as a swing")
}

// Shift with Snap is the groove's off and on: straight, or the last swing that was not
// straight. It exists because the only other route back to straight was entering the entry
// mode and typing 50, which is four actions for the value the program starts on.
func TestShiftSnapTogglesTheGrooveOffAndBack(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)
	setSwing(t, 66)

	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteSnap)
	assert.InDelta(t, straightSwingPct, currentSwingPct(), 1e-9, "Shift and Snap should go straight")
	assert.NotContains(t, screen.row(separatorDisplayRow), "Sw", "the separator should lose its label at straight")

	// Shift latches, so the second press is the same gesture: the toggle has to work
	// without reaching for the modifier again.
	pressButton(t, bank, NoteSnap)
	assert.InDelta(t, 66.0, currentSwingPct(), 1e-9, "the second press should put the groove back")
	assert.Contains(t, screen.row(separatorDisplayRow), "Sw 66", "the separator should carry the groove again")
}

// A session that has never been swung has no groove to put back, so the toggle from
// straight has to do nothing rather than invent one.
func TestSwingToggleWithNoGrooveChangesNothing(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	lastGroove.Store(0)
	setSwing(t, straightSwingPct)

	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteSnap)
	assert.InDelta(t, straightSwingPct, currentSwingPct(), 1e-9,
		"the toggle invented a groove that was never set")
}

// Every route to a swing has to leave the toggle with something to restore, or the toggle
// silently stops working after the groove was set by a controller rather than by hand.
func TestEveryRouteToAGrooveIsRemembered(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)

	for name, set := range map[string]func(){
		"the encoder": func() {
			setSwing(t, straightSwingPct)
			require.NoError(t, bank.ToggleSwingEntry())
			pressEncoder(t, bank, EncoderRight)
			pressEncoder(t, bank, EncoderRight)
		},
		"a controller": func() {
			setSwing(t, straightSwingPct)
			require.NoError(t, sendCC(bank, softwareChannel, CcSwing, 86))
		},
		"a session file": func() {
			setSwing(t, straightSwingPct)
			setSwingPct(swingPctFrom(72))
		},
	} {
		t.Run(name, func(t *testing.T) {
			set()
			groove := currentSwingPct()
			require.NotEqual(t, straightSwingPct, groove, "the setup should have swung the session")

			require.NoError(t, bank.ToggleSwingOnOff())
			assert.InDelta(t, straightSwingPct, currentSwingPct(), 1e-9, "the toggle should go straight")
			require.NoError(t, bank.ToggleSwingOnOff())
			assert.InDelta(t, groove, currentSwingPct(), 1e-9, "the toggle should put the same groove back")
		})
	}
}

// Shift is inert inside swing entry, and inert has to mean "does nothing" rather than
// "discards what the pads typed". A reflex tap while reading the number back used to drop it
// on the floor, and leaving then applied nothing at all.
func TestShiftInsideSwingEntryLeavesTheNumberAlone(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	pressButton(t, bank, NoteSnap)
	pressPad(t, bank, padTyping(6))
	pressPad(t, bank, padTyping(6))
	require.Equal(t, 66, bank.controller.pending, "the keypad should have typed a number")

	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteShift)
	assert.Equal(t, 66, bank.controller.pending, "Shift should not touch a swing being typed")

	pressButton(t, bank, NoteSnap)
	assert.InDelta(t, 66.0, currentSwingPct(), 1e-9, "the number should still be there to apply")
}

// The channel rule names MIDI channels, and the codebase counts them from zero while every
// controller counts them from one. This is the assertion that stops the two being confused:
// the number fireloop calls channel 0 is MIDI channel 1, the one a controller defaults to,
// and the software's own controls have to be aimed above it.
func TestSoftwareControlsAreAboveTheControllerDefault(t *testing.T) {
	require.Equal(t, 0, midi.Channel(midi.MakeCC(fireControlChannel)),
		"the device's channel is MIDI channel 1 in the wire's own numbering")
	require.Equal(t, 1, midi.Channel(midi.MakeCC(fireControlChannel+1)),
		"the software's channel is MIDI channel 2")

	_, bank, _, _ := useTestBanks(t)
	setSwing(t, straightSwingPct)
	// A controller left on its default sends here and is heard as the Fire's own.
	require.NoError(t, sendCC(bank, fireControlChannel, CcSwing, 86))
	assert.InDelta(t, straightSwingPct, currentSwingPct(), 1e-9,
		"MIDI channel 1 belongs to the device, so the software's numbers are ignored")
	// Aimed one above it, it is heard.
	require.NoError(t, sendCC(bank, fireControlChannel+1, CcSwing, 86))
	assert.InDelta(t, 67.2, currentSwingPct(), 1e-9, "MIDI channel 2 is the software's")
}

// A tempo entry that turns out to be out of range is dropped rather than left standing for
// the next Shift gesture to apply.
func TestOutOfRangeTempoEntryIsDropped(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	setBPM(140)

	pressButton(t, bank, NoteShift)
	pressPad(t, bank, padTyping(6))
	require.Equal(t, 6, bank.controller.pending, "the tempo entry should have digits")
	pressButton(t, bank, NoteShift)

	assert.Zero(t, bank.controller.pending, "an out-of-range entry should not survive the release")
	assert.Equal(t, 140, currentBPM(), "an out-of-range entry should not set the tempo")
}

// Row 6 has one owner at a time, and the guard is in printStepStatus rather than in its
// twelve callers, so every gesture that ends at the readout has to leave the row alone.
func TestSwingEntryHoldsTheRowAgainstOtherControls(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)

	pressButton(t, bank, NoteSnap)
	before := screen.row(readoutRow)
	require.NotEmpty(t, before)

	require.NoError(t, bank.JogSelect(1))
	assert.Equal(t, before, screen.row(readoutRow), "jogging the selection took the row")

	require.NoError(t, sendCC(bank, deviceChannel, CCVolume, EncoderRight))
	assert.Equal(t, before, screen.row(readoutRow), "the Volume knob took the row")

	require.NoError(t, bank.MoveStepCursor(1))
	assert.Equal(t, before, screen.row(readoutRow), "the step cursor took the row")

	require.NoError(t, bank.ToggleNoteMode())
	assert.Equal(t, before, screen.row(readoutRow), "note editing took the row")
}

// Entering swing entry over note editing leaves it, the way length mode does, or the
// keypad would take the pads from a palette it cannot hand back.
func TestSwingEntryLeavesNoteEditing(t *testing.T) {
	bank, _ := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	require.True(t, bank.noteEditActive(), "the setup needs note editing on")

	require.NoError(t, bank.ToggleSwingEntry())
	assert.False(t, bank.noteEditActive(), "swing entry should leave note editing")
	assert.True(t, bank.swingEditActive(), "swing entry should be on")
}

// Length mode and swing entry are mutually exclusive, and a pattern jump drops neither.
func TestSwingEntryConflictsAreExclusive(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)

	require.NoError(t, bank.ToggleLengthMode())
	require.True(t, bank.lengthEditActive(), "the setup needs length mode")
	require.NoError(t, bank.ToggleSwingEntry())
	assert.False(t, bank.lengthEditActive(), "swing entry should leave length mode")
	assert.True(t, bank.swingEditActive(), "swing entry should be on")

	require.NoError(t, bank.ToggleLengthMode())
	assert.False(t, bank.swingEditActive(), "length mode should leave swing entry")

	require.NoError(t, bank.ToggleSwingEntry())
	require.NoError(t, bank.Jump(1))
	assert.True(t, bank.swingEditActive(), "a pattern jump should not drop swing entry")
}

// Leaving the pattern view for the arrangement closes swing entry with the other edit
// modes. It is a different path from toggling Snap off, and it is the one that used to
// drop the swing while leaving its number on the readout row.
func TestLeavingThePatternViewClosesSwingEntry(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)

	require.NoError(t, bank.ToggleSwingEntry())
	require.True(t, bank.swingEditActive(), "the setup needs swing entry")
	pressEncoder(t, bank, EncoderRight)

	require.NoError(t, bank.controller.setMode(songView, 0))
	assert.False(t, bank.swingEditActive(), "the arrangement should not inherit swing entry")
	assert.NotContains(t, screen.row(readoutRow), "Swing", "the swing readout outlived the mode")
}

// Entering swing entry must not stop the set: it changes nothing in any pattern, so
// stopping would make the control useless on a running groove.
func TestSwingEntryLeavesPlaybackRunning(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	stopped := 0
	bank.controller.playback = stubSession(&Playback{}, func() error {
		stopped++
		return nil
	})

	pressButton(t, bank, NoteSnap)
	require.True(t, bank.swingEditActive(), "Snap did not enter swing entry")
	assert.Zero(t, stopped, "entering swing entry stopped the set")
}

// printSwing rebuilds the whole row, because there is no partial-row clear and a suffix
// written only while the value was not straight would outlive the value. redraw is the other
// way the label gets drawn, and a pattern jump is the way redraw is reached, so both are here.
func TestSwingSeparatorIsRebuiltAndSurvivesARedraw(t *testing.T) {
	_, bank, _, _ := useTestBanks(t)
	screen := useScreenRecorder(t, &bank.screen)
	setSwing(t, 66)

	require.NoError(t, bank.Jump(2))
	assert.Equal(t, separatorDisplayRowText+" Sw 66", screen.row(separatorDisplayRow),
		"a pattern jump wiped the swing off the display")

	setSwing(t, straightSwingPct)
	require.NoError(t, bank.redraw())
	assert.Equal(t, separatorDisplayRowText, screen.row(separatorDisplayRow),
		"returning to straight should take the label with it")
}

// A non-straight swing survives a save and a load, tenths and all.
func TestSwingSurvivesASessionRoundTrip(t *testing.T) {
	writer := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	setSwing(t, 66.7)
	saveTo(t, path, writer.patbank, writer.songbank, nil)

	setSwing(t, straightSwingPct)
	far := stateTestBank(t, stateKit())
	_, err := loadInto(far, path, nil)
	require.NoError(t, err)
	assert.InDelta(t, 66.7, currentSwingPct(), 1e-9, "the swing did not survive the round trip")
}

// A straight swing is written as no key at all, so a session that has never been swung
// produces the same bytes the previous build wrote.
func TestStraightSwingWritesNoKey(t *testing.T) {
	writer := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	setSwing(t, straightSwingPct)
	saveTo(t, path, writer.patbank, writer.songbank, nil)

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(written), "swingPct", "a straight swing should write no key")
}

// One end-to-end test, and the only one here that is not arithmetic: a real worker on a
// real four-step pattern, with the gaps between captured note-ons compared. It is the only
// thing that catches a wiring mistake the unit tests cannot see, and it is skipped under
// -short because the gaps are real time and the margin is wide on purpose.
func TestSwungPatternWidensTheFirstGap(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test: the gaps are real time and the margin is wide on purpose")
	}

	fire := NewFire(func([]byte) error { return nil })
	kit := trackWindowKit(1, -1)
	controller := useController(t, fire, kit)
	bank := controller.patbank
	voice := kit.voices[0]
	for step := range 4 {
		bank.CurrentPattern().ToggleEvent(Event{Voice: voice, Beat: stepBeat(step), Velocity: 100})
	}

	setBPM(299)
	setSwingPct(70)
	writer := &timedMidiWriter{}
	controller.startPlayback(writer, bank.newPlayback())
	t.Cleanup(func() { _ = controller.stopPlayback() })
	waitFor(t, "four note-ons", func() bool { return len(writer.noteOnTimes()) >= 4 })

	// The swung clock makes two steps of a beat long and two short, so the widest gap has
	// to dwarf the narrowest. Comparing the extremes rather than the first two gaps is what
	// makes the assertion independent of where in the loop the recording started.
	ons := writer.noteOnTimes()
	require.GreaterOrEqual(t, len(ons), 4, "not enough note-ons to compare")
	longest, shortest := ons[1].Sub(ons[0]), ons[1].Sub(ons[0])
	for i := 2; i < len(ons); i++ {
		gap := ons[i].Sub(ons[i-1])
		if gap > longest {
			longest = gap
		}
		if gap < shortest {
			shortest = gap
		}
	}
	assert.Greaterf(t, longest, shortest*2,
		"the long half (%v) should dwarf the short half (%v)", longest, shortest)
}
