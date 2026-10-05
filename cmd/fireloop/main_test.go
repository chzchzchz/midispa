package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/alsa/fake"
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

// A kit file holds either one device or a list of them, and the loader reads both back as
// they were written. Each case goes through a file rather than a Go value, because the
// shape on disk is what a kit author actually writes.
func TestLoadDevicesFileShapes(t *testing.T) {
	tests := []struct {
		name          string
		contents      string
		wantNames     []string
		wantPorts     []string
		wantVoiceChan int
	}{
		{
			name:          "one device as an object",
			contents:      `{"Name":"single","MidiPort":"port","Channel":1,"Voices":[{"Name":"voice","Note":60}]}`,
			wantNames:     []string{"single"},
			wantPorts:     []string{"port"},
			wantVoiceChan: 0,
		},
		{
			name: "several as an array",
			contents: `[
				{"Name":"drums","MidiPort":"port-a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
				{"Name":"lead","MidiPort":"port-b","Channel":1,"Voices":[{"Name":"lead","Channel":3}]}
			]`,
			wantNames:     []string{"drums", "lead"},
			wantPorts:     []string{"port-a", "port-b"},
			wantVoiceChan: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeKitFile(t, dir, "kit.json", tt.contents)
			devices, err := loadDevices(filepath.Join(dir, "kit.json"))
			require.NoError(t, err)
			require.NoError(t, validateDevices(devices))

			names := make([]string, 0, len(devices))
			ports := make([]string, 0, len(devices))
			for _, device := range devices {
				names = append(names, device.Name)
				ports = append(ports, device.MidiPort)
			}
			require.Equal(t, tt.wantNames, names, "the devices came back in another order")
			require.Equal(t, tt.wantPorts, ports, "the ports came back in another order")
			// The first voice of the last device, which is where the array case puts
			// the one channel that is not the device's own. It is read as written:
			// nothing has pointed these voices at a device yet.
			require.Equal(t, tt.wantVoiceChan, devices[len(devices)-1].Voices[0].Channel)
		})
	}
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
// One bad path fails the whole merge; a partial kit would silently drop voices. An empty
// kit list is a run with no kit rather than an empty one.
func TestLoadKitRejectsBadPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	writeKitFile(t, dir, "good.json", `[{"Name":"good","MidiPort":"g","Channel":1,"Voices":[{"Name":"v","Note":60}]}]`)

	tests := []struct {
		name  string
		paths []string
	}{
		{name: "a missing kit among the given ones", paths: []string{good, filepath.Join(dir, "missing.json")}},
		{name: "no kits at all", paths: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadKit(tt.paths)
			require.Error(t, err)
		})
	}
}

// The flag has to accumulate so the same kit can be given more than once on the
// command line, which is what lets several files be merged.
// The flag has to accumulate so the same kit can be given more than once on the command
// line, which is what lets several files be merged. The default kit is only for the run
// where nothing was asked for: merged with a given kit it would fail on any directory
// without a kit.json. Each case states the whole result, so a default that leaked in
// alongside a given kit would show as an extra path rather than pass unnoticed.
func TestKitPathsFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     []string
		wantText string
	}{
		{
			name:     "nothing asked for falls back to the default",
			args:     nil,
			want:     []string{defaultKitPath},
			wantText: defaultKitPath,
		},
		{
			name:     "one kit replaces the default rather than joining it",
			args:     []string{"-kit", "a.json"},
			want:     []string{"a.json"},
			wantText: "a.json",
		},
		{
			name:     "several kits accumulate in the order given",
			args:     []string{"-kit", "a.json", "-kit", "b.json", "-kit", "dir"},
			want:     []string{"a.json", "b.json", "dir"},
			wantText: "a.json,b.json,dir",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths kitPaths
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.Var(&paths, "kit", "kit")
			require.NoError(t, fs.Parse(tt.args))
			require.Equal(t, tt.want, paths.all())
			require.Equal(t, tt.wantText, paths.String())
		})
	}

	// An empty path is refused rather than becoming a kit that is nowhere.
	var paths kitPaths
	fs := flag.NewFlagSet("empty", flag.ContinueOnError)
	fs.Var(&paths, "kit", "kit")
	require.Error(t, fs.Parse([]string{"-kit", ""}), "an empty kit path should be refused")
}

// The refusals each say which rule they are, so a case states the rule rather than only
// that something went wrong. The valid case is here too: a validator that refused
// everything would pass every other row.
func TestValidateDevices(t *testing.T) {
	tests := []struct {
		name    string
		device  Device
		wantErr string
	}{
		{
			name:   "a usable device",
			device: Device{Name: "valid", MidiPort: "port", Channel: 1, Voices: []Voice{{Name: "voice", Note: testNote(60)}}},
		},
		{
			name:    "missing port",
			device:  Device{Channel: 1, Voices: []Voice{{Note: testNote(60)}}},
			wantErr: "empty MidiPort",
		},
		{
			name:    "invalid device channel",
			device:  Device{MidiPort: "port", Channel: 17, Voices: []Voice{{Note: testNote(60)}}},
			wantErr: "invalid channel",
		},
		{
			name:    "invalid voice channel",
			device:  Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(60), Channel: 17}}},
			wantErr: "invalid channel",
		},
		{
			name:    "missing voices",
			device:  Device{MidiPort: "port", Channel: 1},
			wantErr: "has no voices",
		},
		{
			name:    "invalid note",
			device:  Device{MidiPort: "port", Channel: 1, Voices: []Voice{{Note: testNote(128)}}},
			wantErr: "invalid note",
		},
		{
			// A voice takes the device's channel, so a device with none leaves the
			// voice with none either, which is a different complaint from a bad one.
			name:    "missing effective channel",
			device:  Device{MidiPort: "port", Voices: []Voice{{Note: testNote(60)}}},
			wantErr: "has no MIDI channel",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDevices([]Device{tt.device})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// A sequencer that failed to open comes back as a nil *alsa.Seq, and assigning that into
// an interface-typed field leaves something that is not nil but is not there either.
// Every write path asks this question before it writes, so both kinds of absence have to
// be told apart from a writer that really is there.
func TestIsNilMidiWriter(t *testing.T) {
	var unopened *alsa.Seq
	var asWriter alsa.EventWriter = unopened
	var asPortWriter alsa.PortWriter = unopened

	for _, test := range []struct {
		name   string
		aseq   any
		absent bool
	}{
		{name: "nothing at all", aseq: nil, absent: true},
		{name: "unopened sequencer behind an EventWriter", aseq: asWriter, absent: true},
		{name: "unopened sequencer behind a PortWriter", aseq: asPortWriter, absent: true},
		{name: "a writer that is there", aseq: alsa.EventWriter(&captureMidiWriter{}), absent: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.absent, isNilMidiWriter(test.aseq))
		})
	}
}

// shutdownBank is a bank with one chromatic note sounding, which is the state a signal
// arrives in. The display goes through a recorder so a test can see what the unit was left
// showing.
func shutdownBank(t *testing.T) (*Controller, *Playback, *captureMidiWriter, *fireSim) {
	t.Helper()
	sim := newFireSim()
	kit := trackWindowKit(4, 0)
	controller := useController(t, NewFire(sim.write), kit)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	controller.playback = stubSession(playback, nil)
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
	client := fake.New()

	require.NoError(t, shutdown(controller, client))
	require.Len(t, writer.events, 1, "want the sounding note's note-off")
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
	require.Zero(t, playback.activeNoteCount(), "shutdown left a note marked as sounding")
	require.Equal(t, 1, client.Closes(), "the sequencer should close once")
}

// Leaving the unit showing the last frame reads as a sequencer that hung, so shutdown has to
// leave it dark: no lit pad, no lit light, and a cleared screen.
func TestShutdownBlanksTheUnit(t *testing.T) {
	controller, _, _, sim := shutdownBank(t)
	// Light something first, so "dark" is a change rather than the state it started in.
	require.NoError(t, controller.patbank.f.SetLed(NoteMode, LEDGreen))
	require.NoError(t, controller.patbank.f.LightPad(3, 1, 127, 127, 127))
	require.NoError(t, controller.patbank.f.Print(0, 0, "Pattern 001"))

	require.NoError(t, shutdown(controller, fake.New()))

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
	client := fake.New()
	client.CloseErr = errOutOfRange

	require.Error(t, shutdown(controller, client), "a failing close was not reported")
	require.Len(t, writer.events, 1, "want the note-off even when closing fails")
	require.Zero(t, playback.activeNoteCount(), "shutdown left a note marked as sounding")
	require.Equal(t, 1, client.Closes(), "the sequencer should close once")
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
	// The stop handle is what fails here, so the release the stop asks for is the failing
	// one rather than the handler.
	controller.playback = stubSession(playback, func() error { return errOutOfRange })

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
	// The Fire answers twice and then stops, which is what a Fire unplugged mid-session
	// looks like: the events already on their way still arrive.
	reader := fake.New()
	reader.Queue(padMessage(NoteMute1, 100), padMessage(NoteMute2, 100))
	reader.ReadErr = errOutOfRange

	err := readFire(reader, inc)
	require.ErrorIsf(t, err, errOutOfRange, "readFire should return the read failure")
	require.Len(t, inc, 2, "readFire should pump the 2 events the reader held")
}

// Whether a write is shared is one property with two answers, and the difference is the
// only thing under test here.
func TestSharedMIDIDestination(t *testing.T) {
	previous := sharedMIDIDestination
	t.Cleanup(func() { sharedMIDIDestination = previous })
	destination := alsa.SeqAddr{Client: 28, Port: 0}
	message := []byte{midi.MakeNoteOn(0), 60, 100}

	tests := []struct {
		name   string
		shared bool
		want   alsa.SeqAddr
	}{
		{name: "a per-device write keeps its own address", shared: false, want: destination},
		{name: "a shared write goes to subscribers", shared: true, want: alsa.SubsSeqAddr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedMIDIDestination = tt.shared
			writer := &captureMidiWriter{}
			require.NoError(t, writeMidiMsgs(writer, destination, [][]byte{message}))
			require.Equal(t, tt.want, writer.events[0].SeqAddr)
		})
	}
}

// A name is how the palette, the log and the display talk about a device, so two of them
// answering to one name is a kit nobody can read back. A repeated port or channel is not
// the same thing: a kit may put two devices on one destination deliberately, so only the
// name decides. Every case goes through the file loader rather than a Go value, because a
// kit is written on disk before anything looks at it.
func TestValidateDevicesOnRepeatedNames(t *testing.T) {
	tests := []struct {
		name    string
		kit     string
		wantErr string
	}{
		{
			name: "distinct names",
			kit: `[
				{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
				{"Name":"lead","MidiPort":"b","Channel":1,"Voices":[{"Name":"lead"}]}
			]`,
		},
		{
			name: "one name twice",
			kit: `[
				{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
				{"Name":"drums","MidiPort":"b","Channel":1,"Voices":[{"Name":"lead"}]}
			]`,
			wantErr: `device 1 ("drums") has the same name as device 0`,
		},
		{
			// The clash is reported against the first device that took the name, not
			// against whichever one a later sweep happened to meet.
			name: "the clash names the first holder",
			kit: `[
				{"Name":"a","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":60}]},
				{"Name":"b","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":61}]},
				{"Name":"b","MidiPort":"q","Channel":2,"Voices":[{"Name":"v","Note":62}]}
			]`,
			wantErr: `device 2 ("b") has the same name as device 1`,
		},
		{
			// Two devices on one port and one channel is the kit author's choice, and
			// refusing it would forbid a kit that is meant to sound that way.
			name: "the same port and channel twice",
			kit: `[
				{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
				{"Name":"layer","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]}
			]`,
		},
		{
			// Only the name is a duplicate here, so the rest being identical changes
			// nothing about the verdict.
			name: "the same device twice under one name",
			kit: `[
				{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]},
				{"Name":"drums","MidiPort":"a","Channel":10,"Voices":[{"Name":"kick","Note":36}]}
			]`,
			wantErr: "has the same name",
		},
		{
			// A device with no name has always been allowed, and one of them still is.
			name: "one device with no name",
			kit:  `[{"MidiPort":"a","Channel":1,"Voices":[{"Name":"v","Note":60}]}]`,
		},
		{
			// Two of them share the empty name, so the rule catches them.
			name: "two devices with no name",
			kit: `[
				{"MidiPort":"a","Channel":1,"Voices":[{"Name":"v","Note":60}]},
				{"MidiPort":"b","Channel":2,"Voices":[{"Name":"v","Note":61}]}
			]`,
			wantErr: `device 1 ("") has the same name as device 0`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeKitFile(t, dir, "kit.json", tt.kit)
			devices, err := loadDevices(filepath.Join(dir, "kit.json"))
			require.NoError(t, err, "the table case should be a readable kit file")
			if tt.wantErr == "" {
				require.NoError(t, validateDevices(devices))
				return
			}
			require.ErrorContains(t, validateDevices(devices), tt.wantErr)
		})
	}
}

// A kit file that cannot be read has to say so, because the usual cause is a half-saved
// edit and the user needs to know which file to look at. The loader wraps what it gets
// from the decoder with the path, so the cases below only have to name the shape.
func TestLoadDevicesRejectsUnreadableFiles(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  string
		missing  bool
	}{
		{name: "empty file", contents: "", wantErr: "empty kit file"},
		{name: "only whitespace", contents: "  \n\t ", wantErr: "empty kit file"},
		{name: "truncated object", contents: `{"Name":"x","MidiPort":"p"`, wantErr: "unexpected end of JSON input"},
		{name: "truncated array", contents: `[{"Name":"x"`, wantErr: "unexpected end of JSON input"},
		{name: "not JSON at all", contents: "hello", wantErr: "invalid character"},
		{name: "an object where an array was meant", contents: `{"Name":"x","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":60}],}`, wantErr: "invalid character"},
		{name: "an array holding nothing", contents: `[]`, wantErr: "contains no devices"},
		{name: "a directory that is not there", contents: "", wantErr: "no such file or directory", missing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "kit.json")
			if tt.missing {
				path = filepath.Join(dir, "absent.json")
			} else {
				writeKitFile(t, dir, "kit.json", tt.contents)
			}
			_, err := loadDevices(path)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, filepath.Base(path), "the error should name the file")
		})
	}
}

// Loading is not validating. A kit file can be well-formed JSON describing a kit that
// cannot be played, and the refusal has to survive the trip from the file to the
// validator rather than only being reachable by building the value by hand.
func TestLoadKitCarriesValidationFailures(t *testing.T) {
	tests := []struct {
		name    string
		kit     string
		wantErr string
	}{
		{
			name:    "a device with no port",
			kit:     `[{"Name":"x","Channel":1,"Voices":[{"Name":"v","Note":60}]}]`,
			wantErr: "empty MidiPort",
		},
		{
			name:    "a channel above the top",
			kit:     `[{"Name":"x","MidiPort":"p","Channel":17,"Voices":[{"Name":"v","Note":60}]}]`,
			wantErr: "invalid channel",
		},
		{
			name:    "a device with no voices",
			kit:     `[{"Name":"x","MidiPort":"p","Channel":1}]`,
			wantErr: "has no voices",
		},
		{
			name:    "a voice with a note above the top",
			kit:     `[{"Name":"x","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":200}]}]`,
			wantErr: "invalid note",
		},
		{
			name:    "a voice with a channel above the top",
			kit:     `[{"Name":"x","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Channel":17,"Note":60}]}]`,
			wantErr: "invalid channel",
		},
		{
			// A voice with no channel of its own takes the device's, which is how a kit
			// says all of these on channel one. It is the one shape that reaches the
			// voice check and is still usable.
			name: "a voice with no channel inherits the device's",
			kit:  `[{"Name":"x","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":60}]}]`,
		},
		{
			// The refusal reaches a kit that was read off disk, not just one built in
			// a test, which is the only place a user meets it.
			name:    "a repeated name in a loaded kit",
			kit:     `[{"Name":"x","MidiPort":"p","Channel":1,"Voices":[{"Name":"v","Note":60}]},{"Name":"x","MidiPort":"q","Channel":2,"Voices":[{"Name":"v","Note":61}]}]`,
			wantErr: "has the same name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeKitFile(t, dir, "kit.json", tt.kit)
			devices, err := loadKit([]string{filepath.Join(dir, "kit.json")})
			require.NoError(t, err, "the file should be readable even when the kit is not usable")
			if tt.wantErr == "" {
				require.NoError(t, validateDevices(devices))
				return
			}
			require.ErrorContains(t, validateDevices(devices), tt.wantErr)
		})
	}
}
