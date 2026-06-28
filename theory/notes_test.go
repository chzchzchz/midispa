package theory

import (
	"math"
	"testing"
)

func TestMidiNoteFreq(t *testing.T) {
	for _, tt := range []struct {
		n int
		f int
	}{
		{n: 68, f: 415},
		{n: 69, f: 440},
		{n: 70, f: 466},
		{n: 71, f: 494},
	} {
		if ff := int(math.Round(MidiNoteFreq(tt.n))); ff != tt.f {
			t.Errorf("wanted %d, got %d", tt.f, ff)
		}
	}
}

func TestMidiNoteName(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "m0"},
		{20, "m20"},
		{21, "A0"},
		{22, "A#0"},
		{23, "B0"},
		{24, "C1"},
		{25, "C#1"},
		{26, "D1"},
		{27, "D#1"},
		{28, "E1"},
		{29, "F1"},
		{30, "F#1"},
		{31, "G1"},
		{32, "G#1"},
		{33, "A1"},
		{34, "A#1"},
		{35, "B1"},
		{36, "C2"},
		{69, "A4"},
		{60, "C4"},
		{72, "C5"},
		{127, "G9"},
	}
	for _, tt := range tests {
		if got := MidiNoteName(tt.in); got != tt.want {
			t.Errorf("MidiNoteName(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
