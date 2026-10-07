package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
)

// Middle C keeps probe differences attributable to the envelope
// rather than pitch.
const (
	probeNote = 60
	noteGap   = 500 * time.Millisecond
)

type probeStep struct {
	velocity byte
	duration time.Duration
}

// probeMelody plays every duration at each velocity, from the maximum
// down to mid-range, so envelope differences compare within a velocity
// and velocity-sensitive envelopes are exercised across the sweep.
var probeMelody = [...]probeStep{
	{127, 10 * time.Millisecond},
	{127, 100 * time.Millisecond},
	{127, time.Second},
	{99, 10 * time.Millisecond},
	{99, 100 * time.Millisecond},
	{99, time.Second},
	{64, 10 * time.Millisecond},
	{64, 100 * time.Millisecond},
	{64, time.Second},
}

func (player *midiPlayer) playProbeNotes(ctx context.Context) error {
	noteOff := []byte{midi.MakeNoteOff(player.channel), probeNote, 0}
	for index, step := range probeMelody {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		noteOn := []byte{midi.MakeNoteOn(player.channel), probeNote, step.velocity}
		if err := player.send(noteOn); err != nil {
			return err
		}
		if err := player.waitFor(ctx, step.duration); err != nil {
			return err
		}
		if err := player.send(noteOff); err != nil {
			return err
		}
		if index+1 < len(probeMelody) {
			if err := player.waitFor(ctx, noteGap); err != nil {
				return err
			}
		}
	}
	return nil
}

// Absolute deadlines prevent scheduling error from accumulating across the
// events in a long playback file.
func (player *midiPlayer) playPattern(ctx context.Context, pattern *track.Pattern) error {
	if pattern.TicksPerBeat <= 0 || pattern.BPM <= 0 {
		return fmt.Errorf("playback pattern has invalid timing %d ticks per beat at %d BPM", pattern.TicksPerBeat, pattern.BPM)
	}
	tickDuration := pattern.TickDuration()
	start := player.now()
	for index, message := range pattern.Msgs {
		if message.Tick < 0 {
			return fmt.Errorf("playback message %d has negative tick %d", index, message.Tick)
		}
		if err := player.waitUntil(ctx, start.Add(time.Duration(message.Tick)*tickDuration)); err != nil {
			return err
		}
		if err := context.Cause(ctx); err != nil {
			return err
		}
		if err := player.send(message.Raw); err != nil {
			return fmt.Errorf("play message %d: %w", index, err)
		}
	}
	return player.waitUntil(ctx, start.Add(time.Duration(pattern.LastTick)*tickDuration))
}

func (player *midiPlayer) waitUntil(ctx context.Context, deadline time.Time) error {
	for {
		wait := deadline.Sub(player.now())
		if wait <= 0 {
			return nil
		}
		if err := player.waitFor(ctx, wait); err != nil {
			return err
		}
	}
}

func waitForMIDI(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-timer.C:
		return nil
	}
}
