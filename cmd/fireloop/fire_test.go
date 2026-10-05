package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The font holds 256 byte glyphs and nothing else, so a rune outside printable ASCII has
// none to draw. The device must receive exactly what a question mark produces: one glyph,
// and column arithmetic that matches the bitmap sent.
func TestPrintReplacesRunesTheFontCannotDraw(t *testing.T) {
	var written [][]byte
	f := NewFire(func(msg []byte) error {
		written = append(written, msg)
		return nil
	})
	require.NoError(t, f.Print(0, 0, "Körg"))
	require.NoError(t, f.Print(0, 0, "K?rg"))
	require.Len(t, written, 2, "one screen write per Print")
	require.Equal(t, written[1], written[0], "an undrawable rune must draw as a question mark")
}

func TestClearOLEDRowsRejectsInvalidRange(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	tests := []struct {
		name string
		y    int
		n    int
	}{
		{name: "negative start", y: -1, n: 1},
		{name: "empty", y: 0, n: 0},
		{name: "past screen", y: 8, n: 1},
		{name: "past bottom", y: 7, n: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIsf(t, f.ClearOLEDRows(tt.y, tt.n), errOutOfRange, "ClearOLEDRows(%d, %d)", tt.y, tt.n)
		})
	}
}

// A blackout blanks the display and then suppresses all further output, updates and
// explicit clears alike, until a wake.
func TestBlackoutDropsUpdatesAndClears(t *testing.T) {
	recorder := &ledRecorder{}
	f := NewFire(recorder.write)
	require.NoError(t, f.SetLed(NoteAlt, LEDYellow))
	require.NoError(t, f.Blackout())
	require.True(t, f.IsDark(), "blackout did not go dark")
	require.Empty(t, recorder.lit(), "the blackout left lights on")
	require.NotZero(t, recorder.clearsDisplay, "the blackout did not clear the pads or display")

	recorder.reset()
	require.NoError(t, f.SetLed(NoteAlt, LEDYellow))
	require.NoError(t, f.LightPad(0, 0, 127, 127, 127))
	require.NoError(t, f.Print(0, 0, "x"))
	require.NoError(t, f.Off())
	require.Empty(t, recorder.leds, "a light reached the display during a blackout")
	require.Zero(t, recorder.clearsDisplay, "a redraw reached the display during a blackout")
	require.True(t, f.Wake(), "Wake did not report the blackout")
	require.False(t, f.Wake(), "Wake reported a second blackout")
	// A wake puts the display back in service.
	recorder.reset()
	require.NoError(t, f.SetLed(NoteAlt, LEDYellow))
	require.Equal(t, LEDYellow, recorder.leds[NoteAlt], "the display stayed dead after a wake")
}

// Coordinate validation is a property of the call, not of the display state, so a
// blackout must not hide a bad argument.
func TestBlackoutDoesNotHideInvalidCoordinates(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	require.NoError(t, f.Blackout())
	require.ErrorIs(t, f.ClearOLEDRows(0, 99), errOutOfRange)
	require.ErrorIs(t, f.Print(0, 99, "x"), errOutOfRange)
}

func TestLightPadRejectsInvalidCoordinates(t *testing.T) {
	f := NewFire(func([]byte) error { return nil })
	tests := []struct {
		name string
		x    int
		y    int
	}{
		{name: "negative x", x: -1, y: 0},
		{name: "negative y", x: 0, y: -1},
		{name: "x too large", x: 16, y: 0},
		{name: "y too large", x: 0, y: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIsf(t, f.LightPad(tt.x, tt.y, 0, 0, 0), errOutOfRange, "LightPad(%d, %d)", tt.x, tt.y)
		})
	}
}

// The top-left control is not a plain on/off. Measured on the hardware: zero selects
// Channel, and only the mode bit with a zero nibble blanks all four lights, so clearing
// has to send that value.
func TestLedsOffBlanksTheTopLeftLights(t *testing.T) {
	recorder := &ledRecorder{}
	f := NewFire(recorder.write)
	require.NoError(t, f.LedsOff())
	require.Equal(t, CCTopLeftOff, recorder.leds[CCTopLeftLEDs], "the top-left clear value")
	require.Equal(t, LEDOff, recorder.leds[CCMuteLED1], "the track row clear value")
}

// The top-left mask form was measured on the hardware: one bit per light with the mode
// bit set, and a zero nibble blanking them all. The index form below it selects a single
// light instead, which is why zero is Channel rather than an off.
func TestTopLeftMaskMatchesTheMeasuredValues(t *testing.T) {
	tests := []struct {
		lights []int
		want   int
	}{
		{lights: nil, want: 0x10},
		{lights: []int{TopLeftChannel}, want: 0x11},
		{lights: []int{TopLeftMixer}, want: 0x12},
		{lights: []int{TopLeftChannel, TopLeftMixer}, want: 0x13},
		{lights: []int{TopLeftUser1}, want: 0x14},
		{lights: []int{TopLeftUser2}, want: 0x18},
		{lights: []int{TopLeftChannel, TopLeftMixer, TopLeftUser1, TopLeftUser2}, want: 0x1f},
	}
	for _, tt := range tests {
		require.Equalf(t, tt.want, topLeftMask(tt.lights...), "topLeftMask(%v)", tt.lights)
	}
}

// The font is stored one byte per row and the screen takes one byte per column, so every
// glyph is transposed once at startup and cached. A cache that disagrees with the table it
// was built from would draw letters the font never had, with nothing to catch it.
func TestGlyphCacheMatchesTheFontTable(t *testing.T) {
	for code := range glyphCache {
		rows := font6x8[font6x8Rows*code : font6x8Rows*(code+1)]
		for i := range glyphWidth {
			var want byte
			for j := range font6x8Rows {
				if rows[j]&(1<<uint(7-i)) != 0 {
					want |= 1 << uint(j)
				}
			}
			require.Equalf(t, want, glyphCache[code][i], "glyph %d column %d", code, i)
		}
	}
}

// The cache is shared by every draw, so inverted text must be produced on the way out
// rather than by editing the glyph in place. Otherwise the first inverted label would leave
// the letter upside down for the rest of the set.
func TestInvertedGlyphsDoNotAlterTheCache(t *testing.T) {
	plain := appendGlyph(nil, 'A', false)
	inverted := appendGlyph(nil, 'A', true)
	require.Len(t, plain, glyphWidth)
	require.Len(t, inverted, glyphWidth)
	for i := range plain {
		require.Equalf(t, ^plain[i], inverted[i], "inverted column %d must be the complement", i)
	}
	again := appendGlyph(nil, 'A', false)
	for i := range plain {
		require.Equalf(t, plain[i], again[i], "column %d after an inverted draw", i)
	}
}
