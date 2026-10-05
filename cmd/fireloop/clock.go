package main

import (
	"context"
	"time"
)

// stepClock is the time the sequencer loop spends its life inside: where it thinks it is,
// and how it waits for the next step. Production reads realStepClock; a test supplies a
// clock that advances by exactly the duration asked for, which is what lets a whole set be
// played in a test and keeps the drift the loop takes back at a pattern boundary honest.
type stepClock interface {
	// Now is where the clock stands. A pattern is anchored to this reading and the wait
	// back to that anchor is measured from it, so both have to come from one clock or
	// the boundary corrects for drift it never had.
	Now() time.Time
	// Wait blocks for the duration, or until ctx is done, and reports whether it waited
	// the duration out. False means playback was asked to stop rather than that it failed.
	Wait(ctx context.Context, d time.Duration) bool
	// Stop releases what Wait is holding, once the loop is done with it.
	Stop()
}

// realStepClock waits on the wall clock. The timer is built once and reset per step: a step
// timer is short and frequent enough that leaving one per step for the collector is waste,
// and the module's minimum Go version makes a bare Reset safe, because a timer channel is
// synchronous and a Reset cannot expose the previous deadline.
type realStepClock struct {
	timer *time.Timer
}

func (c *realStepClock) Now() time.Time {
	return time.Now()
}

func (c *realStepClock) Wait(ctx context.Context, d time.Duration) bool {
	if c.timer == nil {
		c.timer = time.NewTimer(d)
	} else {
		c.timer.Reset(d)
	}
	select {
	case <-c.timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *realStepClock) Stop() {
	if c.timer != nil {
		c.timer.Stop()
	}
}
