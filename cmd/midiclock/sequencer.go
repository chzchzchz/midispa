package main

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex"
)

type Sequencer struct {
	curBpmInt int
	// clockDur is the duration between PPQN ticks
	clockDur    int64
	contc       chan struct{}
	outc        chan alsa.SeqEvent
	aseq        *alsa.Seq
	wg          sync.WaitGroup
	swingPct    float64
	pulseInBeat int
}

func (s *Sequencer) UpdateClock() {
	if s.curBpmInt < 64 {
		return
	}
	cps := ((float64(s.curBpmInt) / 64.0) / 60.0) * PPQN
	dur := time.Duration(float64(time.Second) / cps)
	atomic.StoreInt64(&s.clockDur, int64(dur))
	log.Printf("midiclock output bpm = %v\n", float64(s.curBpmInt)/64.0)
}

const ppqn = 24

// computeInterval returns the swing-shaped duration of the next per-pulse
// slot for a given beat position. With swingPct = 50 every slot equals
// baseInterval (straight time); with swingPct > 50 the on-beat halves
// (pulses 0..11) are stretched and the off-beat halves (12..23) shrink,
// so pulse 12 lands at swingPct % of the beat.
func computeInterval(baseInterval float64, pulseInBeat int, swingPct float64) float64 {
	if pulseInBeat < ppqn/2 {
		return baseInterval * swingPct / 50.0
	}
	return baseInterval * (100.0 - swingPct) / 50.0
}

// applyJitter scales an interval by ±randpct%, matching the prior
// per-pulse jitter shape in midiclock. A nil rng falls back to the
// package-level rand source so callers without their own generator
// (i.e. the live ClockWriter) still work.
func applyJitter(interval, randpct float64) float64 {
	return applyJitterWith(interval, randpct, nil)
}

// applyJitterWith is applyJitter with an explicit *rand.Rand for tests.
func applyJitterWith(interval, randpct float64, rng *rand.Rand) float64 {
	if randpct != 0 {
		randCoef := randpct / 100.0
		var r float64
		if rng != nil {
			r = rng.Float64()
		} else {
			r = rand.Float64()
		}
		return interval * (1.0 + randCoef*(2.0*r-1.0))
	}
	return interval
}

func (s *Sequencer) ClockWriter(randpct float64) {
	evClock := alsa.MakeEvent([]byte{midi.Clock})
	nextClock := time.Now()
	var nextBeatClock time.Time
	for {
		err := s.aseq.Write(evClock)
		if err != nil {
			panic(err)
		}
		var nextDur time.Duration
		for {
			nextDur = time.Duration(atomic.LoadInt64(&s.clockDur))
			if nextDur != 0 {
				break
			}
			<-s.contc
			nextClock = time.Now()
			nextBeatClock = time.Time{}
			s.pulseInBeat = 0
		}

		if s.pulseInBeat == ppqn-1 && !nextBeatClock.IsZero() {
			// Next beat's first clock is next midi clock message.
			// Must wait until nextBeatClock before sending first clock of next beat.
			nextClock = nextBeatClock
			nextBeatClock = nextBeatClock.Add(ppqn * nextDur)
		} else {
			interval := computeInterval(float64(nextDur), s.pulseInBeat, s.swingPct)
			interval = applyJitter(interval, randpct)
			nextClock = nextClock.Add(time.Duration(interval))
		}

		time.Sleep(time.Until(nextClock))

		s.pulseInBeat = (s.pulseInBeat + 1) % ppqn
		if s.pulseInBeat == 0 && nextBeatClock.IsZero() {
			// This is the start of beat, before sending midi clock.
			// Start of the next beat will therefore be ppqn * nextDur.
			nextBeatClock = nextClock.Add(ppqn * nextDur)
		}
	}
}

func (s *Sequencer) start(ev alsa.SeqEvent) {
	s.aseq.Write(alsa.MakeEvent(ev.Data))
	s.UpdateClock()
	select {
	case s.contc <- struct{}{}:
	default:
	}
	s.outc <- ev
}

func (s *Sequencer) stop(ev alsa.SeqEvent) {
	log.Println("midiclock stopping")
	atomic.StoreInt64(&s.clockDur, int64(0))
	s.outc <- ev
	s.aseq.Write(alsa.MakeEvent(ev.Data))
}

func (s *Sequencer) Read() {
	ev, err := s.aseq.Read()
	if err != nil {
		panic(err)
	}
	cmd := ev.Data[0]
	switch cmd {
	case midi.Clock:
		s.outc <- ev
	case midi.Stop:
		s.stop(ev)
	case midi.Continue, midi.Start:
		s.start(ev)
	case midi.SysEx:
		v := sysex.Decode(ev.Data)
		switch v.(type) {
		case *sysex.Play:
			ev.Data = []byte{midi.Start}
			s.start(ev)
		case *sysex.Stop:
			ev.Data = []byte{midi.Stop}
			s.stop(ev)
		}
	default:
		if midi.IsCC(cmd) {
			cc, v := ev.Data[1], ev.Data[2]
			if cc == CcBpmLsb {
				s.curBpmInt = ((s.curBpmInt >> 7) << 7) | int(v)
				s.UpdateClock()
			} else if cc == CcBpmMsb {
				s.curBpmInt = (s.curBpmInt & 0x7f) | (int(v) << 7)
				s.UpdateClock()
			} else if cc == CcSwing {
				s.swingPct = ccSwingToSwingPct(v)
				log.Printf("midiclock swing = %v%%\n", int(s.swingPct))
			}
		}
	}
}

func ccSwingToSwingPct(v byte) float64 {
	s := 50.0 * (1.0 + (float64(int(v)-64))/64.0)
	s = max(minSwingPct, s)
	s = min(maxSwingPct, s)
	return s
}

func NewClockSequencer(aseq *alsa.Seq, bpmFlag *float64, swingPct, randPct float64) *Sequencer {
	s := &Sequencer{
		aseq:      aseq,
		curBpmInt: int(*bpmFlag * 64.0),
		contc:     make(chan struct{}, 1),
		outc:      make(chan alsa.SeqEvent, 16),
		swingPct:  swingPct,
	}

	// Compute input clock message intervals.
	go func() {
		defer close(s.outc)
		ce := &ClockEvents{}
		ce.Read(s.outc)
	}()

	// Read midi, send to clock events and adjust clock writer duration.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			s.Read()
		}
	}()

	// Write clock messags to output midi port.
	if bpmFlag != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.ClockWriter(randPct)
		}()
	}
	return s
}
