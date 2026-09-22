package sysex

import (
	"testing"
)

// TestBPM verifies the BPM() method for common time signatures.
func TestBPM(t *testing.T) {
	tests := []struct {
		name    string
		sig     *TimeSignatureImmediate
		wantBPM int
	}{
		{
			name: "4/4 at 60 BPM",
			sig: &TimeSignatureImmediate{
				BeatsNumerator:   4,
				BeatsDenominator: 2, // quarter note
				ClocksPerClick:   24,
			},
			wantBPM: 60,
		},
		{
			name: "3/4 at 60 BPM",
			sig: &TimeSignatureImmediate{
				BeatsNumerator:   3,
				BeatsDenominator: 2,
				ClocksPerClick:   24,
			},
			wantBPM: 60,
		},
		{
			name: "4/4 at 120 BPM",
			sig: &TimeSignatureImmediate{
				BeatsNumerator:   4,
				BeatsDenominator: 2,
				ClocksPerClick:   48,
			},
			// Note: BPM is independent of ClocksPerClick
			wantBPM: 60,
		},
		{
			name: "6/8 time",
			sig: &TimeSignatureImmediate{
				BeatsNumerator:   6,
				BeatsDenominator: 3, // eighth note
				ClocksPerClick:   24,
			},
			// 60 * 2^(3-2) = 120
			wantBPM: 120,
		},
		{
			name: "2/2 (cut time)",
			sig: &TimeSignatureImmediate{
				BeatsNumerator:   2,
				BeatsDenominator: 2, // quarter note
				ClocksPerClick:   24,
			},
			wantBPM: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sig.BPM()
			if got != tt.wantBPM {
				t.Errorf("BPM() = %d, want %d", got, tt.wantBPM)
			}
		})
	}
}

// TestTimeSignatureImmediateMarshalBinary verifies the encoding.
// Per the MIDI spec (/home/chz/art/synth/docs/specs/MIDI_Specification.pdf),
// Section 12, Notation sub-ID #1 = 0x03, command NotationTimeSignatureImmediate (0x02):
//
//	F0 7F <device> 03 02 <count> <num> <den> <clk> <n32> F7
func TestTimeSignatureImmediateMarshalBinary(t *testing.T) {
	sig := &TimeSignatureImmediate{
		Device:                  0x10,
		BeatsNumerator:          4,
		BeatsDenominator:        2,
		ClocksPerClick:          24,
		Notated32ndNotesPerBeat: 8,
	}
	b, err := sig.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	// F0 7F 0x10 03 02 04 04 02 18 08 08 F7 (11 bytes)
	expected := []byte{0xf0, 0x7f, 0x10, 0x03, 0x02, 0x04, 0x04, 0x02, 0x18, 0x08, 0xf7}
	if len(b) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(b))
	}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestTimeSignatureImmediateUnmarshalBinary verifies decoding.
// Per the MIDI spec (/home/chz/art/synth/docs/specs/MIDI_Specification.pdf),
// Section 12, Notation sub-ID #1 = 0x03, command NotationTimeSignatureImmediate (0x02):
//
//	F0 7F <device> 03 02 <count> <num> <den> <clk> <n32> F7
func TestTimeSignatureImmediateUnmarshalBinary(t *testing.T) {
	// F0 7F 0x10 03 02 04 04 02 18 08 08 F7
	data := []byte{0xf0, 0x7f, 0x10, 0x03, 0x02, 0x04, 0x04, 0x02, 0x18, 0x08, 0xf7}
	sig := &TimeSignatureImmediate{}
	if err := sig.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}
	if sig.Device != 0x10 {
		t.Errorf("Device = 0x%02x, want 0x10", sig.Device)
	}
	if sig.BeatsNumerator != 4 {
		t.Errorf("BeatsNumerator = %d, want 4", sig.BeatsNumerator)
	}
	if sig.BeatsDenominator != 2 {
		t.Errorf("BeatsDenominator = %d, want 2", sig.BeatsDenominator)
	}
	if sig.ClocksPerClick != 24 {
		t.Errorf("ClocksPerClick = %d, want 24", sig.ClocksPerClick)
	}
}
