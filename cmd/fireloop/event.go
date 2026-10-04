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

// NoteOnMidi uses the event velocity for chromatic voices and the legacy fixed velocity for percussion.
func (ev *Event) NoteOnMidi() []byte {
	velocity := 64
	if ev.IsChromatic() {
		velocity = ev.Velocity
	}
	return []byte{
		midi.MakeNoteOn(protocolChannel(ev.midiChannel())),
		byte(clampMidiDataValue(ev.NoteNumber())),
		byte(clampMidiDataValue(velocity)),
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
