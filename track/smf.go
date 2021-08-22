package track

import (
	"context"
	"os"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"
)

type SMF struct {
	Track
	f	*os.File
	r      smf.Reader
	readyc chan struct{}
}

func NewSMF(ctx context.Context, path string) (*Track, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r := smfreader.New(f, smfreader.NoteOffVelocity())

	readyc := make(chan struct{})
	t := &SMF{
		f:      f,
		r:      r,
		readyc: readyc,
		Track:  newTrack(ctx),
	}
	if err := r.ReadHeader(); err != nil {
		return nil, err
	}
	go t.read()
	<-readyc
	return &t.Track, nil
}

func (t *SMF) read() {
	tick := uint32(0)
	hdr := t.r.Header()
	defer func() {
		t.f.Close()
		close(t.outc)
		if t.readyc != nil {
			close(t.readyc)
		}
	}()
	for {
		m, err := t.r.Read()
		if err != nil {
			if err != smf.ErrFinished {
				t.err = err
			}
			return
		}
		var out []byte
		// fmt.Printf("got %s: %+v; delta=%d\n", m, m.Raw(), t.r.Delta())
		// Filter everything but meta info and channel stuff.
		switch msg := m.(type) {
		case meta.TimeSig:
			t.TicksPerBeat = int(msg.ClocksPerClick)
		case meta.Tempo:
			t.BPM = int(msg.BPM())
		case meta.TrackSequenceName:
			close(t.readyc)
			t.readyc = nil
		case channel.NoteOn:
			out = msg.Raw()
		case channel.NoteOff:
			out = msg.Raw()
		case channel.NoteOffVelocity:
			out = msg.Raw()
		}
		if m == meta.EndOfTrack {
			tick = 0
		} else {
			//  x / {2,4,8,16} => 32,16,8,4ths
			deltaTicks := uint32(t.TicksPerBeat) * hdr.TimeFormat.(smf.MetricTicks).In64ths(t.r.Delta()) / 16
			tick += deltaTicks
		}
		if out != nil {
			select {
			case <-t.ctx.Done():
				t.err = t.ctx.Err()
				return
			case t.outc <- TickMessage{Raw: out, Tick: int(tick)}:
			}
		}
	}
}
