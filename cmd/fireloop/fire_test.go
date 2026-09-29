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

// A blackout blanks the display and then suppresses all further output, updates and
// explicit clears alike, until a wake.
func TestBlackoutDropsUpdatesAndClears(t *testing.T) {
	recorder := &ledRecorder{}
	f := NewFire(recorder.write)
	if err := f.SetLed(NoteAlt, LEDYellow); err != nil {
		t.Fatal(err)
	}
	if err := f.Blackout(); err != nil {
		t.Fatal(err)
	}
	if !f.IsDark() {
		t.Fatal("blackout did not go dark")
	}
	if lit := recorder.lit(); len(lit) != 0 {
		t.Fatalf("blackout left lights on: %v", lit)
	}
	if recorder.clearsDisplay == 0 {
		t.Fatal("blackout did not clear the pads or display")
	}

	recorder.reset()
	if err := f.SetLed(NoteAlt, LEDYellow); err != nil {
		t.Fatal(err)
	}
	if err := f.LightPad(0, 0, 127, 127, 127); err != nil {
		t.Fatal(err)
	}
	if err := f.Print(0, 0, "x"); err != nil {
		t.Fatal(err)
	}
	if err := f.Off(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.leds) != 0 || recorder.clearsDisplay != 0 {
		t.Fatal("output reached the display during a blackout")
	}
	if !f.Wake() {
		t.Fatal("Wake did not report the blackout")
	}
	if f.Wake() {
		t.Fatal("Wake reported a second blackout")
	}
	// A wake puts the display back in service.
	recorder.reset()
	if err := f.SetLed(NoteAlt, LEDYellow); err != nil {
		t.Fatal(err)
	}
	if recorder.leds[NoteAlt] != LEDYellow {
		t.Fatal("the display stayed dead after a wake")
	}
}

// Coordinate validation is a property of the call, not of the display state, so a
// blackout must not hide a bad argument.
func TestBlackoutDoesNotHideInvalidCoordinates(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	if err := f.Blackout(); err != nil {
		t.Fatal(err)
	}
	if err := f.ClearOLEDRows(0, 99); err != errOutOfRange {
		t.Fatalf("ClearOLEDRows error = %v, want %v", err, errOutOfRange)
	}
	if err := f.Print(0, 99, "x"); err != errOutOfRange {
		t.Fatalf("Print error = %v, want %v", err, errOutOfRange)
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
