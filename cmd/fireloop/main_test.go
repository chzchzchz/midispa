package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func testNote(note int) *int {
	return &note
}

func writeKitFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDevicesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `{"Name":"single","MidiPort":"port","Channel":1,"Voices":[{"Name":"voice","Note":60}]}`)

	devices, err := loadDevices(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Name != "single" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestLoadDevicesArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `[
		{"Name":"drums","MidiPort":"port-a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
		{"Name":"lead","MidiPort":"port-b","Channel":1,"Voices":[{"Name":"lead","Channel":3}]}
	]`)

	devices, err := loadDevices(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDevices(devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].MidiPort != "port-a" || devices[1].MidiPort != "port-b" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
	if devices[1].Voices[0].Channel != 3 {
		t.Fatalf("voice channel = %d, want 3", devices[1].Voices[0].Channel)
	}
}

func TestLoadDevicesDirectorySortsByName(t *testing.T) {
	dir := t.TempDir()
	writeKitFile(t, dir, "z.json", `{"Name":"zeta","MidiPort":"z","Channel":1,"Voices":[{"Name":"z","Note":60}]}`)
	writeKitFile(t, dir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":2,"Voices":[{"Name":"a","Note":61}]}`)
	writeKitFile(t, dir, "ignore.txt", "not a kit")

	devices, err := loadDevices(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].Name != "alpha" || devices[1].Name != "zeta" {
		t.Fatalf("unexpected devices: %+v", devices)
	}
}

func TestLoadKitMergesPathsInOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	writeKitFile(t, dir, "first.json", `[{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]}]`)
	writeKitFile(t, dir, "second.json", `[{"Name":"lead","MidiPort":"b","Channel":1,"Voices":[{"Name":"lead"}]}]`)

	devices, err := loadKit([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDevices(devices); err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].Name != "drums" || devices[1].Name != "lead" {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	reversed, err := loadKit([]string{second, first})
	if err != nil {
		t.Fatal(err)
	}
	if len(reversed) != 2 || reversed[0].Name != "lead" || reversed[1].Name != "drums" {
		t.Fatalf("kit order not taken from the command line: %+v", reversed)
	}
}

// A merged kit mixes files and directories, and the voices of a single device keep
// their order, because tracks are numbered by position in the merged list.
func TestLoadKitMergesFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	kitDir := filepath.Join(dir, "kit")
	if err := os.Mkdir(kitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKitFile(t, kitDir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":1,"Voices":[{"Name":"one","Note":60},{"Name":"two","Note":61}]}`)
	single := filepath.Join(dir, "single.json")
	writeKitFile(t, dir, "single.json", `{"Name":"solo","MidiPort":"s","Channel":2,"Voices":[{"Name":"solo","Note":62}]}`)

	devices, err := loadKit([]string{kitDir, single})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(devices))
	for _, dev := range devices {
		names = append(names, dev.Name)
	}
	if len(devices) != 2 || names[0] != "alpha" || names[1] != "solo" {
		t.Fatalf("unexpected devices: %v", names)
	}
	if len(devices[0].Voices) != 2 || devices[0].Voices[0].Name != "one" || devices[0].Voices[1].Name != "two" {
		t.Fatalf("voice order changed: %+v", devices[0].Voices)
	}
}

// One bad path fails the whole merge; a partial kit would silently drop voices.
func TestLoadKitRejectsBadPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	writeKitFile(t, dir, "good.json", `{"Name":"good","MidiPort":"g","Channel":1,"Voices":[{"Name":"v","Note":60}]}`)

	if _, err := loadKit([]string{good, filepath.Join(dir, "missing.json")}); err == nil {
		t.Fatal("expected an error for the missing kit")
	}
	if _, err := loadKit(nil); err == nil {
		t.Fatal("expected an error for an empty kit list")
	}
}

// The flag has to accumulate so the same kit can be given more than once on the
// command line, which is what lets several files be merged.
func TestKitPathsFlagAccumulates(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	if err := fs.Parse([]string{"-kit", "a.json", "-kit", "b.json", "-kit", "dir"}); err != nil {
		t.Fatal(err)
	}
	got := paths.all()
	if len(got) != 3 || got[0] != "a.json" || got[1] != "b.json" || got[2] != "dir" {
		t.Fatalf("kit paths = %v", got)
	}
	if paths.String() != "a.json,b.json,dir" {
		t.Fatalf("kit paths string = %q", paths.String())
	}
	if err := fs.Parse([]string{"-kit", ""}); err == nil {
		t.Fatal("expected an error for an empty kit path")
	}
}

// The default kit is only used when no -kit is given. It must not be merged with the
// kits on the command line, which would fail on any directory without a kit.json.
func TestKitPathsFlagReplacesDefault(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	if err := fs.Parse([]string{"-kit", "a.json", "-kit", "b.json"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths.all() {
		if path == defaultKitPath {
			t.Fatalf("default kit %q loaded alongside the given kits: %v", defaultKitPath, paths.all())
		}
	}

	empty := flag.NewFlagSet("empty", flag.ContinueOnError)
	var untouched kitPaths
	empty.Var(&untouched, "kit", "kit")
	if err := empty.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if got := untouched.all(); len(got) != 1 || got[0] != defaultKitPath {
		t.Fatalf("kit paths without any flag = %v, want the default", got)
	}
	if untouched.String() != defaultKitPath {
		t.Fatalf("kit paths string without any flag = %q", untouched.String())
	}
}

func TestValidateDevices(t *testing.T) {
	valid := []Device{{
		Name:     "valid",
		MidiPort: "port",
		Channel:  1,
		Voices:   []Voice{{Name: "voice", Note: testNote(60)}},
	}}
	if err := validateDevices(valid); err != nil {
		t.Fatalf("valid device rejected: %v", err)
	}

	tests := []struct {
		name   string
		device Device
	}{
		{name: "missing port", device: Device{Channel: 1, Voices: []Voice{{Note: testNote(60)}}}},
		{name: "invalid device channel", device: Device{MidiPort: "port", Channel: 17, Voices: []Voice{{Note: testNote(60)}}}},
		{name: "invalid voice channel", device: Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(60), Channel: 17}}}},
		{name: "missing voices", device: Device{MidiPort: "port", Channel: 1}},
		{name: "invalid note", device: Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(128)}}}},
		{name: "missing effective channel", device: Device{MidiPort: "port", Voices: []Voice{{Note: testNote(60)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDevices([]Device{tt.device}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadDevicesRejectsEmptyKit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	writeKitFile(t, dir, "empty.json", "")

	if _, err := loadDevices(path); err == nil {
		t.Fatal("expected an empty kit error")
	}
}

// fakeSequencer stands in for the ALSA client so the shutdown path can be exercised without
// a port.
type fakeSequencer struct {
	events   []alsa.SeqEvent
	writeErr error
	closeErr error
	closes   int
}

func (f *fakeSequencer) Write(event alsa.SeqEvent) error {
	f.events = append(f.events, event)
	return f.writeErr
}

func (f *fakeSequencer) WritePort(event alsa.SeqEvent, _ int) error {
	f.events = append(f.events, event)
	return f.writeErr
}

func (f *fakeSequencer) Close() error {
	f.closes++
	return f.closeErr
}

// shutdownBank is a bank with one chromatic note sounding, which is the state a signal
// arrives in. The display goes through a recorder so a test can see what the unit was left
// showing.
func shutdownBank(t *testing.T) (*Playback, *captureMidiWriter, *fireSim) {
	t.Helper()
	sim := newFireSim()
	kit := trackWindowKit(4, 0)
	bank := NewPatternBank(NewFire(sim.write), kit)
	usePatternGlobals(t, bank)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	bank.playback = playback
	if err := playback.playChromaticEvent(writer, Event{
		Voice: kit.voices[0], ChromaticNote: 60, Velocity: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if playback.activeNoteCount() != 1 {
		t.Fatal("the test needs a note sounding")
	}
	writer.events = nil
	return playback, writer, sim
}

// Leaving the program must release the note that is sounding, or the instrument holds it
// until its own timeout. It must also close the client.
func TestShutdownReleasesSoundingNotes(t *testing.T) {
	playback, writer, _ := shutdownBank(t)
	client := &fakeSequencer{}

	if err := shutdown(client); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("shutdown wrote %d messages, want the sounding note's note-off", len(writer.events))
	}
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
	if playback.activeNoteCount() != 0 {
		t.Fatal("shutdown left a note marked as sounding")
	}
	if client.closes != 1 {
		t.Fatalf("sequencer closed %d times, want once", client.closes)
	}
}

// Leaving the unit showing the last frame reads as a sequencer that hung, so shutdown has to
// leave it dark: no lit pad, no lit light, and a cleared screen.
func TestShutdownBlanksTheUnit(t *testing.T) {
	_, _, sim := shutdownBank(t)
	// Light something first, so "dark" is a change rather than the state it started in.
	if err := patbank.f.SetLed(NoteMode, LEDGreen); err != nil {
		t.Fatal(err)
	}
	if err := patbank.f.LightPad(3, 1, 127, 127, 127); err != nil {
		t.Fatal(err)
	}
	if err := patbank.f.Print(0, 0, "Pattern 001"); err != nil {
		t.Fatal(err)
	}

	if err := shutdown(&fakeSequencer{}); err != nil {
		t.Fatal(err)
	}

	for index, color := range sim.pads {
		if color != [3]int{} {
			t.Fatalf("pad %d is still lit %v after shutdown", index, color)
		}
	}
	for control, value := range sim.leds {
		// The top-left indicators are not plain on/off lights: zero lights Channel, so
		// their dark state is a value of its own.
		want := 0
		if control == CCTopLeftLEDs {
			want = CCTopLeftOff
		}
		if value != want {
			t.Fatalf("light %d reads %d after shutdown, want %d", control, value, want)
		}
	}
	if sim.fullClears == 0 {
		t.Fatal("shutdown left the screen showing the last frame")
	}
}

// A failing close is reported, and it must not stop the notes from being released or the
// client from being closed.
func TestShutdownReportsCloseFailure(t *testing.T) {
	playback, writer, _ := shutdownBank(t)
	client := &fakeSequencer{closeErr: errOutOfRange}

	if err := shutdown(client); err == nil {
		t.Fatal("a failing close was not reported")
	}
	if len(writer.events) != 1 {
		t.Fatalf("shutdown wrote %d messages, want the note-off even when closing fails", len(writer.events))
	}
	if playback.activeNoteCount() != 0 {
		t.Fatal("shutdown left a note marked as sounding")
	}
	if client.closes != 1 {
		t.Fatalf("sequencer closed %d times, want once", client.closes)
	}
}

// A handler failure runs on a goroutine, where a panic would take the process with it and
// leave every note sounding. It must be reported instead, and playback must stop so that
// nothing keeps sounding notes that are no longer tracked.
func TestHandlerFailureStopsPlaybackWithoutEndingTheProcess(t *testing.T) {
	playback, writer, _ := shutdownBank(t)
	capture := useCaptureLog(t)
	previousProcess := processEvent
	t.Cleanup(func() { processEvent = previousProcess })
	processEvent = func(sequencerWriter, alsa.SeqEvent) error { return errOutOfRange }

	handleIncomingEvent(nil, padMessage(NoteMute1, 100))

	if playback.activeNoteCount() != 0 {
		t.Fatal("a handler failure left playback running")
	}
	if len(writer.events) != 1 {
		t.Fatalf("a handler failure wrote %d messages, want the sounding note released", len(writer.events))
	}
	if messages := capture.messages(); !slices.Contains(messages, "event failed") {
		t.Fatalf("a handler failure was not reported: %v", messages)
	}
}

// A failure to stop as well as to handle is still only reported, never fatal: the release
// is what keeps the instrument from holding a note, so losing that must be visible.
func TestHandlerFailureReportsAFailedStop(t *testing.T) {
	playback, _, _ := shutdownBank(t)
	playback.writer = &failingSequencerWriter{err: errOutOfRange}
	capture := useCaptureLog(t)
	previousProcess := processEvent
	t.Cleanup(func() { processEvent = previousProcess })
	processEvent = func(sequencerWriter, alsa.SeqEvent) error { return errOutOfRange }

	handleIncomingEvent(nil, padMessage(NoteMute1, 100))

	messages := capture.messages()
	if !slices.Contains(messages, "event failed") {
		t.Fatalf("a handler failure was not reported: %v", messages)
	}
	if !slices.Contains(messages, "stopping after an event failure") {
		t.Fatalf("a handler failure that could not stop playback was not reported: %v", messages)
	}
}

// A pad press has to travel the whole way through the real handler to an instrument. That
// used to need a port open, because the handler named the concrete client rather than the
// writer it only ever writes through.
func TestHandlerPlaysAPadWithoutAPort(t *testing.T) {
	// The first voice is a drum and the second is the chromatic one, so the track the bank
	// starts on holds a drum and a pad press adds a step rather than choosing a pitch.
	kit := trackWindowKit(4, 1)
	bank := NewPatternBank(NewFire(newFireSim().write), kit)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	writer := &captureMidiWriter{}

	handleIncomingEvent(writer, padMessage(54, 100))

	// A percussion step sends the legacy pair: the note is silenced and then sounded, so
	// pressing a step that already holds the note does not leave two copies of it ringing.
	note := byte(*kit.voices[0].Note)
	if len(writer.events) != 2 {
		t.Fatalf("a pad press wrote %d messages, want the note and the silence before it", len(writer.events))
	}
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), note, 100})
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), note, 100})
}

// Reading the Fire fails when the port goes away, and that has to reach main as an error:
// the read owns the goroutine, so a failure there is the only thing that can end the
// program from that side, and it must not be swallowed.
func TestReadFireReportsReadFailure(t *testing.T) {
	inc := make(chan alsa.SeqEvent, 4)
	reader := &fakeReader{
		events: []alsa.SeqEvent{padMessage(NoteMute1, 100), padMessage(NoteMute2, 100)},
		fail:   errOutOfRange,
	}

	err := readFire(reader, inc)
	if !errors.Is(err, errOutOfRange) {
		t.Fatalf("readFire returned %v, want the read failure", err)
	}
	if len(inc) != 2 {
		t.Fatalf("readFire pumped %d events, want the 2 the reader held", len(inc))
	}
}

// fakeReader hands out a fixed list of events and then fails, standing in for the blocking
// read of a real client.
type fakeReader struct {
	events []alsa.SeqEvent
	fail   error
	index  int
}

func (f *fakeReader) Read() (alsa.SeqEvent, error) {
	if f.index >= len(f.events) {
		return alsa.SeqEvent{}, f.fail
	}
	ev := f.events[f.index]
	f.index++
	return ev, nil
}

func TestSharedMIDIDestination(t *testing.T) {
	previous := sharedMIDIDestination
	t.Cleanup(func() { sharedMIDIDestination = previous })
	destination := alsa.SeqAddr{Client: 28, Port: 0}
	message := []byte{midi.MakeNoteOn(0), 60, 100}

	sharedMIDIDestination = false
	writer := &captureMidiWriter{}
	if err := writeMidiMsgs(writer, destination, [][]byte{message}); err != nil {
		t.Fatal(err)
	}
	if writer.events[0].SeqAddr != destination {
		t.Fatalf("per-device destination = %v, want %v", writer.events[0].SeqAddr, destination)
	}

	sharedMIDIDestination = true
	writer = &captureMidiWriter{}
	if err := writeMidiMsgs(writer, destination, [][]byte{message}); err != nil {
		t.Fatal(err)
	}
	if writer.events[0].SeqAddr != alsa.SubsSeqAddr {
		t.Fatalf("shared destination = %v, want %v", writer.events[0].SeqAddr, alsa.SubsSeqAddr)
	}
}
