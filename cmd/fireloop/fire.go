package main

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/akai"
)

var errOutOfRange = errors.New("out of range")

// padColumns is the width of the hardware pad grid, which is four rows of sixteen.
const padColumns = 16

// oledTextWidth is how many characters a readout row holds. The screen is 128 columns wide
// and a character is six, so a row could carry 21; readout text is built to 20 so a figure
// at the end is never left half cut, because fitOLEDText trims at this width and a number
// cut in half still reads as a number that is true.
const oledTextWidth = 20

type writeFunc func([]byte) error

// encoderDirection is which way a knob was turned, and whether it was turned at all. A
// knob sends 127 for one way and 1 for the other and nothing else, so a value that is
// neither is not a turn. Reading it as a turn anyway would move whatever the knob drives
// whenever the hardware sent anything else.
func encoderDirection(value int) (direction int, turning bool) {
	switch value {
	case EncoderRight:
		return 1, true
	case EncoderLeft:
		return -1, true
	default:
		return 0, false
	}
}

// Fire hardware definitions. Do not remove entries just because they are
// currently unused; their values map directly to Fire notes and controls.
var (
	CCTopLeftLEDs = 0x1B

	// The four lights at the top left are labelled Channel, Mixer, User 1 and User 2.
	// They are indicators only: there are no buttons there to press, so the sequencer can
	// report state on them but cannot be driven by them. One control drives all four, and
	// the mode bit 0x10 chooses how the low bits are read. Both forms were confirmed on
	// the hardware:
	//
	//	0x00 0x01 0x02 0x03  index form, lights exactly one, in that order
	//	0x10                  mask form with no bits set, all four off
	//	0x11 0x12 0x14 0x18  mask form, one light each, in that order
	//	0x13                  mask form, two lights
	//	0x1f                  mask form, all four
	//
	// Only the mask form can light more than one at a time, since the index form names a
	// single light. Zero is the index for Channel rather than a blank, which is why
	// clearing the indicators sends CCTopLeftOff and not zero.
	CCTopLeftOff           = 0x10
	CCTopLeftMaskBase      = 0x10
	CCTopLeftSelectChannel = 0x00
	CCTopLeftSelectMixer   = 0x01
	CCTopLeftSelectUser1   = 0x02
	CCTopLeftSelectUser2   = 0x03

	// The bit each light occupies in the mask form, for topLeftMask.
	TopLeftChannel = 0
	TopLeftMixer   = 1
	TopLeftUser1   = 2
	TopLeftUser2   = 3

	NoteMode  = 26
	NoteMute1 = 36
	NoteMute2 = 37
	NoteMute3 = 38
	NoteMute4 = 39

	// alt, stop
	LEDYellow    = 1
	LEDMaxYellow = 2

	// 3 max red, 4 max green
	LEDOff     = 0
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

	NoteAccent      = 44
	NoteSnap        = 45
	NoteTap         = 46
	NoteOverview    = 47
	NoteShift       = 48
	NoteAlt         = 49
	NotePatternSong = 50
	NoteMetronome   = 50
	NoteWait        = 51
	NotePlay        = 51
	NoteCountdown   = 52
	NoteStop        = 52
	NoteRecord      = 53
	NoteLoopRec     = 53
)

type Fire struct {
	write writeFunc
	// dark drops display output while a blackout is in effect. The playback worker
	// draws from its own goroutine, so this is read across goroutines.
	dark atomic.Bool
	// blanking is set only while a blackout blanks the display, so that the clear is
	// not suppressed by the flag the blackout has just set.
	blanking atomic.Bool
}

func NewFire(w writeFunc) *Fire {
	return &Fire{write: w}
}

// topLeftMask builds the top-left value that lights the named lights. Lighting more than
// one at a time requires the mask form, because the index form can only name a single
// light. With no lights named it returns the value that blanks all four, which is what
// clearing the indicators wants.
func topLeftMask(lights ...int) int {
	mask := CCTopLeftMaskBase
	for _, light := range lights {
		mask |= 1 << uint(light)
	}
	return mask
}

// Blackout turns the display off and keeps it off. It goes dark before clearing, so a
// playhead draw landing mid-clear cannot light a pad that then stays lit for the rest
// of the blackout. A blackout suppresses all display output; the blanking clear is the
// one exception.
func (f *Fire) Blackout() error {
	if f == nil {
		return nil
	}
	f.dark.Store(true)
	f.blanking.Store(true)
	defer f.blanking.Store(false)
	logger.Info("display blackout", "pads", padColumns*padRows, "buttons", true)
	return f.Off()
}

// out reports whether display output should be written.
func (f *Fire) out() bool {
	return !f.dark.Load() || f.blanking.Load()
}

// Wake reports whether the display was dark, so the caller can redraw what the blackout
// cleared and restore the button lights.
func (f *Fire) Wake() bool {
	if f == nil {
		return false
	}
	return f.dark.Swap(false)
}

// IsDark reports whether a blackout is in effect, which is the state a wake reacts to.
func (f *Fire) IsDark() bool {
	return f != nil && f.dark.Load()
}

func Note2Grid(n int) (int, int, bool) {
	if n < 54 || n > 117 {
		return 0, 0, false
	}
	v := n - 54
	return v % 16, v / 16, true
}

// i++
// return write([]byte{midi.MakeCC(0), byte(CCTopLeftLEDs), 0x10 | (i % 0xf)})

func (f *Fire) LedsOff() error {
	if err := f.PadsOff(); err != nil {
		return err
	}
	for _, n := range []int{
		NoteMute1, NoteMute2, NoteMute3, NoteMute4,
		CCMuteLED1, CCMuteLED2, CCMuteLED3, CCMuteLED4,
		NotePatternUp, NotePatternDown, NoteBrowser,
		NoteGridLeft, NoteGridRight,
		NoteAccent, NoteSnap, NoteTap, NoteOverview, NoteShift, NoteAlt, NoteMode,
		NoteMetronome, NoteWait, NoteCountdown, NoteLoopRec,
	} {
		if err := f.SetLed(n, 0); err != nil {
			return err
		}
	}
	// The top-left control is not a plain on/off: zero lights Channel.
	if err := f.SetLed(CCTopLeftLEDs, CCTopLeftOff); err != nil {
		return err
	}
	return nil
}

// PadsOff blanks the grid a row at a time. A row is the largest pad message the rest of
// the app sends and one known to reach the hardware; a single message covering all 64
// pads is emitted but ignored, which left the grid lit through a blackout.
func (f *Fire) PadsOff() error {
	for start := 0; start < padColumns*padRows; start += padColumns {
		pads := make([]akai.Pad, 0, padColumns)
		for idx := start; idx < start+padColumns; idx++ {
			pads = append(pads, akai.Pad{Idx: idx})
		}
		if err := f.LightPadSlice(pads); err != nil {
			return err
		}
	}
	return nil
}

func (f *Fire) SetLed(n, v int) error {
	if !f.out() {
		return nil
	}
	msg := []byte{midi.MakeCC(0), byte(n), byte(v)}
	logDisplayOut("led", ledName(n), len(msg), msg)
	return f.write(msg)
}

// logDisplayOut records what went to the unit, so a log can tell a display that was never
// drawn from one the hardware ignored. Pads and the screen are separate commands, and only
// a capture of the wire can say which of them arrived.
func logDisplayOut(kind, detail string, size int, msg []byte) {
	if !displayDebug() {
		return
	}
	attrs := []any{"kind", kind, "bytes", size}
	if detail != "" {
		attrs = append(attrs, "what", detail)
	}
	// Only a sysex carries the Fire command byte; a control change is three bytes.
	if len(msg) > 5 && msg[0] == 0xf0 {
		attrs = append(attrs, "command", hexByte(msg[4]))
	}
	logger.Debug("display out", attrs...)
}

func hexByte(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&0xf]})
}

// fontPlaceholder stands in for a rune the 6x8 font has no glyph for, so a kit voice name
// carrying an accent shows a question mark instead of whatever sits at that index of the
// table. Replacing them also leaves one byte per glyph, which is what the width check and
// the column arithmetic in printFont assume when they count with len.
const fontPlaceholder = '?'

// asciiFontText replaces every rune outside printable ASCII with fontPlaceholder, because
// the font is a table of 256 byte glyphs and holds nothing else. Running it a second time
// changes nothing, so a caller may sanitize before clipping to a fixed width.
func asciiFontText(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < ' ' || r > '~' {
			out = append(out, fontPlaceholder)
			continue
		}
		out = append(out, byte(r))
	}
	return string(out)
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
	s = asciiFontText(s)
	if len(s)+x >= 128/6 || x < 0 || y < 0 || y >= 8 {
		return errOutOfRange
	}
	// Validation stays ahead of the blackout check so a coordinate bug still shows up.
	if !f.out() {
		return nil
	}
	bmp := make([]byte, 0, len(s)*glyphWidth)
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
	// The row is named here rather than at the call site above, because a redraw with the
	// trace off should not build a string for a log line that would drop it.
	if displayDebug() {
		logDisplayOut("screen", fmt.Sprintf("row %d", y), len(b), b)
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
	// Validate the range before sizing the bitmap so malformed UI coordinates return an error instead of panicking.
	if n <= 0 || y < 0 || y >= 8 || n > 8-y {
		return errOutOfRange
	}
	// A log has to show what blanked the screen, so every clear is recorded by row.
	logger.Debug("screen clear", "firstRow", y, "rows", n)
	// Validation stays ahead of the blackout check so a coordinate bug still shows up.
	if !f.out() {
		return nil
	}
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
	if x < 0 || x >= 16 || y < 0 || y >= 4 {
		return errOutOfRange
	}
	idx := x + y*16
	if r < 0 || g < 0 || b < 0 || r > 127 || g > 127 || b > 127 {
		return errOutOfRange
	}
	pad := akai.Pad{Idx: idx, Red: r, Green: g, Blue: b}
	return f.LightPadSlice([]akai.Pad{pad})
}

func (f *Fire) LightPadRow(row int, vals [16][3]int) error {
	if row < 0 || row >= 4 {
		return errOutOfRange
	}
	pads := make([]akai.Pad, 0, padColumns)
	for i := 0; i < 16; i++ {
		pads = append(pads, makePad(i, row, vals[i]))
	}
	return f.LightPadSlice(pads)
}

func (f *Fire) LightPadColumn(col int, vals [4][3]int) error {
	if col < 0 || col > 15 {
		return errOutOfRange
	}
	pads := make([]akai.Pad, 0, padRows)
	for row := 0; row < 4; row++ {
		pads = append(pads, makePad(col, row, vals[row]))
	}
	return f.LightPadSlice(pads)
}

func (f *Fire) LightPadSlice(pads []akai.Pad) error {
	if !f.out() {
		return nil
	}
	lp := akai.LightPads{Pads: pads}
	v, err := lp.MarshalBinary()
	if err != nil {
		return err
	}
	logDisplayOut("pads", "", len(v), v)
	return f.write(v)
}
