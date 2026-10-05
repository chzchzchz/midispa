package main

import (
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
