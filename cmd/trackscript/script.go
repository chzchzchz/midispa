package main

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/chzchzchz/midispa/track"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"
	"gitlab.com/gomidi/midi/writer"
)

type Script struct {
	bpm int
	// NOTE: phrases are compiled into patterns
	patterns map[string]*track.Pattern
	song     []*track.Pattern
}

func (s *Script) Duration() time.Duration {
	d := time.Duration(0)
	for _, p := range s.song {
		d += p.Duration()
	}
	return d
}

func (s *Script) WriteSMF(dest io.Writer) error {
	if len(s.song) == 0 {
		return io.EOF
	}
	tpq := smf.MetricTicks(0)
	wr := writer.NewSMF(dest, 1, smfwriter.TimeFormat(tpq), smfwriter.Format(smf.SMF1))
	writer.Instrument(wr, "trackscript")
	if err := writer.Meter(wr, 4, 4); err != nil {
		return err
	}
	if err := writer.TempoBPM(wr, float64(s.bpm)); err != nil {
		return err
	}
	if err := wr.WriteHeader(); err != nil {
		return err
	}
	wr.SetDelta(0)
	for _, p := range s.song {
		if err := p.Write(wr); err != nil {
			return err
		}
	}
	err := writer.EndOfTrack(wr)
	if err != nil && err != smf.ErrFinished {
		return err
	}
	writer.FinishPlanned(wr)
	return nil
}

func (s *Script) AddPattern(id, fname string) *track.Pattern {
	_, ok := s.patterns[id]
	if ok {
		panic("already defined pattern: " + id)
	}
	midiPath := fname
	if strings.HasSuffix(fname, ".abc") {
		midiPath = fname[:len(fname)-3] + "mid"
		if err := abc2midi(fname, midiPath); err != nil {
			panic("could not generate " + fname + ": " + err.Error())
		}
	}
	p, err := track.NewPattern(midiPath)
	if err != nil {
		panic("pattern error: \"" + err.Error() + "\" on " + id)
	}
	s.patterns[id] = p
	return p
}

func (s *Script) findPattern(id string) *track.Pattern {
	if p := s.patterns[id]; p != nil {
		return p
	} else if _, err := os.Stat(id + ".abc"); err == nil {
		return s.AddPattern(id, id+".abc")
	} else if _, err := os.Stat(id + ".mid"); err == nil {
		return s.AddPattern(id, id+".mid")
	}
	panic("could not find pattern " + id)
}
