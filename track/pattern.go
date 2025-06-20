package track

import (
	"bytes"
	"os"
	"sort"
	"time"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/realtime"
	"gitlab.com/gomidi/midi/midimessage/sysex"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"
	"gitlab.com/gomidi/midi/writer"

	"github.com/chzchzchz/midispa/midi"
)

// Patterns have midi data.
type Pattern struct {
	MidiTimeSig

	LastTick uint32
	Msgs     []TickMessage
	Name     string
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

func (p *Pattern) Duration() time.Duration {
	return time.Duration(p.LastTick) * p.TickDuration()
}

func (p *Pattern) Beats() float64 {
	return float64(p.LastTick) / float64(p.TicksPerBeat)
}

func (p *Pattern) Bars() float64 {
	return (p.Beats() * (float64(p.Denominator) / float64(p.Numerator))) / 4.0
}

func (p *Pattern) Merge(p2 *Pattern) {
	if p.TicksPerBeat == 0 {
		p.MidiTimeSig = p2.MidiTimeSig
	}
	if p.TicksPerBeat != p2.TicksPerBeat {
		panic("pattern: adjust ticks to match on merge")
	}
	if p2.LastTick > p.LastTick {
		p.LastTick = p2.LastTick
	}
	p.size += p2.size
	p.Msgs = append(p.Msgs, p2.Msgs...)
	lt := func(i, j int) bool {
		if p.Msgs[i].Tick != p.Msgs[j].Tick {
			return p.Msgs[i].Tick < p.Msgs[j].Tick
		}
		// Prioritize offs before ons.
		iType := midi.Message(p.Msgs[i].Raw[0])
		jType := midi.Message(p.Msgs[j].Raw[0])
		if iType == midi.CC {
			if jType != midi.CC {
				return true
			}
			return p.Msgs[i].Raw[1] < p.Msgs[j].Raw[1]
		}
		return iType == midi.NoteOff && jType == midi.NoteOn
	}
	sort.Slice(p.Msgs, lt)
}

func (p *Pattern) Append(p2 *Pattern) {
	if p.TicksPerBeat == 0 {
		p.MidiTimeSig = p2.MidiTimeSig
	}
	if p.TicksPerBeat != p2.TicksPerBeat {
		panic("pattern: adjust ticks to match on append")
	}
	for i := range p2.Msgs {
		msg := p2.Msgs[i]
		msg.Tick += int(p.LastTick)
		p.Msgs = append(p.Msgs, msg)
	}
	p.LastTick += p2.LastTick
	p.size += p2.size
}

func (p *Pattern) AppendMessage(msg TickMessage) {
	if msg.Tick < int(p.LastTick) {
		panic("append message tick too early")
	}
	p.LastTick = uint32(msg.Tick)
	p.Msgs = append(p.Msgs, msg)
	p.size += len(msg.Raw)
}

func (p *Pattern) read(r smf.Reader) error {
	hdr, dsum := r.Header(), uint32(0)
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
			p.Numerator = int(msg.Numerator)
			p.Denominator = int(msg.Denominator)
		case meta.Tempo:
			p.BPM = int(msg.BPM())
		case channel.ControlChange:
			out = msg.Raw()
			if dsum == 0 {
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
		dsum += r.Delta()
		int64ths := hdr.TimeFormat.(smf.MetricTicks).In64ths(dsum)
		tick := (uint32(p.TicksPerBeat) * int64ths) / 16
		p.LastTick = tick
		if m == meta.EndOfTrack {
			dsum, tick = 0, 0
		}
		if out != nil {
			p.Msgs = append(p.Msgs, TickMessage{Raw: out, Tick: int(tick)})
			p.size += len(out)
		}
	}
}

func (p *Pattern) Write(w *writer.SMF) error {
	// Convert back to messages.
	buf := bytes.NewBuffer(make([]byte, p.size))
	r := midireader.New(buf, func(m realtime.Message) {})
	lastTick := uint32(0)
	for _, tickmsg := range p.Msgs {
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
	if lastTick < p.LastTick {
		delta := p.LastTick - lastTick
		w.SetDelta((delta * w.Ticks4th()) / uint32(p.TicksPerBeat))
	}
	return nil
}
