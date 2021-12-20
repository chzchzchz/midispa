package theory

import (
	"fmt"
)

const a0midinote = 21

var names = []string{"A", "A#", "B", "C", "C#", "D", "D#", "E", "F", "F#", "G", "G#"}

func MidiNoteName(midiNote int) string {
	if midiNote < a0midinote {
		return fmt.Sprintf("m%d", midiNote)
	}
	a0n := midiNote - a0midinote
	return fmt.Sprintf("%s%d", names[a0n%12], 1 + a0n/12)
}
