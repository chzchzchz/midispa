package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gomidi "gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/sysex"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const patchTestTicksPerQuarter = 96

// patchDump is a SysEx payload a test can recognise byte for byte.
var patchDump = sysex.SysEx{0x43, 0x10, 0x7f, 0x00, 0x7a}

// patchDeviceAddr is where a patched device's messages land in a test.
var patchDeviceAddr = alsa.SeqAddr{Client: 10, Port: 20}

// writePatchFile writes a patch holding the given messages. A track name and a time
// signature are always included so the file is a well-formed SMF rather than a bare event
// list, and the end of track flushes the buffered track to the file.
func writePatchFile(t *testing.T, path string, messages ...gomidi.Message) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	writer := smfwriter.New(file,
		smfwriter.NumTracks(1),
		smfwriter.TimeFormat(smf.MetricTicks(patchTestTicksPerQuarter)),
	)
	if err := writer.WriteHeader(); err != nil {
		t.Fatal(err)
	}
	events := []gomidi.Message{
		meta.TrackSequenceName("patch"),
		meta.TimeSig{
			Numerator:                4,
			Denominator:              4,
			ClocksPerClick:           patchTestTicksPerQuarter,
			DemiSemiQuaverPerQuarter: 8,
		},
	}
	events = append(events, messages...)
	events = append(events, meta.EndOfTrack)
	for _, event := range events {
		// Writing the last event of the only track reports the end of the file.
		if err := writer.Write(event); err != nil && err != smf.ErrFinished && err != io.EOF {
			t.Fatal(err)
		}
	}
}

// patchFile writes a patch into dir and returns where it landed.
func patchFile(t *testing.T, dir, name string, messages ...gomidi.Message) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writePatchFile(t, path, messages...)
	return path
}

// patchVoices builds one voice per channel, which is how most of the tests here say "this
// device's voices sit on these channels".
func patchVoices(channels ...int) []Voice {
	voices := make([]Voice, len(channels))
	for i, channel := range channels {
		voices[i] = Voice{Name: fmt.Sprintf("v%d", i), Channel: channel}
	}
	return voices
}

// patchedDevice builds the device nearly every test here needs: one patch, played to
// patchDeviceAddr, with the given voices.
func patchedDevice(dir, patch string, voices ...Voice) Device {
	if voices == nil {
		voices = patchVoices(1)
	}
	return Device{
		Name:     "lead",
		MidiPort: "out",
		Channel:  1,
		Patch:    patch,
		Voices:   voices,
		SeqAddr:  patchDeviceAddr,
		baseDir:  dir,
	}
}

// patchKit builds a kit whose single device carries the given patch, with the patch file
// beside the kit file the way a real kit ships its patches.
func patchKit(t *testing.T, devicePatch string, voices ...Voice) (*VoiceBank, string) {
	t.Helper()
	dir := t.TempDir()
	patchPath := patchFile(t, dir, "instrument.mid",
		channel.Channel(0).ControlChange(74, 90),
		channel.Channel(0).ProgramChange(42),
	)
	kitPath := filepath.Join(dir, "kit.json")
	kit := []Device{patchedDevice(dir, devicePatch, voices...)}
	data, err := json.Marshal(kit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kitPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	devices, err := loadDeviceFile(kitPath)
	if err != nil {
		t.Fatal(err)
	}
	return NewVoiceBank(devices), patchPath
}

// assertEvents compares what a writer captured with what the kit should have sent, order
// and destination included.
func assertEvents(t *testing.T, writer *captureMidiWriter, want []alsa.SeqEvent) {
	t.Helper()
	got := writer.snapshot()
	if len(got) != len(want) {
		t.Fatalf("sent %d events, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].SeqAddr != want[i].SeqAddr {
			t.Fatalf("event %d went to %v, want %v", i, got[i].SeqAddr, want[i].SeqAddr)
		}
		assertMidiData(t, got[i], want[i].Data)
	}
}

func sentTo(data []byte) alsa.SeqEvent {
	return alsa.SeqEvent{SeqAddr: patchDeviceAddr, Data: data}
}

// A voice's Patch wins over its device's, a voice with neither inherits the device's, and
// a relative path resolves beside the kit file rather than the working directory.
func TestPatchPrecedenceAndResolution(t *testing.T) {
	kit, patchPath := patchKit(t, "instrument.mid",
		Voice{Name: "inherits", Channel: 1},
		Voice{Name: "overrides", Channel: 2, Patch: "other.mid"},
		Voice{Name: "no-patch", Channel: 3},
	)
	inherits, overrides, none := kit.voices[0], kit.voices[1], kit.voices[2]

	for _, tc := range []struct {
		voice *Voice
		want  string
	}{
		{inherits, "instrument.mid"},
		{overrides, "other.mid"},
		{none, "instrument.mid"},
	} {
		if got := tc.voice.EffectivePatch(); got != tc.want {
			t.Fatalf("%s resolved to %q, want %q", tc.voice.Name, got, tc.want)
		}
	}

	// The kit file sits in a temp directory, so the patch must be found beside it even
	// though nothing in the test is running from there.
	if got := inherits.patchPath(); got != patchPath {
		t.Fatalf("resolved patch = %q, want the kit-relative %q", got, patchPath)
	}
	// An absolute path is used as given.
	kit.voices[2].Patch = patchPath
	if got := none.patchPath(); got != patchPath {
		t.Fatalf("absolute patch = %q, want %q", got, patchPath)
	}
	// A voice on a kit with no patch anywhere has nothing to play.
	if got := (&Voice{Name: "bare", device: &Device{}}).patchPath(); got != "" {
		t.Fatalf("kit without patches resolved to %q, want nothing", got)
	}
}

// A patch that cannot be read stops startup rather than leaving an instrument on the
// settings it happened to have.
func TestPatchValidationRejectsUnusableFiles(t *testing.T) {
	dir := t.TempDir()
	wrongType := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(wrongType, []byte("not midi"), 0o600); err != nil {
		t.Fatal(err)
	}
	notSMF := filepath.Join(dir, "broken.mid")
	if err := os.WriteFile(notSMF, []byte("MThd but not really"), 0o600); err != nil {
		t.Fatal(err)
	}
	newKit := func(patch string) []Device {
		return []Device{patchedDevice(dir, patch)}
	}

	for _, patch := range []string{"missing.mid", "notes.txt", "broken.mid"} {
		err := validateDevices(newKit(patch))
		if err == nil {
			t.Fatalf("patch %q was accepted", patch)
		}
		if !strings.Contains(err.Error(), patch) {
			t.Fatalf("error for %q does not name the file: %v", patch, err)
		}
	}
	if err := validateDevices(newKit(patchFile(t, dir, "good.mid",
		channel.Channel(0).ControlChange(74, 90)))); err != nil {
		t.Fatalf("a readable patch was rejected: %v", err)
	}
	// A voice's own patch is validated the same way, resolved against the device's kit.
	voices := newKit("")
	voices[0].Voices[0].Patch = "missing.mid"
	if err := validateDevices(voices); err == nil {
		t.Fatal("a voice patch that does not exist was accepted")
	}
	if err := validateDevices(newKit("")); err != nil {
		t.Fatalf("a kit without patches was rejected: %v", err)
	}
}

// The file's channel is the one it was dumped on, which is rarely the channel the voice
// plays on, so every kind of channel message is re-stamped. SysEx addresses a device
// rather than a channel and is left exactly as it was.
func TestPatchRemapsEveryChannelMessageAndKeepsSysEx(t *testing.T) {
	dir := t.TempDir()
	fileChannel := channel.Channel(0)
	voiceChannel := 2 // channel 3, where the voice below plays

	cases := []struct {
		name string
		in   gomidi.Message
		want []byte
	}{
		{"note on", fileChannel.NoteOn(60, 100), []byte{midi.MakeNoteOn(voiceChannel), 60, 100}},
		{"note off", fileChannel.NoteOff(60), []byte{midi.MakeNoteOff(voiceChannel), 60, 0}},
		{"note off with release velocity", fileChannel.NoteOffVelocity(60, 40),
			[]byte{midi.MakeNoteOff(voiceChannel), 60, 40}},
		{"control change", fileChannel.ControlChange(74, 90),
			[]byte{midi.MakeCC(voiceChannel), 74, 90}},
		{"program change", fileChannel.ProgramChange(42),
			[]byte{midi.MakePgm(voiceChannel), 42}},
		// 2000 above centre is 10192 of the 14-bit range, which is 80 and 79 on the wire.
		{"pitch bend", fileChannel.Pitchbend(2000),
			[]byte{midi.Pitch | byte(voiceChannel), 80, 79}},
		{"channel pressure", fileChannel.Aftertouch(60),
			[]byte{midi.ChannelAftertouch | byte(voiceChannel), 60}},
		{"key pressure", fileChannel.PolyAftertouch(60, 70),
			[]byte{midi.KeyAftertouch | byte(voiceChannel), 60, 70}},
		{"syssex", patchDump, patchDump.Raw()},
	}

	// Each kind on its own, so a failure names the kind that broke.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			one := t.TempDir()
			device := patchedDevice(one, patchFile(t, one, "patch.mid", tc.in), patchVoices(3)...)
			writer := &captureMidiWriter{}
			if _, err := sendKitPatches(writer, NewVoiceBank([]Device{device})); err != nil {
				t.Fatal(err)
			}
			assertEvents(t, writer, []alsa.SeqEvent{sentTo(tc.want)})
		})
	}

	// And all of them in one file, which is what a patch actually looks like: the kinds
	// arrive in the order the file holds them.
	all := make([]gomidi.Message, 0, len(cases))
	want := make([]alsa.SeqEvent, 0, len(cases))
	for _, tc := range cases {
		all = append(all, tc.in)
		want = append(want, sentTo(tc.want))
	}
	writer := &captureMidiWriter{}
	device := patchedDevice(dir, patchFile(t, dir, "all.mid", all...), patchVoices(3)...)
	if _, err := sendKitPatches(writer, NewVoiceBank([]Device{device})); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, writer, want)
}

// One rule with several faces: a voice contributes one patch, so the kit sends each
// distinct file once per device and channel it can reach.
func TestKitPatchSendSet(t *testing.T) {
	dir := t.TempDir()
	shared := patchFile(t, dir, "shared.mid", channel.Channel(0).ControlChange(74, 90))
	other := patchFile(t, dir, "other.mid", channel.Channel(0).ControlChange(75, 20))
	dump := patchFile(t, dir, "dump.mid", patchDump)

	// The voices are copied rather than shared between two devices: NewVoiceBank points
	// each voice at the device it came from, so two devices sharing one slice are one
	// device as far as the kit is concerned.
	secondPort := alsa.SeqAddr{Client: 11, Port: 21}
	twin := patchedDevice(dir, dump, patchVoices(2, 3)...)
	twin.SeqAddr = secondPort

	kick := 36
	drums := Device{
		Name:     "drums",
		MidiPort: "out",
		Channel:  1,
		Voices:   []Voice{{Name: "kick", Note: &kick}},
		SeqAddr:  patchDeviceAddr,
	}

	cases := []struct {
		name    string
		devices []Device
		want    []alsa.SeqEvent
	}{
		{
			// Sending the patch once per voice would take as long as the file plays.
			name:    "voices sharing a patch are sent it once",
			devices: []Device{patchedDevice(dir, shared, patchVoices(2, 2, 2)...)},
			want:    []alsa.SeqEvent{sentTo([]byte{midi.MakeCC(1), 74, 90})},
		},
		{
			// A program change or CC on the wrong channel configures nothing.
			name:    "a patch goes once per channel the device's voices use",
			devices: []Device{patchedDevice(dir, shared, patchVoices(5, 9)...)},
			want: []alsa.SeqEvent{
				sentTo([]byte{midi.MakeCC(4), 74, 90}),
				sentTo([]byte{midi.MakeCC(8), 74, 90}),
			},
		},
		{
			name:    "a voice's own patch does not cost its siblings the device's",
			devices: []Device{patchedDevice(dir, shared, Voice{Name: "a", Channel: 2}, Voice{Name: "own", Channel: 2, Patch: other})},
			want: []alsa.SeqEvent{
				sentTo([]byte{midi.MakeCC(1), 74, 90}),
				sentTo([]byte{midi.MakeCC(1), 75, 20}),
			},
		},
		{
			// A dump has no channel to reach, so it is sent once however many there are.
			name:    "a syssex patch is sent once per device",
			devices: []Device{patchedDevice(dir, dump, patchVoices(2, 3, 4)...)},
			want:    []alsa.SeqEvent{sentTo(patchDump.Raw())},
		},
		{
			name:    "two devices are two sends",
			devices: []Device{patchedDevice(dir, dump, patchVoices(2)...), twin},
			want: []alsa.SeqEvent{
				sentTo(patchDump.Raw()),
				{SeqAddr: secondPort, Data: patchDump.Raw()},
			},
		},
		{
			name:    "a kit without patches sends nothing",
			devices: []Device{drums},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writer := &captureMidiWriter{}
			if _, err := sendKitPatches(writer, NewVoiceBank(tc.devices)); err != nil {
				t.Fatal(err)
			}
			assertEvents(t, writer, tc.want)
		})
	}
}

// The patch has to be on the wire before the first note, because an instrument that
// changes voice mid-pattern sounds like a mistake.
func TestPlaybackStartsWithTheKitPatch(t *testing.T) {
	kit, _ := patchKit(t, "instrument.mid", Voice{Name: "lead", Channel: 2})
	bank, _ := quietBank(t, kit)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[0], 60, 100)

	writer := &captureMidiWriter{}
	playbackStop = bank.startSequencer(writer)
	// The worker writes the note a moment after start returns, so wait for it rather
	// than racing it.
	events := writer.waitForEvents(t, 4)
	assertMidiData(t, events[0], []byte{midi.MakeCC(1), 74, 90})
	assertMidiData(t, events[1], []byte{midi.MakePgm(1), 42})
	// What matters is that the patch came before the first note, not what sits between
	// them: the transport's Start goes out to the sync port straight after.
	note := -1
	for i, event := range events {
		if midi.IsNoteOn(event.Data[0]) {
			note = i
			break
		}
	}
	if note < 0 {
		t.Fatalf("no note-on in the %d messages written, want the pattern's first note", len(events))
	}
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}

	// Starting again sends the patch again, because the point is to put the instrument
	// back the way the kit says even if something else moved it in between.
	restarted := &captureMidiWriter{}
	playbackStop = bank.startSequencer(restarted)
	again := restarted.waitForEvents(t, 2)
	assertMidiData(t, again[0], []byte{midi.MakeCC(1), 74, 90})
	assertMidiData(t, again[1], []byte{midi.MakePgm(1), 42})
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
}

// Song playback goes through its own sequencer, and the kit's patches belong there too.
func TestSongPlaybackStartsWithTheKitPatch(t *testing.T) {
	kit, _ := patchKit(t, "instrument.mid", Voice{Name: "lead", Channel: 2})
	bank, _ := quietBank(t, kit)
	songs := NewSongBank(NewFire(func([]byte) error { return nil }), bank)

	writer := &captureMidiWriter{}
	songs.startSequencer(writer)()
	// Only the patch is checked here: stopping the song also writes the transport's
	// Start and Stop to the sync port, and the timing between them is another test's job.
	events := writer.snapshot()
	if len(events) < 2 {
		t.Fatalf("song playback wrote %d messages, want the kit patch", len(events))
	}
	assertMidiData(t, events[0], []byte{midi.MakeCC(1), 74, 90})
	assertMidiData(t, events[1], []byte{midi.MakePgm(1), 42})
}

// A dump has to be absorbed before a note can sound the new patch, so a device can ask
// playback to wait for it. A kit that does not ask waits for nothing, and a patch with
// no dump in it needs no wait however long a device asked for.
func TestSettleComesFromTheDeviceAndOnlyAfterADump(t *testing.T) {
	dir := t.TempDir()
	channels := patchFile(t, dir, "channels.mid", channel.Channel(0).ControlChange(74, 90))
	dump := patchFile(t, dir, "dump.mid", channel.Channel(0).ControlChange(74, 90), patchDump)

	for _, tc := range []struct {
		name   string
		patch  string
		settle Settle
		want   time.Duration
	}{
		{"a device that asks and a dump that was sent", dump, Settle(150 * time.Millisecond), 150 * time.Millisecond},
		{"a device that asks but sends no dump", channels, Settle(150 * time.Millisecond), 0},
		{"a dump with no device settle", dump, 0, 0},
		{"controllers with a device settle", channels, Settle(150 * time.Millisecond), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			device := patchedDevice(dir, tc.patch)
			device.Settle = tc.settle
			writer := &captureMidiWriter{}
			settle, err := sendKitPatches(writer, NewVoiceBank([]Device{device}))
			if err != nil {
				t.Fatal(err)
			}
			if settle != tc.want {
				t.Fatalf("settle = %v, want %v", settle, tc.want)
			}
		})
	}

	// Devices load at the same time, so their waits overlap and the longest one is what
	// the first note waits for.
	slow := patchedDevice(dir, dump)
	slow.Settle = Settle(300 * time.Millisecond)
	fast := patchedDevice(dir, dump)
	fast.SeqAddr = alsa.SeqAddr{Client: 11, Port: 21}
	settle, err := sendKitPatches(&captureMidiWriter{}, NewVoiceBank([]Device{slow, fast}))
	if err != nil {
		t.Fatal(err)
	}
	if settle != 300*time.Millisecond {
		t.Fatalf("settle across two devices = %v, want the longest at 300ms", settle)
	}

	// A kit with no patches has nothing to settle for either.
	settle, err = sendKitPatches(&captureMidiWriter{}, NewVoiceBank([]Device{
		{Name: "drums", MidiPort: "out", Channel: 1, Voices: patchVoices(1), Settle: Settle(time.Second)},
	}))
	if err != nil || settle != 0 {
		t.Fatalf("kit without patches settled for %v (%v), want no wait", settle, err)
	}
}

// A kit says how long in text, so a device can be given time without the kit having to
// know that a duration is counted in nanoseconds.
func TestSettleIsReadFromTheKitAsText(t *testing.T) {
	for _, tc := range []struct {
		settle string
		want   Settle
	}{
		{`"100ms"`, Settle(100 * time.Millisecond)},
		{`"1s"`, Settle(time.Second)},
		{`""`, 0},
		{"null", 0},
	} {
		var device Device
		if err := json.Unmarshal([]byte(`{"Name":"kit","MidiPort":"out","Channel":1,"Settle":`+tc.settle+`}`), &device); err != nil {
			t.Fatalf("Settle %s was rejected: %v", tc.settle, err)
		}
		if device.Settle != tc.want {
			t.Fatalf("Settle %s = %v, want %v", tc.settle, device.Settle, tc.want)
		}
	}
	// A value nobody can read is reported rather than taken as no wait, since a settle
	// that is silently dropped is a first note that sounds the previous patch. A number
	// counts as unreadable too: nanoseconds are not what a kit is asked to say.
	for _, settle := range []string{`"soon"`, `100`} {
		var device Device
		if err := json.Unmarshal([]byte(`{"Settle":`+settle+`}`), &device); err == nil {
			t.Fatalf("Settle %s was read as %v", settle, device.Settle)
		}
	}
	// A duration that parses but would be waited backwards is a kit mistake, caught when
	// the kit is validated rather than turned into no wait at playback.
	if err := validateDevices([]Device{{
		Name: "kit", MidiPort: "out", Channel: 1, Settle: Settle(-time.Second),
		Voices: []Voice{{Name: "voice", Channel: 1}},
	}}); err == nil {
		t.Fatal("a negative settle was accepted")
	}
	// A kit that is read and written keeps saying what it says in text. Without this a
	// device's settle marshals as a count of nanoseconds that its own reader rejects, so
	// a kit written out could not be loaded back.
	device := Device{Name: "kit", MidiPort: "out", Channel: 1, Settle: Settle(150 * time.Millisecond)}
	data, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"150ms"`) {
		t.Fatalf("a settle was written as %s, want the text a kit is written in", data)
	}
	var reloaded Device
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("a written kit could not be read back: %v", err)
	}
	if reloaded.Settle != device.Settle {
		t.Fatalf("settle came back as %v, want %v", time.Duration(reloaded.Settle), time.Duration(device.Settle))
	}
}

// The settle has to hold the first note back, or it is not holding anything back: the
// dump goes out at once and the pattern waits behind it.
func TestSettleKeepsTheFirstNoteBack(t *testing.T) {
	dir := t.TempDir()
	device := patchedDevice(dir, patchFile(t, dir, "dump.mid", patchDump), patchVoices(1)...)
	device.Settle = Settle(500 * time.Millisecond)
	bank, kit := quietBank(t, NewVoiceBank([]Device{device}))
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[0], 60, 100)

	writer := &captureMidiWriter{}
	start := time.Now()
	playbackStop = bank.startSequencer(writer)
	// The dump is written before the worker starts, so it is the only thing on the wire
	// while the instrument loads.
	if got := len(writer.snapshot()); got != 1 {
		t.Fatalf("%d messages written at once, want the dump alone", got)
	}
	// A timer never fires early, so a note well before the settle would mean the wait is
	// not happening at all.
	writer.waitForEvents(t, 3)
	if elapsed := time.Since(start); elapsed < time.Duration(device.Settle)/2 {
		t.Fatalf("the first note arrived after %v, want it held back by the %v settle",
			elapsed, time.Duration(device.Settle))
	}
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
}

// Stopping during the settle must not wait it out: the instrument is still loading and
// the user wants the Fire to stop now.
func TestStopDuringPatchSettleReturnsPromptly(t *testing.T) {
	dir := t.TempDir()
	device := patchedDevice(dir, patchFile(t, dir, "dump.mid", patchDump))
	device.Settle = Settle(30 * time.Second)
	bank, _ := quietBank(t, NewVoiceBank([]Device{device}))

	stop := bank.startSequencer(&captureMidiWriter{})
	stopped := make(chan error, 1)
	go func() { stopped <- stop() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop reported %v, want the playback to end quietly", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("stop waited out the %v settle", time.Duration(device.Settle))
	}
}

// A patch that turns out to be unreadable once the kit is running must not take the Fire
// with it: the event loop treats any error from a button handler as fatal.
func TestUnreadablePatchDoesNotStopPlayback(t *testing.T) {
	dir := t.TempDir()
	device := patchedDevice(dir, patchFile(t, dir, "patch.mid",
		channel.Channel(0).ControlChange(74, 90)))
	bank, _ := quietBank(t, NewVoiceBank([]Device{device}))
	if err := os.Remove(device.Patch); err != nil {
		t.Fatal(err)
	}

	writer := &captureMidiWriter{}
	playbackStop = bank.startSequencer(writer)
	if playbackStop == nil {
		t.Fatal("playback did not start")
	}
	if err := stopPlayback(); err != nil {
		t.Fatalf("stop reported the patch failure: %v", err)
	}
}

// A kit states a channel as 1-16 and the wire counts from zero. The conversion belongs at
// the bytes, so a voice on channel 1 reaches channel index 0 and channel 16 reaches 15,
// whatever else is in between.
func TestProtocolChannelIsWhereTheKitNumberingEnds(t *testing.T) {
	for _, tc := range []struct {
		channel  int
		wantByte byte
	}{
		// Channel one is index zero, which is the pair a sentinel would collide with.
		{1, 0x90},
		{2, 0x91},
		{10, 0x99}, // the kit example's drum channel
		{16, 0x9f}, // the last channel is the last index
	} {
		if got := protocolChannel(tc.channel); got != int(tc.wantByte&0x0f) {
			t.Fatalf("channel %d became index %d, want %d", tc.channel, got, tc.wantByte&0x0f)
		}
		device := Device{Name: "kit", MidiPort: "out", Channel: 1}
		voice := &Voice{Name: "v", Channel: tc.channel, device: &device}
		event := &Event{Voice: voice, ChromaticNote: 60, Velocity: 100}
		want := []byte{tc.wantByte, 60, 100}
		if got := event.NoteOnMidi(); string(got) != string(want) {
			t.Fatalf("channel %d wrote % X, want % X", tc.channel, got, want)
		}
	}
}

// A patch with no channel message has no channel to re-stamp, which has to be sayable
// without borrowing channel one's index. Zero is channel one on the wire, so a sentinel
// set to zero makes a patch that really did use channel one log as though it used none.
func TestPatchWithoutAChannelIsNotConfusedWithChannelOne(t *testing.T) {
	withControllers := []gomidi.Message{channel.Channel(0).ControlChange(74, 90)}

	if patchChannelNone >= 0 {
		t.Fatalf("the no-channel sentinel is %d, which is inside the 0-15 the protocol numbers channels with", patchChannelNone)
	}
	for _, tc := range []struct {
		name     string
		channel  int
		messages []gomidi.Message
		want     int
		wantName string
	}{
		{"controllers on a voice playing channel one", 1, withControllers, 0, "1"},
		{"controllers on a voice playing channel ten", 10, withControllers, 9, "10"},
		{"a patch holding only a dump", 1, []gomidi.Message{patchDump}, patchChannelNone, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := patchChannel(tc.channel, tc.messages); got != tc.want {
				t.Fatalf("channel index = %d, want %d", got, tc.want)
			}
			if got := patchChannelName(tc.want); got != tc.wantName {
				t.Fatalf("channel logged as %q, want %q", got, tc.wantName)
			}
		})
	}

	// A channel message with nowhere to go is dropped rather than written as an index of
	// -1, which would reach whatever is listening on channel 15.
	if _, ok := patchMessage(withControllers[0], patchChannelNone); ok {
		t.Fatal("a channel message was encoded with no channel to send it on")
	}
}

// A relative Patch belongs to the kit that declares it. Falling back to the working
// directory would let an unrelated file of the same name be played on the instrument, and
// the wrong patch sounds like a fault in fireloop rather than a kit that lost its file.
func TestRelativePatchIsNotTakenFromTheWorkingDirectory(t *testing.T) {
	kitDir := t.TempDir()
	beside := patchFile(t, kitDir, "instrument.mid", channel.Channel(0).ControlChange(74, 90))

	// A file of the same name standing where fireloop happens to be started from.
	working := t.TempDir()
	decoy := patchFile(t, working, "instrument.mid", channel.Channel(0).ControlChange(75, 20))
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(working); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restoring the working directory: %v", err)
		}
	})

	// With the file beside the kit, that is the one played.
	device := patchedDevice(kitDir, "instrument.mid")
	if got := resolvePatchPath(&device, device.Patch); got != beside {
		t.Fatalf("resolved to %q, want the kit's own %q", got, beside)
	}
	if err := validateDevices([]Device{device}); err != nil {
		t.Fatalf("a kit with its patch beside it was rejected: %v", err)
	}

	// With it missing from the kit, the decoy is not substituted: startup says which file
	// was looked for, so the kit can be fixed.
	if err := os.Remove(beside); err != nil {
		t.Fatal(err)
	}
	if got := resolvePatchPath(&device, device.Patch); got == decoy {
		t.Fatalf("resolved to the working directory's %q instead of the kit's", decoy)
	}
	err = validateDevices([]Device{device})
	if err == nil {
		t.Fatal("a kit missing its patch was accepted because a file of that name stood in the working directory")
	}
	if !strings.Contains(err.Error(), beside) {
		t.Fatalf("error does not name the file that was looked for: %v", err)
	}
}

// A kit's patches do not change while fireloop runs, so each file is read once instead of
// on every Play. The read happens on the goroutine that answers a button press, which is
// why it is worth keeping off the second and third send.
func TestPatchIsReadOnceAndKeptForTheKit(t *testing.T) {
	dir := t.TempDir()
	path := patchFile(t, dir, "instrument.mid", channel.Channel(0).ControlChange(74, 90))
	kit := NewVoiceBank([]Device{patchedDevice(dir, "instrument.mid")})
	want := []alsa.SeqEvent{sentTo([]byte{midi.MakeCC(0), 74, 90})}

	first := &captureMidiWriter{}
	if _, err := sendKitPatches(first, kit); err != nil {
		t.Fatal(err)
	}
	assertEvents(t, first, want)

	// With the file gone, a second send can only happen from what was already read.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	second := &captureMidiWriter{}
	if _, err := sendKitPatches(second, kit); err != nil {
		t.Fatalf("the second send went back to the file: %v", err)
	}
	assertEvents(t, second, want)
}

// A file that could not be read is not remembered as broken, so a kit on removable media
// is picked up on the next Play once the medium comes back.
func TestPatchThatCouldNotBeReadIsTriedAgain(t *testing.T) {
	dir := t.TempDir()
	kit := NewVoiceBank([]Device{patchedDevice(dir, "later.mid")})
	if _, err := sendKitPatches(&captureMidiWriter{}, kit); err == nil {
		t.Fatal("a kit whose patch is not there yet reported nothing")
	}
	patchFile(t, dir, "later.mid", channel.Channel(0).ControlChange(74, 90))

	writer := &captureMidiWriter{}
	if _, err := sendKitPatches(writer, kit); err != nil {
		t.Fatalf("the file appeared but the kit still reported it missing: %v", err)
	}
	assertEvents(t, writer, []alsa.SeqEvent{sentTo([]byte{midi.MakeCC(0), 74, 90})})
}

// One broken file is one problem, however many voices name it: sixteen copies of the same
// line bury everything else the log has to say.
func TestOneBrokenPatchIsReportedOnce(t *testing.T) {
	dir := t.TempDir()
	device := patchedDevice(dir, "gone.mid", patchVoices(1, 2, 3, 4, 5, 6)...)
	_, err := sendKitPatches(&captureMidiWriter{}, NewVoiceBank([]Device{device}))
	if err == nil {
		t.Fatal("a kit whose patch is missing sent nothing and reported nothing")
	}
	// Counted against the path rather than the bare name, so one file whose name ends in
	// another's does not count as a second mention of it.
	if got := strings.Count(err.Error(), `/gone.mid"`); got != 1 {
		t.Fatalf("the missing file is named %d times across %d voices, want once: %v",
			got, len(device.Voices), err)
	}
	// Two files that are both broken are two problems, and both have to be visible.
	second := patchedDevice(dir, "also-gone.mid", patchVoices(1)...)
	second.Voices = append(second.Voices, Voice{Name: "own", Channel: 2, Patch: "gone.mid"})
	_, err = sendKitPatches(&captureMidiWriter{}, NewVoiceBank([]Device{device, second}))
	if err == nil {
		t.Fatal("two kits with missing patches reported nothing")
	}
	for _, name := range []string{`/gone.mid"`, `/also-gone.mid"`} {
		if got := strings.Count(err.Error(), name); got != 1 {
			t.Fatalf("%s is named %d times, want once: %v", name, got, err)
		}
	}
}