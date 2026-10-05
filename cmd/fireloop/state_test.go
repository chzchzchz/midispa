package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateKit is a kit with one chromatic voice and the rest percussive, so a session has to
// carry both kinds of event across a save. It is built fresh each time: a loaded session
// resolves onto the kit of the run that loaded it, not the one that wrote the file.
func stateKit() *VoiceBank {
	return trackWindowKit(6, 2)
}

func stateTestBank(t *testing.T, kit *VoiceBank) *Controller {
	t.Helper()
	return useController(t, NewFire(func([]byte) error { return nil }), kit)
}

// withSession points a controller at the file a save writes and a load reads. The banks
// already exist and already have their owner, so this only names the file.
func withSession(controller *Controller, path string, kit []string) *Controller {
	controller.sessionPath = path
	controller.sessionKit = kit
	return controller
}

// stateFilePath is where a test saves. Each call gets its own directory, so two tests can
// never write over each other's file.
func stateFilePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "set.json")
}

// saveTo writes the session a test is holding, so a test that only cares that the file
// landed does not have to carry the same error check as one that reads the report back.
func saveTo(t *testing.T, path string, bank *PatternBank, songs *SongBank, kit []string) {
	t.Helper()
	_, err := saveState(path, bank, songs, kit)
	require.NoError(t, err)
}

// loadInto installs the session at path into a controller's banks, which is the far side of
// a round trip. It goes through the controller because stopping playback and the banks the
// load applies to belong to one, so the test drives the same path a panel gesture would.
func loadInto(controller *Controller, path string, kit []string) (stateReport, error) {
	controller.sessionKit = kit
	return controller.loadState(path)
}

// loadedSession is the far side of a round trip: a bank and a song bank built from a fresh
// copy of the kit, with a saved session installed in them.
type loadedSession struct {
	kit    *VoiceBank
	bank   *PatternBank
	songs  *SongBank
	report stateReport
}

// roundTrip saves a session and loads it into banks built from a fresh copy of the kit.
// Sharing voices with the writer would hide a restore that resolves against the wrong ones.
// The far side gets its own controller because it is its own session, not a view of the
// one that wrote the file.
func roundTrip(t *testing.T, bank *PatternBank, songs *SongBank, kit []string) loadedSession {
	t.Helper()
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, kit)
	loadedKit := stateKit()
	far := stateTestBank(t, loadedKit)
	report, err := loadInto(far, path, kit)
	require.NoError(t, err)
	return loadedSession{kit: loadedKit, bank: far.patbank, songs: far.songbank, report: report}
}

// loadCrafted writes a hand-built file and loads it, for the tests about a truncated,
// hand-edited or foreign file. The error is returned so a case can assert a refusal.
func loadCrafted(t *testing.T, file stateFile, kit *VoiceBank) (*PatternBank, stateReport, error) {
	t.Helper()
	path := stateFilePath(t)
	_, err := writeStateFile(path, file)
	require.NoError(t, err)
	session := stateTestBank(t, kit)
	report, err := loadInto(session, path, nil)
	return session.patbank, report, err
}

// fillStateSession writes through the production editing calls, so the state under test is
// one the controls can actually produce: percussive steps, chromatic notes, a tie, a
// shortened pattern, a song, a remapped track voice, and a tempo.
func fillStateSession(t *testing.T, bank *PatternBank, songs *SongBank, kit *VoiceBank) {
	t.Helper()
	chromatic := kit.voices[2]
	pattern := bank.CurrentPattern()
	for _, col := range []int{0, 4, 8} {
		_, err := bank.ToggleEvent(0, col, 100)
		require.NoError(t, err)
	}
	pattern.SetChromaticNote(2, chromatic, 60, 100)
	pattern.SetChromaticNote(5, chromatic, 62, 90)
	require.True(t, pattern.TieEventsAtSteps(2, 5, chromatic), "tie was refused on the pattern being written")
	require.Equal(t, 8, pattern.SetLengthSteps(8))
	_, err := bank.ToggleEvent(1, 3, 100)
	require.NoError(t, err)

	// A second pattern, so the file has more than one and the song has something to pick.
	// Jump is a relative move, so this is a step rather than a pattern number.
	require.NoError(t, bank.Jump(2))
	third := bank.selPatIdx
	second := bank.CurrentPattern()
	second.SetChromaticNote(1, chromatic, 64, 70)
	_, err = bank.ToggleEvent(0, 1, 100)
	require.NoError(t, err)

	// The song places the first pattern in two measures and the second in another, which is
	// what proves the measures and the bank end up sharing one pattern each after a load.
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(bank.Patterns[1], 0)
	song.SetPattern(bank.Patterns[1], 1)
	song.SetPattern(second, 4)
	require.Equalf(t, second, bank.Patterns[third], "pattern %d is not the one the bank selected", third)

	// Only the track's voice, which is part of the set. Scrolling the window, moving the
	// cursor and turning the palette are view state and are no longer saved, so setting
	// them here would only make the round trip look busier than it is.
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.JogSelect(2))
	setBPM(150)
}

func mustStateJSON(t *testing.T, state stateFile) string {
	t.Helper()
	data, err := json.MarshalIndent(state, "", stateIndent)
	require.NoError(t, err)
	return string(data)
}

// assertSameSession compares everything a save holds. The timestamp is the one field a round
// trip cannot reproduce, and the only reason two files of one session differ.
func assertSameSession(t *testing.T, want, got stateFile) {
	t.Helper()
	want.SavedAt = time.Time{}
	got.SavedAt = time.Time{}
	require.Equalf(t, want, got, "the session changed across the round trip\nwant:\n%s\ngot:\n%s",
		mustStateJSON(t, want), mustStateJSON(t, got))
}

// A session has to come back whole: every note, tie, length, track assignment and tempo.
// The comparison runs through the same snapshot a save uses, so a field that is neither
// saved nor restored shows up as a difference here.
func TestStateRoundTrip(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	fillStateSession(t, bank, songs, kit)
	loaded := roundTrip(t, bank, songs, []string{"kits/gm_drums.json"})
	assertSameSession(t, stateFrom(bank, songs, nil), stateFrom(loaded.bank, loaded.songs, nil))
}

// A measure refers to the pattern itself, so a load has to rebuild one pattern per index and
// share those pointers. A field-by-field restore would give each measure its own copy, and
// an edit through one measure would stop reaching the others.
func TestStateKeepsMeasuresSharingOnePattern(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(pattern, 0)
	song.SetPattern(pattern, 1)

	loaded := roundTrip(t, bank, songs, nil)
	first, second := loaded.songs.CurrentSong().GetPattern(0), loaded.songs.CurrentSong().GetPattern(1)
	require.NotNil(t, first, "measure 0 was not restored")
	require.NotNil(t, second, "measure 1 was not restored")
	require.Same(t, first, second, "the two measures stopped sharing one pattern after the load")
	require.Same(t, loaded.bank.Patterns[1], first, "the measure is not the object the bank selects")

	// An edit through the measure has to be the edit the bank sees.
	first.SetChromaticNote(0, loaded.kit.voices[2], 72, 100)
	event, ok := loaded.bank.Patterns[1].EventAtStep(0, loaded.kit.voices[2])
	require.True(t, ok, "editing the measure removed the note")
	require.Equal(t, 72, event.ChromaticNote, "editing the measure did not reach the pattern")
}

// A kit that has lost voices cannot supply the events naming them. Losing one drum must not
// cost the set, but the loss has to be counted rather than passing unnoticed.
func TestStateReportsVoicesTheKitNoLongerHas(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	// A percussive voice near the end of the kit, which the smaller kit will not have.
	pattern.ToggleEvent(Event{Voice: kit.voices[5], Beat: stepBeat(1), Velocity: 100})

	path := stateFilePath(t)
	saveTo(t, path, bank, songs, []string{"kits/gm_drums.json"})
	smaller := trackWindowKit(4, 2)
	far := stateTestBank(t, smaller)
	loaded := far.patbank
	report, err := loadInto(far, path, []string{"kits/gm_drums.json"})
	require.NoError(t, err)
	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{name: "dropped count", got: report.Dropped, want: 1},
		{name: "events kept", got: patternEventCount(loaded.Patterns[1]), want: 1},
	} {
		assert.Equalf(t, tt.want, tt.got, tt.name)
	}
	event, ok := loaded.Patterns[1].EventAtStep(0, smaller.voices[2])
	require.True(t, ok, "the surviving event was dropped")
	require.Equal(t, 60, event.ChromaticNote, "the surviving event changed pitch")
	_, kept := loaded.Patterns[1].EventAtStep(1, smaller.voices[2])
	require.False(t, kept, "an event naming a voice this kit lacks was kept anyway")
	require.Contains(t, report.loadText(), "dropped", "the readout should say what was dropped")
}

// The file records the kit it was written against, so a load against a different one says so
// rather than playing the wrong drum. The difference is reported, not refused: a kit edited
// to add an instrument has not damaged the set.
func TestStateRecordsAndReportsTheKit(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	written := []string{"kits/gm_drums.json", "my_leads.json"}
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, written)
	saved, err := readStateFile(path)
	require.NoError(t, err)
	require.Equal(t, written, saved.Kit)
	require.Equal(t, len(kit.voices), saved.VoiceCount)

	far := stateTestBank(t, stateKit())
	report, err := loadInto(far, path, []string{"kits/other.json"})
	require.NoErrorf(t, err, "a different kit path should be reported, not refused")
	require.Equal(t, 0, report.Dropped, "a kit of the same size should lose nothing")
	changed, reason := saved.kitChanged([]string{"kits/other.json"}, len(stateKit().voices))
	require.True(t, changed, "a different kit path should be reported")
	require.Contains(t, reason, "kit")
}

// The voice on each track is part of the set, not the view, and is not recoverable from the
// patterns: those record which voice sounds on a step, not which row it sits on.
func TestStateRestoresTheKitAssignment(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	// Move every track's voice away from where the bank starts it, so a load that fell back
	// to the defaults would be visibly wrong rather than coincidentally right.
	want := make([]int, len(bank.trackVoices))
	for row := 1; row <= len(want); row++ {
		require.NoError(t, bank.SelectTrackRow(row))
		require.NoError(t, bank.JogSelect(3))
	}
	copy(want, bank.trackVoices)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	// Wipe the bank's own assignment afterwards, so a load that fell back to the defaults
	// would be visibly wrong rather than coincidentally right.
	for index := range bank.trackVoices {
		bank.trackVoices[index] = 0
	}
	loadedKit := stateKit()
	far := stateTestBank(t, loadedKit)
	loaded := far.patbank
	_, err := loadInto(far, path, nil)
	require.NoError(t, err)
	require.Equal(t, want, loaded.trackVoices, "the kit assignment should come back whole")
	// The voices themselves have to belong to the kit that loaded the file.
	for row, index := range loaded.trackVoices {
		require.NotSamef(t, kit.voices[index], loadedKit.voices[index],
			"track %d resolved onto a voice of the writing kit", row+1)
	}
}

// Patterns are written in index order, with no trailing empty measures. Go randomises map
// iteration, so without that two saves of one session would differ and the file could not be
// diffed.
func TestStateSavesInIndexOrder(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	for _, index := range []int{40, 3, 17, 8} {
		require.NoError(t, bank.Jump(index))
		bank.CurrentPattern().SetChromaticNote(index%maxPatternSteps, kit.voices[2], 60+index%12, 90)
		song.SetPattern(bank.Patterns[bank.selPatIdx], index%8)
	}
	first := stateFrom(bank, songs, nil)
	for i := 1; i < len(first.Patterns); i++ {
		require.LessOrEqualf(t, first.Patterns[i-1].Index, first.Patterns[i].Index,
			"patterns are out of order at %d", i)
	}
	require.Len(t, first.Songs, 1)
	measures := first.Songs[0].Measures
	require.NotEqual(t, emptyMeasure, measures[len(measures)-1], "the trailing empties should be trimmed")

	// Two saves of the same session have to produce the same payload. The timestamp is the
	// one field that moves, and its length varies with the precision of the clock.
	again := stateFrom(bank, songs, nil)
	first.SavedAt = time.Time{}
	again.SavedAt = time.Time{}
	require.Equal(t, mustStateJSON(t, first), mustStateJSON(t, again), "two saves of one session differ")
}

// A song with no measures is not written to a file, so a bank sitting on one loads a set
// that has no such song. The selection is left where it was, so the slot has to be made
// here: the arrangement view reads the selected song on every pad press, and a nil one
// takes the whole view down.
func TestStateLoadKeepsTheSelectedSong(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	// Song 1 gets a measure so the file carries it, and song 7 stays empty so it does not.
	songs.CurrentSong().SetPattern(bank.Patterns[1], 0)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	require.NoError(t, songs.Jump(6))
	require.Equal(t, 7, songs.selSongIdx)

	withSession(session, path, nil)
	_, err := loadInto(session, path, nil)
	require.NoError(t, err)
	require.NotNilf(t, songs.CurrentSong(), "selected song %d is nil after the load; the bank holds %v",
		songs.selSongIdx, songs.Songs)
	// The arrangement view has to work against it, which is what reads the selected song.
	// A seek with nothing playing schedules nothing and is not an error.
	songs.JumpMeasure(0, 0)
}

// A song the bank was never on is still not the one the view is showing, so the load must
// not put it back: an empty song restores as an empty song rather than as a stale one.
func TestStateLoadEmptiesTheSelectedSong(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	songs.CurrentSong().SetPattern(bank.Patterns[1], 0)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 72, 100)

	withSession(session, path, nil)
	_, err := loadInto(session, path, nil)
	require.NoError(t, err)
	require.Samef(t, bank.Patterns[1], songs.CurrentSong().GetPattern(0), "measure 0 is not the saved pattern")
}

// The trailing measures of an arrangement are trimmed by the one rule SetPattern applies,
// so a save that trimmed them itself would have a second copy of that rule to keep in step.
// A gap in the middle is not a trailing measure and has to survive.
func TestStateSavesTheArrangementSetPatternWouldLeave(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	first := bank.CurrentPattern()
	require.NoError(t, bank.Jump(1))
	second := bank.CurrentPattern()
	require.NoError(t, songs.Jump(0))
	song := songs.CurrentSong()
	song.SetPattern(first, 0)
	song.SetPattern(first, 4) // a gap at 1-3 is interior and stays
	song.SetPattern(second, 7)
	song.SetPattern(nil, 8) // a trailing empty is not a measure the file should carry
	tail := len(song.measurePatterns())

	saved := stateFrom(bank, songs, nil)
	require.Len(t, saved.Songs, 1)
	measures := saved.Songs[0].Measures
	require.Equalf(t, tail, len(measures), "wrote the wrong number of measures")
	for measure, want := range map[int]int{0: 1, 1: emptyMeasure, 4: 1, 7: 2} {
		assert.Equalf(t, want, measures[measure], "measure %d", measure)
	}
}

// The readout row holds twenty characters, so a failure that puts the whole error there is
// cut off mid-sentence and says nothing. What fits has to be short enough to survive the
// cut and still name which failure it was.
func TestStateFailureTextFitsTheRow(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, "foreign.json")
	require.NoError(t, os.WriteFile(foreign, []byte(`{"app":"somethingelse"}`), 0o644))
	_, foreignErr := readStateFile(foreign)
	future := filepath.Join(dir, "future.json")
	require.NoError(t, os.WriteFile(future, []byte(`{"app":"fireloop","version":99}`), 0o644))
	_, versionErr := readStateFile(future)
	_, missingErr := os.Stat(filepath.Join(dir, "does-not-exist.json"))

	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{name: "no file", err: missingErr, want: "Load failed: no file"},
		{name: "another program's file", err: foreignErr, want: "Load failed: foreign"},
		{name: "a version this build cannot read", err: versionErr, want: "Load failed: version"},
		{name: "anything else", err: os.ErrPermission, want: "Load failed: error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text := stateFailureText("Load failed", tt.err)
			assert.Equal(t, tt.want, text)
			assert.Equalf(t, text, fitOLEDText(text), "readout %q would be cut on the row", text)
		})
	}
}

// Every message the panel writes has to be built to fit the row rather than trimmed by it,
// because a message cut at the width can end in a different number than it began with. A
// set large enough to overflow is ordinary, not a hand-edited file: the banks hold hundreds
// of patterns, and one against a smaller kit drops whatever the kit no longer has.
func TestReportTextFitsTheRow(t *testing.T) {
	for _, tt := range []struct {
		name   string
		report stateReport
	}{
		{name: "an empty set", report: stateReport{}},
		{name: "a whole set", report: stateReport{Patterns: maxPatternIndex, Songs: maxPatternIndex}},
		{name: "a few lost", report: stateReport{Patterns: 100, Dropped: 15}},
		{name: "a loss too long for both counts", report: stateReport{Patterns: 999, Dropped: 100}},
		{name: "more lost than a bank can hold", report: stateReport{Patterns: 999, Dropped: 16000}},
		{name: "nothing but a loss", report: stateReport{Dropped: 7}},
		{name: "a small file", report: stateReport{Bytes: 1024}},
		{name: "a file over a megabyte", report: stateReport{Bytes: 5 << 20}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, text := range []string{tt.report.loadText(), tt.report.saveText()} {
				assert.LessOrEqualf(t, len(text), oledTextWidth, "readout %q is too long for the row", text)
				assert.Equalf(t, text, fitOLEDText(text), "readout %q would be cut on the row", text)
			}
		})
	}
	// The counts a reader acts on have to survive whole, which is the part a plain width
	// check cannot see: a number cut short still fits.
	assert.Contains(t, (stateReport{Patterns: 100, Dropped: 15}).loadText(), "15",
		"the readout does not carry the whole dropped count")
	assert.Contains(t, (stateReport{Patterns: 999, Dropped: 16000}).loadText(), "16000",
		"the readout does not carry the whole dropped count")
}

// The session path is an argument to building the controller rather than something set
// afterwards, because a controller that is built and then reconfigured is one where a whole
// configuration can be dropped without anything noticing. This pins the path from the flag
// through construction to a save: -state was silently dead when run built a controller and
// then copied a fresh one over the top of it.
func TestControllerSavesThroughThePathItWasBuiltWith(t *testing.T) {
	path := stateFilePath(t)
	controller := newController(NewFire(func([]byte) error { return nil }), stateKit(), path)
	require.Equal(t, path, controller.sessionPath)
	require.NoError(t, controller.saveSession())
	waitFor(t, "a controller built with a session path to write it", func() bool {
		_, err := os.Stat(path)
		return err == nil
	})
}

// A save used to publish on the event loop, which held it for the fsync: on this
// filesystem that is tens of milliseconds of a button press nobody asked for. Only the
// write is off the loop now, and this is what that buys: the controls keep working while a
// publish is outstanding.
func TestAnOutstandingSaveDoesNotStallTheLoop(t *testing.T) {
	session := stateTestBank(t, stateKit())
	controller := withSession(session, stateFilePath(t), nil)
	bank := controller.patbank

	// Stand in for a publish that has not finished, released once the loop has come back.
	// Nothing on the event path waits for it, so a loop that did would deadlock here rather
	// than fail, which is the clearest way to show that it does not.
	controller.saves.Add(1)

	inc := make(chan alsa.SeqEvent, 2)
	inc <- padMessage(54, 100) // a pad press, which adds a step to the first track
	inc <- padMessage(54, 100) // and another, which removes it again
	// Closing rather than sending the leave request keeps the exit save, which does wait
	// for outstanding publishes, out of this test.
	close(inc)
	controller.processIncomingEvents(nil, inc)

	controller.saves.Done()

	// Both presses ran while the publish was outstanding: the step is there and then it is
	// not, which is what two presses on one pad do.
	_, left := bank.CurrentPattern().EventAtStep(0, bank.trackVoice(1))
	require.Falsef(t, left, "the step was left behind: two presses should cancel")
}

// A panic in the handler ends the process, so the session has to be on disk before it goes.
// Without that, the one save this feature exists for is the one a crash skips.
func TestLeavingSavesAfterAPanic(t *testing.T) {
	session := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	controller := withSession(session, path, nil)
	// The unit falls over while the handler is repainting the grid, which is after the edit
	// has been made. That is the case the exit save exists for: a half-finished gesture
	// reaching the disk rather than an untouched session.
	// Every way the bank talks to the unit has to be the failing one, because which of
	// the two seams a gesture happens to touch is not something this test should depend on.
	falling := NewFire(func(data []byte) error {
		// The pad repaint is the first thing the unit is sent after the edit lands, and
		// lights leave as a SysEx block rather than as a note.
		if isPadLight(data) {
			panic("the unit fell over after the edit")
		}
		return nil
	})
	controller.patbank.screen, controller.patbank.pads = falling, falling

	inc := make(chan alsa.SeqEvent, 1)
	inc <- padMessage(54, 100) // a pad press, which adds a step to the first track
	func() {
		defer func() {
			if problem := recover(); problem == nil {
				t.Fatal("the panic was swallowed, so a failure would leave the unit playing")
			}
		}()
		controller.processIncomingEvents(nil, inc)
	}()

	saved, err := readStateFile(path)
	require.NoErrorf(t, err, "the panic took the session with it")
	require.Lenf(t, saved.Patterns, 1, "the edit the handler was making should be the one saved")
	require.Len(t, saved.Patterns[0].Events, 1)
}

// A write publishes the whole file with a rename, so what is on disk is always one whole
// session: never a mixture of the old file and the new, and never nothing at all.
func TestStatePublishesOneWholeFile(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	dir := t.TempDir()
	path := filepath.Join(dir, "set.json")

	// write sets every step on the first track and saves, reporting the size.
	write := func() int64 {
		t.Helper()
		_, err := bank.ToggleEvent(0, 0, 100)
		require.NoError(t, err)
		saveTo(t, path, bank, songs, nil)
		info, err := os.Stat(path)
		require.NoError(t, err)
		return info.Size()
	}
	full := write()
	for range maxPatternSteps {
		write()
	}
	empty := write()
	require.Less(t, empty, full, "the file should shrink once the notes are off the track")

	state, err := readStateFile(path)
	require.NoErrorf(t, err, "the replaced file is not readable")
	for _, pattern := range state.Patterns {
		require.Emptyf(t, pattern.Events, "pattern %d still holds events after the notes were removed", pattern.Index)
	}

	// A save that cannot be written leaves the previous set exactly where it was, and says
	// so rather than reporting a file it never wrote.
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = saveState(filepath.Join(dir, "no-such-dir", "set.json"), bank, songs, nil)
	require.Error(t, err, "a save into a missing directory was accepted")
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "a failed save changed the file already there")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotEqualf(t, stateTempSuffix, filepath.Ext(entry.Name()), "a failed save left %s behind", entry.Name())
	}
}

// Saving is meant to be safe on stage, so a snapshot taken while the playback worker is
// running has to be the same snapshot taken while nothing is playing.
func TestStateSaveWhilePlayingMatchesSaveAtRest(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	fillStateSession(t, bank, songs, kit)
	atRest := stateFrom(bank, songs, nil)

	writer := &captureMidiWriter{}
	controller := bank.controller
	controller.startPlayback(writer, bank.newPlayback())
	// Let the worker get into the pattern and start moving the playhead.
	time.Sleep(20 * time.Millisecond)
	playing := stateFrom(bank, songs, nil)
	require.NoError(t, bank.controller.stopPlayback())
	require.NotEmpty(t, writer.events, "the playback worker wrote nothing, so this test proves nothing")
	assertSameSession(t, atRest, playing)
}

// patternEventCount reads a pattern's events through its own snapshot rather than reaching
// into the collection, so a test cannot take a reading while an edit is landing.
func patternEventCount(pattern *Pattern) int {
	events, _ := pattern.snapshot()
	return len(events)
}

// stateEventsText renders a restored pattern as its steps and ties, so a table case states
// its expectation in one line rather than a block of per-event assertions.
func stateEventsText(bank *PatternBank, index int) string {
	if pattern := bank.Patterns[index]; pattern != nil {
		events, _ := pattern.snapshot()
		var steps []string
		for _, event := range events {
			text := fmt.Sprintf("step%d", eventStep(event))
			if event.Tie {
				text += ":tie"
			}
			steps = append(steps, text)
		}
		return strings.Join(steps, " ")
	}
	return ""
}

// A file that is not a session, or that carries values the grid or kit cannot supply, is
// what a truncated or hand-edited file looks like. One that cannot be trusted is refused
// whole; one that is merely wrong is taken as far as it goes and says what it could not use.
func TestStateHandlesUnusableFiles(t *testing.T) {
	chromatic := func() *VoiceBank { return trackWindowKit(2, 0) }
	for _, tt := range []struct {
		name string
		kit  func() *VoiceBank
		file stateFile
		// refuse is the error a file must not be applied from; wantEvents is what a
		// pattern must look like once it is.
		refuse     string
		wantEvents string
	}{
		{
			name:   "not a fireloop session",
			kit:    stateKit,
			file:   stateFile{App: "somethingelse", Version: stateVersion},
			refuse: "not a fireloop session",
		},
		{
			name:   "a version this build cannot read",
			kit:    stateKit,
			file:   stateFile{App: stateApp, Version: stateVersion + 1},
			refuse: "version",
		},
		{
			name:   "a version from before",
			kit:    stateKit,
			file:   stateFile{App: stateApp, Version: stateVersion - 1},
			refuse: "version",
		},
		{
			name: "a tie with no successor is repaired",
			kit:  chromatic,
			file: stateFile{App: stateApp, Version: stateVersion, VoiceCount: 2, Patterns: []statePattern{{
				Index: 1,
				Events: []stateEvent{
					{Voice: 0, Step: 0, Velocity: 100, Tie: true},
					{Voice: 0, Step: 2, Velocity: 100, Tie: true},
				},
			}}},
			wantEvents: "step0:tie step2",
		},
		{
			name: "an event past the restored length is dropped",
			kit:  chromatic,
			file: stateFile{App: stateApp, Version: stateVersion, VoiceCount: 1, Patterns: []statePattern{{
				Index:       1,
				LengthSteps: 4,
				Events: []stateEvent{
					{Voice: 0, Step: 0, Velocity: 100, Tie: true},
					{Voice: 0, Step: 9, Velocity: 100},
				},
			}}},
			wantEvents: "step0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bank, _, err := loadCrafted(t, tt.file, tt.kit())
			if tt.refuse != "" {
				require.Error(t, err, "the load was accepted")
				require.Contains(t, err.Error(), tt.refuse)
				// A refused file changes nothing that was already on the display.
				require.Len(t, bank.Patterns, 1, "a refused load changed the running session")
				return
			}
			require.NoError(t, err)
			got := stateEventsText(bank, 1)
			if tt.wantEvents != "" {
				assert.Equal(t, tt.wantEvents, got)
			}
		})
	}
	_, err := readStateFile(filepath.Join(t.TempDir(), "does-not-exist.json"))
	require.Error(t, err, "a missing file was not an error")
}

// A set with one bad number in it is still mostly a set, so out-of-range values are clamped
// and the ones that cost a note or a measure are counted.
func TestStateClampsOutOfRangeValues(t *testing.T) {
	kit := trackWindowKit(2, 1)
	bank, report, err := loadCrafted(t, stateFile{
		App:         stateApp,
		Version:     stateVersion,
		VoiceCount:  2,
		BPM:         900,
		TrackVoices: []int{1, 9, -4},
		Patterns: []statePattern{{
			Index:       2,
			LengthSteps: 99,
			Events: []stateEvent{
				{Voice: 0, Step: 3, Velocity: 200},
				{Voice: 0, Step: 99},
				{Voice: 44, Step: 4},
			},
		}},
		Songs: []stateSong{{Index: 1, Measures: []int{2, 7}}},
	}, kit)
	require.NoError(t, err)
	pattern := bank.Patterns[2]
	require.NotNil(t, pattern, "the pattern the file names is missing after the load")
	velocity, _ := pattern.EventAtStep(3, kit.voices[0])
	voicesInKit := 0
	for _, voice := range bank.trackVoices {
		if voice >= 0 && voice < len(kit.voices) {
			voicesInKit++
		}
	}
	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{name: "tempo", got: currentBPM(), want: stateTempoMax},
		{name: "pattern length", got: pattern.LengthSteps(), want: maxPatternSteps},
		{name: "event velocity", got: velocity.Velocity, want: midiNoteMax},
		{name: "dropped count", got: report.Dropped, want: 3},
		{name: "events kept", got: patternEventCount(pattern), want: 1},
		{name: "tracks padded out to the pad rows", got: len(bank.trackVoices), want: padRows},
		{name: "tracks holding a voice of this kit", got: voicesInKit, want: padRows},
		// Where the user was looking is not part of a session, so a load leaves the
		// selection alone rather than restoring a position the file does not carry.
		{name: "selected pattern", got: bank.selPatIdx, want: 1},
	} {
		assert.Equalf(t, tt.want, tt.got, tt.name)
	}
}

// An index the banks have no room for is refused and counted rather than clamped. Clamping
// a song index would land the arrangement on a measure the file never described, which is
// the one outcome a load must not produce.
func TestStateRefusesIndexesTheBankHasNoRoomFor(t *testing.T) {
	bank, report, err := loadCrafted(t, stateFile{
		App:        stateApp,
		Version:    stateVersion,
		VoiceCount: 2,
		BPM:        defaultBPM,
		Patterns:   []statePattern{{Index: 0}, {Index: maxPatternIndex + 1}, {Index: 4}},
		Songs: []stateSong{
			{Index: 0},
			{Index: maxPatternIndex + 1},
			{Index: 2, Measures: []int{4}},
		},
	}, trackWindowKit(2, 0))
	require.NoError(t, err)
	require.Equal(t, 4, report.Dropped, "the two patterns and two songs the bank has no room for")
	require.NotNil(t, bank.Patterns[4], "the pattern the bank does have room for was not installed")
	require.Nil(t, bank.Patterns[0], "a refused pattern index was installed anyway")
	require.Nil(t, bank.Patterns[maxPatternIndex+1], "a refused pattern index was installed anyway")
}

// Shift with Browser saves and Shift with Accent loads, in either mode. Neither button does
// anything on its own, so a stray press cannot overwrite a set or replace one.
func TestStateGesturesSaveAndLoad(t *testing.T) {
	for _, tt := range []struct {
		name string
		song bool
	}{{name: "pattern mode"}, {name: "song mode", song: true}} {
		t.Run(tt.name, func(t *testing.T) {
			kit := stateKit()
			session := stateTestBank(t, kit)
			bank, songs := session.patbank, session.songbank
			pattern := bank.CurrentPattern()
			pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
			song := songs.Songs[songs.selSongIdx]
			song.SetPattern(pattern, 0)
			song.SetPattern(pattern, 3)
			path := stateFilePath(t)
			controller := withSession(session, path, nil)
			recorder := useScreenRecorder(t, &bank.screen)
			readout := func() string { return recorder.row(readoutRow) }
			if tt.song {
				controller.mode = songView
			}
			t.Cleanup(func() { controller.mode = patternView })

			for _, note := range []int{NoteBrowser, NoteAccent} {
				pressButton(t, bank, note)
			}
			_, err := os.Stat(path)
			require.True(t, os.IsNotExist(err), "a plain press of Browser wrote the state file")

			pressButton(t, bank, NoteShift)
			pressButton(t, bank, NoteBrowser)
			// The publish runs off the event loop, so the press is answered before the
			// file exists. Both the file and the row it reported on arrive together.
			waitFor(t, "the off-loop save to land", func() bool {
				_, err := os.Stat(path)
				return err == nil
			})
			waitFor(t, "the save to report itself", func() bool {
				return strings.HasPrefix(readout(), "Saved")
			})

			// Shift stays engaged for the whole gesture, the way a held modifier does, so
			// the load below is a second press rather than a new one.
			bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 72, 100)
			pressButton(t, bank, NoteAccent)
			assert.Truef(t, strings.HasPrefix(readout(), "Loaded"), "readout = %q, want it to report the load", readout())
			event, _ := bank.Patterns[1].EventAtStep(0, kit.voices[2])
			assert.Equalf(t, 60, event.ChromaticNote, "pitch after the load")
			first, second := songs.CurrentSong().GetPattern(0), songs.CurrentSong().GetPattern(3)
			if assert.NotNil(t, first, "measure 0 holds nothing") {
				assert.Same(t, first, second, "both measures should hold the same pattern")
			}
			pressButton(t, bank, NoteShift)
		})
	}
}

// With no -state path there is nowhere to save, and the unit says so rather than writing
// somewhere it was not asked to.
func TestStateGesturesWithoutAPathSaySo(t *testing.T) {
	session := stateTestBank(t, stateKit())
	bank := session.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	for _, gesture := range []struct {
		name string
		run  func() error
	}{
		{name: "save", run: session.saveSession},
		{name: "load", run: session.loadSession},
	} {
		require.NoErrorf(t, gesture.run(), "the %s gesture reported an error", gesture.name)
		assert.Containsf(t, recorder.row(readoutRow), "-state",
			"readout after %s should say there is no state path", gesture.name)
	}
}

// A load that cannot be read reports why and leaves the running session alone.
func TestStateLoadFailureLeavesTheSessionAlone(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank := session.patbank
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	path := stateFilePath(t)
	controller := withSession(session, path, nil)
	recorder := useScreenRecorder(t, &bank.screen)
	for _, tt := range []struct {
		name    string
		content string
	}{
		{name: "no file", content: ""},
		{name: "not a session", content: "this is not a session"},
	} {
		if tt.content != "" {
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o644))
		} else {
			err := os.Remove(path)
			require.Truef(t, err == nil || os.IsNotExist(err), "removing the file: %v", err)
		}
		require.NoErrorf(t, controller.loadSession(), "%s", tt.name)
		assert.Containsf(t, recorder.row(readoutRow), "Load failed",
			"%s: the failure should be reported", tt.name)
		_, ok := bank.Patterns[1].EventAtStep(0, kit.voices[2])
		require.Truef(t, ok, "%s: a failed load changed the running session", tt.name)
	}
}

// A load repaints the view it was asked from, so the display agrees with what was installed.
func TestStateLoadRepaintsTheCurrentView(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(pattern, 0)
	song.SetPattern(pattern, 5)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	controller := withSession(session, path, nil)
	recorder := useScreenRecorder(t, &bank.screen)
	// Both banks draw through one recorder on purpose: which view a load repaints is the
	// thing under test, so the same rows have to be readable whichever mode is showing.
	songs.screen = recorder

	for _, tt := range []struct {
		name  string
		song  bool
		want  string
		other string
	}{
		{name: "pattern mode", song: false, want: "Pattern 001"},
		{name: "song mode", song: true, want: "Song 001", other: "M 001-048"},
	} {
		controller.mode = patternView
		if tt.song {
			controller.mode = songView
		}
		require.NoErrorf(t, controller.loadSession(), "%s", tt.name)
		assert.Containsf(t, recorder.row(0), tt.want, "%s header", tt.name)
		if tt.other != "" {
			assert.Containsf(t, recorder.row(3), tt.other, "%s view", tt.name)
		}
	}
}

// Leaving saves the session on the goroutine that owns the banks, so the save comes after
// whatever that goroutine was already doing: reading the pattern map while the handler is
// writing it is a fatal concurrent access, not a lost note. It is written once, however the
// exit is reached.
func TestLeavingSavesBehindTheQueuedEditsOnce(t *testing.T) {
	session := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	controller := withSession(session, path, nil)

	inc := make(chan alsa.SeqEvent, 2)
	inc <- padMessage(54, 100) // a pad press, which adds a step to the first track
	inc <- alsa.SeqEvent{}     // then the leave request
	controller.processIncomingEvents(nil, inc)

	saved, err := os.ReadFile(path)
	require.NoErrorf(t, err, "leaving did not leave a readable file")
	state, err := readStateFile(path)
	require.NoError(t, err)
	require.Len(t, state.Patterns, 1, "the queued edit should land before the save")
	require.Len(t, state.Patterns[0].Events, 1)

	controller.saveSessionOnExit() // reaching the exit by another route must not write again
	again, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(saved), string(again), "the session was written a second time")
}

// An untouched pattern is not written out: every pattern the user lands on has an entry, and
// hundreds of empty placeholders make the file harder to read without making it more
// complete. A shortened empty one still goes in, and comes back even though the selection
// does not.
func TestStateOmitsUntouchedPatternsButKeepsShortenedOnes(t *testing.T) {
	kit := stateKit()
	session := stateTestBank(t, kit)
	bank, songs := session.patbank, session.songbank
	for _, index := range []int{2, 5, 9, 6} {
		require.NoError(t, bank.Jump(index))
	}
	shortened := bank.selPatIdx
	bank.CurrentPattern().SetLengthSteps(4)
	state := stateFrom(bank, songs, nil)
	require.Len(t, state.Patterns, 1, "only the shortened pattern should be written")
	require.Equalf(t, shortened, state.Patterns[0].Index, "the wrong pattern was written")
	require.Equal(t, 4, state.Patterns[0].LengthSteps)

	path := stateFilePath(t)
	_, err := writeStateFile(path, state)
	require.NoError(t, err)
	far := stateTestBank(t, stateKit())
	loaded := far.patbank
	_, err = loadInto(far, path, nil)
	require.NoError(t, err)
	require.Equal(t, 4, loaded.Patterns[shortened].LengthSteps(), "restored length")

	// The bank stays on whatever pattern it was showing: the selection is the display's
	// business, and the file does not carry it.
	require.NotEqualf(t, shortened, loaded.selPatIdx, "the selection should be left where it was")
}
