package main

import (
	"context"
	"math"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

func startSequencer(aseq *alsa.Seq) {
	ctx, cancel := context.WithCancel(context.Background())
	cancelPlayback = cancel
	go runSequencer(ctx, aseq)
}

func runSequencer(ctx context.Context, aseq *alsa.Seq) error {
	var curPattern *Pattern
	var curBeat float32
	var curBpm int
	var lastColumn int
	reloadPattern := func() {
		curPattern = patbank.CurrentPattern()
		curBpm = bpm
		curBeat = 0
		lastColumn = 15
	}
	reloadPattern()

	// compute measures w/r/t this start time + now() to avoid drift
	start := time.Now()
	for {
		evs := curPattern.FindBeat(curBeat)
		nextBeat := float32(0)
		i := 0
		for i < len(evs) {
			if evs[i].Beat > curBeat {
				// No more events to send for now.
				nextBeat = evs[i].Beat
				break
			}
			// Send out this event.
			msgs := evs[i].ToMidi()
			for _, msg := range msgs {
				err := aseq.Write(alsa.SeqEvent{evs[i].device.SeqAddr, msg})
				if err != nil {
					return err
				}
			}
			i++
		}

		// Light up column if new position
		thisColumn := int(math.Floor(float64(curBeat * 4)))
		if thisColumn != lastColumn {
			// Reset last column
			if err := patbank.drawPadColumn(lastColumn); err != nil {
				return err
			}
			// Set new column
			if err := patbank.drawPadColumnInvert(thisColumn); err != nil {
				return err
			}
			lastColumn = thisColumn
		}

		// Wait until next event, if any.
		next16th := float32(math.Floor(float64(curBeat*4.0))+1.0) / 4.0
		if (nextBeat == 0 || nextBeat > next16th) && curBeat < 4.0 {
			nextBeat = next16th
		}
		if nextBeat >= 4.0 {
			// past measure; reset
			nextBeat = 0
		}
		var waitUntil time.Duration
		if nextBeat != 0 {
			waitTime := (nextBeat - curBeat) * float32(time.Minute/time.Duration(curBpm))
			curBeat = nextBeat
			waitUntil = time.Duration(waitTime)
		} else {
			// reset to next measure
			measureLength := 4 * (time.Minute / time.Duration(curBpm))
			start = start.Add(measureLength)
			reloadPattern()
			waitUntil = time.Until(start)
		}

		select {
		case <-time.After(waitUntil):
		case <-ctx.Done():
			return nil
		}
	}
}
