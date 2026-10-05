package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// newTestController builds the banks and leaves only the tempo to restore, which is the one
// piece of state still in the package because the playback worker samples it from its own
// goroutine. The banks are built here rather than handed in so a test cannot end up with one
// that has no owner to stop playback through.
func newTestController(t testing.TB, fire *Fire, kit *VoiceBank) *Controller {
	t.Helper()
	previousBPM := bpm.Load()
	t.Cleanup(func() { bpm.Store(previousBPM) })
	setBPM(defaultBPM)
	return newController(fire, kit, "")
}

// useController is the state almost every test starts from: the banks, sitting on pattern 1.
func useController(t testing.TB, fire *Fire, kit *VoiceBank) *Controller {
	t.Helper()
	controller := newTestController(t, fire, kit)
	require.NoErrorf(t, controller.patbank.Jump(1), "sitting on pattern 1")
	return controller
}

// useEmptyController is for the one case that needs a bank which has never been on a
// pattern, which is what makes a readout reachable from there rather than from an edit.
func useEmptyController(t testing.TB, fire *Fire, kit *VoiceBank) *Controller {
	t.Helper()
	return newTestController(t, fire, kit)
}

// useStateController is useController for the save and load gestures, which read a session
// path and a kit to measure it against.
func useStateController(t testing.TB, path string, fire *Fire, kit *VoiceBank, sessionKit []string) *Controller {
	t.Helper()
	controller := useController(t, fire, kit)
	controller.sessionPath = path
	controller.sessionKit = sessionKit
	return controller
}

// dispatch applies one Fire event the way the program does: through the controller that
// owns the bank, which decides the view. A test presses controls this way rather than
// naming a handler, so the mode choice is exercised on the way to the same place.
func dispatch(bank *PatternBank, ev alsa.SeqEvent) error {
	return bank.controller.Handle(nil, ev)
}

// stubSession installs a set with no worker behind it. Nothing is playing, but stopping it
// releases the notes the playback holds and then calls the handle given, which is what a
// worker does on its way out and is how a test hears whether a handler stopped playback
// without running a worker to stop.
func stubSession(playback *Playback, stop playbackStopFunc) *PlaybackSession {
	if stop == nil {
		stop = func() error { return nil }
	}
	session := newPlaybackSession(playback)
	session.stop = func() error {
		if err := stop(); err != nil {
			return err
		}
		return playback.releaseAll(playback.writer)
	}
	close(session.done)
	return session
}

// installedPlayback is the playback behind whatever set the controller has installed, and
// it does not insist that a worker is running: a press of Play with no port behind it
// publishes the painters and starts nothing, which is the state a test wants when it is
// driving the playhead by hand rather than waiting for a worker to move it.
func installedPlayback(t *testing.T, controller *Controller) *Playback {
	t.Helper()
	require.NotNil(t, controller.playback, "nothing is installed")
	return controller.playback.playback
}

func assertVisibleTracks(t *testing.T, bank *PatternBank, first int) {
	t.Helper()
	for row := 1; row <= padRows; row++ {
		want := first + row - 1
		require.Equalf(t, want, bank.trackForPadRow(row), "pad row %d shows the wrong track", row)
		voice := bank.vb.voices[bank.trackVoices[want-1]]
		require.Samef(t, voice, bank.trackVoice(row), "pad row %d holds the wrong voice", row)
	}
}

func TestAltPatternButtonsScrollTracks(t *testing.T) {
	recorder := &ledRecorder{}
	bank := quietBankOn(t, trackWindowKit(8, -1), recorder.write)
	require.Equal(t, "Pattern 001  1/4", bank.headerText(), "initial header")
	assertVisibleTracks(t, bank, 1)

	// Alt lights up to show that the pattern buttons now move the track window.
	pressButton(t, bank, NoteAlt)
	require.True(t, bank.controller.alt, "Alt did not engage")
	require.Equal(t, LEDYellow, recorder.leds[NoteAlt], "the Alt light")

	pressButton(t, bank, NotePatternUp)
	require.Equal(t, 1, bank.selPatIdx, "Alt up moved the pattern selection")
	require.Equal(t, 1, bank.TrackOffset(), "Alt up moved the window")
	require.Equal(t, "Pattern 001  2/5", bank.headerText(), "scrolled header")
	assertVisibleTracks(t, bank, 2)

	pressButton(t, bank, NotePatternUp)
	require.Equal(t, 1, bank.selPatIdx, "a second Alt up moved the pattern selection")
	require.Equal(t, 2, bank.TrackOffset(), "a second Alt up moved the window")
	require.Equal(t, "Pattern 001  3/6", bank.headerText(), "scrolled header")
	assertVisibleTracks(t, bank, 3)

	pressButton(t, bank, NotePatternDown)
	require.Equal(t, 1, bank.TrackOffset(), "Alt down")

	// Releasing Alt returns the buttons to pattern selection.
	pressButton(t, bank, NoteAlt)
	require.False(t, bank.controller.alt, "Alt did not release")
	require.Equal(t, 0, recorder.leds[NoteAlt], "the Alt light is still on")
	pressButton(t, bank, NotePatternUp)
	require.Equal(t, 2, bank.selPatIdx, "a plain press should move the pattern")
	require.Equal(t, 1, bank.TrackOffset(), "a plain press should keep the window")
}

// A pattern only takes on tracks the user scrolls to, so the count follows the window
// instead of the size of the kit.
func TestTrackWindowGrowsOnDemand(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, -1))
	require.Equal(t, padRows, bank.TrackCount(), "starting track count")
	for want := 1; want <= 4; want++ {
		require.NoError(t, bank.ScrollTracks(1))
		require.Equalf(t, want, bank.TrackOffset(), "scroll %d offset", want)
		require.Equalf(t, padRows+want, bank.TrackCount(), "scroll %d track count", want)
	}
	// The kit caps the growth, and the last track lands on the bottom row.
	require.NoError(t, bank.ScrollTracks(1))
	require.Equal(t, 4, bank.TrackOffset(), "offset past the kit")
	require.Equal(t, 8, bank.TrackCount(), "the kit should cap the growth")
	require.Equal(t, "Pattern 001  5/8", bank.headerText(), "tail header")
	assertVisibleTracks(t, bank, 5)

	require.NoError(t, bank.ScrollTracks(-8))
	require.Equal(t, "Pattern 001  1/8", bank.headerText(), "head header")
	assertVisibleTracks(t, bank, 1)
}

func TestLargeKitStartsWithPadRows(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(100, -1))
	require.Equal(t, padRows, bank.TrackCount(), "a large kit should open on the pad rows")
	require.NoError(t, bank.ScrollTracks(1))
	require.Equal(t, padRows+1, bank.TrackCount(), "one scroll")
	require.NoError(t, bank.ScrollTracks(-1))
	require.NoError(t, bank.ScrollTracks(-1))
	require.Equal(t, 0, bank.TrackOffset(), "scrolled above the first track")
}

func TestTrackWindowStopsOnRealTracks(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(10, -1))
	require.NoError(t, bank.ScrollTracks(50))
	// A jump past the end grows tracks no further than the kit, and the last track has
	// to land on the bottom row, otherwise the pads would show empty rows past the end.
	require.Equal(t, 10, bank.TrackCount(), "the kit's 10 voices")
	require.Equal(t, 6, bank.TrackOffset(), "max offset")
	require.Equal(t, "Pattern 001  7/10", bank.headerText(), "tail header")
	assertVisibleTracks(t, bank, 7)
	require.NoError(t, bank.ScrollTracks(1))
	require.Equal(t, 6, bank.TrackOffset(), "scrolling past the last track")
}

// A wide kit must still fit the header, and every character in it needs a font glyph.
func TestTrackWindowHeaderStaysReadable(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(100, -1))
	require.NoError(t, bank.ScrollTracks(96))
	header := bank.headerText()
	require.Equal(t, "Pattern 001 97/100", header)
	for _, ch := range header {
		if ch == ' ' {
			continue
		}
		blank := true
		for _, column := range appendGlyph(nil, byte(ch), false) {
			if column != 0 {
				blank = false
				break
			}
		}
		require.Falsef(t, blank, "the font renders %q blank in header %q", ch, header)
	}
}

// The playback worker draws the visible tracks from its own goroutine while the event
// loop scrolls the window and blacks the display out, so both sides have to be guarded.
// Run under -race to catch a regression.
func TestTrackWindowIsSafeDuringPlaybackDraws(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	bank := useController(t, fire, voiceBank).patbank
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
	// These keep going on failure so the overlap is not cut short by the first problem.
	for range 200 {
		assert.NoError(t, bank.ScrollTracks(1))
		assert.NoError(t, bank.ScrollTracks(-1))
		assert.NoError(t, fire.Blackout())
		// Waking up is a read-only redraw, so it must not touch the selected pattern
		// index the worker reads while a pattern plays.
		if fire.Wake() {
			assert.NoError(t, bank.redraw())
		}
	}
	wg.Wait()
	require.Equal(t, 0, bank.TrackOffset(), "offset after an even number of scrolls")
}

// Every writer wraps or clamps the voice index, so a track never points outside the
// kit. The track lookup relies on that rather than re-checking each time.
func TestTrackVoicesStayInsideTheKit(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(6, -1))
	assertTrackVoicesInKit(t, bank)
	for row := 1; row <= padRows; row++ {
		require.NoError(t, bank.SelectTrackRow(row))
		// Jog past both ends of the kit and back, several times over.
		for range 2 * len(voiceBank.voices) {
			for _, delta := range []int{1, -1} {
				require.NoError(t, bank.JogSelect(delta))
			}
			assertTrackVoicesInKit(t, bank)
		}
	}
	// Growing the window to the kit's size keeps every track on a real voice.
	require.NoError(t, bank.ScrollTracks(len(voiceBank.voices)))
	require.Equal(t, len(voiceBank.voices), bank.TrackCount(), "the window should reach the kit's voices")
	assertTrackVoicesInKit(t, bank)
}

func assertTrackVoicesInKit(t *testing.T, bank *PatternBank) {
	t.Helper()
	bank.trackMu.RLock()
	defer bank.trackMu.RUnlock()
	for i, voice := range bank.trackVoices {
		require.LessOrEqualf(t, 0, voice, "track %d points below the kit", i+1)
		require.Lessf(t, voice, len(bank.vb.voices), "track %d points above the kit's %d voices", i+1, len(bank.vb.voices))
	}
}

func TestSmallKitFillsEveryRow(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(2, -1))
	require.Equal(t, padRows, bank.TrackCount())
	require.NoError(t, bank.ScrollTracks(1))
	require.Equal(t, 0, bank.TrackOffset(), "a kit smaller than the pad rows should not scroll")
	assertVisibleTracks(t, bank, 1)
}

func TestTrackWindowEditsVisibleTracks(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(8, 6))
	require.NoError(t, bank.ScrollTracks(2))
	require.NoError(t, bank.SelectTrackRow(1))
	// Row 1 now edits track 3, so the encoder and pad presses follow the window.
	require.Same(t, voiceBank.voices[2], bank.SelectedVoice(), "row 1 should edit track 3")
	require.NoError(t, bank.JogSelect(1))
	require.Same(t, voiceBank.voices[3], bank.SelectedVoice(), "after a jog, row 1 edits track 4")

	_, err := bank.ToggleEvent(0, 4, 100)
	require.NoError(t, err)
	_, onVisible := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[3])
	require.True(t, onVisible, "the pad press did not reach the visible track")
	_, onHidden := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[2])
	require.False(t, onHidden, "the pad press edited a track outside the window")

	// Clearing a row clears the track the row shows, not the row's original track.
	require.NoError(t, bank.ClearTrackRow(1))
	_, stillThere := bank.CurrentPattern().EventAtStep(4, voiceBank.voices[3])
	require.False(t, stillThere, "clearing the row left its track in place")
}

func TestTrackWindowChromaticEditingFollowsRow(t *testing.T) {
	bank, voiceBank := quietBank(t, trackWindowKit(8, 5))
	voice := voiceBank.voices[5]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 90)
	pattern.SetChromaticNote(2, voice, 62, 40)
	require.NoError(t, bank.ScrollTracks(2))
	require.NoError(t, bank.SelectTrackRow(4))
	require.Same(t, voice, bank.SelectedVoice(), "the chromatic track should be the one in view")
	require.NoError(t, bank.MoveStepCursor(2))
	event, ok := pattern.EventAtStep(bank.StepCursor(), voice)
	require.True(t, ok, "the step the cursor moved to holds nothing")
	require.Equal(t, 40, event.Velocity, "the moved-to step is not the visible track's event")

	// Scrolling away releases the palette so the next window starts in step mode.
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, bank.ScrollTracks(1))
	require.False(t, bank.NoteEditActive(), "scrolling kept note-edit mode on the previous track")
}

func TestScrollTracksClosesNoteEditOnScrolledRow(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	require.True(t, bank.NoteEditActive(), "a chromatic track did not enter note-edit mode")
	bank.pressPad(0, 3)
	require.NoError(t, bank.ScrollTracks(1))
	require.Zero(t, bank.pressedPads, "scrolling kept a pad held from the previous window")
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
	controller := useController(t, fire, kit)
	bank := controller.patbank
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[0], 40, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	previousBPM := currentBPM()
	setBPM(300)
	t.Cleanup(func() { setBPM(previousBPM) })
	bank.controller.startPlayback(&captureMidiWriter{}, bank.newPlayback())
	// Wait for the worker to move the playhead a few steps, so the switches below land on
	// top of it rather than in the first moments of playback.
	waitFor(t, "the playhead to move", func() bool { return writes.Load() > stepPaints })

	before := writes.Load()
	// Keep switching for long enough that the worker is certainly running against it: one
	// pass can finish between two of the worker's steps and prove nothing.
	for i := range switchRounds {
		// Alternate two rows: pressing the row that is already selected toggles it off,
		// which is a different behaviour from moving the selection.
		row := i%2 + 2 // rows two and three, neither of which starts selected
		// Spread over several beats, so the switches interleave with the worker's own reads.
		time.Sleep(3 * time.Millisecond)
		require.NoErrorf(t, dispatch(bank, padMessage(NoteMute1+row-1, 100)), "row %d", row)
		require.NotNilf(t, bank.controller.playback, "selecting track row %d stopped the pattern", row)
		require.Equalf(t, row, bank.selTrackRow, "the selected row after pressing it")
	}
	// The worker must have painted while those switches were happening, or the race this
	// test exists to catch was never exercised.
	require.Greater(t, writes.Load(), before, "the sequencer worker did not paint while the selection moved")
	require.NoError(t, bank.controller.stopPlayback())
	require.Nil(t, bank.controller.playback, "stopping left the sequencer armed")
}

// A press that cannot change the set must not end it. The pattern buttons stop playback
// because moving between patterns rewrites what the worker reads, but at the end of the
// list there is nowhere to move to, and the stop belongs to the bank that knows whether the
// selection moved rather than to the handler that asked.
func TestAPressThatChangesNothingDoesNotStopTheSet(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	require.Equal(t, 1, bank.selPatIdx, "the test needs the first pattern selected to start with")

	stopped := 0
	install := func() {
		bank.controller.playback = stubSession(&Playback{}, func() error {
			stopped++
			return nil
		})
	}

	// Pattern indices start at one, so up from the first is the move.
	install()
	require.NoError(t, dispatch(bank, padMessage(NotePatternUp, 100)))
	require.Equal(t, 2, bank.selPatIdx, "the pattern did not move")
	require.Positive(t, stopped, "moving to another pattern left the set playing")

	// Down moves back, which changes what would be played just as much.
	stopped = 0
	install()
	require.NoError(t, dispatch(bank, padMessage(NotePatternDown, 100)))
	require.Equal(t, 1, bank.selPatIdx, "the pattern did not move back")
	require.Positive(t, stopped, "moving back to the first pattern left the set playing")

	// And now there is nothing before the first one.
	stopped = 0
	install()
	require.NoError(t, dispatch(bank, padMessage(NotePatternDown, 100)))
	require.Equal(t, 1, bank.selPatIdx, "the selection moved past the first pattern")
	require.Zero(t, stopped, "pressing down at the end of the pattern list stopped the set")
}

// Alt plus a solo button clears the row's notes, which changes what is being played, so
// that one does stop playback. It must not drag the plain selection path down with it.
func TestAltSoloStillStopsPlayback(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().ToggleEvent(Event{Voice: voice, Beat: stepBeat(0), Velocity: 100})
	stopped := 0
	bank.controller.playback = stubSession(&Playback{}, func() error {
		stopped++
		return nil
	})
	bank.controller.alt = true
	require.NoError(t, dispatch(bank, padMessage(NoteMute1, 100)))
	require.Positive(t, stopped, "clearing a row left the pattern playing")
	_, cleared := bank.CurrentPattern().EventAtStep(0, voice)
	require.False(t, cleared, "Alt plus the solo button did not clear the row")

	// The same button without Alt is a selection, and it leaves the sequencer alone.
	bank.controller.playback = stubSession(&Playback{}, func() error {
		stopped++
		return nil
	})
	bank.controller.alt = false
	require.NoError(t, dispatch(bank, padMessage(NoteMute1, 100)))
	require.Equal(t, 1, stopped, "only the clear should have stopped playback")
}
