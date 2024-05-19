package main

import (
	"flag"
	"log"

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

	'f': 12,
	'e': 13,
	'd': 14,
	'c': 15,

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
	cc     int
	data   int
	desc   string
	rgbOn  [3]byte
	rgbOff [3]byte
}

func off(rgb *sayo.Device) {
	for i := 0; i < 24; i++ {
		rgb.Write(sayo.ModeSwitchOnce, i, 0, 0, 0)
	}
}

func reset(rgb *sayo.Device, ccs []CC) {
	for i, cc := range ccs {
		if cc.cc != 0 {
			writeCC(rgb, &cc, i)
		}
	}
}

func writeCC(rgb *sayo.Device, cc *CC, idx int) {
	c := cc.rgbOn
	if cc.data == 0 {
		c = cc.rgbOff
	}
	rgb.Write(sayo.ModeSwitchOnce, idx, c[0], c[1], c[2])
}

func setupCCs() []CC {
	ccs := make([]CC, 24)
	red := [3]byte{0x80, 0, 0}
	blu := [3]byte{0, 0, 0xf0}
	blk := [3]byte{0, 0, 0}
	ccs[0] = CC{80, 0, "percussion enable", red, blk}
	ccs[1] = CC{81, 0, "percussion decay", red, blu}
	ccs[2] = CC{82, 0, "percussion harmonic", red, blu}
	ccs[3] = CC{83, 0, "percussion volume", red, blu}
	ccs[4] = CC{64, 0, "rotary speed", red, blu}
	ccs[5] = CC{65, 0, "overdrive enable", red, blk}
	ccs[6] = CC{30, 0, "vibrato lower ", red, blk}
	ccs[7] = CC{31, 0, "vibrato upper", red, blk}
	// rotary speed select is more complicated
	return ccs
}

func main() {
	cnFlag := flag.String("name", "macropad", "midi client name")
	evFlag := flag.String("event",
		"/dev/input/by-id/usb-SayoDevice_SayoDevice_6x4F_03008CB81CA71454-event-kbd",
		"event file for device")
	hidFlag := flag.String("hidraw", "/dev/hidraw4", "hidraw device for rgb")
	flag.Parse()

	ccs := setupCCs()

	rgb, err := sayo.NewDevice(*hidFlag)
	if err != nil {
		panic(err)
	}
	off(rgb)
	reset(rgb, ccs)

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
		cc := &ccs[idx]
		if cc.cc == 0 {
			continue
		}
		if cc.data == 0 {
			cc.data = 0x7f
		} else {
			cc.data = 0
		}
		writeCC(rgb, cc, idx)
		msg := []byte{ccCmd, byte(cc.cc), byte(cc.data)}
		outc <- alsa.SeqEvent{Data: msg}
	}
}
