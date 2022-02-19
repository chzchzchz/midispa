package main

import (
	"errors"

	"github.com/chzchzchz/midispa/sysex/akai"
)

var errOutOfRange = errors.New("out of range")

type writeFunc func([]byte) error

var (
	CCTopLeftLEDs = 0x1B

	NoteMode  = 26
	NoteMute1 = 36
	NoteMute2 = 37
	NoteMute3 = 38
	NoteMute4 = 39

	// 0 off, 3 max red, 4 max green
	LEDRed     = 1
	LEDGreen   = 2
	CCMuteLED1 = 0x28
	CCMuteLED2 = 0x29
	CCMuteLED3 = 0x2a
	CCMuteLED4 = 0x2b

	NotePatternUp   = 31
	NotePatternDown = 32
	NoteBrowser     = 33

	EncoderLeft  = 127
	EncoderRight = 1
	CCSelect     = 118
	CCVolume     = 16
	CCPan        = 17
	CCFilter     = 18
	CCResonance  = 19

	NoteGridLeft  = 34
	NoteGridRight = 35

	NoteAccent    = 44
	NoteSnap      = 45
	NoteTap       = 46
	NoteOverview  = 47
	NoteShift     = 48
	NoteAlt       = 49
	NoteMetronome = 50
	NoteWait      = 51
	NotePlay      = 51
	NoteCountdown = 52
	NoteStop      = 52
	NoteLoopRec   = 53
)

type Fire struct {
	write writeFunc
}

func NewFire(w writeFunc) *Fire {
	return &Fire{w}
}

func Note2Grid(n int) (int, int, bool) {
	if n < 54 || n > 117 {
		return 0, 0, false
	}
	v := n - 54
	return v % 16, v / 16, true
}

// i++
// return write([]byte{0xb0, byte(CCTopLeftLEDs), 0x10 | (i % 0xf)})

func (f *Fire) LedsOff() error {
	lp := akai.LightPads{}
	for i := 0; i < 64; i++ {
		lp.Pads = append(lp.Pads, akai.Pad{Idx: i})
	}
	b, _ := lp.MarshalBinary()
	if err := f.write(b); err != nil {
		return err
	}
	for _, n := range []int{
		CCTopLeftLEDs,
		NoteMute1, NoteMute2, NoteMute3, NoteMute4,
		CCMuteLED1, CCMuteLED2, CCMuteLED3, CCMuteLED4,
		NotePatternUp, NotePatternDown, NoteBrowser,
		NoteGridLeft, NoteGridRight,
		NoteAccent, NoteSnap, NoteTap, NoteOverview, NoteShift, NoteAlt,
		NoteMetronome, NoteWait, NoteCountdown, NoteLoopRec,
	} {
		if err := f.SetLed(n, 0); err != nil {
			return err
		}
	}
	return nil
}

func (f *Fire) SetLed(n, v int) error {
	return f.write([]byte{0xb0, byte(n), byte(v)})
}

// Print rasterizes a string using character coordinates.
func (f *Fire) Print(x, y int, s string) error {
	return f.printFont(x, y, s, byte2glyph)
}

func (f *Fire) PrintInvert(x, y int, s string) error {
	font := func(b byte) []byte {
		v := byte2glyph(b)
		for i := range v {
			v[i] = ^v[i]
		}
		return v
	}
	return f.printFont(x, y, s, font)
}

func (f *Fire) printFont(x, y int, s string, font func(byte) []byte) error {
	if len(s)+x >= 128/6 || x < 0 || y < 0 || y >= 8 {
		return errOutOfRange
	}
	var bmp []byte
	for _, v := range s {
		bmp = append(bmp, font(byte(v))...)
	}
	su := akai.ScreenUpdate{
		BandStart:   y,
		BandEnd:     y,
		ColumnStart: x * 6,
		ColumnEnd:   (x+len(s))*6 - 1,
		Bitmap:      bmp,
	}
	b, err := su.MarshalBinary()
	if err != nil {
		return err
	}
	return f.write(b)
}

func (f *Fire) Off() error {
	if err := f.LedsOff(); err != nil {
		return err
	}
	return f.ClearOLED()
}

func (f *Fire) ClearOLED() error {
	return f.ClearOLEDRows(0, 8)
}

func (f *Fire) ClearOLEDRows(y, n int) error {
	su := akai.ScreenUpdate{
		BandStart:   y,
		BandEnd:     y + n - 1,
		ColumnStart: 0,
		ColumnEnd:   0x7f,
		Bitmap:      make([]byte, 128*n),
	}
	b, err := su.MarshalBinary()
	if err != nil {
		return err
	}
	return f.write(b)
}

func (f *Fire) LightPad(x, y, r, g, b int) error {
	idx := x + y*16
	if idx < 0 || idx >= 64 {
		return errOutOfRange
	} else if r < 0 || g < 0 || b < 0 || r > 127 || g > 127 || b > 127 {
		return errOutOfRange
	}
	pad := akai.Pad{Idx: idx, Red: r, Green: g, Blue: b}
	lp := akai.LightPads{Pads: []akai.Pad{pad}}
	v, _ := lp.MarshalBinary()
	return f.write(v)
}

func (f *Fire) LightPadRow(row int, vals [16][3]int) error {
	if row < 0 || row >= 4 {
		return errOutOfRange
	}
	lp := akai.LightPads{}
	for i := 0; i < 16; i++ {
		lp.Pads = append(
			lp.Pads,
			akai.Pad{
				Idx:   row*16 + i,
				Red:   vals[i][0],
				Green: vals[i][1],
				Blue:  vals[i][2],
			})
	}
	v, _ := lp.MarshalBinary()
	return f.write(v)
}

func (f *Fire) LightPadColumn(col int, vals [4][3]int) error {
	if col < 0 || col > 15 {
		return errOutOfRange
	}
	lp := akai.LightPads{}
	for row := 0; row < 4; row++ {
		lp.Pads = append(
			lp.Pads,
			akai.Pad{
				Idx:   row*16 + col,
				Red:   vals[row][0],
				Green: vals[row][1],
				Blue:  vals[row][2],
			})
	}
	v, _ := lp.MarshalBinary()
	return f.write(v)
}
