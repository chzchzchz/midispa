package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
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

const CcBpmMsb = 16
const CcBpmLsb = 16 + 32

func main() {
	cnFlag := "midiclock"
	flag.StringVar(&cnFlag, "name", "midiclock", "midi client name")
	flag.StringVar(&cnFlag, "n", "midiclock", "midi client name")

	bpmFlag := flag.Float64("bpm", 120.0, "send clock signals at given bpm")
	randPctFlag := flag.Float64("randpct", 0.0, "percentage to swing the clock")

	runFlag := false
	flag.BoolVar(&runFlag, "run", false, "run at start")
	flag.BoolVar(&runFlag, "r", false, "run at start")

	flag.Parse()

	if !runFlag {
		fmt.Printf("beginning in %q stopped mode\n", cnFlag)
	}

	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(cnFlag)
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

	curBpmInt := int(*bpmFlag * 64.0)

	clockDur := int64(0)
	updateClockDur := func() {
		if curBpmInt < 64 {
			return
		}
		cps := ((float64(curBpmInt) / 64.0) / 60.0) * PPQN
		dur := time.Duration(float64(time.Second) / cps)
		atomic.StoreInt64(&clockDur, int64(dur))
		fmt.Printf("bpm = %v\n", float64(curBpmInt)/64.0)
	}
	if runFlag {
		updateClockDur()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	contc := make(chan struct{}, 1)
	go func() {
		defer wg.Done()
		for {
			ev, err := aseq.Read()
			if err != nil {
				panic(err)
			}
			cmd := ev.Data[0]
			if cmd == midi.Clock {
				outc <- ev
			} else if cmd == midi.Stop {
				atomic.StoreInt64(&clockDur, int64(0))
			} else if cmd == midi.Continue {
				updateClockDur()
				select {
				case contc <- struct{}{}:
				default:
				}
			} else if midi.IsCC(cmd) {
				cc, v := ev.Data[1], ev.Data[2]
				if cc == CcBpmLsb {
					curBpmInt = ((curBpmInt >> 7) << 7) | int(v)
					updateClockDur()
				} else if cc == CcBpmMsb {
					curBpmInt = (curBpmInt & 0x7f) | (int(v) << 7)
					updateClockDur()
				}
			}
		}
	}()

	randCoef := *randPctFlag / 100.0
	clockWriter := func() {
		defer wg.Done()
		evClock := alsa.SeqEvent{alsa.SubsSeqAddr, []byte{midi.Clock}}
		nextClock := time.Now()
		for {
			err := aseq.Write(evClock)
			if err != nil {
				panic(err)
			}
			var nextDur time.Duration
			for {
				nextDur = time.Duration(atomic.LoadInt64(&clockDur))
				if nextDur != 0 {
					break
				}
				<-contc
				nextClock = time.Now()
			}

			swing := randCoef * (2.0 * (rand.Float64() - 0.5))
			nextDurSwing := time.Duration(swing * float64(nextDur))
			nextClockSwing := nextClock.Add(nextDurSwing)
			nextClock = nextClock.Add(nextDur)
			//fmt.Println(nextDur, nextDurSwing)
			time.Sleep(time.Until(nextClockSwing))
		}
	}
	if bpmFlag != nil {
		wg.Add(1)
		go clockWriter()
	}
	wg.Wait()
}
