package main

import "testing"

func TestClearOLEDRowsRejectsInvalidRange(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	tests := []struct {
		name string
		y    int
		n    int
	}{
		{name: "negative start", y: -1, n: 1},
		{name: "empty", y: 0, n: 0},
		{name: "past screen", y: 8, n: 1},
		{name: "past bottom", y: 7, n: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := f.ClearOLEDRows(tt.y, tt.n); err != errOutOfRange {
				t.Fatalf("ClearOLEDRows(%d, %d) = %v, want %v", tt.y, tt.n, err, errOutOfRange)
			}
		})
	}
}

func TestLightPadRejectsInvalidCoordinates(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	tests := []struct {
		name string
		x    int
		y    int
	}{
		{name: "negative x", x: -1, y: 0},
		{name: "negative y", x: 0, y: -1},
		{name: "x too large", x: 16, y: 0},
		{name: "y too large", x: 0, y: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := f.LightPad(tt.x, tt.y, 0, 0, 0); err != errOutOfRange {
				t.Fatalf("LightPad(%d, %d) = %v, want %v", tt.x, tt.y, err, errOutOfRange)
			}
		})
	}
}
