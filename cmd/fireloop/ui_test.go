package main

import (
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/midi"
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
func useTestBanks(t *testing.T) (*ledRecorder, *PatternBank, *SongBank) {
	t.Helper()
	recorder := &ledRecorder{}
	voices := make([]Voice, 8)
	for i := range voices {
		voices[i] = Voice{Name: fmt.Sprintf("v%02d", i), Note: testNote(36 + i), Channel: 1}
	}
	voiceBank := NewVoiceBank([]Device{{Channel: 1, Voices: voices}})
	patternBank := NewPatternBank(NewFire(recorder.write), voiceBank)
	if err := patternBank.Jump(1); err != nil {
		t.Fatal(err)
	}
	songBank := NewSongBank(patternBank.f, patternBank)
	if err := songBank.Jump(0); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, patternBank)
	songbank = songBank
	return recorder, patternBank, songBank
}

func pressButton(t *testing.T, note int) {
	t.Helper()
	if err := processEvent(nil, padMessage(note, 100)); err != nil {
		t.Fatal(err)
	}
}

// Alt is a mode rather than a momentary modifier, so a clear action must not release it.
func TestAltStaysEngagedAfterClear(t *testing.T) {
	_, patternBank, _ := useTestBanks(t)
	pressButton(t, NoteAlt)
	if !altOn {
		t.Fatal("Alt did not engage")
	}
	patternBank.CurrentPattern().ToggleEvent(Event{Voice: patternBank.vb.voices[0], Beat: 0, Velocity: 100})
	pressButton(t, NoteMute1)
	if !altOn {
		t.Fatal("clearing a track row released Alt")
	}
	if _, ok := patternBank.CurrentPattern().EventAtStep(0, patternBank.vb.voices[0]); ok {
		t.Fatal("clearing a track row left its event in place")
	}
	pressButton(t, NoteStop)
	if !altOn {
		t.Fatal("clearing the pattern released Alt")
	}
	if len(patternBank.CurrentPattern().Events) != 0 {
		t.Fatal("clearing the pattern left events behind")
	}
	// An engaged Alt keeps the pattern buttons on the track window.
	pressButton(t, NotePatternUp)
	if patternBank.selPatIdx != 1 || patternBank.TrackOffset() != 1 {
		t.Fatalf("pattern up with Alt = pattern %d offset %d", patternBank.selPatIdx, patternBank.TrackOffset())
	}
}

// A blackout is a display state: the lights go off, the controls keep their state, and
// the next press brings everything back.
func TestBlackoutHidesAndRestoresTheDisplay(t *testing.T) {
	recorder, patternBank, _ := useTestBanks(t)
	// The blackout is Shift plus Alt, so arm the rest first.
	pressButton(t, NoteMute2)
	pressButton(t, NoteShift)
	pressButton(t, NoteOverview)
	pressButton(t, NoteRecord)
	if !shiftOn || patternBank.selTrackRow != 2 || !patternBank.editingLength || patternClipboard == nil {
		t.Fatalf("state before blackout: shift=%v row=%d length=%v clipboard=%v",
			shiftOn, patternBank.selTrackRow, patternBank.editingLength, patternClipboard != nil)
	}
	pressButton(t, NoteAlt)
	if !patbank.f.IsDark() {
		t.Fatal("Shift plus Alt did not blackout")
	}
	// The controls keep their state, and the display stays dark afterwards.
	if !shiftOn || patternBank.selTrackRow != 2 || !patternBank.editingLength || patternClipboard == nil {
		t.Fatalf("blackout changed the control state: shift=%v row=%d length=%v clipboard=%v",
			shiftOn, patternBank.selTrackRow, patternBank.editingLength, patternClipboard != nil)
	}
	if lit := recorder.lit(); len(lit) != 0 {
		t.Fatalf("blackout left lights on: %v", lit)
	}
	if recorder.clearsDisplay == 0 {
		t.Fatal("blackout did not clear the pads or display")
	}
	// Output stays suppressed while dark, so a redraw cannot undo the blackout.
	recorder.reset()
	if err := patternBank.redraw(); err != nil {
		t.Fatal(err)
	}
	if recorder.clearsDisplay != 0 {
		t.Fatal("a redraw reached the display during a blackout")
	}

	// The next press wakes the display and still does what it says.
	pressButton(t, NoteMute3)
	if patbank.f.IsDark() {
		t.Fatal("the display stayed black after a press")
	}
	if patternBank.selTrackRow != 3 {
		t.Fatalf("selected row = %d, want the waking press to have been handled", patternBank.selTrackRow)
	}
	if recorder.clearsDisplay == 0 {
		t.Fatal("waking up did not redraw the display")
	}
	if recorder.leds[NoteShift] != LEDRed {
		t.Fatalf("Shift light = %d after waking, want the state it was left in", recorder.leds[NoteShift])
	}
	if recorder.leds[NoteOverview] != LEDRed {
		t.Fatalf("Overview light = %d after waking, want length mode still on", recorder.leds[NoteOverview])
	}
	if recorder.leds[NoteRecord] != LEDGreen {
		t.Fatalf("Record light = %d after waking, want the copy still armed", recorder.leds[NoteRecord])
	}
	if recorder.leds[CCMuteLED3] != LEDGreen {
		t.Fatalf("mute light = %d after waking, want row 3 selected", recorder.leds[CCMuteLED3])
	}
	if recorder.leds[CCMuteLED2] != LEDOff {
		t.Fatalf("previous row light = %d after waking, want off", recorder.leds[CCMuteLED2])
	}
}

// The Alt light goes dark for a blackout and comes back to the state Alt is in.
func TestBlackoutRestoresAltLight(t *testing.T) {
	recorder, _, _ := useTestBanks(t)
	pressButton(t, NoteAlt)
	if recorder.leds[NoteAlt] != LEDYellow {
		t.Fatalf("Alt light = %d while engaged, want yellow", recorder.leds[NoteAlt])
	}
	pressButton(t, NoteShift)
	pressButton(t, NoteAlt)
	if !patbank.f.IsDark() {
		t.Fatal("Shift plus Alt did not blackout with Alt engaged")
	}
	if !altOn {
		t.Fatal("blackout released Alt")
	}
	if recorder.leds[NoteAlt] != LEDOff {
		t.Fatalf("Alt light = %d during a blackout, want off", recorder.leds[NoteAlt])
	}
	// Waking up restores the light to the state Alt was left in.
	pressButton(t, NoteGridRight)
	if patbank.f.IsDark() || recorder.leds[NoteAlt] != LEDYellow {
		t.Fatalf("Alt light = %d after waking, want the engaged state restored", recorder.leds[NoteAlt])
	}
	// Releasing Alt, with Shift released first so the press is not a blackout, goes dark.
	pressButton(t, NoteShift)
	pressButton(t, NoteAlt)
	if altOn {
		t.Fatal("Alt did not release")
	}
	if recorder.leds[NoteAlt] != LEDOff {
		t.Fatalf("Alt light = %d after releasing, want off", recorder.leds[NoteAlt])
	}
}

// The modifier buttons belong to pattern mode, so switching modes releases them.
func TestModeSwitchReleasesModifiers(t *testing.T) {
	recorder, _, _ := useTestBanks(t)
	pressButton(t, NoteAlt)
	pressButton(t, NotePatternSong)
	if altOn || shiftOn {
		t.Fatalf("mode switch carried alt=%v shift=%v into song mode", altOn, shiftOn)
	}
	if recorder.leds[NoteAlt] != LEDOff {
		t.Fatalf("Alt light = %d after the switch, want off", recorder.leds[NoteAlt])
	}
	// Alt has no song-mode meaning, so it must not engage there.
	pressButton(t, NoteAlt)
	if altOn {
		t.Fatal("Alt engaged in song mode")
	}
	pressButton(t, NotePatternSong)
	if processEvent == nil {
		t.Fatal("no event handler after returning to pattern mode")
	}
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
			press: func(t *testing.T) { pressPatternButton(t, NoteOverview) },
			want:  func() bool { return bank.editingLength },
		},
		{
			name: "length mode again",
			press: func(t *testing.T) {
				pressPatternButton(t, NoteOverview)
			},
			want: func() bool { return !bank.editingLength },
		},
		{
			name: "note mode on a chromatic track",
			press: func(t *testing.T) {
				if err := bank.SelectTrackRow(4); err != nil {
					t.Fatal(err)
				}
				pressPatternButton(t, NoteMode)
			},
			want: func() bool { return bank.editingNote },
		},
		{
			name: "note mode again",
			press: func(t *testing.T) { pressPatternButton(t, NoteMode) },
			want:  func() bool { return !bank.editingNote },
		},
		{
			name:  "alt",
			press: func(t *testing.T) { pressPatternButton(t, NoteAlt) },
			want:  func() bool { return altOn },
		},
		{
			name:  "alt released",
			press: func(t *testing.T) { pressPatternButton(t, NoteAlt) },
			want:  func() bool { return !altOn },
		},
		{
			name:  "shift",
			press: func(t *testing.T) { pressPatternButton(t, NoteShift) },
			want:  func() bool { return shiftOn },
		},
		{
			name:  "shift released",
			press: func(t *testing.T) { pressPatternButton(t, NoteShift) },
			want:  func() bool { return !shiftOn },
		},
		{
			name:  "a pad edit",
			press: func(t *testing.T) { pressPatternButton(t, NoteMute1) },
			want:  func() bool { return bank.selTrackRow == 1 },
		},
		{
			name:  "the tempo readout",
			press: func(t *testing.T) { pressPatternButton(t, NoteTap) },
			want:  func() bool { return currentBPM() != 0 },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.press(t)
			if !tt.want() {
				t.Fatalf("%s did not take effect", tt.name)
			}
		})
	}
}
