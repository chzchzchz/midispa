package main

import (
	"log"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

type ClockEvents struct {
	t     []time.Time
	start int
}

func (ce *ClockEvents) Mean() time.Duration {
	if len(ce.t) == 0 {
		return 0
	}
	sum := time.Duration(0)
	for i := 0; i < len(ce.t)-1; i++ {
		a := (i + ce.start) % len(ce.t)
		b := (i + ce.start + 1) % len(ce.t)
		diff := ce.t[b].Sub(ce.t[a])
		sum += diff
	}
	return time.Duration(float64(sum) / float64((len(ce.t) - 1)))
}

func (ce *ClockEvents) Add(t time.Time) {
	if len(ce.t) < 96*2 {
		ce.t = append(ce.t, t)
		return
	}
	ce.t[ce.start] = t
	ce.start = (ce.start + 1) % len(ce.t)
}

func (ce *ClockEvents) Read(inc <-chan alsa.SeqEvent) {
	lastMean := ce.Mean()
	c := 0
	// Drain any messages that were queued up.
	for {
		<-inc
		time.Sleep(time.Millisecond)
		if len(inc) == 0 {
			break
		}
	}
	// Read messages as they appear.
	for ev := range inc {
		cmd := ev.Data[0]
		if cmd == midi.Stop || cmd == midi.Start {
			c = 0
		}
		if cmd != midi.Clock {
			continue
		}
		ce.Add(time.Now())
		newMean := ce.Mean()
		c++
		if c != 24 {
			continue
		}
		if newMean != lastMean {
			bpm := (60.0 / newMean.Seconds()) / PPQN
			log.Printf("input bpm = %g (%v)\n", bpm, newMean)
			lastMean = newMean
		}
		c = 0
	}
}
