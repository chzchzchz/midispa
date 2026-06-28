package theory

var note2semi = map[byte]int{
	'C': 0,
	'D': 2,
	'E': 4,
	'F': 5,
	'G': 7,
	'A': 9,
	'B': 11,
}

var semiStrings = []string{
	"C", "C#", "D", "D#", "E",
	"F", "F#", "G", "G#", "A", "A#", "B",
}

func str2semi(s string) int {
	v := note2semi[s[0]]
	if len(s) == 1 {
		return v
	} else if s[1] == 'b' {
		return ((v - 1) + 12) % 12
	} else if s[1] == '#' {
		return (v + 1) % 12
	}
	panic("???")
}

func semi2str(semi int, flat bool) string {
	s := semiStrings[semi]
	if len(s) == 1 || !flat {
		return s
	}
	s = semiStrings[(semi+1)%12]
	return string(s[0]) + "b"
}
