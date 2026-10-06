package main

import (
	"github.com/chzchzchz/midispa/midi"
)

// midiNoteMax is the highest MIDI note and data value. It is the data range rather than a
// separate number, so a pitch and a velocity cannot drift onto different scales.
const midiNoteMax = midi.DataMax

type Event struct {
	*Voice
	Beat          float32
	ChromaticNote int  // MIDI note 0 through 127; chromatic voices only
	Tie           bool // this event continues into the next event for this voice
	// Triplet marks this event as the first note of a triplet group: the
	// three notes that share one beat of four cells, or one eighth of two.
	// It sits on the first note rather than on all three for the reason Tie
	// does, one flag a repair has to look at, and a group that cannot be
	// repaired then has one flag to clear rather than three. The kind is
	// the span in cells, so the repair and the session file can tell the
	// two sizes apart without a second lookup. The field sits beside Tie
	// because the two one-byte flags share the padding after the note: Go
	// lays fields out in declaration order, so anywhere else the flag
	// would grow the event by a whole word, and every copy the bank, the
	// playhead and a session file make is 8 bytes heavier for it.
	Triplet  tripletKind
	Velocity int // [0,127]
}

// IsChromatic reports whether the event's configured voice supplies its own pitch.
func (ev *Event) IsChromatic() bool {
	return ev != nil && ev.Voice != nil && ev.Voice.IsChromatic()
}

// NoteNumber resolves the percussion note or the event's chromatic pitch.
func (ev *Event) NoteNumber() int {
	if ev == nil {
		return 0
	}
	if ev.IsChromatic() {
		return clampMidiDataValue(ev.ChromaticNote)
	}
	if note, ok := ev.Voice.PercussionNote(); ok {
		return clampMidiDataValue(note)
	}
	return 0
}

func (ev *Event) midiChannel() int {
	if ev == nil || ev.Voice.EffectiveChannel() == 0 {
		panic("no midi channel on voice")
	}
	channel := ev.Voice.EffectiveChannel()
	if channel < 1 || channel > midiChannelMax {
		panic("invalid midi channel on voice")
	}
	return channel
}

// clampMidiDataValue keeps a value inside the MIDI data range.
func clampMidiDataValue(value int) int {
	return min(max(value, 0), midi.DataMax)
}

// minPercussionVelocity is how quiet a percussive step may be turned down. Velocity zero is
// not a silent note but a note-off on the wire, so a step allowed to reach it would stop
// sounding altogether while the readout still claimed a velocity, and a note that never
// starts cannot be softened any further.
const minPercussionVelocity = 1

// clampStepVelocity keeps a velocity inside the range its voice can send: the MIDI data
// range for every voice, and never zero for a percussive one.
func clampStepVelocity(voice *Voice, velocity int) int {
	velocity = clampMidiDataValue(velocity)
	if voice != nil && !voice.IsChromatic() {
		return max(velocity, minPercussionVelocity)
	}
	return velocity
}

// NoteOnMidi sends the velocity the event carries, so a percussive step sounds with the
// dynamics it was hit at rather than a fixed one.
func (ev *Event) NoteOnMidi() []byte {
	return []byte{
		midi.MakeNoteOn(protocolChannel(ev.midiChannel())),
		byte(clampMidiDataValue(ev.NoteNumber())),
		byte(clampStepVelocity(ev.Voice, ev.Velocity)),
	}
}

// NoteOffMidi uses velocity zero for chromatic voices so tied notes can be released safely.
func (ev *Event) NoteOffMidi() []byte {
	velocity := ev.Velocity
	if ev.IsChromatic() {
		velocity = 0
	}
	return []byte{
		midi.MakeNoteOff(protocolChannel(ev.midiChannel())),
		byte(clampMidiDataValue(ev.NoteNumber())),
		byte(clampMidiDataValue(velocity)),
	}
}

// ToMidi preserves the legacy percussion pair; chromatic playback supplies its own lifecycle.
func (ev *Event) ToMidi() [][]byte {
	if ev == nil || ev.Voice == nil {
		return nil
	}
	if ev.IsChromatic() {
		return [][]byte{ev.NoteOnMidi()}
	}
	return [][]byte{ev.NoteOffMidi(), ev.NoteOnMidi()}
}
