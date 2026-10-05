package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testNote(note int) *int {
	return &note
}

func writeKitFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644))
}

func TestLoadDevicesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `{"Name":"single","MidiPort":"port","Channel":1,"Voices":[{"Name":"voice","Note":60}]}`)

	devices, err := loadDevices(path)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, "single", devices[0].Name)
}

func TestLoadDevicesArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kit.json")
	writeKitFile(t, dir, "kit.json", `[
		{"Name":"drums","MidiPort":"port-a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
		{"Name":"lead","MidiPort":"port-b","Channel":1,"Voices":[{"Name":"lead","Channel":3}]}
	]`)

	devices, err := loadDevices(path)
	require.NoError(t, err)
	require.NoError(t, validateDevices(devices))
	require.Len(t, devices, 2)
	require.Equal(t, "port-a", devices[0].MidiPort)
	require.Equal(t, "port-b", devices[1].MidiPort)
	require.Equal(t, 3, devices[1].Voices[0].Channel)
}

func TestLoadDevicesDirectorySortsByName(t *testing.T) {
	dir := t.TempDir()
	writeKitFile(t, dir, "z.json", `{"Name":"zeta","MidiPort":"z","Channel":1,"Voices":[{"Name":"z","Note":60}]}`)
	writeKitFile(t, dir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":2,"Voices":[{"Name":"a","Note":61}]}`)
	writeKitFile(t, dir, "ignore.txt", "not a kit")

	devices, err := loadDevices(dir)
	require.NoError(t, err)
	require.Len(t, devices, 2)
	require.Equal(t, "alpha", devices[0].Name)
	require.Equal(t, "zeta", devices[1].Name)
}

func TestLoadKitMergesPathsInOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	writeKitFile(t, dir, "first.json", `[{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]}]`)
	writeKitFile(t, dir, "second.json", `[{"Name":"lead","MidiPort":"b","Channel":1,"Voices":[{"Name":"lead"}]}]`)

	devices, err := loadKit([]string{first, second})
	require.NoError(t, err)
	require.NoError(t, validateDevices(devices))
	require.Len(t, devices, 2)
	require.Equal(t, "drums", devices[0].Name)
	require.Equal(t, "lead", devices[1].Name)

	reversed, err := loadKit([]string{second, first})
	require.NoError(t, err)
	require.Len(t, reversed, 2)
	require.Equal(t, "lead", reversed[0].Name, "the kit order should come from the command line")
	require.Equal(t, "drums", reversed[1].Name, "the kit order should come from the command line")
}

// A merged kit mixes files and directories, and the voices of a single device keep
// their order, because tracks are numbered by position in the merged list.
func TestLoadKitMergesFilesAndDirectories(t *testing.T) {
	dir := t.TempDir()
	kitDir := filepath.Join(dir, "kit")
	require.NoError(t, os.Mkdir(kitDir, 0o755))
	writeKitFile(t, kitDir, "a.json", `{"Name":"alpha","MidiPort":"a","Channel":1,"Voices":[{"Name":"one","Note":60},{"Name":"two","Note":61}]}`)
	single := filepath.Join(dir, "single.json")
	writeKitFile(t, dir, "single.json", `{"Name":"solo","MidiPort":"s","Channel":2,"Voices":[{"Name":"solo","Note":62}]}`)

	devices, err := loadKit([]string{kitDir, single})
	require.NoError(t, err)
	names := make([]string, 0, len(devices))
	for _, dev := range devices {
		names = append(names, dev.Name)
	}
	require.Equal(t, []string{"alpha", "solo"}, names)
	require.Len(t, devices[0].Voices, 2)
	require.Equal(t, "one", devices[0].Voices[0].Name, "the voice order changed")
	require.Equal(t, "two", devices[0].Voices[1].Name, "the voice order changed")
}

// One bad path fails the whole merge; a partial kit would silently drop voices.
func TestLoadKitRejectsBadPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	writeKitFile(t, dir, "good.json", `{"Name":"good","MidiPort":"g","Channel":1,"Voices":[{"Name":"v","Note":60}]}`)

	_, err := loadKit([]string{good, filepath.Join(dir, "missing.json")})
	require.Error(t, err, "a missing kit in the list should fail the merge")
	_, err = loadKit(nil)
	require.Error(t, err, "an empty kit list should fail")
}

// The flag has to accumulate so the same kit can be given more than once on the
// command line, which is what lets several files be merged.
func TestKitPathsFlagAccumulates(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	require.NoError(t, fs.Parse([]string{"-kit", "a.json", "-kit", "b.json", "-kit", "dir"}))
	require.Equal(t, []string{"a.json", "b.json", "dir"}, paths.all())
	require.Equal(t, "a.json,b.json,dir", paths.String())
	require.Error(t, fs.Parse([]string{"-kit", ""}), "an empty kit path should be refused")
}

// The default kit is only used when no -kit is given. It must not be merged with the
// kits on the command line, which would fail on any directory without a kit.json.
func TestKitPathsFlagReplacesDefault(t *testing.T) {
	var paths kitPaths
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")

	require.NoError(t, fs.Parse([]string{"-kit", "a.json", "-kit", "b.json"}))
	for _, path := range paths.all() {
		require.NotEqualf(t, defaultKitPath, path,
			"the default kit was loaded alongside the given kits: %v", paths.all())
	}

	empty := flag.NewFlagSet("empty", flag.ContinueOnError)
	var untouched kitPaths
	empty.Var(&untouched, "kit", "kit")
	require.NoError(t, empty.Parse(nil))
	require.Equal(t, []string{defaultKitPath}, untouched.all())
	require.Equal(t, defaultKitPath, untouched.String())
}

func TestValidateDevices(t *testing.T) {
	valid := []Device{{
		Name:     "valid",
		MidiPort: "port",
		Channel:  1,
		Voices:   []Voice{{Name: "voice", Note: testNote(60)}},
	}}
	require.NoError(t, validateDevices(valid), "a valid device was rejected")

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
			require.Error(t, validateDevices([]Device{tt.device}), "expected a validation error")
		})
	}
}

func TestLoadDevicesRejectsEmptyKit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	writeKitFile(t, dir, "empty.json", "")

	_, err := loadDevices(path)
	require.Error(t, err, "an empty kit should be refused")
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
func shutdownBank(t *testing.T) (*Controller, *Playback, *captureMidiWriter, *fireSim) {
	t.Helper()
	sim := newFireSim()
	kit := trackWindowKit(4, 0)
	controller := useController(t, NewFire(sim.write), kit)
	bank := controller.patbank
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	bank.playback = playback
	require.NoError(t, playback.playChromaticEvent(writer, Event{
		Voice: kit.voices[0], ChromaticNote: 60, Velocity: 100,
	}))
	require.Equal(t, 1, playback.activeNoteCount(), "the test needs a note sounding")
	writer.events = nil
	return controller, playback, writer, sim
}

// Leaving the program must release the note that is sounding, or the instrument holds it
// until its own timeout. It must also close the client.
func TestShutdownReleasesSoundingNotes(t *testing.T) {
	controller, playback, writer, _ := shutdownBank(t)
	client := &fakeSequencer{}

	require.NoError(t, shutdown(controller, client))
	require.Len(t, writer.events, 1, "want the sounding note's note-off")
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
	require.Zero(t, playback.activeNoteCount(), "shutdown left a note marked as sounding")
	require.Equal(t, 1, client.closes, "the sequencer should close once")
}

// Leaving the unit showing the last frame reads as a sequencer that hung, so shutdown has to
// leave it dark: no lit pad, no lit light, and a cleared screen.
func TestShutdownBlanksTheUnit(t *testing.T) {
	controller, _, _, sim := shutdownBank(t)
	// Light something first, so "dark" is a change rather than the state it started in.
	require.NoError(t, controller.patbank.f.SetLed(NoteMode, LEDGreen))
	require.NoError(t, controller.patbank.f.LightPad(3, 1, 127, 127, 127))
	require.NoError(t, controller.patbank.f.Print(0, 0, "Pattern 001"))

	require.NoError(t, shutdown(controller, &fakeSequencer{}))

	for index, color := range sim.pads {
		require.Equalf(t, [3]int{}, color, "pad %d is still lit after shutdown", index)
	}
	for control, value := range sim.leds {
		// The top-left indicators are not plain on/off lights: zero lights Channel, so
		// their dark state is a value of its own.
		want := 0
		if control == CCTopLeftLEDs {
			want = CCTopLeftOff
		}
		require.Equalf(t, want, value, "light %d reads the wrong value after shutdown", control)
	}
	require.NotZero(t, sim.fullClears, "shutdown left the screen showing the last frame")
}

// A failing close is reported, and it must not stop the notes from being released or the
// client from being closed.
func TestShutdownReportsCloseFailure(t *testing.T) {
	controller, playback, writer, _ := shutdownBank(t)
	client := &fakeSequencer{closeErr: errOutOfRange}

	require.Error(t, shutdown(controller, client), "a failing close was not reported")
	require.Len(t, writer.events, 1, "want the note-off even when closing fails")
	require.Zero(t, playback.activeNoteCount(), "shutdown left a note marked as sounding")
	require.Equal(t, 1, client.closes, "the sequencer should close once")
}

// A handler failure runs on a goroutine, where a panic would take the process with it and
// leave every note sounding. It must be reported instead, and playback must stop so that
// nothing keeps sounding notes that are no longer tracked.
func TestHandlerFailureStopsPlaybackWithoutEndingTheProcess(t *testing.T) {
	controller, playback, writer, _ := shutdownBank(t)
	capture := useCaptureLog(t)
	controller.patbank.f = NewFire(func([]byte) error { return errOutOfRange })

	controller.handleIncomingEvent(nil, padMessage(NoteMute1, 100))

	require.Zero(t, playback.activeNoteCount(), "a handler failure left playback running")
	require.Len(t, writer.events, 1, "want the sounding note released")
	assert.Contains(t, capture.messages(), "event failed", "a handler failure was not reported")
}

// A failure to stop as well as to handle is still only reported, never fatal: the release
// is what keeps the instrument from holding a note, so losing that must be visible.
func TestHandlerFailureReportsAFailedStop(t *testing.T) {
	controller, playback, _, _ := shutdownBank(t)
	playback.writer = &failingSequencerWriter{err: errOutOfRange}
	controller.patbank.f = NewFire(func([]byte) error { return errOutOfRange })
	capture := useCaptureLog(t)
	controller.patbank.playback = playback
	// The stop handle is what fails here, so the release the stop asks for is the failing
	// one rather than the handler.
	controller.playback = func() error { return errOutOfRange }

	controller.handleIncomingEvent(nil, padMessage(NoteMute1, 100))

	messages := capture.messages()
	assert.Contains(t, messages, "event failed", "a handler failure was not reported")
	assert.Contains(t, messages, "stopping after an event failure",
		"a failure to stop as well as to handle was not reported")
}

// A pad press has to travel the whole way through the real handler to an instrument. That
// used to need a port open, because the handler named the concrete client rather than the
// writer it only ever writes through.
func TestHandlerPlaysAPadWithoutAPort(t *testing.T) {
	// The first voice is a drum and the second is the chromatic one, so the track the bank
	// starts on holds a drum and a pad press adds a step rather than choosing a pitch.
	kit := trackWindowKit(4, 1)
	controller := useController(t, NewFire(newFireSim().write), kit)
	writer := &captureMidiWriter{}

	controller.handleIncomingEvent(writer, padMessage(54, 100))

	// A percussion step sends the legacy pair: the note is silenced and then sounded, so
	// pressing a step that already holds the note does not leave two copies of it ringing.
	note := byte(*kit.voices[0].Note)
	require.Len(t, writer.events, 2, "want the note and the silence before it")
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
	require.ErrorIsf(t, err, errOutOfRange, "readFire should return the read failure")
	require.Len(t, inc, 2, "readFire should pump the 2 events the reader held")
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
	require.NoError(t, writeMidiMsgs(writer, destination, [][]byte{message}))
	require.Equal(t, destination, writer.events[0].SeqAddr, "a per-device write keeps its own address")

	sharedMIDIDestination = true
	writer = &captureMidiWriter{}
	require.NoError(t, writeMidiMsgs(writer, destination, [][]byte{message}))
	require.Equal(t, alsa.SubsSeqAddr, writer.events[0].SeqAddr, "a shared write goes to subscribers")
}
