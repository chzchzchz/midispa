package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

// stateKit is a kit with one chromatic voice and the rest percussive, so a session has to
// carry both kinds of event across a save. It is built fresh each time: a loaded session
// resolves onto the kit of the run that loaded it, not the one that wrote the file.
func stateKit() *VoiceBank {
	return trackWindowKit(6, 2)
}

func stateTestBank(t *testing.T, kit *VoiceBank) (*PatternBank, *SongBank) {
	t.Helper()
	fire := NewFire(func([]byte) error { return nil })
	bank := NewPatternBank(fire, kit)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	return bank, NewSongBank(fire, bank)
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
	if _, err := saveState(path, bank, songs, kit); err != nil {
		t.Fatal(err)
	}
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
func roundTrip(t *testing.T, bank *PatternBank, songs *SongBank, kit []string) loadedSession {
	t.Helper()
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, kit)
	loadedKit := stateKit()
	loadedBank, loadedSongs := stateTestBank(t, loadedKit)
	report, err := loadState(path, loadedBank, loadedSongs, kit)
	if err != nil {
		t.Fatal(err)
	}
	return loadedSession{kit: loadedKit, bank: loadedBank, songs: loadedSongs, report: report}
}

// loadCrafted writes a hand-built file and loads it, for the tests about a truncated,
// hand-edited or foreign file. The error is returned so a case can assert a refusal.
func loadCrafted(t *testing.T, file stateFile, kit *VoiceBank) (*PatternBank, stateReport, error) {
	t.Helper()
	path := stateFilePath(t)
	if _, err := writeStateFile(path, file); err != nil {
		t.Fatal(err)
	}
	bank, songs := stateTestBank(t, kit)
	report, err := loadState(path, bank, songs, nil)
	return bank, report, err
}

// fillStateSession writes through the production editing calls, so the state under test is
// one the controls can actually produce: percussive steps, chromatic notes, a tie, a
// shortened pattern, a song, a remapped track voice, and a tempo.
func fillStateSession(t *testing.T, bank *PatternBank, songs *SongBank, kit *VoiceBank) {
	t.Helper()
	chromatic := kit.voices[2]
	pattern := bank.CurrentPattern()
	for _, col := range []int{0, 4, 8} {
		if _, err := bank.ToggleEvent(0, col, 100); err != nil {
			t.Fatal(err)
		}
	}
	pattern.SetChromaticNote(2, chromatic, 60, 100)
	pattern.SetChromaticNote(5, chromatic, 62, 90)
	if !pattern.TieEventsAtSteps(2, 5, chromatic) {
		t.Fatal("tie was refused on the pattern being written")
	}
	if got := pattern.SetLengthSteps(8); got != 8 {
		t.Fatalf("pattern length = %d, want 8", got)
	}
	if _, err := bank.ToggleEvent(1, 3, 100); err != nil {
		t.Fatal(err)
	}

	// A second pattern, so the file has more than one and the song has something to pick.
	// Jump is a relative move, so this is a step rather than a pattern number.
	if err := bank.Jump(2); err != nil {
		t.Fatal(err)
	}
	third := bank.selPatIdx
	second := bank.CurrentPattern()
	second.SetChromaticNote(1, chromatic, 64, 70)
	if _, err := bank.ToggleEvent(0, 1, 100); err != nil {
		t.Fatal(err)
	}

	// The song places the first pattern in two measures and the second in another, which is
	// what proves the measures and the bank end up sharing one pattern each after a load.
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(bank.Patterns[1], 0)
	song.SetPattern(bank.Patterns[1], 1)
	song.SetPattern(second, 4)
	if bank.Patterns[third] != second {
		t.Fatalf("pattern %d is not the one the bank selected", third)
	}

	// Only the track's voice, which is part of the set. Scrolling the window, moving the
	// cursor and turning the palette are view state and are no longer saved, so setting
	// them here would only make the round trip look busier than it is.
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.JogSelect(2); err != nil {
		t.Fatal(err)
	}
	setBPM(150)
}

func mustStateJSON(t *testing.T, state stateFile) string {
	t.Helper()
	data, err := json.MarshalIndent(state, "", stateIndent)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// assertSameSession compares everything a save holds. The timestamp is the one field a round
// trip cannot reproduce, and the only reason two files of one session differ.
func assertSameSession(t *testing.T, want, got stateFile) {
	t.Helper()
	want.SavedAt = time.Time{}
	got.SavedAt = time.Time{}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("session changed across the round trip\nwant:\n%s\ngot:\n%s", mustStateJSON(t, want), mustStateJSON(t, got))
	}
}

// A session has to come back whole: every note, tie, length, track assignment and tempo.
// The comparison runs through the same snapshot a save uses, so a field that is neither
// saved nor restored shows up as a difference here.
func TestStateRoundTrip(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	fillStateSession(t, bank, songs, kit)
	loaded := roundTrip(t, bank, songs, []string{"kits/gm_drums.json"})
	assertSameSession(t, stateFrom(bank, songs, nil), stateFrom(loaded.bank, loaded.songs, nil))
}

// A measure refers to the pattern itself, so a load has to rebuild one pattern per index and
// share those pointers. A field-by-field restore would give each measure its own copy, and
// an edit through one measure would stop reaching the others.
func TestStateKeepsMeasuresSharingOnePattern(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(pattern, 0)
	song.SetPattern(pattern, 1)

	loaded := roundTrip(t, bank, songs, nil)
	first, second := loaded.songs.CurrentSong().GetPattern(0), loaded.songs.CurrentSong().GetPattern(1)
	if first == nil || second == nil {
		t.Fatalf("measures = %v/%v, want both restored", first, second)
	}
	if first != second {
		t.Fatal("the two measures stopped sharing one pattern after the load")
	}
	if first != loaded.bank.Patterns[1] {
		t.Fatal("the measure is not the same object as the pattern the bank selects")
	}
	// An edit through the measure has to be the edit the bank sees.
	first.SetChromaticNote(0, loaded.kit.voices[2], 72, 100)
	if event, ok := loaded.bank.Patterns[1].EventAtStep(0, loaded.kit.voices[2]); !ok || event.ChromaticNote != 72 {
		t.Fatalf("editing the measure did not reach the pattern: %+v/%v", event, ok)
	}
}

// A kit that has lost voices cannot supply the events naming them. Losing one drum must not
// cost the set, but the loss has to be counted rather than passing unnoticed.
func TestStateReportsVoicesTheKitNoLongerHas(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	// A percussive voice near the end of the kit, which the smaller kit will not have.
	pattern.ToggleEvent(Event{Voice: kit.voices[5], Beat: stepBeat(1), Velocity: 100})

	path := stateFilePath(t)
	saveTo(t, path, bank, songs, []string{"kits/gm_drums.json"})
	smaller := trackWindowKit(4, 2)
	loaded, loadedSongs := stateTestBank(t, smaller)
	report, err := loadState(path, loaded, loadedSongs, []string{"kits/gm_drums.json"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{name: "dropped count", got: report.Dropped, want: 1},
		{name: "events kept", got: len(loaded.Patterns[1].Events), want: 1},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
	if event, ok := loaded.Patterns[1].EventAtStep(0, smaller.voices[2]); !ok || event.ChromaticNote != 60 {
		t.Fatalf("the surviving event = %+v/%v, want it kept", event, ok)
	}
	if _, ok := loaded.Patterns[1].EventAtStep(1, smaller.voices[2]); ok {
		t.Fatal("an event named a voice this kit does not have and was kept anyway")
	}
	if !strings.Contains(report.loadText(), "dropped") {
		t.Fatalf("report text = %q, want it to say what was dropped", report.loadText())
	}
}

// The file records the kit it was written against, so a load against a different one says so
// rather than playing the wrong drum. The difference is reported, not refused: a kit edited
// to add an instrument has not damaged the set.
func TestStateRecordsAndReportsTheKit(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	written := []string{"kits/gm_drums.json", "my_leads.json"}
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, written)
	saved, err := readStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Kit, written) {
		t.Fatalf("kit = %v, want %v", saved.Kit, written)
	}
	if saved.VoiceCount != len(kit.voices) {
		t.Fatalf("voice count = %d, want %d", saved.VoiceCount, len(kit.voices))
	}
	loaded, loadedSongs := stateTestBank(t, stateKit())
	report, err := loadState(path, loaded, loadedSongs, []string{"kits/other.json"})
	if err != nil {
		t.Fatalf("a different kit path refused the load: %v", err)
	}
	if report.Dropped != 0 {
		t.Fatalf("dropped = %d for a kit of the same size, want nothing lost", report.Dropped)
	}
	changed, reason := saved.kitChanged([]string{"kits/other.json"}, len(stateKit().voices))
	if !changed || !strings.Contains(reason, "kit") {
		t.Fatalf("kit change = %v/%q, want it reported", changed, reason)
	}
}

// The voice on each track is part of the set, not the view, and is not recoverable from the
// patterns: those record which voice sounds on a step, not which row it sits on.
func TestStateRestoresTheKitAssignment(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	// Move every track's voice away from where the bank starts it, so a load that fell back
	// to the defaults would be visibly wrong rather than coincidentally right.
	want := make([]int, len(bank.trackVoices))
	for row := 1; row <= len(want); row++ {
		if err := bank.SelectTrackRow(row); err != nil {
			t.Fatal(err)
		}
		if err := bank.JogSelect(3); err != nil {
			t.Fatal(err)
		}
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
	loaded, loadedSongs := stateTestBank(t, loadedKit)
	if _, err := loadState(path, loaded, loadedSongs, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.trackVoices, want) {
		t.Fatalf("track voices = %v, want the kit assignment %v", loaded.trackVoices, want)
	}
	// The voices themselves have to belong to the kit that loaded the file.
	for row, index := range loaded.trackVoices {
		if loadedKit.voices[index] == kit.voices[index] {
			t.Fatalf("track %d resolved onto a voice of the writing kit, not the loading one", row+1)
		}
	}
}

// Patterns are written in index order, with no trailing empty measures. Go randomises map
// iteration, so without that two saves of one session would differ and the file could not be
// diffed.
func TestStateSavesInIndexOrder(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	for _, index := range []int{40, 3, 17, 8} {
		if err := bank.Jump(index); err != nil {
			t.Fatal(err)
		}
		bank.CurrentPattern().SetChromaticNote(index%maxPatternSteps, kit.voices[2], 60+index%12, 90)
		song.SetPattern(bank.Patterns[bank.selPatIdx], index%8)
	}
	first := stateFrom(bank, songs, nil)
	for i := 1; i < len(first.Patterns); i++ {
		if first.Patterns[i-1].Index > first.Patterns[i].Index {
			t.Fatalf("patterns are out of order at %d: %d then %d",
				i, first.Patterns[i-1].Index, first.Patterns[i].Index)
		}
	}
	if len(first.Songs) != 1 {
		t.Fatalf("songs = %d, want one", len(first.Songs))
	}
	measures := first.Songs[0].Measures
	if measures[len(measures)-1] == emptyMeasure {
		t.Fatalf("measures = %v, want the trailing empties trimmed", measures)
	}
	// Two saves of the same session have to produce the same payload. The timestamp is the
	// one field that moves, and its length varies with the precision of the clock.
	again := stateFrom(bank, songs, nil)
	first.SavedAt = time.Time{}
	again.SavedAt = time.Time{}
	if mustStateJSON(t, first) != mustStateJSON(t, again) {
		t.Fatalf("two saves of one session differ:\n%s\n%s", mustStateJSON(t, first), mustStateJSON(t, again))
	}
}

// A song with no measures is not written to a file, so a bank sitting on one loads a set
// that has no such song. The selection is left where it was, so the slot has to be made
// here: the arrangement view reads the selected song on every pad press, and a nil one
// takes the whole view down.
func TestStateLoadKeepsTheSelectedSong(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	// Song 1 gets a measure so the file carries it, and song 7 stays empty so it does not.
	songs.CurrentSong().SetPattern(bank.Patterns[1], 0)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	if err := songs.Jump(6); err != nil {
		t.Fatal(err)
	}
	if songs.selSongIdx != 7 {
		t.Fatalf("selected song = %d, want 7", songs.selSongIdx)
	}

	useStateGlobals(t, path, bank, songs, nil)
	if _, err := loadState(path, bank, songs, nil); err != nil {
		t.Fatal(err)
	}
	if songs.CurrentSong() == nil {
		t.Fatalf("selected song %d is nil after the load; the bank holds %v", songs.selSongIdx, songs.Songs)
	}
	// The arrangement view has to work against it, which is what reads the selected song.
	songs.playback = &Playback{}
	songs.JumpMeasure(0, 0)
}

// A song the bank was never on is still not the one the view is showing, so the load must
// not put it back: an empty song restores as an empty song rather than as a stale one.
func TestStateLoadEmptiesTheSelectedSong(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	songs.CurrentSong().SetPattern(bank.Patterns[1], 0)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 72, 100)

	useStateGlobals(t, path, bank, songs, nil)
	if _, err := loadState(path, bank, songs, nil); err != nil {
		t.Fatal(err)
	}
	if pattern := songs.CurrentSong().GetPattern(0); pattern != bank.Patterns[1] {
		t.Fatalf("measure 0 = %p, want the saved pattern %p", pattern, bank.Patterns[1])
	}
}

// The trailing measures of an arrangement are trimmed by the one rule SetPattern applies,
// so a save that trimmed them itself would have a second copy of that rule to keep in step.
// A gap in the middle is not a trailing measure and has to survive.
func TestStateSavesTheArrangementSetPatternWouldLeave(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	first := bank.CurrentPattern()
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	second := bank.CurrentPattern()
	if err := songs.Jump(0); err != nil {
		t.Fatal(err)
	}
	song := songs.CurrentSong()
	song.SetPattern(first, 0)
	song.SetPattern(first, 4) // a gap at 1-3 is interior and stays
	song.SetPattern(second, 7)
	song.SetPattern(nil, 8) // a trailing empty is not a measure the file should carry
	tail := len(song.measurePatterns())

	saved := stateFrom(bank, songs, nil)
	if len(saved.Songs) != 1 {
		t.Fatalf("wrote %d songs, want one", len(saved.Songs))
	}
	measures := saved.Songs[0].Measures
	if len(measures) != tail {
		t.Fatalf("wrote %d measures, want the %d SetPattern leaves", len(measures), tail)
	}
	for measure, want := range map[int]int{0: 1, 1: emptyMeasure, 4: 1, 7: 2} {
		if measures[measure] != want {
			t.Errorf("measure %d = %d, want %d", measure, measures[measure], want)
		}
	}
}

// The readout row holds twenty characters, so a failure that puts the whole error there is
// cut off mid-sentence and says nothing. What fits has to be short enough to survive the
// cut and still name which failure it was.
func TestStateFailureTextFitsTheRow(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, "foreign.json")
	if err := os.WriteFile(foreign, []byte(`{"app":"somethingelse"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, foreignErr := readStateFile(foreign)
	future := filepath.Join(dir, "future.json")
	if err := os.WriteFile(future, []byte(`{"app":"fireloop","version":99}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
			if text != tt.want {
				t.Errorf("readout = %q, want %q", text, tt.want)
			}
			if fitted := fitOLEDText(text); fitted != text {
				t.Errorf("readout %q would be cut to %q on the row", text, fitted)
			}
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
				if len(text) > oledTextWidth {
					t.Errorf("readout %q is %d characters, want at most %d", text, len(text), oledTextWidth)
				}
				if fitted := fitOLEDText(text); fitted != text {
					t.Errorf("readout %q would be cut to %q on the row", text, fitted)
				}
			}
		})
	}
	// The counts a reader acts on have to survive whole, which is the part a plain width
	// check cannot see: a number cut short still fits.
	if text := (stateReport{Patterns: 100, Dropped: 15}).loadText(); !strings.Contains(text, "15") {
		t.Errorf("readout %q does not carry the whole dropped count", text)
	}
	if text := (stateReport{Patterns: 999, Dropped: 16000}).loadText(); !strings.Contains(text, "16000") {
		t.Errorf("readout %q does not carry the whole dropped count", text)
	}
}

// A panic in the handler ends the process, so the session has to be on disk before it goes.
// Without that, the one save this feature exists for is the one a crash skips.
func TestLeavingSavesAfterAPanic(t *testing.T) {
	bank, songs := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	exitSaveOnce = sync.Once{} // the once is process-wide
	useStateGlobals(t, path, bank, songs, nil)
	previousProcess := processEvent
	t.Cleanup(func() { processEvent = previousProcess })
	processEvent = func(aseq sequencerWriter, ev alsa.SeqEvent) error {
		// The edit lands through the production handler first, so what the save has to
		// catch is a half-finished gesture rather than an untouched session.
		if err := processPatternEvent(aseq, ev); err != nil {
			return err
		}
		panic("the handler fell over after the edit")
	}

	inc := make(chan alsa.SeqEvent, 1)
	inc <- padMessage(54, 100) // a pad press, which adds a step to the first track
	func() {
		defer func() {
			if problem := recover(); problem == nil {
				t.Fatal("the panic was swallowed, so a failure would leave the unit playing")
			}
		}()
		processIncomingEvents(nil, inc)
	}()

	saved, err := readStateFile(path)
	if err != nil {
		t.Fatalf("the panic took the session with it: %v", err)
	}
	if len(saved.Patterns) != 1 || len(saved.Patterns[0].Events) != 1 {
		t.Fatalf("saved patterns = %+v, want the edit the handler was making", saved.Patterns)
	}
}

// A write publishes the whole file with a rename, so what is on disk is always one whole
// session: never a mixture of the old file and the new, and never nothing at all.
func TestStatePublishesOneWholeFile(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	dir := t.TempDir()
	path := filepath.Join(dir, "set.json")

	// write sets every step on the first track and saves, reporting the size.
	write := func() int64 {
		t.Helper()
		if _, err := bank.ToggleEvent(0, 0, 100); err != nil {
			t.Fatal(err)
		}
		saveTo(t, path, bank, songs, nil)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Size()
	}
	full := write()
	for range maxPatternSteps {
		write()
	}
	if empty := write(); empty >= full {
		t.Fatalf("the file did not shrink: %d then %d bytes", full, empty)
	}
	state, err := readStateFile(path)
	if err != nil {
		t.Fatalf("the replaced file is not readable: %v", err)
	}
	for _, pattern := range state.Patterns {
		if len(pattern.Events) > 0 {
			t.Fatalf("pattern %d still holds %d events after the notes were removed", pattern.Index, len(pattern.Events))
		}
	}

	// A save that cannot be written leaves the previous set exactly where it was, and says
	// so rather than reporting a file it never wrote.
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saveState(filepath.Join(dir, "no-such-dir", "set.json"), bank, songs, nil); err == nil {
		t.Fatal("a save into a missing directory was accepted")
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(before) {
		t.Fatal("a failed save changed the file that was already there")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), stateTempSuffix) {
			t.Fatalf("a failed save left %s behind", entry.Name())
		}
	}
}

// Saving is meant to be safe on stage, so a snapshot taken while the playback worker is
// running has to be the same snapshot taken while nothing is playing.
func TestStateSaveWhilePlayingMatchesSaveAtRest(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	usePatternGlobals(t, bank)
	fillStateSession(t, bank, songs, kit)
	atRest := stateFrom(bank, songs, nil)

	writer := &captureMidiWriter{}
	playbackStop = bank.startSequencer(writer)
	// Let the worker get into the pattern and start moving the playhead.
	time.Sleep(20 * time.Millisecond)
	playing := stateFrom(bank, songs, nil)
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) == 0 {
		t.Fatal("the playback worker wrote nothing, so this test proves nothing")
	}
	assertSameSession(t, atRest, playing)
}

// stateEventsText renders a restored pattern as its steps and ties, so a table case states
// its expectation in one line rather than a block of per-event assertions.
func stateEventsText(bank *PatternBank, index int) string {
	if pattern := bank.Patterns[index]; pattern != nil {
		var steps []string
		for _, event := range pattern.Events {
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
				if err == nil {
					t.Fatal("the load was accepted")
				}
				if !strings.Contains(err.Error(), tt.refuse) {
					t.Fatalf("error = %q, want it to mention %q", err, tt.refuse)
				}
				// A refused file changes nothing that was already on the display.
				if len(bank.Patterns) != 1 {
					t.Fatal("a refused load changed the running session")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := stateEventsText(bank, 1); tt.wantEvents != "" && got != tt.wantEvents {
				t.Errorf("restored events = %q, want %q", got, tt.wantEvents)
			}
		})
	}
	if _, err := readStateFile(filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatal("a missing file was not an error")
	}
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
	if err != nil {
		t.Fatal(err)
	}
	pattern := bank.Patterns[2]
	if pattern == nil {
		t.Fatal("the pattern the file names is missing after the load")
	}
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
		{name: "events kept", got: len(pattern.Events), want: 1},
		{name: "tracks padded out to the pad rows", got: len(bank.trackVoices), want: padRows},
		{name: "tracks holding a voice of this kit", got: voicesInKit, want: padRows},
		// Where the user was looking is not part of a session, so a load leaves the
		// selection alone rather than restoring a position the file does not carry.
		{name: "selected pattern", got: bank.selPatIdx, want: 1},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
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
	if err != nil {
		t.Fatal(err)
	}
	if report.Dropped != 4 {
		t.Errorf("dropped = %d, want the two patterns and two songs the bank has no room for", report.Dropped)
	}
	if bank.Patterns[4] == nil {
		t.Error("the pattern the bank does have room for was not installed")
	}
	if bank.Patterns[0] != nil || bank.Patterns[maxPatternIndex+1] != nil {
		t.Error("a refused pattern index was installed anyway")
	}
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
			bank, songs := stateTestBank(t, kit)
			pattern := bank.CurrentPattern()
			pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
			song := songs.Songs[songs.selSongIdx]
			song.SetPattern(pattern, 0)
			song.SetPattern(pattern, 3)
			path := stateFilePath(t)
			useStateGlobals(t, path, bank, songs, nil)
			recorder := useScreenRecorder(t, &bank.screen)
			readout := func() string { return recorder.row(lengthDisplayRow) }
			if tt.song {
				processEvent = processSongEvent
			}
			t.Cleanup(func() { processEvent = processPatternEvent })

			for _, note := range []int{NoteBrowser, NoteAccent} {
				pressButton(t, note)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("a plain press of Browser wrote the state file")
			}

			pressButton(t, NoteShift)
			pressButton(t, NoteBrowser)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("Shift plus Browser did not write the file: %v", err)
			}
			if text := readout(); !strings.HasPrefix(text, "Saved") {
				t.Errorf("readout = %q, want it to report the save", text)
			}

			// Shift stays engaged for the whole gesture, the way a held modifier does, so
			// the load below is a second press rather than a new one.
			bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 72, 100)
			pressButton(t, NoteAccent)
			if text := readout(); !strings.HasPrefix(text, "Loaded") {
				t.Errorf("readout = %q, want it to report the load", text)
			}
			if event, _ := bank.Patterns[1].EventAtStep(0, kit.voices[2]); event.ChromaticNote != 60 {
				t.Errorf("pitch after the load = %d, want the saved 60", event.ChromaticNote)
			}
			first, second := songs.CurrentSong().GetPattern(0), songs.CurrentSong().GetPattern(3)
			if first == nil || first != second {
				t.Errorf("measures = %v/%v, want both to hold the same pattern", first, second)
			}
			pressButton(t, NoteShift)
		})
	}
}

// With no -state path there is nowhere to save, and the unit says so rather than writing
// somewhere it was not asked to.
func TestStateGesturesWithoutAPathSaySo(t *testing.T) {
	bank, _ := stateTestBank(t, stateKit())
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, &bank.screen)
	for _, gesture := range []struct {
		name string
		run  func() error
	}{
		{name: "save", run: func() error { return saveSession() }},
		{name: "load", run: func() error { return loadSession(false) }},
	} {
		if err := gesture.run(); err != nil {
			t.Fatal(err)
		}
		if text := recorder.row(lengthDisplayRow); !strings.Contains(text, "-state") {
			t.Errorf("readout after %s = %q, want it to say there is no state path", gesture.name, text)
		}
	}
}

// A load that cannot be read reports why and leaves the running session alone.
func TestStateLoadFailureLeavesTheSessionAlone(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)
	path := stateFilePath(t)
	useStateGlobals(t, path, bank, songs, nil)
	recorder := useScreenRecorder(t, &bank.screen)
	for _, tt := range []struct {
		name    string
		content string
	}{
		{name: "no file", content: ""},
		{name: "not a session", content: "this is not a session"},
	} {
		if tt.content != "" {
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
		if err := loadSession(false); err != nil {
			t.Fatal(err)
		}
		if text := recorder.row(lengthDisplayRow); !strings.Contains(text, "Load failed") {
			t.Errorf("%s: readout = %q, want the failure reported", tt.name, text)
		}
		if _, ok := bank.Patterns[1].EventAtStep(0, kit.voices[2]); !ok {
			t.Fatalf("%s: a failed load changed the running session", tt.name)
		}
	}
}

// A load repaints the view it was asked from, so the display agrees with what was installed.
func TestStateLoadRepaintsTheCurrentView(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, kit.voices[2], 60, 100)
	song := songs.Songs[songs.selSongIdx]
	song.SetPattern(pattern, 0)
	song.SetPattern(pattern, 5)
	path := stateFilePath(t)
	saveTo(t, path, bank, songs, nil)
	useStateGlobals(t, path, bank, songs, nil)
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
		processEvent = processPatternEvent
		if tt.song {
			processEvent = processSongEvent
		}
		if err := loadSession(tt.song); err != nil {
			t.Fatal(err)
		}
		if text := recorder.row(0); !strings.Contains(text, tt.want) {
			t.Errorf("%s header = %q, want it to show %q", tt.name, text, tt.want)
		}
		if tt.other != "" {
			if text := recorder.row(3); !strings.Contains(text, tt.other) {
				t.Errorf("%s view = %q, want it to show %q", tt.name, text, tt.other)
			}
		}
	}
	processEvent = processPatternEvent
}

// Leaving saves the session on the goroutine that owns the banks, so the save comes after
// whatever that goroutine was already doing: reading the pattern map while the handler is
// writing it is a fatal concurrent access, not a lost note. It is written once, however the
// exit is reached.
func TestLeavingSavesBehindTheQueuedEditsOnce(t *testing.T) {
	bank, songs := stateTestBank(t, stateKit())
	path := stateFilePath(t)
	exitSaveOnce = sync.Once{} // the once is process-wide
	useStateGlobals(t, path, bank, songs, nil)

	inc := make(chan alsa.SeqEvent, 2)
	inc <- padMessage(54, 100) // a pad press, which adds a step to the first track
	inc <- alsa.SeqEvent{}     // then the leave request
	processIncomingEvents(nil, inc)

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leaving did not leave a readable file: %v", err)
	}
	state, err := readStateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Patterns) != 1 || len(state.Patterns[0].Events) != 1 {
		t.Fatalf("saved patterns = %+v, want the queued edit to land before the save", state.Patterns)
	}
	saveSessionOnExit() // reaching the exit by another route must not write again
	if again, err := os.ReadFile(path); err != nil || string(again) != string(saved) {
		t.Fatal("the session was written a second time")
	}
}

// An untouched pattern is not written out: every pattern the user lands on has an entry, and
// hundreds of empty placeholders make the file harder to read without making it more
// complete. A shortened empty one still goes in, and comes back even though the selection
// does not.
func TestStateOmitsUntouchedPatternsButKeepsShortenedOnes(t *testing.T) {
	kit := stateKit()
	bank, songs := stateTestBank(t, kit)
	for _, index := range []int{2, 5, 9, 6} {
		if err := bank.Jump(index); err != nil {
			t.Fatal(err)
		}
	}
	shortened := bank.selPatIdx
	bank.CurrentPattern().SetLengthSteps(4)
	state := stateFrom(bank, songs, nil)
	if len(state.Patterns) != 1 {
		t.Fatalf("wrote %d patterns, want only the shortened one", len(state.Patterns))
	}
	if state.Patterns[0].Index != shortened || state.Patterns[0].LengthSteps != 4 {
		t.Fatalf("wrote pattern %+v, want the shortened pattern %d", state.Patterns[0], shortened)
	}

	path := stateFilePath(t)
	if _, err := writeStateFile(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, loadedSongs := stateTestBank(t, stateKit())
	if _, err := loadState(path, loaded, loadedSongs, nil); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{name: "restored length", got: loaded.Patterns[shortened].LengthSteps(), want: 4},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
	// The bank stays on whatever pattern it was showing: the selection is the display's
	// business, and the file does not carry it.
	if loaded.selPatIdx == shortened {
		t.Errorf("selected pattern = %d, want the selection left where it was", shortened)
	}
}
