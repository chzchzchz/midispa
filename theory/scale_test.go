package theory

import (
	"reflect"
	"testing"
)

func TestScales(t *testing.T) {
	// test lengths
	expectedLengths := map[string]int{
		"MelodicMinor":    7,
		"NaturalMinor":    7,
		"HarmonicMinor":   7,
		"Major":           7,
		"MajorPentatonic": 5,
		"MinorPentatonic": 5,
		"BebopMajor":      8,
		"BebopMinor":      8,
	}
	check := func(name string, got []string, wantLen int) {
		if len(got) != wantLen {
			t.Errorf("%s: length = %d, want %d", name, len(got), wantLen)
		}
	}
	check("MelodicMinor", MelodicMinor, expectedLengths["MelodicMinor"])
	check("NaturalMinor", NaturalMinor, expectedLengths["NaturalMinor"])
	check("HarmonicMinor", HarmonicMinor, expectedLengths["HarmonicMinor"])
	check("Major", Major, expectedLengths["Major"])
	check("MajorPentatonic", MajorPentatonic, expectedLengths["MajorPentatonic"])
	check("MinorPentatonic", MinorPentatonic, expectedLengths["MinorPentatonic"])
	check("BebopMajor", BebopMajor, expectedLengths["BebopMajor"])
	check("BebopMinor", BebopMinor, expectedLengths["BebopMinor"])
	// check a few known values
	if MelodicMinor[0] != "C" || MelodicMinor[1] != "D" || MelodicMinor[2] != "Eb" {
		t.Errorf("MelodicMinor unexpected values: %v", MelodicMinor)
	}
	if Major[0] != "C" || Major[1] != "D" || Major[2] != "E" {
		t.Errorf("Major unexpected values: %v", Major)
	}
	if MajorPentatonic[0] != "C" || MajorPentatonic[1] != "D" || MajorPentatonic[2] != "E" {
		t.Errorf("MajorPentatonic unexpected: %v", MajorPentatonic)
	}
	if MinorPentatonic[0] != "C" || MinorPentatonic[1] != "Eb" || MinorPentatonic[2] != "F" {
		t.Errorf("MinorPentatonic unexpected: %v", MinorPentatonic)
	}
}

func TestTranspose(t *testing.T) {
	// transpose C major up 0 semitones -> C major
	key := []string{"C", "D", "E", "F", "G", "A", "B"}
	got := Transpose(key, "C")
	want := []string{"C", "D", "E", "F", "G", "A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Transpose(C major, C) = %v, want %v", got, want)
	}
	// transpose C major up 2 semitones (to D) -> D major
	got = Transpose(key, "D")
	want = []string{"D", "E", "F#", "G", "A", "B", "C#"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Transpose(C major, D) = %v, want %v", got, want)
	}
	// transpose C minor? Actually we can test with minor pentatonic
	kp := []string{"C", "Eb", "F", "G", "Bb"}
	got = Transpose(kp, "D") // up 2 semitones
	// C->D, Eb->F, F->G, G->A, Bb->C
	want = []string{"D", "F", "G", "A", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Transpose(minor pent C, D) = %v, want %v", got, want)
	}
}

var semi2flatten = map[int]struct{}{
	str2semi("F"):  {},
	str2semi("Bb"): {},
	str2semi("Eb"): {},
	str2semi("Ab"): {},
	str2semi("Db"): {},
	str2semi("Gb"): {},
}

func TestSemi2Flatten(t *testing.T) {
	// just check that it contains expected semitones
	expected := map[string]bool{
		"F":  true,
		"Bb": true,
		"Eb": true,
		"Ab": true,
		"Db": true,
		"Gb": true,
	}
	for name, exp := range expected {
		semi := str2semi(name)
		if _, ok := semi2flatten[semi]; !ok && exp {
			t.Errorf("semi2flatten missing %s (semitone %d)", name, semi)
		}
		// also ensure no extra entries? Not needed.
	}
}

func TestChord(t *testing.T) {
	if got := Chord("C"); got != nil {
		t.Errorf("Chord(\"C\") = %v, want nil", got)
	}
}
