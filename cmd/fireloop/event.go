package main

import (
	"github.com/chzchzchz/midispa/midi"
)

type Event struct {
	*Voice
	Beat          float32
	ChromaticNote int  // MIDI note 0 through 127; chromatic voices only
	Tie           bool // this event continues into the next event for this voice
	Velocity      int  // [0,127]
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

func clampMidiDataValue(value int) int {
	if value < 0 {
		return 0
	}
	if value > midi.DataMax {
		return midi.DataMax
	}
	return value
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
	if voice != nil && !voice.IsChromatic() && velocity < minPercussionVelocity {
		return minPercussionVelocity
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
