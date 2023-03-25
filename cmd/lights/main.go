package main

import (
	"context"
	"flag"
	"log"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/examples/lib/dev"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func WriteColor(c ble.Client, ch *ble.Characteristic, r, g, b int) error {
	cmd := make([]byte, 7)
	cmd[0] = 0x56
	cmd[1] = byte(r)
	cmd[2] = byte(g)
	cmd[3] = byte(b)
	cmd[4] = 0
	cmd[5] = 0xf0
	cmd[6] = 0xaa
	return c.WriteCharacteristic(ch, cmd, true)
}

type NoteOn struct {
	note int
	vel  int
}

type NoteState struct {
	on    map[int]struct{}
	order []NoteOn
}

func NewNoteState() *NoteState {
	return &NoteState{on: make(map[int]struct{})}
}

func (ns *NoteState) On(n, vel int) {
	if _, ok := ns.on[n]; ok {
		ns.Off(n)
	}
	ns.on[n] = struct{}{}
	ns.order = append(ns.order, NoteOn{n, vel})
}

func (ns *NoteState) Off(n int) {
	for i := range ns.order {
		if ns.order[i].note == n {
			ns.order = append(ns.order[:i], ns.order[i+1:]...)
			break
		}
	}
}

func (ns *NoteState) Mono() (int, int) {
	if l := len(ns.order); l > 0 {
		no := ns.order[l-1]
		return no.note, no.vel
	}
	return 0, 0
}

func main() {
	macFlag := flag.String("mac", "52:06:C2:00:0E:A9", "mac address")
	devFlag := flag.String("dev", "default", "ble device")
	flag.Parse()

	d, err := dev.NewDevice(*devFlag)
	if err != nil {
		panic(err)
	}
	ble.SetDefaultDevice(d)
	a := ble.NewAddr(*macFlag)
	c, err := ble.Dial(context.TODO(), a)
	if err != nil {
		panic(err)
	}
	defer c.Conn().Close()
	log.Println("connected to", *macFlag)

	c.DiscoverProfile(true)
	var outCh *ble.Characteristic
	wrUUID := ble.UUID16(0xffd9)
	for _, s := range c.Profile().Services {
		//fmt.Printf("%+v\n", *s)
		for _, c := range s.Characteristics {
			//fmt.Printf("===%+v\n", *c)
			if c.UUID.Equal(wrUUID) {
				outCh = c
			}
		}
	}
	if outCh == nil {
		panic("no outch")
	}

	aseq, err := alsa.OpenSeq("midi-lights")
	if err != nil {
		panic(err)
	}
	ns := NewNoteState()
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		colors := func(note, vel byte) (int, int, int) {
			r, g, b := 0, 0, 0
			v := int(255 * (float64(vel) / 127.0))
			if note&0x1 == 0 {
				r = v
			}
			if note&0x2 == 0 {
				g = v
			}
			if note&0x4 == 0 {
				b = v
			}
			if note%8 == 7 {
				r, g, b = v, v/2, v/2
			}
			return r, g, b
		}
		status := ev.Data[0]
		if midi.IsNoteOff(status) {
			ns.Off(int(ev.Data[1]))
			note, vel := ns.Mono()
			r, g, b := colors(byte(note), byte(vel))
			WriteColor(c, outCh, r, g, b)
		} else if midi.IsNoteOn(status) {
			// on
			ns.On(int(ev.Data[1]), int(ev.Data[2]))
			r, g, b := colors(ev.Data[1], ev.Data[2])
			WriteColor(c, outCh, r, g, b)
		}
	}
	log.Println("done")

}
