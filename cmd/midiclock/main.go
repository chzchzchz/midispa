package main

import (
	"flag"
	"fmt"
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

// pulse per quarter note
const PPQN = 24.0

func main() {
	cnFlag := flag.String("name", "midiclock", "midi client name")
	flag.Parse()
	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	outc := make(chan alsa.SeqEvent, 16)
	ce := &ClockEvents{}
	go func() {
		defer close(outc)
		lastMean := ce.Mean()
		c := 0
		for {
			<-outc
			time.Sleep(time.Millisecond)
			if len(outc) == 0 {
				break
			}
		}
		for range outc {
			ce.Add(time.Now())
			newMean := ce.Mean()
			c++
			if c != 24 {
				continue
			}
			if newMean != lastMean {
				bpm := (60.0 / newMean.Seconds()) / PPQN
				fmt.Printf("bpm = %g (%v)\n", bpm, newMean)
				lastMean = newMean
			}
			c = 0
		}
	}()
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		cmd := ev.Data[0]
		if cmd == midi.Clock {
			outc <- ev
		}
	}
}
