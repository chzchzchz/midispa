package main

import (
	"io"

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
