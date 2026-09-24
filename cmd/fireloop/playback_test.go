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
