package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex"
)

type Sequencer struct {
	aseq      *alsa.Seq
	outdir    string
	outfile   string
	start     time.Time
	startRec  time.Time
	running   bool
	recording bool
	pgm       [16]int

	track Track
	bpm   int
}

func NewSequencer(aseq *alsa.Seq, outdir string) *Sequencer {
	return &Sequencer{
		aseq:   aseq,
		outdir: outdir,
		bpm:    120,
		track:  Track{bpm: 120},
	}
}

func NewSingleShotSequencer(aseq *alsa.Seq, outfile string) *Sequencer {
	s := &Sequencer{
		aseq:    aseq,
		outfile: outfile,
		bpm:     120,
		track:   Track{bpm: 120},
	}
	s.startRunning()
	s.startRec, s.recording = time.Now(), true
	return s
}

func (s *Sequencer) processEvents() error {
	for {
		ev, err := s.aseq.Read()
		if err != nil {
			return err
		}
		if err := s.processEvent(ev); err != nil {
			if s.outfile != "" && err == io.EOF {
				return nil
			}
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
	if s.recording {
		t := s.startRec
		if s.running {
			t = s.start
		}
		s.track.Add(Event{time.Since(t), data})
		// log.Printf("recorded %+v", data)
	}
}

func (s *Sequencer) updatePgmLink(midipath string) {
	ch := s.track.channel
	if ch < 0 || ch > 16 {
		return
	}
	pgm := s.pgm[ch]
	log.Printf("relinking channel %d, pgm %d", ch, pgm)
	chS, pgmS := strconv.Itoa(ch), strconv.Itoa(pgm)
	os.Mkdir(filepath.Join(s.outdir, chS), 0755)
	linkPath := filepath.Join(s.outdir, chS, pgmS+".mid")
	os.Remove(linkPath)
	hardPath := filepath.Join("..", filepath.Base(midipath))
	os.Symlink(hardPath, linkPath)
}

func (s *Sequencer) stopAndSave() error {
	if len(s.track.Events()) == 0 {
		log.Println("nothing to save")
		return nil
	}
	// TODO: insert stop event so save() has trailing silence
	t := time.Now()
	tf := t.Format("2006-01-02-03:04:05")
	midipath := filepath.Join(s.outdir, tf+".mid")
	log.Printf("saving to %q", midipath)
	err := s.track.save(midipath)
	if err == nil && s.outdir != "" {
		s.updatePgmLink(midipath)
	}
	s.track = Track{bpm: s.bpm}
	s.recording = false
	return err
}

func (s *Sequencer) startRunning() {
	s.start, s.running = time.Now(), true
	log.Println("started")
}

func (s *Sequencer) processEvent(ev alsa.SeqEvent) error {
	cmd := ev.Data[0]
	switch cmd {
	case midi.SysEx:
		sx := sysex.Decode(ev.Data)
		switch sx.(type) {
		case *sysex.RecordStrobe:
			if !s.recording {
				log.Println("started recording")
				s.startRec = time.Now()
				// TODO: insert start event
			} else {
				log.Println("stopped recording")
				// TODO: remove start event if only one event
			}
			s.recording = !s.recording
			return nil
		case *sysex.RecordExit:
			log.Println("record exit")
			s.recording = false
			return nil
		case *sysex.Eject, *sysex.Stop:
			return s.stopAndSave()
		}
	case midi.Start:
		s.startRunning()
		return nil
	case midi.Stop:
		s.running = false
		log.Println("stopped")
		if s.outfile != "" {
			s.track.save(s.outfile)
			return io.EOF
		}
		return nil
	}
	switch midi.Message(cmd) {
	case midi.CC:
		// cc
		if midi.Channel(cmd) == 0xf {
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
	case midi.NoteOff:
		s.record(ev.Data)
	case midi.NoteOn:
		s.record(ev.Data)
	case midi.Pgm:
		s.pgm[midi.Channel(cmd)] = int(ev.Data[1])
	}
	return nil
}
