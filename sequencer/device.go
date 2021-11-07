package sequencer

import (
	"fmt"
	"log"
	"time"

	"github.com/chzchzchz/midispa/track"

	amidi "github.com/scgolang/midi"
)

type Device struct {
	Sequencer
	d *amidi.Device
}

func NewDevice(inc <-chan track.TickMessage, d *amidi.Device) *Device {
	dev := &Device{
		Sequencer: newSequencer(inc),
		d:         d,
	}
	go dev.loop()
	return dev
}

func (m *Device) loop() {
	defer close(m.donec)
	var q []track.TickMessage

	log.Printf("device %q waiting on tempo", m.d.Name)
	m.Tempo = <-m.Tempoc
	ticks, ticker := 0, time.NewTicker(m.TickDuration())
	defer ticker.Stop()
	log.Printf("device %q clock duration: %v", m.d.Name, m.TickDuration())

	defer func() {
		log.Println("loop exiting for", m.d.Name)
	}()

	inc := m.inc
	for {
		var out []byte
		select {
		case msg := <-inc:
			if msg.Raw == nil {
				// Channel closed.
				inc = nil
			} else if msg.Tick > ticks {
				q = append(q, msg)
				if len(q) > MaxQueueSize {
					// Apply backpressure.
					inc = nil
				}
			} else if msg.Tick == ticks {
				out = msg.Raw
			} else {
				panic(fmt.Errorf("past deadline %d vs %d", msg.Tick, ticks))
			}
		case m.Tempo = <-m.Tempoc:
			ticker.Reset(m.TickDuration())
		case <-ticker.C:
			ticks++
			for _, msg := range q {
				if msg.Tick > ticks {
					break
				}
				out = append(out, msg.Raw...)
				q = q[1:]
			}
			if len(q) <= MaxQueueSize {
				inc = m.inc
			}
		}
		m.Tick = int32(ticks)
		if out != nil {
			if _, err := m.d.Write(out); err != nil {
				panic(err)
			}
		}
		if inc == nil && len(q) == 0 {
			return
		}
	}
}
