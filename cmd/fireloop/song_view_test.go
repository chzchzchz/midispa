package main

import (
	"sync"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

// newArrangementTest builds both banks and leaves the controller in the arrangement view,
// which is the view these tests press controls in. Dispatching through the controller is
// what puts them there: naming the song handler directly would skip the choice the program
// actually makes.
func newArrangementTest(t *testing.T) (*SongBank, *PatternBank) {
	t.Helper()
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "voice", Note: testNote(60), Channel: 1}},
	}})
	controller := useController(t, fire, voiceBank)
	require.NoError(t, controller.songbank.Jump(0))
	controller.mode = songView
	return controller.songbank, controller.patbank
}

func arrangementNote(note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(0), byte(note), 1}}
}

// fillMeasures places a distinct pattern in each of the first count
// measures, so a test can tell by colour which measure a pad is showing.
func fillMeasures(songBank *SongBank, patternBank *PatternBank, count int) {
	for measure := range count {
		pattern := &Pattern{}
		patternBank.Patterns[measure+1] = pattern
		songBank.CurrentSong().SetPattern(pattern, measure)
	}
}

func TestArrangementPatternViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	for index := 1; index <= 40; index++ {
		patternBank.Patterns[index] = &Pattern{}
	}
	require.NoError(t, songBank.ScrollPatterns(1))
	require.Equal(t, 17, songBank.patternStart, "the viewport should move one page")
	require.Equal(t, 1, patternBank.selPatIdx, "scrolling should not move the selection")

	require.NoError(t, songBank.ScrollPatterns(100))
	require.Equal(t, maxPatternIndex-patternViewSize+1, songBank.patternStart, "the viewport should clamp")

	require.NoError(t, songBank.ScrollPatterns(-100))
	require.Equal(t, 1, songBank.patternStart, "the viewport should return to the start")
}

func TestArrangementPatternSelection(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	songBank.patternStart = 17
	require.NoError(t, songBank.SelectPatternSlot(3))
	require.Equal(t, 20, patternBank.selPatIdx, "a visible slot should select that pattern")

	require.NoError(t, songBank.SelectPattern(999))
	require.Equal(t, maxPatternIndex-patternViewSize+1, songBank.patternStart,
		"selecting the last pattern should reveal it")

	patternBank.selPatIdx = 16
	songBank.patternStart = 1
	require.NoError(t, songBank.MovePatternSelection(1))
	require.Equal(t, 17, patternBank.selPatIdx, "a one-slot move should land on 17")
	require.Equal(t, 2, songBank.patternStart, "a one-slot move should reveal it")
}

func TestArrangementMeasureViewport(t *testing.T) {
	songBank, _ := newArrangementTest(t)
	require.NoError(t, songBank.ScrollMeasures(measurePageSize))
	require.Equal(t, measurePageSize, songBank.measureStart, "a page of measures")
	require.NoError(t, songBank.ScrollMeasures(-measureFinePageSize))
	require.Equal(t, 12, songBank.measureStart, "a fine measure scroll")
	require.NoError(t, songBank.ScrollMeasures(-100))
	require.NoError(t, songBank.ScrollMeasures(1000))
	require.Equal(t, maxMeasureIndex-measureViewSize+1, songBank.measureStart, "the measure viewport should clamp")
}

func TestArrangementMeasureEditingUsesViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	songBank.measureStart = 16
	require.NoError(t, songBank.ToggleMeasure(0, 0))
	require.Same(t, patternBank.Patterns[1], songBank.CurrentSong().GetPattern(16),
		"the pad should address the measure in view")
	require.Nil(t, songBank.CurrentSong().GetPattern(0), "measure 0 was not touched")

	require.NoError(t, songBank.ToggleMeasure(11, 3))
	require.Same(t, patternBank.Patterns[1], songBank.CurrentSong().GetPattern(63),
		"a pad further down should address its own measure")
}

func TestArrangementViewportSurvivesSongRefresh(t *testing.T) {
	songBank, _ := newArrangementTest(t)
	require.NoError(t, songBank.ScrollPatterns(1))
	require.NoError(t, songBank.ScrollMeasures(measurePageSize))
	require.NoError(t, songBank.Jump(0))
	require.Equal(t, 17, songBank.patternStart, "a refresh reset the pattern viewport")
	require.Equal(t, measurePageSize, songBank.measureStart, "a refresh reset the measure viewport")
}

func TestSongModeScrollBindings(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	controller := patternBank.controller
	controller.shift = false

	require.NoError(t, dispatch(patternBank, arrangementNote(NotePatternUp)))
	require.Equal(t, 17, songBank.patternStart, "pattern up should scroll a page")

	songBank.patternStart = 1
	patternBank.selPatIdx = 1
	controller.shift = true
	require.NoError(t, dispatch(patternBank, arrangementNote(NotePatternUp)))
	require.Equal(t, 2, patternBank.selPatIdx, "shift pattern up should move the selection")
	require.Equal(t, 1, songBank.patternStart, "shift pattern up should not scroll the viewport")

	controller.shift = false
	require.NoError(t, dispatch(patternBank, arrangementNote(NoteGridRight)))
	require.Equal(t, measurePageSize, songBank.measureStart, "grid right should scroll a page")

	controller.shift = true
	require.NoError(t, dispatch(patternBank, arrangementNote(NoteGridRight)))
	require.Equal(t, measurePageSize+measureFinePageSize, songBank.measureStart,
		"shift grid right should fine-scroll")
}

func TestPatternLengthControls(t *testing.T) {
	_, patternBank := newArrangementTest(t)
	patternBank.controller.mode = patternView
	patternBank.CurrentPattern().SetLengthSteps(8)
	event, err := patternBank.ToggleEvent(0, 8, 127)
	require.NoError(t, err)
	require.Zero(t, event.Velocity, "a step past the end should create nothing")

	require.NoError(t, dispatch(patternBank, arrangementNote(NoteOverview)))
	require.True(t, patternBank.lengthEditActive(), "Overview did not enter length mode")

	require.NoError(t, dispatch(patternBank, encoderTurn(EncoderRight)))
	require.Equal(t, 9, patternBank.CurrentPattern().LengthSteps(), "the encoder should lengthen")

	left := encoderTurn(EncoderLeft)
	require.NoError(t, dispatch(patternBank, left))
	require.Equal(t, 8, patternBank.CurrentPattern().LengthSteps(), "the encoder should shorten")

	require.NoError(t, dispatch(patternBank, arrangementNote(NoteOverview)))
	require.False(t, patternBank.lengthEditActive(), "Overview did not leave length mode")
}

// A bank the user has not chosen a pattern on yet has nothing to report a length from.
// NewPatternBank leaves the map empty and every other path into length mode goes through
// Jump, so this is the one shape of bank that can reach the readout without a pattern.
func TestLengthModeWithoutAPatternReportsNothing(t *testing.T) {
	kit := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "voice", Note: testNote(60), Channel: 1}},
	}})
	bank := useEmptyController(t, NewFire(func([]byte) error { return nil }), kit).patbank
	recorder := useScreenRecorder(t, &bank.screen)
	require.NoError(t, bank.ToggleLengthMode())
	require.Empty(t, recorder.row(readoutRow), "the length row should be left blank")
}

func TestSongModePatternPadUsesViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	patternBank.controller.shift = false
	songBank.patternStart = 17

	require.NoError(t, dispatch(patternBank, arrangementNote(66)))
	require.Equal(t, 17, patternBank.selPatIdx, "the pad should select the pattern in view")
}

// Selecting a pattern in song mode is a view change over the arrangement, not
// an edit of what is playing, so the set keeps running through the press. The
// stub session counts the stops, which is how a selection that still stopped
// playback would be heard here.
func TestSongPatternSelectionKeepsTheSetPlaying(t *testing.T) {
	_, patternBank := newArrangementTest(t)
	stopped := 0
	patternBank.controller.playback = stubSession(&Playback{}, func() error {
		stopped++
		return nil
	})

	// The pad in the first slot of the pattern bank selects the pattern in view.
	require.NoError(t, dispatch(patternBank, arrangementNote(66)))
	require.Equal(t, 1, patternBank.selPatIdx, "the pad should select the pattern in view")
	require.Zero(t, stopped, "selecting a pattern stopped the set")
	require.NotNil(t, patternBank.controller.playback, "the set was uninstalled")

	// The next slot's pattern does not exist yet, so the press also claims a slot.
	require.NoError(t, dispatch(patternBank, arrangementNote(67)))
	require.Equal(t, 2, patternBank.selPatIdx, "the second pad should select the next pattern")
	require.Zero(t, stopped, "selecting an unused pattern stopped the set")
	require.NotNil(t, patternBank.controller.playback, "the set was uninstalled")

	// Shift plus pattern up moves the selection the same way.
	stopped = 0
	patternBank.controller.shift = true
	require.NoError(t, dispatch(patternBank, arrangementNote(NotePatternUp)))
	require.Equal(t, 3, patternBank.selPatIdx, "shift pattern up should move the selection")
	require.Zero(t, stopped, "moving the selection stopped the set")
	require.NotNil(t, patternBank.controller.playback, "the set was uninstalled")
}

// The playback worker reads the pattern map from its own goroutine, to colour
// the measures and to name the pattern each one plays, while a pattern selected
// in song mode claims its slot on the event loop without stopping the set. Both
// sides hold the window lock, which is what this test pins; run under -race to
// catch a regression.
func TestSongPatternSelectionIsSafeDuringPlaybackDraws(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	var painter sync.WaitGroup
	painter.Add(1)
	go func() {
		defer painter.Done()
		for range 100 {
			patternBank.PatternIdxMap()
			songBank.patternsToColors()
		}
	}()
	for index := 1; index <= 50; index++ {
		require.NoError(t, songBank.SelectPattern(index))
	}
	painter.Wait()
	require.Equal(t, 50, patternBank.selPatIdx, "the selection should be on the last pattern")
}

// A song whose measures turn over quickly keeps a worker reading the pattern map
// while the pattern pads are pressed, which is the arrangement the change is
// for: the selection lands without stopping the set, and the worker keeps
// writing the song through the presses. The sequencer is started for real here,
// and the race detector watches the map the selection claims slots in against
// the worker that reads it.
func TestSongPatternPadsKeepTheSongPlaying(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	controller := patternBank.controller
	kit := patternBank.vb
	// A one-step pattern turns a measure over every sixteenth of a beat, so
	// the worker crosses measures, and reads the map on each one, while the
	// pads below are pressed.
	pattern := patternBank.CurrentPattern()
	pattern.SetLengthSteps(1)
	pattern.ToggleEvent(Event{Voice: kit.voices[0], Beat: stepBeat(0), Velocity: 100})
	for measure := range 8 {
		songBank.CurrentSong().SetPattern(pattern, measure)
	}
	previousBPM := currentBPM()
	setBPM(300)
	t.Cleanup(func() { setBPM(previousBPM) })
	writer := &captureMidiWriter{}
	require.NoError(t, controller.startSongPlayback(writer))
	require.NotNil(t, controller.playback, "the song did not start")
	writer.waitForEvents(t, 4)

	// Columns 12 to 15 of each pad row hold the pattern slots.
	for _, note := range []int{66, 67, 68, 69, 82, 83, 84, 85} {
		require.NoError(t, dispatch(patternBank, arrangementNote(note)))
		require.NotNil(t, controller.playback, "pressing a pattern pad stopped the song")
	}
	require.Equal(t, 8, patternBank.selPatIdx, "the pads should have moved the selection")
	require.True(t, controller.playing(), "the song stopped during the presses")
	// The worker is still writing the song after the presses, not just installed.
	writer.waitForEvents(t, 12)
	require.NoError(t, controller.stopPlayback())
	require.Nil(t, controller.playback, "stopping left the sequencer armed")
}

// The pad a measure is drawn on has to be the pad a press edits it with:
// measureIndex reads a press back to a measure, so measurePadPosition has to
// return that same pad for every measure the window holds. The window moves in
// pages, so the round trip is checked at the top of the song, one page in, and
// at the end, where the clamp keeps the last measures reachable.
func TestMeasurePadPositionInvertsMeasureIndex(t *testing.T) {
	songBank, _ := newArrangementTest(t)
	for _, windowStart := range []int{0, measurePageSize, maxMeasureIndex - measureViewSize + 1} {
		songBank.measureStart = windowStart
		start, end := songBank.measureWindow()
		for measure := start; measure <= end; measure++ {
			x, y, ok := songBank.measurePadPosition(measure)
			require.Truef(t, ok, "measure %d in the window should sit on a pad", measure)
			idx, ok := songBank.measureIndex(x, y)
			require.True(t, ok)
			require.Equalf(t, measure, idx, "pad (%d,%d) should address the measure it shows", x, y)
		}
	}
}

// The playhead lights the measure it reached at full brightness and the measure
// it left dimmed, each on the pad that measure is drawn on. Five measures hold
// one pattern each here: the first four sit across the top row and the fifth
// below the first, because a bank of sixteen measures fills its four columns
// before it fills a row.
func TestToggleMeasureBrightnessLightsTheMeasureItReached(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	recorder := usePadRecorder(t, &songBank.pads)
	fillMeasures(songBank, patternBank, 5)

	// The playhead crosses from the fourth measure to the fifth.
	require.NoError(t, songBank.ToggleMeasureBrightness(3, 4))
	require.Equal(t, dimColor(patternColor(4)), recorder.pad(3, 0),
		"the measure left is dimmed on its own pad")
	require.Equal(t, patternColor(5), recorder.pad(0, 1),
		"the measure reached lights bright on its own pad")

	// A measure outside the window is playing nowhere the view shows.
	require.NoError(t, songBank.ToggleMeasureBrightness(measureViewSize, measureViewSize+1))
	require.Equal(t, patternColor(5), recorder.pad(0, 1),
		"a measure past the window lights no pad")
}

// The arrangement view draws its whole window on the pads measureIndex reads a
// press from. The corners and the bank boundaries pin the fill order the view
// documents: three banks of sixteen measures, each filled left to right and top
// to bottom, so the fifth measure sits below the first rather than beside it.
func TestDrawPadMeasuresFillsEachBankLeftToRight(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	recorder := usePadRecorder(t, &songBank.pads)
	fillMeasures(songBank, patternBank, measureViewSize)
	require.NoError(t, songBank.DrawPadMeasures())
	for _, corner := range []struct {
		measure int
		pad     [2]int
	}{
		{0, [2]int{0, 0}}, {3, [2]int{3, 0}}, {4, [2]int{0, 1}}, {15, [2]int{3, 3}},
		{16, [2]int{4, 0}}, {31, [2]int{7, 3}}, {32, [2]int{8, 0}}, {47, [2]int{11, 3}},
	} {
		require.Equalf(t, dimColor(patternColor(corner.measure+1)), recorder.pad(corner.pad[0], corner.pad[1]),
			"measure %d should be drawn on pad (%d,%d)", corner.measure+1, corner.pad[0], corner.pad[1])
	}
	require.Equal(t, [3]int{}, recorder.pad(12, 0), "the measure area ends at the twelfth column")
}
