package main

import (
	"testing"
)

func TestFindBeat(t *testing.T) {
	evs := []Event{
		{Beat: 1},
		{Beat: 2},
		{Beat: 3},
		{Beat: 4},
	}
	p := Pattern{Events: evs}
	for i, tt := range []struct {
		beat float32
		evs  int
	}{
		{0, 4}, {1, 4}, {2, 3}, {3, 2}, {4, 1}, {4.1, 0},
	} {
		if v := len(p.FindBeat(tt.beat)); v != tt.evs {
			t.Errorf("test#%d: expected %d, got %d", i, tt.evs, v)
		}
	}
}
