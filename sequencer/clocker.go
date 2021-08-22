package sequencer

import (
	"github.com/chzchzchz/midispa/track"
)

// Adjusts incoming pattern to match global ticks.
type Clocker struct {
	Sequencer
	dstc chan<- track.TickMessage
	dst  *Sequencer
}

func NewClocker(inc <-chan track.TickMessage, dstc chan<- track.TickMessage, dst *Sequencer) *Clocker {
	c := &Clocker{
		Sequencer: newSequencer(inc),
		dstc:      dstc,
		dst:       dst,
	}
	go c.loop()
	return c
}

func (m *Clocker) loop() {
	defer close(m.donec)

	baseTick := int32(0)
	for {
		var msg track.TickMessage
		select {
		case msg = <-m.inc:
		case m.Tempo = <-m.Tempoc:
			m.dst.Tempoc <- m.Tempo
			// TODO: possibly wait for ack by having dst close a 'last tempo' channel.
		}
		if msg.Raw == nil {
			return
		} else if msg.Tick == 0 {
			baseTick = m.Now()
		} else {
			msg.Tick += int(baseTick)
		}
		select {
		case m.dstc <- msg:
		case <-m.dst.donec:
			return
		}
	}
}
