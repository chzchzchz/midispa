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

type Tempo struct {
	bpm             int
	beatsPerMeasure int
	ppqn            int
}

func (t *Tempo) MeasureTicks() int { return t.ppqn * t.beatsPerMeasure }

var defaultTempo = Tempo{120, 4, 24}

type Sequencer struct {
	aseq          *alsa.Seq
	outdir        string
	outfile       string
	matchMeasure  bool
	start         time.Time
	startRec      time.Time
	startRecTicks int
	running       bool
	recording     bool

	// Program slot for current track.
	slot int

	track      Track
	clockTicks int

	Tempo
}

func NewSequencer(aseq *alsa.Seq, outdir string) *Sequencer {
	slotsPath := filepath.Join(outdir, "slots")
	os.Mkdir(slotsPath, 0755)
	return &Sequencer{
		aseq:   aseq,
		outdir: outdir,
		track:  Track{bpm: defaultTempo.bpm},
		Tempo:  defaultTempo,
	}
}

func NewSingleShotSequencer(aseq *alsa.Seq, outfile string) *Sequencer {
	s := &Sequencer{
		aseq:    aseq,
		outfile: outfile,
		track:   Track{bpm: defaultTempo.bpm},
		Tempo:   defaultTempo,
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

func (s *Sequencer) linkPath() string {
	slotsPath := filepath.Join(s.outdir, "slots")
	return filepath.Join(slotsPath, strconv.Itoa(s.slot)+".mid")
}

func (s *Sequencer) removeSlotLink() {
	if !s.recording || s.outdir == "" {
		// Only remove if recording and continous mode.
		return
	}
	os.Remove(s.linkPath())
}

func (s *Sequencer) updateSlotLink(midipath string) {
	if s.outdir == "" {
		// single shot does not update slots
		return
	}
	linkPath := s.linkPath()
	os.Remove(linkPath)
	hardPath := filepath.Join("..", filepath.Base(midipath))
	os.Symlink(hardPath, linkPath)
}

func (s *Sequencer) adjustTrackToMeasure() {
	t := time.Now()

	// Conversion factor from duration to ticks.
	tickDur := t.Sub(s.start) / time.Duration(s.clockTicks)

	// Note durations are recorded since s.start, not s.startRec since the
	// clock is running. There's no need to adjust based on startRecTicks.
	// Round first event tick to nearest 16th.
	firstEvTick := int(s.track.First().clock / tickDur)
	alignment := s.ppqn / 4
	firstEvTick = alignment * ((firstEvTick + alignment/2) / alignment)
	// Round up to nearest starting measure.
	mt := s.MeasureTicks()
	firstMeasureTick := mt * ((firstEvTick + mt/2) / mt)

	if firstEvTick < firstMeasureTick {
		// Move first event tick forward to first measure tick if too early.
		measureAlign := tickDur * time.Duration((firstMeasureTick - firstEvTick))
		s.track.ShiftTime(measureAlign)
	}
	// Subtract time from start of recording to first measure.
	measureShift := time.Duration(firstMeasureTick) * tickDur
	s.track.ShiftTime(-measureShift)

	if c := s.track.First().clock; c < 0 {
		s.track.ShiftTime(-c)
	}

	// Trim last measure; usually the note offs are here.
	lastEvTick := int(s.track.Last().clock / tickDur)
	endTick := mt * ((lastEvTick + mt - 1) / mt)
	endTickDur := time.Duration(endTick) * tickDur

	s.track.Erase(endTickDur+1, t.Sub(s.start))

	// log.Printf("start time: %v, end time: %v", s.track.First().clock, s.track.Last().clock)
	// All notes off.
	for _, ch := range s.track.Channels() {
		s.track.Add(Event{
			endTickDur,
			[]byte{midi.MakeCC(ch), midi.AllNotesOff, 0x7f}})
	}
	// log.Printf("start time: %v, end time: %v", s.track.First().clock, s.track.Last().clock)
}

func (s *Sequencer) save() error {
	if s.track.Empty() {
		log.Println("nothing to save")
		return nil
	}
	// TODO: insert stop event so save() has trailing silence
	t := time.Now()
	tf := t.Format("2006-01-02-03:04:05")
	midipath := filepath.Join(s.outdir, tf+".mid")
	log.Printf("saving to %q on last clock %d", midipath, s.clockTicks)

	if s.matchMeasure {
		s.adjustTrackToMeasure()
	}

	// Save it and update any links.
	if err := s.track.save(midipath); err != nil {
		return err
	}
	s.updateSlotLink(midipath)
	return nil
}

func (s *Sequencer) startRunning() {
	s.start, s.running = time.Now(), true
	log.Println("midirec started")
}

func (s *Sequencer) processCC(data []byte) {
	if midi.Channel(data[0]) == 0xf {
		// TODO: tempo control
		switch data[1] {
		case CCTempoLSB:
			panic("lsb")
		case CCTempoMSB:
			panic("msb")
		}
		return
	}
	s.record(data)
}

func (s *Sequencer) processSysex(data []byte) error {
	sx := sysex.Decode(data)
	switch sx.(type) {
	case *sysex.RecordStrobe:
		log.Println("midirec recording at tick", s.clockTicks)
		s.track = Track{bpm: s.bpm}
		s.startRec, s.startRecTicks = time.Now(), s.clockTicks
		s.recording = true
		// This used to toggle recording, but it shouldn't based
		// on the MMC spec.
		// I like resetting the recording data when strobing twice,
		// but it's not strictly what is intended by the MMC spec.
	case *sysex.RecordExit:
		log.Println("midirec record exit")
		s.recording = false
	case *sysex.Eject, *sysex.Stop:
		err := s.save()
		s.track = Track{bpm: s.bpm}
		s.recording = false
		return err
	}
	return nil
}

func (s *Sequencer) processEvent(ev alsa.SeqEvent) error {
	cmd := ev.Data[0]
	switch cmd {
	case midi.SysEx:
		return s.processSysex(ev.Data)
	case midi.Start:
		s.clockTicks = 0
		s.startRunning()
		return nil
	case midi.Stop:
		s.running = false
		log.Println("midirec stopped")
		if s.outfile != "" {
			s.track.save(s.outfile)
			return io.EOF
		}
		return nil
	}
	switch midi.Message(cmd) {
	case midi.CC:
		s.processCC(ev.Data)
	case midi.NoteOff:
		s.record(ev.Data)
	case midi.NoteOn:
		s.record(ev.Data)
	case midi.SongSelect:
		s.slot = int(ev.Data[1])
		s.removeSlotLink()
	case midi.Clock:
		/*
			if s.clockTicks%s.MeasureTicks() == 0 {
				m := s.clockTicks/s.MeasureTicks()
				log.Println("begin measure", m)
			}
		*/
		s.clockTicks++
	}
	return nil
}
