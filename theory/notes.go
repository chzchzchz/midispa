package theory

import (
	"fmt"
	"math"
)

const a0midinote = 21

var names = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func MidiNoteName(midiNote int) string {
	if midiNote < a0midinote {
		return fmt.Sprintf("m%d", midiNote)
	}
	return fmt.Sprintf("%s%d", names[midiNote%12], midiNote/12-1)
}

func MidiNoteFreq(midiNote int) float64 {
	return 440.0 * math.Pow(2.0, float64(midiNote-69)/12.0)
}
