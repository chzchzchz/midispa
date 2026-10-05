package main

import (
	"bytes"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"

	"github.com/chzchzchz/midispa/sysex/akai"
	"github.com/stretchr/testify/require"
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

// newTestBank is the part every bank in a test shares: a controller, its pattern bank on
// pattern 1, and the arrangement built alongside it.
func newTestBank(t *testing.T, fire *Fire, kit *VoiceBank) *PatternBank {
	t.Helper()
	return useController(t, fire, kit).patbank
}

// pressVelocity is the strength the tests hit a palette pad with.
const pressVelocity = 100

// captureMidiWriter records what would reach a device. Writes are guarded because the
// playback worker writes from its own goroutine, so a test that starts a worker and then
// reads the capture has to go through snapshot.
type captureMidiWriter struct {
	mu     sync.Mutex
	events []alsa.SeqEvent
	// onWrite runs once an event is recorded, outside the lock, which is how a test changes
	// the set at a known point in the sequence rather than racing a worker to reach it.
	onWrite func(event alsa.SeqEvent)
}

func (w *captureMidiWriter) Write(event alsa.SeqEvent) error {
	w.mu.Lock()
	w.events = append(w.events, event)
	hook := w.onWrite
	w.mu.Unlock()
	if hook != nil {
		hook(event)
	}
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

// encoderTurn is the Select encoder as the hardware sends it: a control change, not a note.
// The two share numbers on this device and on none other, which is why the handlers tell them
// apart by the status byte rather than by what arrives in the second byte.
func encoderTurn(turn int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(fireControlChannel), byte(CCSelect), byte(turn)}}
}

// deviceChannel and softwareChannel name the two halves of the control rule from a test's
// side, so a test says which side it means rather than counting from fireControlChannel.
const (
	deviceChannel   = fireControlChannel
	softwareChannel = fireControlChannel + 1
)

// sendCC is a control change as a controller sends it, on the channel asked for.
func sendCC(bank *PatternBank, channel, controller, value int) error {
	return dispatch(bank, alsa.SeqEvent{Data: []byte{
		midi.MakeCC(channel), byte(controller), byte(value),
	}})
}

// pressPad is a grid pad press, which is how the keypads are reached.
func pressPad(t *testing.T, bank *PatternBank, note int) {
	t.Helper()
	require.NoError(t, dispatch(bank, padMessage(note, 100)))
}

// pressEncoder is the Select encoder as the hardware sends it.
func pressEncoder(t *testing.T, bank *PatternBank, turn int) {
	t.Helper()
	require.NoError(t, dispatch(bank, encoderTurn(turn)))
}

// padTyping is the note whose key types each digit, found by asking the keypad rather than
// written down, so a change to the pad arithmetic moves the tests with it instead of leaving
// them passing on a number no pad produces.
func padTyping(digit int) int {
	for note := 54; note <= 117; note++ {
		x, y, onGrid := Note2Grid(note)
		if onGrid && padDigit(y, x) == digit {
			return note
		}
	}
	panic("no pad types " + strconv.Itoa(digit))
}

// setSwing puts the swing back after a test, because the store is package state and a test
// that leaves it swung changes every test that runs after it. The remembered groove is saved
// with it: it is the same kind of state, it is written by setSwingPct on every route to a
// swing, and a test that left one behind would decide what the toggle restores for the next.
func setSwing(t *testing.T, value float64) {
	t.Helper()
	previous, previousGroove := swing.Load(), lastGroove.Load()
	t.Cleanup(func() {
		swing.Store(previous)
		lastGroove.Store(previousGroove)
	})
	setSwingPct(value)
}

// timedMidiWriter records when each note-on reached the writer, which is what a playback
// worker writing through a stub can offer instead of the hardware's own clock.
type timedMidiWriter struct {
	mu    sync.Mutex
	times []time.Time
}

func (w *timedMidiWriter) Write(event alsa.SeqEvent) error {
	if len(event.Data) == 3 && midi.IsNoteOn(event.Data[0]) && event.Data[2] > 0 {
		w.mu.Lock()
		w.times = append(w.times, time.Now())
		w.mu.Unlock()
	}
	return nil
}

func (w *timedMidiWriter) WritePort(event alsa.SeqEvent, _ int) error {
	return w.Write(event)
}

func (w *timedMidiWriter) noteOnTimes() []time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]time.Time(nil), w.times...)
}

func assertMidiData(t *testing.T, event alsa.SeqEvent, want []byte) {
	t.Helper()
	if !bytes.Equal(event.Data, want) {
		t.Fatalf("MIDI data = %v, want %v", event.Data, want)
	}
}

// padRecorder stands in for the pads and the button lights. It records what the bank asked
// for rather than drawing it, which is what the seam is for: a test can read what a method
// left showing without running an event through the handler that made it, and without the
// display being involved at all.
type padRecorder struct {
	pads [padRows][padColumns][3]int
	leds map[int]int
	dark bool
}

func (r *padRecorder) LightPadRow(row int, vals [padColumns][3]int) error {
	if row >= 0 && row < padRows {
		r.pads[row] = vals
	}
	return nil
}

func (r *padRecorder) LightPadColumn(col int, vals [padRows][3]int) error {
	if col >= 0 && col < padColumns {
		for row := range r.pads {
			r.pads[row][col] = vals[row]
		}
	}
	return nil
}

func (r *padRecorder) LightPadColor(x, y int, color [3]int) error {
	if x >= 0 && x < padColumns && y >= 0 && y < padRows {
		r.pads[y][x] = color
	}
	return nil
}

func (r *padRecorder) LightPadSlice(pads []akai.Pad) error {
	for _, pad := range pads {
		x, y := pad.Idx%padColumns, pad.Idx/padColumns
		if x >= 0 && x < padColumns && y >= 0 && y < padRows {
			r.pads[y][x] = [3]int{pad.Red, pad.Green, pad.Blue}
		}
	}
	return nil
}

// A blackout is remembered rather than ignored, so IsDark answers what happened rather
// than always saying the unit is awake.
func (r *padRecorder) Blackout() error {
	r.dark = true
	return nil
}

func (r *padRecorder) Wake() bool {
	woken := r.dark
	r.dark = false
	return woken
}

func (r *padRecorder) SetLed(n, v int) error {
	if r.leds == nil {
		r.leds = make(map[int]int)
	}
	r.leds[n] = v
	return nil
}

// led reads one button light. It is a method so a test reads it the way it wrote it.
func (r *padRecorder) led(note int) int {
	return r.leds[note]
}

// pad reads one pad's colour, with y counting rows from the top of the grid and x columns
// from its left.
func (r *padRecorder) pad(x, y int) [3]int {
	return r.pads[y][x]
}

// usePadRecorder points a bank's pads and lights at a recorder for the duration of a test.
func usePadRecorder(t *testing.T, pads *unitScreen) *padRecorder {
	t.Helper()
	recorder := &padRecorder{}
	previous := *pads
	*pads = recorder
	t.Cleanup(func() { *pads = previous })
	return recorder
}
