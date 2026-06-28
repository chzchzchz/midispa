package theory

import "testing"

func TestStr2Semi(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"C", 0},
		{"D", 2},
		{"E", 4},
		{"F", 5},
		{"G", 7},
		{"A", 9},
		{"B", 11},
		{"C#", 1},
		{"D#", 3},
		{"F#", 6},
		{"G#", 8},
		{"A#", 10},
		{"Cb", 11}, // C flat is B
		{"Db", 1},
		{"Eb", 3},
		{"Fb", 4},
		{"Gb", 6},
		{"Ab", 8},
		{"Bb", 10},
		{"E#", 5}, // E sharp is F
		{"B#", 0}, // B sharp is C (wraps)
	}
	for _, tt := range tests {
		if got := str2semi(tt.in); got != tt.want {
			t.Errorf("str2semi(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSemi2Str(t *testing.T) {
	// natural notes (white keys) - same regardless of flat flag
	naturals := []struct {
		semi int
		name string
	}{
		{0, "C"},
		{2, "D"},
		{4, "E"},
		{5, "F"},
		{7, "G"},
		{9, "A"},
		{11, "B"},
	}
	for _, tt := range naturals {
		if got := semi2str(tt.semi, false); got != tt.name {
			t.Errorf("semi2str(%d, false) = %q, want %q", tt.semi, got, tt.name)
		}
		if got := semi2str(tt.semi, true); got != tt.name {
			t.Errorf("semi2str(%d, true) = %q, want %q", tt.semi, got, tt.name)
		}
	}
	// black keys: test sharps vs flats
	black := []struct {
		semi  int
		sharp string
		flat  string
	}{
		{1, "C#", "Db"},
		{3, "D#", "Eb"},
		{6, "F#", "Gb"},
		{8, "G#", "Ab"},
		{10, "A#", "Bb"},
	}
	for _, tt := range black {
		if got := semi2str(tt.semi, false); got != tt.sharp {
			t.Errorf("semi2str(%d, false) = %q, want %q", tt.semi, got, tt.sharp)
		}
		if got := semi2str(tt.semi, true); got != tt.flat {
			t.Errorf("semi2str(%d, true) = %q, want %q", tt.semi, got, tt.flat)
		}
	}
}
