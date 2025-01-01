package ui

import (
	"log"

	"github.com/gvalkov/golang-evdev"
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

func Kbd(path string) (<-chan byte, error) {
	kbd, err := evdev.Open(path)
	if err != nil {
		return nil, err
	}
	log.Println("attached", path)
	kbd.Grab()
	ch := make(chan byte, 3)
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
			keyev := evdev.NewKeyEvent(ev)
			v, ok := ev2key[int(keyev.Scancode)]
			if !ok || keyev.State != evdev.KeyDown {
				continue
			}
			ch <- v[0]
		}
	}()
	return ch, nil
}
