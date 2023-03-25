package main

import (
	"bytes"
	"os"
	"sort"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/realtime"
	"gitlab.com/gomidi/midi/midimessage/sysex"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"
	"gitlab.com/gomidi/midi/writer"

	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
)

// Patterns have midi data.
type Pattern struct {
	track.MidiTimeSig
	lastTick uint32
	msgs     []track.TickMessage
	size     int
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

func EmptyPattern() *Pattern { return &Pattern{} }

func (p *Pattern) Merge(p2 *Pattern) {
	if p.TicksPerBeat == 0 {
		p.MidiTimeSig = p2.MidiTimeSig
	}
	if p.TicksPerBeat != p2.TicksPerBeat {
		panic("pattern: adjust ticks to match on merge")
	}
	if p2.lastTick > p.lastTick {
		p.lastTick = p2.lastTick
	}
	p.size += p2.size
	p.msgs = append(p.msgs, p2.msgs...)
	lt := func(i, j int) bool {
		if p.msgs[i].Tick != p.msgs[j].Tick {
			return p.msgs[i].Tick < p.msgs[j].Tick
		}
		// Prioritize offs before ons.
		iType := midi.Message(p.msgs[i].Raw[0])
		jType := midi.Message(p.msgs[j].Raw[0])
		if iType == midi.CC {
			if jType != midi.CC {
				return true
			}
			return p.msgs[i].Raw[1] < p.msgs[j].Raw[1]
		}
		return iType == midi.NoteOff && jType == midi.NoteOn
	}
	sort.Slice(p.msgs, lt)
}

func (p *Pattern) Append(p2 *Pattern) {
	if p.TicksPerBeat == 0 {
		p.MidiTimeSig = p2.MidiTimeSig
	}
	if p.TicksPerBeat != p2.TicksPerBeat {
		panic("pattern: adjust ticks to match on append")
	}
	for i := range p2.msgs {
		msg := p2.msgs[i]
		msg.Tick += int(p.lastTick)
		p.msgs = append(p.msgs, msg)
	}
	p.lastTick += p2.lastTick
	p.size += p2.size
}

func (p *Pattern) read(r smf.Reader) error {
	tick := uint32(0)
	hdr := r.Header()
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
		int64ths := hdr.TimeFormat.(smf.MetricTicks).In64ths(r.Delta())
		deltaTicks := (uint32(p.TicksPerBeat) * int64ths) / 16
		tick += deltaTicks
		p.lastTick = tick
		if m == meta.EndOfTrack {
			tick = 0
		}
		if out != nil {
			p.msgs = append(p.msgs, track.TickMessage{Raw: out, Tick: int(tick)})
			p.size += len(out)
		}
	}
}

func (p *Pattern) write(w *writer.SMF) error {
	// Convert back to messages.
	buf := bytes.NewBuffer(make([]byte, p.size))
	r := midireader.New(buf, func(m realtime.Message) {})
	lastTick := uint32(0)
	for _, tickmsg := range p.msgs {
		if _, err := buf.Write(tickmsg.Raw); err != nil {
			panic(err)
		}
		msg, err := r.Read()
		if err != nil {
			panic(err)
		}
		if uint32(tickmsg.Tick) < lastTick {
			panic("out of order delta")
		}
		delta := uint32(tickmsg.Tick) - lastTick
		//fmt.Printf("got %s: %+v; delta=%d; tick=%d\n", msg, msg.Raw(), delta, tickmsg.Tick)
		delta = (delta * w.Ticks4th()) / uint32(p.TicksPerBeat)
		if delta > 0 {
			w.SetDelta(delta)
		}
		w.Write(msg)
		lastTick = uint32(tickmsg.Tick)
	}
	// Append any empty time.
	if lastTick < p.lastTick {
		delta := p.lastTick - lastTick
		w.SetDelta((delta * w.Ticks4th()) / uint32(p.TicksPerBeat))
	}
	return nil
}
