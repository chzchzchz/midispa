package main

import (
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

// ledRecorder captures the three-byte control changes that drive the Fire's buttons and
// counts the bulk messages used to clear lights and draw the display.
type ledRecorder struct {
	leds          map[int]int
	clearsDisplay int
}

func (r *ledRecorder) write(data []byte) error {
	if len(data) == 3 && midi.IsCC(data[0]) {
		if r.leds == nil {
			r.leds = make(map[int]int)
		}
		r.leds[int(data[1])] = int(data[2])
		return nil
	}
	// Pad lights and screen updates are the only bulk messages the Fire receives.
	r.clearsDisplay++
	return nil
}

// ledOffValue is the value that puts a control out. Most take zero, but the top-left
// cluster selects a button at zero and is blanked by CCTopLeftOff instead, so treating
// zero as off would report Channel as lit.
func ledOffValue(control int) int {
	if control == CCTopLeftLEDs {
		return CCTopLeftOff
	}
	return LEDOff
}

func (r *ledRecorder) lit() []int {
	var out []int
	for control, value := range r.leds {
		if value != ledOffValue(control) {
			out = append(out, control)
		}
	}
	return out
}

func (r *ledRecorder) reset() {
	r.leds = nil
	r.clearsDisplay = 0
}

// useTestBanks points the event handlers at a fresh pattern bank and song bank.
func useTestBanks(t *testing.T) (*ledRecorder, *PatternBank, *SongBank, *Fire) {
	t.Helper()
	recorder := &ledRecorder{}
	voices := make([]Voice, 8)
	for i := range voices {
		voices[i] = Voice{Name: fmt.Sprintf("v%02d", i), Note: testNote(36 + i), Channel: 1}
	}
	voiceBank := NewVoiceBank([]Device{{Channel: 1, Voices: voices}})
	fire := NewFire(recorder.write)
	controller := useController(t, fire, voiceBank)
	require.NoError(t, controller.songbank.Jump(0))
	// The unit comes back with the banks because a blackout is the unit's own state, not
	// something a bank holds, so a test that wants to know asks the thing that has it.
	return recorder, controller.patbank, controller.songbank, fire
}

// pressButton presses a control through the bank under test, which is the path a Fire
// event takes, so the test exercises the mode choice rather than naming a handler.
func pressButton(t *testing.T, bank *PatternBank, note int) {
	t.Helper()
	require.NoError(t, dispatch(bank, padMessage(note, 100)))
}

// Alt is a mode rather than a momentary modifier, so a clear action must not release it.
func TestAltStaysEngagedAfterClear(t *testing.T) {
	_, patternBank, _, _ := useTestBanks(t)
	pressButton(t, patternBank, NoteAlt)
	require.True(t, patternBank.controller.alt, "Alt did not engage")
	patternBank.CurrentPattern().ToggleEvent(Event{Voice: patternBank.vb.voices[0], Beat: 0, Velocity: 100})
	pressButton(t, patternBank, NoteMute1)
	require.True(t, patternBank.controller.alt, "clearing a track row released Alt")
	_, still := patternBank.CurrentPattern().EventAtStep(0, patternBank.vb.voices[0])
	require.False(t, still, "clearing a track row left its event in place")
	pressButton(t, patternBank, NoteStop)
	require.True(t, patternBank.controller.alt, "clearing the pattern released Alt")
	cleared, _ := patternBank.CurrentPattern().snapshot()
	require.Empty(t, cleared, "clearing the pattern left events behind")
	// An engaged Alt keeps the pattern buttons on the track window.
	pressButton(t, patternBank, NotePatternUp)
	require.Equal(t, 1, patternBank.selPatIdx, "pattern up with Alt")
	require.Equal(t, 1, patternBank.TrackOffset())
}

// A blackout is a display state: the lights go off, the controls keep their state, and
// the next press brings everything back.
func TestBlackoutHidesAndRestoresTheDisplay(t *testing.T) {
	recorder, patternBank, _, fire := useTestBanks(t)
	// The blackout is Shift plus Alt, so arm the rest first.
	pressButton(t, patternBank, NoteMute2)
	pressButton(t, patternBank, NoteShift)
	pressButton(t, patternBank, NoteOverview)
	pressButton(t, patternBank, NoteRecord)
	require.True(t, patternBank.controller.shift)
	require.Equal(t, 2, patternBank.selTrackRow)
	require.True(t, patternBank.lengthEditActive())
	require.NotNil(t, patternBank.controller.clipboard)
	pressButton(t, patternBank, NoteAlt)
	require.True(t, fire.IsDark(), "Shift plus Alt did not blackout")
	// The controls keep their state, and the display stays dark afterwards.
	require.True(t, patternBank.controller.shift, "the blackout changed the control state")
	require.Equal(t, 2, patternBank.selTrackRow, "the blackout changed the control state")
	require.True(t, patternBank.lengthEditActive(), "the blackout changed the control state")
	require.NotNil(t, patternBank.controller.clipboard, "the blackout changed the control state")
	require.Empty(t, recorder.lit(), "the blackout left lights on")
	require.NotZero(t, recorder.clearsDisplay, "the blackout did not clear the pads or display")
	// Output stays suppressed while dark, so a redraw cannot undo the blackout.
	recorder.reset()
	require.NoError(t, patternBank.redraw())
	require.Zero(t, recorder.clearsDisplay, "a redraw reached the display during a blackout")

	// The next press wakes the display and still does what it says.
	pressButton(t, patternBank, NoteMute3)
	require.False(t, fire.IsDark(), "the display stayed black after a press")
	require.Equal(t, 3, patternBank.selTrackRow, "the waking press was not handled")
	require.NotZero(t, recorder.clearsDisplay, "waking up did not redraw the display")
	require.Equal(t, LEDRed, recorder.leds[NoteShift], "Shift keeps the state it was left in")
	require.Equal(t, LEDRed, recorder.leds[NoteOverview], "length mode is still on")
	require.Equal(t, LEDGreen, recorder.leds[NoteRecord], "the copy is still armed")
	require.Equal(t, LEDGreen, recorder.leds[CCMuteLED3], "row 3 is selected")
	require.Equal(t, LEDOff, recorder.leds[CCMuteLED2], "the previous row light goes off")
}

// The Alt light goes dark for a blackout and comes back to the state Alt is in.
func TestBlackoutRestoresAltLight(t *testing.T) {
	recorder, patternBank, _, fire := useTestBanks(t)
	pressButton(t, patternBank, NoteAlt)
	require.Equal(t, LEDYellow, recorder.leds[NoteAlt], "Alt light while engaged")
	pressButton(t, patternBank, NoteShift)
	pressButton(t, patternBank, NoteAlt)
	require.True(t, fire.IsDark(), "Shift plus Alt did not blackout with Alt engaged")
	require.True(t, patternBank.controller.alt, "the blackout released Alt")
	require.Equal(t, LEDOff, recorder.leds[NoteAlt], "Alt light during a blackout")
	// Waking up restores the light to the state Alt was left in.
	pressButton(t, patternBank, NoteGridRight)
	require.False(t, fire.IsDark(), "the display stayed dark")
	require.Equal(t, LEDYellow, recorder.leds[NoteAlt], "the engaged state was not restored")
	// Releasing Alt, with Shift released first so the press is not a blackout, goes dark.
	pressButton(t, patternBank, NoteShift)
	pressButton(t, patternBank, NoteAlt)
	require.False(t, patternBank.controller.alt, "Alt did not release")
	require.Equal(t, LEDOff, recorder.leds[NoteAlt], "Alt light after releasing")
}

// The modifier buttons belong to pattern mode, so switching modes releases them.
func TestModeSwitchReleasesModifiers(t *testing.T) {
	recorder, patternBank, _, _ := useTestBanks(t)
	pressButton(t, patternBank, NoteAlt)
	pressButton(t, patternBank, NotePatternSong)
	require.False(t, patternBank.controller.alt, "the mode switch carried Alt into song mode")
	require.False(t, patternBank.controller.shift, "the mode switch carried Shift into song mode")
	require.Equal(t, LEDOff, recorder.leds[NoteAlt], "Alt light after the switch")
	// Alt has no song-mode meaning, so it must not engage there.
	pressButton(t, patternBank, NoteAlt)
	require.False(t, patternBank.controller.alt, "Alt engaged in song mode")
	pressButton(t, patternBank, NotePatternSong)
	require.Equal(t, patternView, patternBank.controller.mode, "the view did not return to pattern mode")
}

// Every view control reports itself through a light or the readout row as well as changing
// state, so each one reaches for the display while it works. A bank whose display goes
// nowhere must still take the state change, because that is the whole of what these tests
// are able to see: a control that only lit a light would pass here having done nothing.
func TestButtonsTakeEffectWithNowhereToDraw(t *testing.T) {
	kit := trackWindowKit(4, 3)
	bank := newTestBank(t, NewFire(func([]byte) error { return nil }), kit)

	for _, tt := range []struct {
		name  string
		press func(t *testing.T)
		want  func() bool
	}{
		{
			name:  "length mode",
			press: func(t *testing.T) { pressButton(t, bank, NoteOverview) },
			want:  func() bool { return bank.lengthEditActive() },
		},
		{
			name: "length mode again",
			press: func(t *testing.T) {
				pressButton(t, bank, NoteOverview)
			},
			want: func() bool { return !bank.lengthEditActive() },
		},
		{
			name: "note mode on a chromatic track",
			press: func(t *testing.T) {
				require.NoError(t, bank.SelectTrackRow(4))
				pressButton(t, bank, NoteMode)
			},
			want: func() bool { return bank.noteEditActive() },
		},
		{
			name:  "note mode again",
			press: func(t *testing.T) { pressButton(t, bank, NoteMode) },
			want:  func() bool { return !bank.noteEditActive() },
		},
		{
			name:  "alt",
			press: func(t *testing.T) { pressButton(t, bank, NoteAlt) },
			want:  func() bool { return bank.controller.alt },
		},
		{
			name:  "alt released",
			press: func(t *testing.T) { pressButton(t, bank, NoteAlt) },
			want:  func() bool { return !bank.controller.alt },
		},
		{
			name:  "shift",
			press: func(t *testing.T) { pressButton(t, bank, NoteShift) },
			want:  func() bool { return bank.controller.shift },
		},
		{
			name:  "shift released",
			press: func(t *testing.T) { pressButton(t, bank, NoteShift) },
			want:  func() bool { return !bank.controller.shift },
		},
		{
			name:  "a pad edit",
			press: func(t *testing.T) { pressButton(t, bank, NoteMute1) },
			want:  func() bool { return bank.selTrackRow == 1 },
		},
		{
			name:  "the tempo readout",
			press: func(t *testing.T) { pressButton(t, bank, NoteTap) },
			want:  func() bool { return currentBPM() != 0 },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.press(t)
			require.Truef(t, tt.want(), "%s did not take effect", tt.name)
		})
	}
}
