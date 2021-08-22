package sequencer

import (
	"time"

	"github.com/chzchzchz/midispa/track"
)

const MaxQueueSize = 1024

type Tempo struct {
	track.MidiTimeSig
	Base time.Time
	Tick int32
}

func (t *Tempo) Now() int32 {
	return int32(time.Since(t.Base) / t.TickDuration())
}

type Sequencer struct {
	Tempo
	Tempoc chan Tempo
	donec  chan struct{}
	inc    <-chan track.TickMessage
}

func newSequencer(inc <-chan track.TickMessage) Sequencer {
	return Sequencer{
		donec:  make(chan struct{}),
		Tempoc: make(chan Tempo, 1),
		inc:    inc,
	}
}

func (s *Sequencer) Close() {
	<-s.donec
}
