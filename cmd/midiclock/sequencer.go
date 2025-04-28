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
	clockDur  int64
	contc     chan struct{}
	outc      chan alsa.SeqEvent
	aseq      *alsa.Seq
	wg        sync.WaitGroup
}

func (s *Sequencer) UpdateClock() {
	if s.curBpmInt < 64 {
		return
	}
	cps := ((float64(s.curBpmInt) / 64.0) / 60.0) * PPQN
	dur := time.Duration(float64(time.Second) / cps)
	atomic.StoreInt64(&s.clockDur, int64(dur))
	log.Printf("output bpm = %v\n", float64(s.curBpmInt)/64.0)
}

func (s *Sequencer) ClockWriter(randpct float64) {
	randCoef := randpct / 100.0
	evClock := alsa.MakeEvent([]byte{midi.Clock})
	nextClock := time.Now()
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
		}

		swing := randCoef * (2.0 * (rand.Float64() - 0.5))
		nextDurSwing := time.Duration(swing * float64(nextDur))
		nextClockSwing := nextClock.Add(nextDurSwing)
		nextClock = nextClock.Add(nextDur)
		//fmt.Println(nextDur, nextDurSwing)
		time.Sleep(time.Until(nextClockSwing))
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
		atomic.StoreInt64(&s.clockDur, int64(0))
		s.outc <- ev
		s.aseq.Write(alsa.MakeEvent(ev.Data))
	case midi.Continue, midi.Start:
		s.start(ev)
	case midi.SysEx:
		v := sysex.Decode(ev.Data)
		if _, ok := v.(*sysex.Play); ok {
			ev.Data = []byte{midi.Start}
			s.start(ev)
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
			}
		}
	}
}

func NewClockSequencer(aseq *alsa.Seq, bpmFlag *float64, randPct float64) *Sequencer {
	s := &Sequencer{
		aseq:      aseq,
		curBpmInt: int(*bpmFlag * 64.0),
		contc:     make(chan struct{}, 1),
		outc:      make(chan alsa.SeqEvent, 16),
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
