package main

import (
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func newArrangementTest(t *testing.T) (*SongBank, *PatternBank) {
	t.Helper()
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "voice", Note: 60, Channel: 1}},
	}})
	patternBank := NewPatternBank(fire, voiceBank)
	if err := patternBank.Jump(1); err != nil {
		t.Fatal(err)
	}
	songBank := NewSongBank(fire, patternBank)
	if err := songBank.Jump(0); err != nil {
		t.Fatal(err)
	}
	return songBank, patternBank
}

func arrangementNote(note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(0), byte(note), 1}}
}

func TestArrangementPatternViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	for index := 1; index <= 40; index++ {
		patternBank.Patterns[index] = &Pattern{}
	}
	if err := songBank.ScrollPatterns(1); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != 17 || patternBank.selPatIdx != 1 {
		t.Fatalf("unexpected pattern viewport: start=%d selected=%d", songBank.patternStart, patternBank.selPatIdx)
	}
	if err := songBank.ScrollPatterns(100); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != maxPatternIndex-patternViewSize+1 {
		t.Fatalf("pattern viewport did not clamp: %d", songBank.patternStart)
	}
	if err := songBank.ScrollPatterns(-100); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != 1 {
		t.Fatalf("pattern viewport did not return to start: %d", songBank.patternStart)
	}
}

func TestArrangementPatternSelection(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	songBank.patternStart = 17
	if err := songBank.SelectPatternSlot(3); err != nil {
		t.Fatal(err)
	}
	if patternBank.selPatIdx != 20 {
		t.Fatalf("visible slot selected pattern %d, want 20", patternBank.selPatIdx)
	}
	if err := songBank.SelectPattern(999); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != maxPatternIndex-patternViewSize+1 {
		t.Fatalf("selection did not reveal final pattern: %d", songBank.patternStart)
	}
	patternBank.selPatIdx = 16
	songBank.patternStart = 1
	if err := songBank.MovePatternSelection(1); err != nil {
		t.Fatal(err)
	}
	if patternBank.selPatIdx != 17 || songBank.patternStart != 2 {
		t.Fatalf("one-slot selection moved incorrectly: selected=%d start=%d", patternBank.selPatIdx, songBank.patternStart)
	}
}

func TestArrangementMeasureViewport(t *testing.T) {
	songBank, _ := newArrangementTest(t)
	if err := songBank.ScrollMeasures(measurePageSize); err != nil {
		t.Fatal(err)
	}
	if songBank.measureStart != measurePageSize {
		t.Fatalf("measure page = %d, want %d", songBank.measureStart, measurePageSize)
	}
	if err := songBank.ScrollMeasures(-measureFinePageSize); err != nil {
		t.Fatal(err)
	}
	if songBank.measureStart != 12 {
		t.Fatalf("fine measure scroll = %d, want 12", songBank.measureStart)
	}
	if err := songBank.ScrollMeasures(-100); err != nil {
		t.Fatal(err)
	}
	if err := songBank.ScrollMeasures(1000); err != nil {
		t.Fatal(err)
	}
	if songBank.measureStart != maxMeasureIndex-measureViewSize+1 {
		t.Fatalf("measure viewport did not clamp: %d", songBank.measureStart)
	}
}

func TestArrangementMeasureEditingUsesViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	songBank.measureStart = 16
	if err := songBank.ToggleMeasure(0, 0); err != nil {
		t.Fatal(err)
	}
	if got := songBank.CurrentSong().GetPattern(16); got != patternBank.Patterns[1] {
		t.Fatalf("measure 16 points to %p, want %p", got, patternBank.Patterns[1])
	}
	if got := songBank.CurrentSong().GetPattern(0); got != nil {
		t.Fatalf("measure 0 unexpectedly points to %p", got)
	}
	if err := songBank.ToggleMeasure(11, 3); err != nil {
		t.Fatal(err)
	}
	if got := songBank.CurrentSong().GetPattern(63); got != patternBank.Patterns[1] {
		t.Fatalf("measure 63 points to %p, want %p", got, patternBank.Patterns[1])
	}
}

func TestArrangementViewportSurvivesSongRefresh(t *testing.T) {
	songBank, _ := newArrangementTest(t)
	if err := songBank.ScrollPatterns(1); err != nil {
		t.Fatal(err)
	}
	if err := songBank.ScrollMeasures(measurePageSize); err != nil {
		t.Fatal(err)
	}
	if err := songBank.Jump(0); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != 17 || songBank.measureStart != measurePageSize {
		t.Fatalf("refresh reset viewport: patterns=%d measures=%d", songBank.patternStart, songBank.measureStart)
	}
}

func TestSongModeScrollBindings(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	previousSongbank, previousShift := songbank, shiftOn
	t.Cleanup(func() {
		songbank = previousSongbank
		shiftOn = previousShift
	})
	songbank = songBank
	shiftOn = false

	if err := processSongEvent(nil, arrangementNote(NotePatternUp)); err != nil {
		t.Fatal(err)
	}
	if songBank.patternStart != 17 {
		t.Fatalf("pattern up did not scroll: %d", songBank.patternStart)
	}
	songBank.patternStart = 1
	patternBank.selPatIdx = 1
	shiftOn = true
	if err := processSongEvent(nil, arrangementNote(NotePatternUp)); err != nil {
		t.Fatal(err)
	}
	if patternBank.selPatIdx != 2 || songBank.patternStart != 1 {
		t.Fatalf("shift pattern up moved incorrectly: selected=%d start=%d", patternBank.selPatIdx, songBank.patternStart)
	}
	shiftOn = false
	if err := processSongEvent(nil, arrangementNote(NoteGridRight)); err != nil {
		t.Fatal(err)
	}
	if songBank.measureStart != measurePageSize {
		t.Fatalf("grid right did not scroll a page: %d", songBank.measureStart)
	}
	shiftOn = true
	if err := processSongEvent(nil, arrangementNote(NoteGridRight)); err != nil {
		t.Fatal(err)
	}
	if songBank.measureStart != measurePageSize+measureFinePageSize {
		t.Fatalf("shift grid right did not fine-scroll: %d", songBank.measureStart)
	}
}

func TestPatternLengthControls(t *testing.T) {
	_, patternBank := newArrangementTest(t)
	previousPatbank := patbank
	t.Cleanup(func() {
		patbank = previousPatbank
	})
	patbank = patternBank
	patternBank.CurrentPattern().SetLengthSteps(8)
	if event, err := patternBank.ToggleEvent(0, 8, 127); err != nil {
		t.Fatal(err)
	} else if event.Velocity != 0 {
		t.Fatalf("out-of-range step created velocity %d", event.Velocity)
	}

	if err := processPatternEvent(nil, arrangementNote(NoteOverview)); err != nil {
		t.Fatal(err)
	}
	if !patternBank.editingLength {
		t.Fatal("Overview did not enter length mode")
	}
	if err := processPatternEvent(nil, arrangementNote(CCSelect)); err != nil {
		t.Fatal(err)
	}
	if got := patternBank.CurrentPattern().LengthSteps(); got != 9 {
		t.Fatalf("encoder increased length to %d, want 9", got)
	}
	left := alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCSelect), byte(EncoderLeft)}}
	if err := processPatternEvent(nil, left); err != nil {
		t.Fatal(err)
	}
	if got := patternBank.CurrentPattern().LengthSteps(); got != 8 {
		t.Fatalf("encoder decreased length to %d, want 8", got)
	}
	if err := processPatternEvent(nil, arrangementNote(NoteOverview)); err != nil {
		t.Fatal(err)
	}
	if patternBank.editingLength {
		t.Fatal("Overview did not leave length mode")
	}
}

func TestSongModePatternPadUsesViewport(t *testing.T) {
	songBank, patternBank := newArrangementTest(t)
	previousSongbank, previousShift := songbank, shiftOn
	t.Cleanup(func() {
		songbank = previousSongbank
		shiftOn = previousShift
	})
	songbank = songBank
	shiftOn = false
	songBank.patternStart = 17

	if err := processSongEvent(nil, arrangementNote(66)); err != nil {
		t.Fatal(err)
	}
	if patternBank.selPatIdx != 17 {
		t.Fatalf("pattern pad selected %d, want 17", patternBank.selPatIdx)
	}
}
