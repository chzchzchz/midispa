package main

import (
	"os"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/sysex"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"

	"github.com/chzchzchz/midispa/track"
)

type Pattern struct {
	track.MidiTimeSig
	lastTick uint32
	msgs     []track.TickMessage
}

func NewPattern(path string) (*Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := smfreader.New(f, smfreader.NoteOffVelocity())
	if err := r.ReadHeader(); err != nil {
		return nil, err
	}
	p := &Pattern{}
	p.read(r)
	return p, nil
}

func (p *Pattern) read(r smf.Reader) error {
	hdr, tick, dsum := r.Header(), uint32(0), uint32(0)
	for {
		m, err := r.Read()
		if err != nil {
			if err != smf.ErrFinished {
				return err
			}
			return nil
		}
		var out []byte
		//fmt.Printf("read %s: %+v; delta=%d\n", m, m.Raw(), r.Delta())
		switch msg := m.(type) {
		case meta.TimeSig:
			p.TicksPerBeat = int(msg.ClocksPerClick)
		case meta.Tempo:
			p.BPM = int(msg.BPM())
		case channel.ControlChange:
			out = msg.Raw()
			if tick == 0 {
				// ignore rosegarden cc's
				switch msg.Controller() {
				case 7, 10, 91, 93:
					out = nil
				}
			}
		case channel.Message:
			out = msg.Raw()
		case sysex.Message:
			out = msg.Raw()
		}
		//  x / {2,4,8,16} => 32,16,8,4ths
		delta := r.Delta()
		dsum += delta
		int64ths := hdr.TimeFormat.(smf.MetricTicks).In64ths(dsum)
		tick = (uint32(p.TicksPerBeat) * int64ths) / 16
		p.lastTick = tick
		if m == meta.EndOfTrack {
			dsum, tick = 0, 0
		}
		if out != nil {
			p.msgs = append(p.msgs, track.TickMessage{Raw: out, Tick: int(tick)})
		}
	}
}
