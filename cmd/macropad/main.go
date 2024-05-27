package main

import (
	"context"
	"flag"
	"log"
	"time"

	sayo "github.com/chzchzchz/sayo-rgb"
	"github.com/gvalkov/golang-evdev"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

var ev2key = map[int]string{
	evdev.KEY_1: "1",
	evdev.KEY_2: "2",
	evdev.KEY_3: "3",
	evdev.KEY_4: "4",
	evdev.KEY_5: "5",
	evdev.KEY_6: "6",
	evdev.KEY_7: "7",
	evdev.KEY_8: "8",
	evdev.KEY_9: "9",
	evdev.KEY_0: "0",
	evdev.KEY_A: "a",
	evdev.KEY_B: "b",
	evdev.KEY_C: "c",
	evdev.KEY_D: "d",
	evdev.KEY_E: "e",
	evdev.KEY_F: "f",
	evdev.KEY_G: "g",
	evdev.KEY_H: "h",
	evdev.KEY_I: "i",
	evdev.KEY_J: "j",
	evdev.KEY_K: "k",
	evdev.KEY_L: "l",
	evdev.KEY_M: "m",
	evdev.KEY_N: "n",
}

var key2idx = map[byte]int{
	'1': 0,
	'2': 1,
	'3': 2,
	'4': 3,

	'5': 4,
	'6': 5,
	'7': 6,
	'8': 7,

	'9': 8,
	'0': 9,
	'a': 10,
	'b': 11,

	'c': 12,
	'd': 13,
	'e': 14,
	'f': 15,

	'g': 16,
	'h': 17,
	'i': 18,
	'j': 19,

	'n': 20,
	'm': 21,
	'l': 22,
	'k': 23,
}

func kbd(path string) (<-chan *evdev.KeyEvent, error) {
	kbd, err := evdev.Open(path)
	if err != nil {
		return nil, err
	}
	log.Println("attached", path)
	kbd.Grab()
	ch := make(chan *evdev.KeyEvent, 3)
	go func() {
		defer kbd.File.Close()
		defer kbd.Release()
		defer close(ch)
		for {
			ev, err := kbd.ReadOne()
			if err != nil {
				log.Println("failed to read", err)
				return
			}
			if ev.Type != evdev.EV_KEY {
				continue
			}
			ch <- evdev.NewKeyEvent(ev)
		}
	}()
	return ch, nil
}

type CC struct {
	cc   int
	data int
}

type SetCCFunc func(bool, *CC)
type SetRGBFunc func(bool, *RgbQueue)

type Key struct {
	idx    int
	on     bool
	rgbOn  [3]byte
	rgbOff [3]byte
	desc   string
	setCC  SetCCFunc
	setRGB SetRGBFunc

	*CC
	bank *Bank
}

type Bank struct {
	keys []*Key
}

func (b *Bank) add(k *Key) {
	if k.bank != nil {
		panic("bank already assigned")
	}
	b.keys = append(b.keys, k)
	k.bank = b
}

func off(rgb *RgbQueue) {
	for i := 0; i < 24; i++ {
		rgb.Write(i, [3]byte{0, 0, 0})
	}
}

func reset(rgb *RgbQueue, keys []Key) {
	for _, k := range keys {
		if k.CC != nil {
			k.updateCC()
			k.updateRGB(rgb)
		}
	}
}

func (k *Key) updateCC() {
	if k.setCC != nil {
		k.setCC(k.on, k.CC)
		return
	}
	if k.on {
		k.data = 0x7f
	} else {
		k.data = 0
	}
}

type RgbCmd struct {
	color [3]byte
	idx   int
}

type RgbQueue struct {
	c     chan RgbCmd
	donec chan struct{}
	dev   *sayo.Device
}

func NewRgbQueue(dev *sayo.Device) *RgbQueue {
	return &RgbQueue{c: make(chan RgbCmd, 16), donec: make(chan struct{}), dev: dev}
}

func (q *RgbQueue) Write(idx int, color [3]byte) {
	q.c <- RgbCmd{idx: idx, color: color}
}

func (q *RgbQueue) Loop(ctx context.Context) {
	defer close(q.donec)
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-q.c:
			r, g, b := cmd.color[0], cmd.color[1], cmd.color[2]
			q.dev.Write(sayo.ModeSwitchOnce, cmd.idx, r, g, b)
		}
		// Macropad does not like rapid changes from bank toggles.
		time.Sleep(5 * time.Millisecond)
	}
}

func (k *Key) updateRGB(rgb *RgbQueue) {
	c := k.rgbOn
	if !k.on {
		c = k.rgbOff
	}
	rgb.Write(k.idx, c)
}

func (k *Key) off(rgb *RgbQueue) {
	rgb.Write(k.idx, [3]byte{0, 0, 0})
}

func (k *Key) toggle(rgb *RgbQueue) {
	k.on = !k.on
	// At most one key may be active for a bank.
	if b := k.bank; k.on && b != nil {
		for _, kk := range b.keys {
			if kk.on && kk != k {
				kk.on = false
				kk.updateCC()
				kk.updateRGB(rgb)
			}
		}
	}
	k.updateCC()
	k.updateRGB(rgb)
	if k.setRGB != nil {
		k.setRGB(k.on, rgb)
	}
}

func setupKeys() []Key {
	keys := make([]Key, 24)
	red := [3]byte{0x80, 0, 0}
	blu := [3]byte{0, 0, 0xf0}
	blk := [3]byte{0, 0, 0}
	grn := [3]byte{0, 0x80, 0}
	rotary := &CC{102, 0} // midi.controller.upper.102=rotary.speed-select

	setPerc := func(on bool, rgb *RgbQueue) {
		for i := 1; i <= 3; i++ {
			if on {
				keys[i].updateRGB(rgb)
			} else {
				keys[i].off(rgb)
			}
		}
	}
	keys[0] = Key{CC: &CC{80, 0}, desc: "percussion enable", rgbOn: red, rgbOff: blk, setRGB: setPerc}
	keys[1] = Key{CC: &CC{81, 0}, desc: "percussion decay", rgbOn: red, rgbOff: blu}
	keys[2] = Key{CC: &CC{82, 0}, desc: "percussion harmonic", rgbOn: red, rgbOff: blu}
	keys[3] = Key{CC: &CC{83, 0}, desc: "percussion volume", rgbOn: red, rgbOff: blu}

	mkSetHornValue := func(v int) SetCCFunc {
		return func(on bool, cc *CC) {
			d := (cc.data / 15) % 3
			if on {
				d = d + v
			}
			cc.data = 15 * d
		}
	}
	keys[5] = Key{CC: rotary, desc: "horn chorale", rgbOn: grn, rgbOff: blk, setCC: mkSetHornValue(3)}
	keys[6] = Key{CC: rotary, desc: "horn tremolo", rgbOn: grn, rgbOff: blk, setCC: mkSetHornValue(6)}
	keys[7] = Key{CC: &CC{31, 0}, desc: "vibrato upper", rgbOn: red, rgbOff: blk}

	mkSetDrumValue := func(v int) SetCCFunc {
		return func(on bool, cc *CC) {
			d := 3 * ((cc.data / 15) / 3)
			if on {
				d = d + v
			}
			cc.data = 15 * d
		}
	}
	keys[10] = Key{
		CC: rotary, desc: "drum chorale", rgbOn: grn, rgbOff: blk, setCC: mkSetDrumValue(1),
	}
	keys[9] = Key{
		CC: rotary, desc: "drum tremolo", rgbOn: grn, rgbOff: blk, setCC: mkSetDrumValue(2),
	}
	keys[8] = Key{CC: &CC{30, 0}, desc: "vibrato lower ", rgbOn: red, rgbOff: blk}

	keys[15] = Key{CC: &CC{65, 0}, desc: "overdrive enable", rgbOn: red, rgbOff: blk}

	for i := range keys {
		keys[i].idx = i
	}

	hornBank := &Bank{}
	hornBank.add(&keys[5])
	hornBank.add(&keys[6])

	drumBank := &Bank{}
	drumBank.add(&keys[9])
	drumBank.add(&keys[10])

	// rotary speed select is more complicated
	return keys
}

func main() {
	cnFlag := flag.String("name", "macropad", "midi client name")
	evFlag := flag.String("event",
		"/dev/input/by-id/usb-SayoDevice_SayoDevice_6x4F_03008CB81CA71454-event-kbd",
		"event file for device")
	hidFlag := flag.String("hidraw", "/dev/hidraw4", "hidraw device for rgb")
	flag.Parse()

	rgb, err := sayo.NewDevice(*hidFlag)
	if err != nil {
		panic(err)
	}
	rgbq := NewRgbQueue(rgb)
	go rgbq.Loop(context.Background())

	keys := setupKeys()
	off(rgbq)
	reset(rgbq, keys)

	ch, err := kbd(*evFlag)
	if err != nil {
		panic(err)
	}

	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	outc := make(chan alsa.SeqEvent, 16)
	go func() {
		defer close(outc)
		for ev := range outc {
			ev.SeqAddr = alsa.SubsSeqAddr
			if err := aseq.Write(ev); err != nil {
				log.Printf("write failed: %v", err)
				panic(err)
			}
		}
	}()
	go func() {
		for {
			if _, err := aseq.Read(); err != nil {
				return
			}
		}
	}()

	midiChannel := 0
	ccCmd := midi.MakeCC(midiChannel)
	for ev := range ch {
		v, ok := ev2key[int(ev.Scancode)]
		if !ok || ev.State != evdev.KeyDown {
			continue
		}
		idx := key2idx[v[0]]
		if k := &keys[idx]; k.CC != nil {
			k.toggle(rgbq)
			msg := []byte{ccCmd, byte(k.cc), byte(k.data)}
			outc <- alsa.SeqEvent{Data: msg}
		}
	}
}
