package theory

var MelodicMinor = []string{"C", "D", "Eb", "F", "G", "A", "B"}
var NaturalMinor = []string{"C", "D", "Eb", "F", "G", "Ab", "Bb"}
var HarmonicMinor = []string{"C", "D", "Eb", "F", "G", "Ab", "B"}
var Major = []string{"C", "D", "E", "F", "G", "A", "B"}
var MajorPentatonic = []string{"C", "D", "E", "G", "A"}
var MinorPentatonic = []string{"C", "Eb", "F", "G", "Bb"}
var BebopMajor = []string{"C", "D", "E", "F", "G", "G#", "A", "B"}
var BebopMinor = []string{"C", "D", "Eb", "F", "G", "G#", "A", "B"}

// Emaj; Emaj7
// Emin; Emin7
// E7 / Edom7
// E9
// Esus2 / Esus / Esus9
// Esus4
func Chord(s string) []int {
	return nil
}

func Transpose(key []string, root string) []string {
	ret := []string{}
	rootSemi := str2semi(root)
	for _, s := range key {
		semi := (str2semi(s) + rootSemi) % 12
		ret = append(ret, semiStrings[semi])
	}
	return ret
}
