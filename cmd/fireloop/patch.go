package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	gomidi "gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/sysex"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const (
	patchExtensionMIDI = ".mid"
	patchExtensionSMF  = ".smf"

	// patchChannelNone is the channel a patch is sent on when it holds no channel
	// message at all: a dump addresses a device, so there is no channel to re-stamp. It
	// sits outside the 0-15 the protocol numbers channels with, because 0 is channel
	// one and a sentinel that shares a value with a real channel silently renames it.
	patchChannelNone = -1
)

// Settle is a duration a kit writes as text, e.g. "100ms", so a kit says how long an
// instrument needs rather than how many nanoseconds that is. An absent or empty value
// means no wait, which is the default: most instruments have the patch the moment the
// bytes are written, and only some need settling after a vendor dump.
type Settle time.Duration

// MarshalJSON writes the duration back as text, so a kit that is read and written keeps
// saying "100ms" rather than turning into a count of nanoseconds nobody meant.
func (s Settle) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(s).String())
}

// UnmarshalJSON reads a duration written as text. It reports a value it cannot read
// rather than quietly taking it as no wait, because a settle that is silently ignored
// is a first note that sounds the previous patch.
func (s *Settle) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("settle must be a duration in text such as \"100ms\"")
	}
	if text == "" {
		*s = 0
		return nil
	}
	parsed, err := time.ParseDuration(text)
	if err != nil {
		return fmt.Errorf("settle %q is not a duration: %w", text, err)
	}
	*s = Settle(parsed)
	return nil
}

// resolvePatchPath turns a kit's Patch into a file. A relative path is looked for beside
// the kit file that declared it, because a kit travels with its patches and fireloop is
// started from wherever the user happens to be. It is deliberately not also looked for in
// the working directory: that would let an unrelated file of the same name standing there
// be played in place of the one the kit asked for, and the wrong patch on an instrument
// sounds like a fault in fireloop rather than a kit that lost its file.
func resolvePatchPath(dev *Device, patch string) string {
	if patch == "" || filepath.IsAbs(patch) {
		return patch
	}
	if dev == nil || dev.baseDir == "" {
		return patch
	}
	return filepath.Join(dev.baseDir, patch)
}

// validPatchExtension keeps a Patch to the one file type fireloop knows how to play. A
// path that names something else is a kit mistake, not a file that happens to be missing.
func validPatchExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == patchExtensionMIDI || ext == patchExtensionSMF
}

// readPatchSMF reads a patch file into the events that go on the wire. Meta events are
// dropped as they are read: tempo, time signature, track name and end-of-track describe
// the file rather than the MIDI in it.
func readPatchSMF(path string) ([]gomidi.Message, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := smfreader.New(file, smfreader.NoteOffVelocity())
	if err := reader.ReadHeader(); err != nil {
		return nil, err
	}
	var messages []gomidi.Message
	for {
		message, err := reader.Read()
		if err == smf.ErrFinished {
			return messages, nil
		}
		if err != nil {
			return nil, err
		}
		if _, isMeta := message.(meta.Message); isMeta {
			continue
		}
		messages = append(messages, message)
	}
}

// patchReadReason is the reason a patch could not be read, with the path the filesystem
// put in it taken off again. A send names the file it was reading anyway, and a line that
// repeats a long absolute path twice is harder to read than one that names it once. The
// startup check keeps the wrapped error, because there the resolved path is the thing
// being reported: it is where fireloop looked.
func patchReadReason(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// patchCache holds the events of the kit's patch files, read once rather than on every
// Play. A kit's patches do not change while fireloop runs, and the read happens on the
// goroutine that handles a button press, so parsing a vendor dump there delays the Fire's
// answer to whoever pressed it. A file that could not be read is not remembered, so a kit
// on removable media is picked up once the medium comes back.
//
// The lock is held across the read so that two sends starting at once parse the file
// between them rather than both. Nothing else in the cache needs it: the map is only
// written here.
type patchCache struct {
	mu       sync.Mutex
	messages map[string][]gomidi.Message
}

// read returns the events in the patch at path, reading the file the first time and
// keeping what it found for the next send.
func (c *patchCache) read(path string) ([]gomidi.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if messages, ok := c.messages[path]; ok {
		return messages, nil
	}
	messages, err := readPatchSMF(path)
	if err != nil {
		return nil, err
	}
	if c.messages == nil {
		c.messages = make(map[string][]gomidi.Message)
	}
	c.messages[path] = messages
	return messages, nil
}

// patchMessage converts one decoded event into the bytes to send. Channel events are
// re-encoded onto the given protocol index with this repo's midi package, so the wire
// layout stays defined in one place; SysEx is passed through untouched, because a dump
// addresses a device rather than a channel. ok is false for a kind fireloop cannot send,
// which is skipped rather than failing the whole patch.
func patchMessage(msg gomidi.Message, channelIndex int) (data []byte, ok bool) {
	// patchChannel leaves a target without a channel only when its file holds no channel
	// message, so a channel event cannot arrive here with nowhere to go. It is refused
	// rather than written anyway, because an index of -1 would go out as a real channel
	// and configure whatever was listening on it.
	if _, isChannel := msg.(channel.Message); isChannel && channelIndex == patchChannelNone {
		return nil, false
	}
	switch event := msg.(type) {
	case sysex.Message:
		return event.Raw(), true
	case channel.NoteOn:
		return []byte{midi.MakeNoteOn(channelIndex), event.Key(), event.Velocity()}, true
	case channel.NoteOff:
		return []byte{midi.MakeNoteOff(channelIndex), event.Key(), 0}, true
	case channel.NoteOffVelocity:
		return []byte{midi.MakeNoteOff(channelIndex), event.Key(), event.Velocity()}, true
	case channel.ControlChange:
		return []byte{midi.MakeCC(channelIndex), event.Controller(), event.Value()}, true
	case channel.ProgramChange:
		return []byte{midi.MakePgm(channelIndex), event.Program()}, true
	case channel.Pitchbend:
		low, high := midi.MakePitchBend(event.AbsValue())
		return []byte{midi.MakePitch(channelIndex), low, high}, true
	case channel.Aftertouch:
		return []byte{midi.MakeChannelAftertouch(channelIndex), event.Pressure()}, true
	case channel.PolyAftertouch:
		return []byte{midi.MakeKeyAftertouch(channelIndex), event.Key(), event.Pressure()}, true
	}
	return nil, false
}

// patchChannel is the protocol index a patch is sent on: the index of the voice's
// effective channel, or patchChannelNone when the file carries nothing a channel can
// reach. A SysEx-only patch lands on patchChannelNone for every voice, so it is sent
// once per device rather than once per channel the device happens to use.
func patchChannel(voiceChannel int, messages []gomidi.Message) int {
	for _, msg := range messages {
		if _, isChannel := msg.(channel.Message); isChannel {
			return protocolChannel(voiceChannel)
		}
	}
	return patchChannelNone
}

// patchHasSysEx reports whether a patch carries a vendor dump, which is the only kind of
// message an instrument needs settling time for.
func patchHasSysEx(messages []gomidi.Message) bool {
	for _, msg := range messages {
		if _, isSysEx := msg.(sysex.Message); isSysEx {
			return true
		}
	}
	return false
}

// patchSettle is how long the device needs after one of its patches carried a dump, and
// zero when it needs nothing: a device that does not ask, or a patch with no dump in it,
// is state the instrument has as soon as the bytes are written. A negative settle is
// rejected when the kit is validated, so it never reaches here to be waited out.
func patchSettle(device *Device, messages []gomidi.Message) time.Duration {
	if device == nil || !patchHasSysEx(messages) {
		return 0
	}
	return time.Duration(device.Settle)
}

// patchTarget is one patch send: the file, the device it goes to, and the protocol index
// its channel messages carry, which is patchChannelNone when it addresses the device
// rather than a channel. Voices inheriting one device patch on one channel share a
// target, so the file goes out once for all of them.
type patchTarget struct {
	device  *Device
	path    string
	channel int
}

// sendKitPatches writes every distinct patch in the kit, once per device and target
// channel, in kit order. Each file is read through the kit's cache, so it is parsed on the
// first send and reused for the channels that share it and for every send after that. The
// returned duration is the longest settle any device asked for, which is zero unless a kit
// sets one and a dump went out. Errors are joined and returned; the caller logs them and
// plays on regardless, because an instrument with the wrong settings is better than a Fire
// that has stopped.
func sendKitPatches(aseq alsa.EventWriter, vb *VoiceBank) (time.Duration, error) {
	if isNilMidiWriter(aseq) || vb == nil {
		return 0, nil
	}
	sent := make(map[patchTarget]bool)
	failed := make(map[string]bool)
	settle := time.Duration(0)
	var errs []error
	for _, voice := range vb.voices {
		path := voice.patchPath()
		if path == "" {
			continue
		}
		messages, err := vb.patches.read(path)
		if err != nil {
			// One report per file rather than per voice: a kit of sixteen voices sharing
			// one broken patch would otherwise bury the log under sixteen copies of a
			// single problem.
			if !failed[path] {
				failed[path] = true
				errs = append(errs, fmt.Errorf("patch %q for %s: %w", path, voiceLabel(voice), patchReadReason(err)))
			}
			continue
		}
		channelIndex := patchChannel(voice.EffectiveChannel(), messages)
		target := patchTarget{device: voice.device, path: path, channel: channelIndex}
		if sent[target] {
			continue
		}
		sent[target] = true
		if err := writePatch(aseq, target, messages); err != nil {
			errs = append(errs, fmt.Errorf("patch %q for %s: %w", path, voiceLabel(voice), err))
			continue
		}
		if wait := patchSettle(target.device, messages); wait > settle {
			// The waits overlap rather than add up: every device is loading at the same
			// time, so the longest is the one the first note has to wait for.
			settle = wait
		}
	}
	return settle, errors.Join(errs...)
}

// writePatch converts the events to the target's channel and writes them to its device's
// port. The destination goes through writeMidiMsgs like every other message, so a shared
// MIDI destination applies to patches as it does to notes.
func writePatch(aseq alsa.EventWriter, target patchTarget, messages []gomidi.Message) error {
	if target.device == nil {
		return fmt.Errorf("patch %q has no device to send it to", filepath.Base(target.path))
	}
	batch := make([][]byte, 0, len(messages))
	for _, msg := range messages {
		data, ok := patchMessage(msg, target.channel)
		if !ok {
			logger.Debug("patch message skipped", "device", target.device.Name,
				"message", fmt.Sprintf("%T", msg))
			continue
		}
		batch = append(batch, data)
	}
	if len(batch) == 0 {
		return nil
	}
	// The file is named because the log says what an instrument was set up from, which
	// is the first thing to want to know when it sounds wrong.
	logger.Info("patch", "device", target.device.Name, "file", filepath.Base(target.path),
		"channel", patchChannelName(target.channel), "messages", len(batch))
	return writeMidiMsgs(aseq, target.device.SeqAddr, batch)
}

// patchChannelName names the channel a patch went out on, in the 1-16 a kit is written
// in, so a log line and a kit file read the same way. patchChannelNone is the patch that
// carried no channel message and is called out as such rather than numbered.
func patchChannelName(channelIndex int) string {
	if channelIndex == patchChannelNone {
		return "none"
	}
	return strconv.Itoa(channelIndex + 1)
}

// validatePatch checks a patch before any port is opened, so an unreadable file stops
// startup rather than quietly leaving an instrument on whatever settings it had. what
// names the kit entry that declared it.
func validatePatch(what string, dev *Device, patch string) error {
	if patch == "" {
		return nil
	}
	path := resolvePatchPath(dev, patch)
	if !validPatchExtension(path) {
		return fmt.Errorf("%s has patch %q, want a %s or %s file",
			what, patch, patchExtensionMIDI, patchExtensionSMF)
	}
	if _, err := readPatchSMF(path); err != nil {
		return fmt.Errorf("%s has patch %q: %w", what, patch, err)
	}
	return nil
}
