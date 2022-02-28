package main

import (
	"bytes"
	"io"
	"log"
	"path/filepath"
	"time"

	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/realtime"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

type Sequencer struct {
	aseq      *alsa.Seq
	outdir    string
	playback  io.Writer
	start     time.Time
	startRec  time.Time
	running   bool
	recording bool

	track Track
}

func NewSequencer(aseq *alsa.Seq, outdir string) *Sequencer {
	return &Sequencer{
		aseq:   aseq,
		outdir: outdir,
	}
}

func (s *Sequencer) processEvents() error {
	for {
		ev, err := s.aseq.Read()
		if err != nil {
			return err
		}
		if err := s.processEvent(ev); err != nil {
			return err
		}
	}
}

const (
	// TODO: a few synths use these CC's; allow alternative mappings
	// bpm = 20 + lsb/2 + msb => 20 + 64 + 127 = 211 max
	CCTempoLSB = 117
	CCTempoMSB = 118
)

func (s *Sequencer) record(data []byte) {
	if s.recording && s.running {
		s.track.Add(Event{time.Since(s.start), data})
		log.Printf("recorded %+v", data)
	}
}

var bpm = 139

func (s *Sequencer) save() error {
	tpq := smf.MetricTicks(960)
	msg2midi := func(data []byte) (midi.Message, error) {
		rd := midireader.New(bytes.NewBuffer(data), func(m realtime.Message) {})
		return rd.Read()
	}
	writeMIDI := func(wr smf.Writer) {
		// Microseconds per quarter note.
		sig := meta.TimeSig{Numerator: 4, Denominator: 3, ClocksPerClick: 24, DemiSemiQuaverPerQuarter: 8}
		must(wr.Write(sig))
		must(wr.Write(meta.Tempo(uint32((60.0 / float64(bpm)) * 1e6))))
		lastClock := time.Duration(0)
		evs := s.track.Events()
		for _, ev := range evs {
			wr.SetDelta(tpq.Ticks(uint32(bpm), ev.clock-lastClock))
			mm, err := msg2midi(ev.data)
			must(err)
			must(wr.Write(mm))
			lastClock = ev.clock
		}
		log.Printf("wrote %d events", len(evs))
		wr.Write(meta.EndOfTrack)
	}
	t := time.Now()
	tf := t.Format("2006-01-02-03:04:05")
	midipath := filepath.Join(s.outdir, tf+".mid")
	log.Printf("ejected to %q", midipath)
	err := smfwriter.WriteFile(
		midipath, writeMIDI, smfwriter.NumTracks(1), smfwriter.TimeFormat(tpq))
	if err != smf.ErrFinished {
		return err
	}
	s.track = Track{}
	return nil
}

func (s *Sequencer) processEvent(ev alsa.SeqEvent) error {
	cmd := ev.Data[0]
	switch cmd {
	case 0xf0:
		sx := sysex.Decode(ev.Data)
		switch sx.(type) {
		case *sysex.RecordStrobe:
			log.Println("recording")
			s.startRec, s.recording = time.Now(), true
			return nil
		case *sysex.Eject:
			return s.save()
		}
	case 0xfa:
		s.start, s.running = time.Now(), true
		log.Println("started")
		return nil
	case 0xfc:
		s.running = false
		log.Println("stopped")
		return nil
	}
	switch cmd & 0xf0 {
	case 0xb0:
		// cc
		if cmd&0xf == 0xf {
			// TODO: tempo control
			switch ev.Data[1] {
			case CCTempoLSB:
				panic("lsb")
			case CCTempoMSB:
				panic("msb")
			}
			return nil
		}
		s.record(ev.Data)
	case 0x80:
		// note off
		s.record(ev.Data)
	case 0x90:
		// note on
		s.record(ev.Data)
	}
	return nil
}
