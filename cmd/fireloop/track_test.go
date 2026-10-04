package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// trackWindowKit builds a kit whose voices are percussive unless chromatic is set.
func trackWindowKit(count int, chromatic int) *VoiceBank {
	voices := make([]Voice, count)
	for i := range voices {
		voice := Voice{Name: fmt.Sprintf("v%02d", i), Channel: 1}
		if i != chromatic {
			voice.Note = testNote(36 + i)
		}
		voices[i] = voice
	}
	return NewVoiceBank([]Device{{Channel: 1, Voices: voices}})
}

// usePatternGlobals points the event handlers at a bank for the duration of a test and
// restores every global a handler can dirty, so the helpers above do not drift apart.
func usePatternGlobals(t *testing.T, bank *PatternBank) {
	t.Helper()
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	previousProcess, previousClipboard := processEvent, patternClipboard
	previousTapTimes := tapTempoTimes
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
		processEvent, patternClipboard = previousProcess, previousClipboard
		tapTempoTimes = previousTapTimes
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil
	processEvent, patternClipboard = processPatternEvent, nil
	tapTempoTimes = nil
}

func pressPatternButton(t *testing.T, note int) {
	t.Helper()
	if err := processPatternEvent(nil, padMessage(note, 100)); err != nil {
		t.Fatal(err)
	}
}

func assertVisibleTracks(t *testing.T, bank *PatternBank, first int) {
	t.Helper()
	for row := 1; row <= padRows; row++ {
		want := first + row - 1
		if got := bank.trackForPadRow(row); got != want {
			t.Fatalf("pad row %d shows track %d, want %d", row, got, want)
		}
		voice := bank.vb.voices[bank.trackVoices[want-1]]
		if got := bank.trackVoice(row); got != voice {
			t.Fatalf("pad row %d voice = %+v, want %+v", row, got, voice)
		}
	}
}

func TestAltPatternButtonsScrollTracks(t *testing.T) {
	recorder := &ledRecorder{}
	bank := quietBankOn(t, trackWindowKit(8, -1), recorder.write)
	if got := bank.headerText(); got != "Pattern 001  1/4" {
		t.Fatalf("initial header = %q", got)
	}
	assertVisibleTracks(t, bank, 1)

	// Alt lights up to show that the pattern buttons now move the track window.
	pressPatternButton(t, NoteAlt)
	if !altOn || recorder.leds[NoteAlt] != LEDYellow {
		t.Fatalf("alt state = %v/%d, want engaged and lit", altOn, recorder.leds[NoteAlt])
	}
	pressPatternButton(t, NotePatternUp)
	if bank.selPatIdx != 1 || bank.TrackOffset() != 1 {
		t.Fatalf("alt up = pattern %d offset %d, want pattern 1 offset 1", bank.selPatIdx, bank.TrackOffset())
	}
	if got := bank.headerText(); got != "Pattern 001  2/5" {
		t.Fatalf("scrolled header = %q", got)
	}
	assertVisibleTracks(t, bank, 2)
	pressPatternButton(t, NotePatternUp)
	if bank.selPatIdx != 1 || bank.TrackOffset() != 2 {
		t.Fatalf("second alt up = pattern %d offset %d, want pattern 1 offset 2", bank.selPatIdx, bank.TrackOffset())
	}
	if got := bank.headerText(); got != "Pattern 001  3/6" {
		t.Fatalf("scrolled header = %q", got)
	}
	assertVisibleTracks(t, bank, 3)
	pressPatternButton(t, NotePatternDown)
	if bank.TrackOffset() != 1 {
		t.Fatalf("alt down offset = %d, want 1", bank.TrackOffset())
	}

	// Releasing Alt returns the buttons to pattern selection.
	pressPatternButton(t, NoteAlt)
	if altOn || recorder.leds[NoteAlt] != 0 {
		t.Fatalf("alt release = %v/%d, want released and dark", altOn, recorder.leds[NoteAlt])
	}
	pressPatternButton(t, NotePatternUp)
	if bank.selPatIdx != 2 || bank.TrackOffset() != 1 {
		t.Fatalf("plain up = pattern %d offset %d, want pattern 2 keeping its window", bank.selPatIdx, bank.TrackOffset())
	}
}

// A pattern only takes on tracks the user scrolls to, so the count follows the window
// instead of the size of the kit.
func TestTrackWindowGrowsOnDemand(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, -1))
	if bank.TrackCount() != padRows {
		t.Fatalf("starting track count = %d, want the %d pad rows", bank.TrackCount(), padRows)
	}
	for want := 1; want <= 4; want++ {
		if err := bank.ScrollTracks(1); err != nil {
			t.Fatal(err)
		}
		if bank.TrackOffset() != want {
			t.Fatalf("scroll %d = offset %d, want %d", want, bank.TrackOffset(), want)
		}
		if got := bank.TrackCount(); got != padRows+want {
			t.Fatalf("scroll %d = %d tracks, want %d", want, got, padRows+want)
		}
	}
	// The kit caps the growth, and the last track lands on the bottom row.
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.TrackOffset() != 4 || bank.TrackCount() != 8 {
		t.Fatalf("past the kit = offset %d of %d tracks, want offset 4 of 8", bank.TrackOffset(), bank.TrackCount())
	}
	if got := bank.headerText(); got != "Pattern 001  5/8" {
		t.Fatalf("tail header = %q", got)
	}
	assertVisibleTracks(t, bank, 5)
	if err := bank.ScrollTracks(-8); err != nil {
		t.Fatal(err)
	}
	if got := bank.headerText(); got != "Pattern 001  1/8" {
		t.Fatalf("head header = %q", got)
	}
	assertVisibleTracks(t, bank, 1)
}

func TestLargeKitStartsWithPadRows(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(100, -1))
	if bank.TrackCount() != padRows {
		t.Fatalf("large kit opened %d tracks, want %d", bank.TrackCount(), padRows)
	}
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.TrackCount() != padRows+1 {
		t.Fatalf("one scroll = %d tracks, want %d", bank.TrackCount(), padRows+1)
	}
	if err := bank.ScrollTracks(-1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ScrollTracks(-1); err != nil {
		t.Fatal(err)
	}
	if bank.TrackOffset() != 0 {
		t.Fatalf("scrolled above the first track to %d", bank.TrackOffset())
	}
}

func TestTrackWindowStopsOnRealTracks(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(10, -1))
	if err := bank.ScrollTracks(50); err != nil {
		t.Fatal(err)
	}
	// A jump past the end grows tracks no further than the kit, and the last track has
	// to land on the bottom row, otherwise the pads would show empty rows past the end.
	if bank.TrackCount() != 10 {
		t.Fatalf("track count = %d, want the kit's 10 voices", bank.TrackCount())
	}
	if bank.TrackOffset() != 6 {
		t.Fatalf("max offset = %d, want 6", bank.TrackOffset())
	}
	if got := bank.headerText(); got != "Pattern 001  7/10" {
		t.Fatalf("tail header = %q", got)
	}
	assertVisibleTracks(t, bank, 7)
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.TrackOffset() != 6 {
		t.Fatalf("scrolling past the last track moved to %d", bank.TrackOffset())
	}
}

// A wide kit must still fit the header, and every character in it needs a font glyph.
func TestTrackWindowHeaderStaysReadable(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(100, -1))
	if err := bank.ScrollTracks(96); err != nil {
		t.Fatal(err)
	}
	header := bank.headerText()
	if header != "Pattern 001 97/100" {
		t.Fatalf("wide header = %q", header)
	}
	for _, ch := range header {
		if ch == ' ' {
			continue
		}
		blank := true
		for _, column := range byte2glyph(byte(ch)) {
			if column != 0 {
				blank = false
				break
			}
		}
		if blank {
			t.Fatalf("font renders %q blank in header %q", ch, header)
		}
	}
}

// The playback worker draws the visible tracks from its own goroutine while the event
// loop scrolls the window and blacks the display out, so both sides have to be guarded.
// Run under -race to catch a regression.
func TestTrackWindowIsSafeDuringPlaybackDraws(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	pattern := bank.CurrentPattern()
	for i, voice := range voiceBank.voices {
		pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(i), Velocity: 100})
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for col := 0; col < maxPatternSteps; col++ {
			if err := bank.drawPadColumn(col); err != nil {
				return
			}
			if err := bank.drawPadColumnInvert(col); err != nil {
				return
			}
			if err := fire.SetLed(NoteAlt, LEDYellow); err != nil {
				return
			}
		}
	}()
	for i := 0; i < 200; i++ {
		if err := bank.ScrollTracks(1); err != nil {
			t.Error(err)
		}
		if err := bank.ScrollTracks(-1); err != nil {
			t.Error(err)
		}
		if err := fire.Blackout(); err != nil {
			t.Error(err)
		}
		// Waking up is a read-only redraw, so it must not touch the selected pattern
		// index the worker reads while a pattern plays.
		if fire.Wake() {
			if err := bank.redraw(); err != nil {
				t.Error(err)
			}
		}
	}
	wg.Wait()
	if bank.TrackOffset() != 0 {
		t.Fatalf("offset = %d after an even number of scrolls, want 0", bank.TrackOffset())
	}
}

// Every writer wraps or clamps the voice index, so a track never points outside the
// kit. The track lookup relies on that rather than re-checking each time.
func TestTrackVoicesStayInsideTheKit(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(6, -1))
	assertTrackVoicesInKit(t, bank)
	for row := 1; row <= padRows; row++ {
		if err := bank.SelectTrackRow(row); err != nil {
			t.Fatal(err)
		}
		// Jog past both ends of the kit and back, several times over.
		for i := 0; i < 2*len(voiceBank.voices); i++ {
			for _, delta := range []int{1, -1} {
				if err := bank.JogSelect(delta); err != nil {
					t.Fatal(err)
				}
			}
			assertTrackVoicesInKit(t, bank)
		}
	}
	// Growing the window to the kit's size keeps every track on a real voice.
	if err := bank.ScrollTracks(len(voiceBank.voices)); err != nil {
		t.Fatal(err)
	}
	if bank.TrackCount() != len(voiceBank.voices) {
		t.Fatalf("track count = %d, want the kit's %d voices", bank.TrackCount(), len(voiceBank.voices))
	}
	assertTrackVoicesInKit(t, bank)
}

func assertTrackVoicesInKit(t *testing.T, bank *PatternBank) {
	t.Helper()
	bank.trackMu.RLock()
	defer bank.trackMu.RUnlock()
	for i, voice := range bank.trackVoices {
		if voice < 0 || voice >= len(bank.vb.voices) {
			t.Fatalf("track %d points at voice %d, outside the kit's %d voices",
				i+1, voice, len(bank.vb.voices))
		}
	}
}

func TestSmallKitFillsEveryRow(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(2, -1))
	if bank.TrackCount() != padRows {
		t.Fatalf("track count = %d, want %d", bank.TrackCount(), padRows)
	}
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.TrackOffset() != 0 {
		t.Fatalf("small kit scrolled to offset %d", bank.TrackOffset())
	}
	assertVisibleTracks(t, bank, 1)
}

func TestTrackWindowEditsVisibleTracks(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(8, 6))
	if err := bank.ScrollTracks(2); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	// Row 1 now edits track 3, so the encoder and pad presses follow the window.
	if got := bank.SelectedVoice(); got != voiceBank.voices[2] {
		t.Fatalf("selected voice = %s, want track 3", got.Name)
	}
	if err := bank.JogSelect(1); err != nil {
		t.Fatal(err)
	}
	if got := bank.SelectedVoice(); got != voiceBank.voices[3] {
		t.Fatalf("jogged voice = %s, want track 4", got.Name)
	}
	if _, err := bank.ToggleEvent(0, 4, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[3]); !ok {
		t.Fatal("pad press did not reach the visible track")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[2]); ok {
		t.Fatal("pad press edited a track outside the window")
	}

	// Clearing a row clears the track the row shows, not the row's original track.
	if err := bank.ClearTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[3]); ok {
		t.Fatal("clearing the row left its track in place")
	}
}

func TestTrackWindowChromaticEditingFollowsRow(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(8, 5))
	voice := voiceBank.voices[5]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 90)
	pattern.SetChromaticNote(2, voice, 62, 40)
	if err := bank.ScrollTracks(2); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(4); err != nil {
		t.Fatal(err)
	}
	if got := bank.SelectedVoice(); got != voice {
		t.Fatalf("selected voice = %+v, want the chromatic track in view", got)
	}
	if err := bank.MoveStepCursor(2); err != nil {
		t.Fatal(err)
	}
	if event, ok := pattern.EventAtStep(bank.StepCursor(), voice); !ok || event.Velocity != 40 {
		t.Fatalf("moved-to step = %d/%v, want the visible track's event", event.Velocity, ok)
	}
	// Scrolling away releases the palette so the next window starts in step mode.
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.NoteEditActive() {
		t.Fatal("scrolling kept note-edit mode on the previous track")
	}
}

func TestScrollTracksClosesNoteEditOnScrolledRow(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if !bank.NoteEditActive() {
		t.Fatal("chromatic track did not enter note-edit mode")
	}
	bank.pressPad(0, 3)
	if err := bank.ScrollTracks(1); err != nil {
		t.Fatal(err)
	}
	if bank.pressedPads != 0 {
		t.Fatal("scrolling kept a pad held from the previous window")
	}
}

// stepPaints is how many pad writes the worker must make before a test starts switching
// tracks, so the switch overlaps a running playhead.
const stepPaints = 12

// switchRounds is how many times the test moves the selection while the worker runs.
const switchRounds = 60

// Pressing a solo button selects a track, which is a view change rather than an edit, so it
// must not cut a running pattern short. The sequencer is started for real here: its worker
// is the reader that makes the switch a shared state question, and the race detector is the
// point of the test.
func TestTrackSwitchKeepsPatternPlaying(t *testing.T) {
	var writes atomic.Int32
	fire := NewFire(func([]byte) error {
		writes.Add(1)
		return nil
	})
	kit := trackWindowKit(8, 0)
	bank := NewPatternBank(fire, kit)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[0], 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousBPM := currentBPM()
	setBPM(300)
	t.Cleanup(func() { setBPM(previousBPM) })
	playbackStop = bank.startSequencer(&captureMidiWriter{})
	// Wait for the worker to move the playhead a few steps, so the switches below land on
	// top of it rather than in the first moments of playback.
	waitFor(t, "the playhead to move", func() bool { return writes.Load() > stepPaints })

	before := writes.Load()
	// Keep switching for long enough that the worker is certainly running against it: one
	// pass can finish between two of the worker's steps and prove nothing.
	for i := 0; i < switchRounds; i++ {
		// Alternate two rows: pressing the row that is already selected toggles it off,
		// which is a different behaviour from moving the selection.
		row := i%2 + 2 // rows two and three, neither of which starts selected
		// Spread over several beats, so the switches interleave with the worker's own reads.
		time.Sleep(3 * time.Millisecond)
		if err := processPatternEvent(nil, padMessage(NoteMute1+row-1, 100)); err != nil {
			t.Fatal(err)
		}
		if playbackStop == nil {
			t.Fatalf("selecting track row %d stopped the pattern", row)
		}
		if bank.selTrackRow != row {
			t.Fatalf("selected row = %d, want %d", bank.selTrackRow, row)
		}
	}
	// The worker must have painted while those switches were happening, or the race this
	// test exists to catch was never exercised.
	if writes.Load() <= before {
		t.Fatal("the sequencer worker did not paint while the selection moved")
	}
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
	if playbackStop != nil {
		t.Fatal("stopping left the sequencer armed")
	}
}

// Alt plus a solo button clears the row's notes, which changes what is being played, so
// that one does stop playback. It must not drag the plain selection path down with it.
func TestAltSoloStillStopsPlayback(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().ToggleEvent(Event{Voice: voice, Beat: stepBeat(0), Velocity: 100})
	stopped := 0
	playbackStop = func() error {
		stopped++
		return nil
	}
	altOn = true
	if err := processPatternEvent(nil, padMessage(NoteMute1, 100)); err != nil {
		t.Fatal(err)
	}
	if stopped == 0 {
		t.Fatal("clearing a row left the pattern playing")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); ok {
		t.Fatal("Alt plus the solo button did not clear the row")
	}
	// The same button without Alt is a selection, and it leaves the sequencer alone.
	playbackStop = func() error {
		stopped++
		return nil
	}
	altOn = false
	if err := processPatternEvent(nil, padMessage(NoteMute1, 100)); err != nil {
		t.Fatal(err)
	}
	if stopped != 1 {
		t.Fatalf("selection stopped playback %d times, want only the clear to", stopped)
	}
}
