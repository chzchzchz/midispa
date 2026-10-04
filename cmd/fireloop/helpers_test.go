package main

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// The setup every test starts from: a kit, a bank on it with the package globals pointed at
// it, and pattern 1 selected. Almost every test needs exactly that, so the three variants
// below differ only in where the display goes. A test that reads pads, lights or text asks
// for a recording; one that only inspects state discards them.

// chromaBank is the bank a test about pitch starts from: one chromatic voice, pattern 1, and
// the globals pointed at it. Track and percussive tests bring their own kit through quietBank.
func chromaBank(t *testing.T) (*PatternBank, *Voice) {
	t.Helper()
	return chromaBankOn(t, func([]byte) error { return nil })
}

// chromaBankOn is chromaBank with a display the test supplies, for the few tests that count
// what Fireloop writes to the unit.
func chromaBankOn(t *testing.T, write writeFunc) (*PatternBank, *Voice) {
	t.Helper()
	kit := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead", Channel: 1}}}})
	bank := newTestBank(t, NewFire(write), kit)
	return bank, kit.voices[0]
}

// quietBank discards display writes. It returns the kit as well as the bank, because a test
// that names a voice by index reads it from here rather than rebuilding the kit.
func quietBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank) {
	t.Helper()
	bank := quietBankOn(t, kit, func([]byte) error { return nil })
	return bank, kit
}

// quietBankOn is quietBank with a display the test supplies, for the tests that watch which
// buttons light rather than what the pads show.
func quietBankOn(t *testing.T, kit *VoiceBank, write writeFunc) *PatternBank {
	t.Helper()
	return newTestBank(t, NewFire(write), kit)
}

// recordedBank keeps the whole display: pad colours, button lights and a note of how many
// times the screen was wiped, which is how a blackout is told from a readout.
func recordedBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank, *fireSim) {
	t.Helper()
	sim := newFireSim()
	bank := newTestBank(t, NewFire(sim.write), kit)
	return bank, kit, sim
}

// screenBank records only the OLED text, for the tests that read what a row says and have no
// reason to look at a pad.
func screenBank(t *testing.T, kit *VoiceBank) (*PatternBank, *VoiceBank, *screenRecorder) {
	t.Helper()
	bank, kit := quietBank(t, kit)
	return bank, kit, useScreenRecorder(t, &bank.screen)
}

// newTestBank is the part every bank in a test shares.
func newTestBank(t *testing.T, fire *Fire, kit *VoiceBank) *PatternBank {
	t.Helper()
	bank := NewPatternBank(fire, kit)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	return bank
}

// pressVelocity is the strength the tests hit a palette pad with.
const pressVelocity = 100

// captureMidiWriter records what would reach a device. Writes are guarded because the
// playback worker writes from its own goroutine, so a test that starts a worker and then
// reads the capture has to go through snapshot.
type captureMidiWriter struct {
	mu     sync.Mutex
	events []alsa.SeqEvent
}

func (w *captureMidiWriter) Write(event alsa.SeqEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, event)
	return nil
}

func (w *captureMidiWriter) WritePort(event alsa.SeqEvent, _ int) error {
	return w.Write(event)
}

// snapshot returns the events written so far, for a test reading while a worker is running.
func (w *captureMidiWriter) snapshot() []alsa.SeqEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]alsa.SeqEvent(nil), w.events...)
}

// waitForEvents waits for the capture to hold at least want events, which a playback
// worker reaches a moment after it is started rather than before start returns.
func (w *captureMidiWriter) waitForEvents(t *testing.T, want int) []alsa.SeqEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		events := w.snapshot()
		if len(events) >= want {
			return events
		}
		if time.Now().After(deadline) {
			t.Fatalf("capture held %d events, want at least %d", len(events), want)
		}
		time.Sleep(time.Millisecond)
	}
}

func chromaticTestVoice(t *testing.T, notes ...*int) (*Voice, *Device) {
	t.Helper()
	voices := make([]Voice, len(notes))
	for i, note := range notes {
		voices[i] = Voice{Name: "voice", Note: note, Channel: 1}
	}
	device := &Device{
		Name:     "chromatic-test",
		MidiPort: "out",
		Channel:  1,
		Voices:   voices,
		SeqAddr:  alsa.SeqAddr{Client: 10, Port: 20},
	}
	bank := NewVoiceBank([]Device{*device})
	return bank.voices[0], device
}

func padMessage(note, velocity int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(0), byte(note), byte(velocity)}}
}

func releaseMessage(note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOff(0), byte(note), 0}}
}

func assertMidiData(t *testing.T, event alsa.SeqEvent, want []byte) {
	t.Helper()
	if !bytes.Equal(event.Data, want) {
		t.Fatalf("MIDI data = %v, want %v", event.Data, want)
	}
}
