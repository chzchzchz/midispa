package main

import (
	"fmt"
	"strings"

	"github.com/bendahl/uinput"
)

type Keyboard struct {
	uinput.Keyboard
}

func newKeyboard() (*Keyboard, error) {
	kbd, err := uinput.CreateKeyboard("/dev/uinput", []byte("midi2macro"))
	if err != nil {
		return nil, err
	}
	return &Keyboard{kbd}, nil
}

var strMap = map[string]int{
	"0": uinput.Key0,
	"1": uinput.Key1,
	"2": uinput.Key2,
	"3": uinput.Key3,
	"4": uinput.Key4,
	"5": uinput.Key5,
	"6": uinput.Key6,
	"7": uinput.Key7,
	"8": uinput.Key8,
	"9": uinput.Key9,

	"a": uinput.KeyA,
	"b": uinput.KeyB,
	"c": uinput.KeyC,
	"d": uinput.KeyD,
	"e": uinput.KeyE,
	"f": uinput.KeyF,
	"g": uinput.KeyG,
	"h": uinput.KeyH,
	"i": uinput.KeyI,
	"j": uinput.KeyJ,
	"k": uinput.KeyK,
	"l": uinput.KeyL,
	"m": uinput.KeyM,
	"n": uinput.KeyN,
	"o": uinput.KeyO,
	"p": uinput.KeyP,
	"q": uinput.KeyQ,
	"r": uinput.KeyR,
	"s": uinput.KeyS,
	"t": uinput.KeyT,
	"u": uinput.KeyU,
	"v": uinput.KeyV,
	"w": uinput.KeyW,
	"x": uinput.KeyX,
	"y": uinput.KeyY,
	"z": uinput.KeyZ,

	"A": uinput.KeyA | 0x8000,
	"B": uinput.KeyB | 0x8000,
	"C": uinput.KeyC | 0x8000,
	"D": uinput.KeyD | 0x8000,
	"E": uinput.KeyE | 0x8000,
	"F": uinput.KeyF | 0x8000,
	"G": uinput.KeyG | 0x8000,
	"H": uinput.KeyH | 0x8000,
	"I": uinput.KeyI | 0x8000,
	"J": uinput.KeyJ | 0x8000,
	"K": uinput.KeyK | 0x8000,
	"L": uinput.KeyL | 0x8000,
	"M": uinput.KeyM | 0x8000,
	"N": uinput.KeyN | 0x8000,
	"O": uinput.KeyO | 0x8000,
	"P": uinput.KeyP | 0x8000,
	"Q": uinput.KeyQ | 0x8000,
	"R": uinput.KeyR | 0x8000,
	"S": uinput.KeyS | 0x8000,
	"T": uinput.KeyT | 0x8000,
	"U": uinput.KeyU | 0x8000,
	"V": uinput.KeyV | 0x8000,
	"W": uinput.KeyW | 0x8000,
	"X": uinput.KeyX | 0x8000,
	"Y": uinput.KeyY | 0x8000,
	"Z": uinput.KeyZ | 0x8000,

	"/": uinput.KeySlash,
	"?": uinput.KeySlash | 0x8000,
	":": uinput.KeySemicolon | 0x8000,
	".": uinput.KeyDot,
	"#": uinput.Key3 | 0x8000,
	"-": uinput.KeyMinus,
	"_": uinput.KeyMinus | 0x8000,
	"+": uinput.KeyEqual | 0x8000,
	"=": uinput.KeyEqual,

	"Space":  uinput.KeySpace,
	"Ctrl":   uinput.KeyLeftctrl,
	"LCtrl":  uinput.KeyLeftctrl,
	"RCtrl":  uinput.KeyRightctrl,
	"PgUp":   uinput.KeyPageup,
	"PgDown": uinput.KeyPagedown,
	"Insert": uinput.KeyInsert,
	"Enter":  uinput.KeyEnter,
}

func (u *Keyboard) Play(keys []int) error {
	hasShift := false
	for _, k := range keys {
		isShift := (k & 0x8000) != 0
		k &= 0x7fff
		if isShift && !hasShift {
			if err := u.KeyDown(uinput.KeyLeftshift); err != nil {
				return err
			}
			hasShift = true
		}
		if err := u.KeyDown(k); err != nil {
			return err
		}
	}
	for i, _ := range keys {
		k := keys[len(keys)-1-i]
		if err := u.KeyUp(k & 0x7fff); err != nil {
			return err
		}
	}
	if hasShift {
		if err := u.KeyUp(uinput.KeyLeftshift); err != nil {
			return err
		}
	}
	return nil
}

func MacroKeys(s string) (ret []int, err error) {
	fields := strings.Fields(s)
	for _, f := range fields {
		v, ok := strMap[f]
		if !ok {
			return nil, fmt.Errorf("unhandled string %q", s)
		}
		ret = append(ret, v)
	}
	return ret, nil
}
