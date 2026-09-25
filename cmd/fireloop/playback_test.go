package main

import (
	"testing"
	"time"
)

func TestBeatDuration(t *testing.T) {
	tests := []struct {
		name string
		bpm  int
		want time.Duration
	}{
		{name: "sixty", bpm: 60, want: time.Second},
		{name: "one hundred twenty", bpm: 120, want: 500 * time.Millisecond},
		{name: "invalid", bpm: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := beatDuration(tt.bpm); got != tt.want {
				t.Fatalf("beatDuration(%d) = %s, want %s", tt.bpm, got, tt.want)
			}
		})
	}
}

func TestPatternDuration(t *testing.T) {
	short := &Pattern{}
	short.SetLengthSteps(4)
	long := &Pattern{}
	long.SetLengthSteps(8)
	if got := patternDuration(short, 120); got != 500*time.Millisecond {
		t.Fatalf("one-beat pattern duration = %s, want 500ms", got)
	}
	if got := patternDuration(long, 120); got != time.Second {
		t.Fatalf("two-beat pattern duration = %s, want 1s", got)
	}
}
