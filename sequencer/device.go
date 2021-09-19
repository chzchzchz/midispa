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

	m.Tempo = <-m.Tempoc
	ticks, ticker := 0, time.NewTicker(m.TickDuration())
	defer ticker.Stop()
	log.Println("clock duration: ", m.TickDuration())

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

func openBy(f func(d *amidi.Device) bool) (*amidi.Device, error) {
	devs, err := amidi.Devices()
	if err != nil {
		return nil, err
	}
	for _, dev := range devs {
		if f(dev) {
			if err = dev.Open(); err != nil {
				return nil, err
			}
			return dev, nil
		}
	}
	return nil, nil
}

func OpenDeviceById(id string) (*amidi.Device, error) {
	return openBy(func(d *amidi.Device) bool { return d.ID == id })
}

func OpenDeviceByName(name string) (d *amidi.Device, err error) {
	return openBy(func(d *amidi.Device) bool { return d.Name == name })
}
