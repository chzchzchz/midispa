package main

import (
	"io"
	"log"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const PPQN = 24
const BeatsPerMeasure = 4

type Sequencer struct {
	b    *Bank
	aseq *alsa.Seq

	reader         Reader
	pausedReader   Reader
	currentPattern int
	nextPattern    int
	clockTicks     int
	patTicks       int

	used [16]bool
}

func emptyReader() Reader {
	return NewEmptyReader(PPQN * BeatsPerMeasure)
}

func NewSequencer(aseq *alsa.Seq, b *Bank) *Sequencer {
	return &Sequencer{
		b:      b,
		aseq:   aseq,
		reader: emptyReader(),
	}
}

func (s *Sequencer) next() {
	// log.Printf("next pattern %d on tick %d", s.nextPattern, s.clockTicks)
	s.reader, s.patTicks = s.b.Reader(s.nextPattern), 0
	s.currentPattern = s.nextPattern
}

func (s *Sequencer) clock() {
	evs, err := s.reader.Read()
	if err == io.EOF {
		for i := range s.used {
			s.used[i] = false
		}
		s.next()
		evs, err = s.reader.Read()
		must(err)
	}
	for _, ev := range evs {
		if midi.IsNoteOn(ev.Raw[0]) {
			s.used[midi.Channel(ev.Raw[0])] = true
		}
		must(s.aseq.Write(alsa.MakeEvent(ev.Raw)))
	}
	s.clockTicks++
	s.patTicks++
}

func (s *Sequencer) Run() error {
	for {
		ev, err := s.aseq.Read()
		if err != nil {
			return err
		}
		cmd := ev.Data[0]
		switch {
		case cmd == midi.SongSelect:
			s.nextPattern = int(ev.Data[1])
			if s.currentPattern == s.nextPattern {
				// Force reload.
				// s.reader = s.b.Reader(s.nextPattern)
				// TODO Update reader
			}
			log.Printf("looper pattern switch to %d", s.nextPattern)
		case midi.IsNoteOff(cmd):
		case cmd == midi.Clock:
			s.clock()
		case cmd == midi.Start:
			s.clockTicks, s.patTicks = 0, 0
			s.next()
		case cmd == midi.Stop:
			s.pausedReader = s.reader
			for i := 0; i < 16; i++ {
				if !s.used[i] {
					continue
				}
				s.used[i] = false
				s.aseq.Write(alsa.MakeEvent(
					[]byte{midi.MakeCC(i), midi.AllNotesOff, 0}))
			}
			s.reader = emptyReader()
		case cmd == midi.Continue:
			s.reader, s.pausedReader = s.pausedReader, nil
		}
	}
}
