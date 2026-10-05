package main

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// Swing is midiclock's, measured against fireloop's own grid: the first half of a beat is
// stretched by swingPct/50 and the second shrunk by the same factor the other way, so the beat
// still takes beatDuration however far the swing is pushed. What changes is where the beat's
// middle falls, not how long the beat is. See cmd/midiclock/sequencer.go computeInterval,
// which shapes the same clock over 24 pulses to the quarter; this is that shape over the four
// steps fireloop calls a beat, so the boundary sits between step 1 and step 2 rather than
// between step 0 and step 1. Swinging on step parity would sound like a shuffle.

const (
	// minSwingPct and maxSwingPct are the bounds midiclock validates its -swingpct flag
	// against, copied so a percentage that came off a knob in one program means the same
	// thing in the other.
	minSwingPct = 0.1
	maxSwingPct = 99.9
	// straightSwingPct is named rather than written inline because the session file has to be
	// able to recognise it: state.go writes it as zero so omitempty leaves the key out.
	straightSwingPct = 50.0
	// swingTenths is the resolution the store keeps, so a tenth of a percent is the finest
	// value that survives the trip through the JSON file as an integer.
	swingTenths = 10
	// swingStepPct is what one encoder detent moves the swing by, in the same unit
	// setSwingPct takes. It is deliberately not expressed in tenths: swingTenths is the
	// store's resolution and swingStepPct is a control's step, and a constant named for the
	// first but used as the second would move the swing ten times as far as it says it does.
	swingStepPct = 1.0
	// minTypedSwingPct and maxTypedSwingPct are the bounds the pad keypad accepts, and they
	// are deliberately narrower than the stored ones. The keypad has no decimal point, so a
	// typed value is whole percents; and one stray digit is a mis-hit rather than a groove,
	// which is the same judgement applyPendingSwing makes about discarding rather than
	// clamping. The tenths below ten and the fractions above ninety-nine are still reachable
	// from the encoder and from a controller, and the file still stores the full range.
	minTypedSwingPct = 10
	maxTypedSwingPct = 99
	// separatorDisplayRow is the row under the header, and separatorDisplayRowText is what it
	// carries on its own. printSwing rebuilds the row from both, so the separator is stated
	// once rather than written in redraw and rebuilt somewhere else.
	separatorDisplayRow     = 1
	separatorDisplayRowText = "-----------"
	// swingEntryColumn is where swing entry's own number is shown. A mode's readout is not
	// indented the way a number being typed is, so it takes column zero.
	swingEntryColumn = 0
	// halfBeat is where the swing boundary falls inside a beat, so the long half runs out
	// there. It is half a beat rather than step 2 so the arithmetic below needs no mention
	// of the grid: swungFraction was written over steps once, and every patternStepsPerBeat
	// it multiplied by was divided straight back out.
	halfBeat = 0.5
)

// swing holds tenths of a percent, so 500 is 50.0% and the value crossing goroutines stays an
// integer. Go has no float atomic without punning bits through an integer, and every consumer
// of this wants a number it can print.
//
// The playback worker samples it from its own goroutine while the event loop is still setting
// it, which is the same reason bpm is a package atomic rather than a controller field: it has
// to stay readable from both without a lock that would sit on the step path.
var swing atomic.Int64

// lastGroove is the last swing that was not straight, so the toggle can put it back.
//
// It is a second store rather than a field on the first because "off" is not a value the
// player ever chose to hear: straight is the absence of a swing, and reaching it by winding
// the encoder seventeen detents is the gesture the toggle exists to replace. Zero means the
// session has never been swung, and a toggle from straight with nothing to restore does
// nothing rather than inventing a groove.
//
// It is deliberately not saved with the session. A file that recorded it would restore a
// groove the saved swing never mentioned, and the saved swing is the whole truth.
var lastGroove atomic.Int64

func init() {
	setSwingPct(straightSwingPct)
}

// currentSwingPct and setSwingPct mirror currentBPM and setBPM, converting between percent and
// the store's tenths at the edge so that neither the worker nor the display has to know which.
func currentSwingPct() float64 {
	return float64(swing.Load()) / swingTenths
}

// setSwingPct writes the store and remembers any value that is not straight, so every route
// to a groove — the encoder, typed digits, a controller, a session file — leaves the toggle
// with something to restore. Writing straight deliberately does not clear it.
func setSwingPct(value float64) {
	raw := int64(math.Round(value * swingTenths))
	swing.Store(raw)
	if float64(raw)/swingTenths != straightSwingPct {
		lastGroove.Store(raw)
	}
}

// clampSwingPct is the range the controls can produce. A file's value is not clamped by the
// setter but by swingPctFrom, because callers clamp here and the file has a rule of its own
// about what zero means.
func clampSwingPct(value float64) float64 {
	return math.Max(minSwingPct, math.Min(maxSwingPct, value))
}

// ccSwingToSwingPct is midiclock's mapping, copied rather than reinvented, so that the same
// knob means the same thing in both programs. It takes an int rather than midiclock's byte
// because fireloop has the controller number as an int by the time it reaches a case.
//
// 64 is straight and 127 is 99.2, which is where a musician expects a knob's centre and its
// top to be.
func ccSwingToSwingPct(value int) float64 {
	return clampSwingPct(50.0 * (1.0 + float64(value-64)/64.0))
}

// swingPctFor is what goes in the file. A straight swing is written as zero so that omitempty
// leaves the key out, which is what keeps a straight session's file identical to what the
// previous build wrote.
func swingPctFor(pct float64) float64 {
	if pct == straightSwingPct {
		return 0
	}
	return pct
}

// swingPctFrom is the file's swing on the way in. Zero is straight, because that is what
// swingPctFor writes for it and what a file written before swing existed decodes to. Anything
// else is a real value, and is clamped into the range the controls could have produced.
func swingPctFrom(pct float64) float64 {
	if pct == 0 {
		return straightSwingPct
	}
	return clampSwingPct(pct)
}

// swungFraction maps a position inside one beat, 0 to 1, to how far into that beat the swung
// clock has run. The first half is stretched by swingPct/50 and the second shrunk by the same
// factor the other way, which is the shape midiclock gives its 24 pulses measured on the step
// grid this package plays on.
func swungFraction(withinBeat, swingPct float64) float64 {
	if withinBeat < halfBeat {
		return withinBeat * swingPct / 50.0
	}
	return swingPct/100.0 + (withinBeat-halfBeat)*(100.0-swingPct)/50.0
}

// swungBeatTime is how far the swung clock has run by a beat position in a pattern. The shaping
// repeats every beat, so the whole beats are counted straight and only the remainder is handed
// to swungFraction.
//
// Flooring is what makes the beat total fall out for free: the two halves of a whole beat sum
// to exactly one however far swing is pushed, so swungBeatTime(4, 500ms, 66) is 2s, the same
// as straight. Swing moves the steps inside a measure; it does not change how long a measure
// takes, which is what keeps a swung pattern from drifting against the anchor playback carries
// it from.
func swungBeatTime(beat float32, beatDur time.Duration, swingPct float64) float64 {
	if beatDur <= 0 {
		return 0
	}
	whole := math.Floor(float64(beat))
	return (whole + swungFraction(float64(beat)-whole, swingPct)) * float64(beatDur)
}

// swungSpan is how long the stretch between two beat positions takes, swung.
//
// The guard on a non-positive beat duration is not decoration: beatDuration returns zero for a
// tempo that is not positive rather than dividing by it, and a span of zero for every beat
// would have the worker spinning instead of playing.
func swungSpan(fromBeat, toBeat float32, beatDur time.Duration, swingPct float64) time.Duration {
	from := swungBeatTime(fromBeat, beatDur, swingPct)
	return time.Duration(swungBeatTime(toBeat, beatDur, swingPct) - from)
}

// swingLabel is the number as the display shows it: whole percents without their tenths, and a
// tenth when there is one. Both readouts go through it, because they show the value at the
// same time and two formatters are two chances for them to disagree.
func swingLabel(swing float64) string {
	if swing == math.Trunc(swing) {
		return fmt.Sprintf("%.0f", swing)
	}
	return fmt.Sprintf("%.1f", swing)
}

// swingText is the separator's trailing label, or empty at straight. The leading space is part
// of the label rather than of the layout, because the row is rebuilt whole each time and there
// is no column to place the text at.
func swingText(swing float64) string {
	if swing == straightSwingPct {
		return ""
	}
	return " Sw " + swingLabel(swing)
}

// Swing entry: Snap toggles it, the encoder moves the swing a percent a detent, and the pads
// are a keypad that types it outright. It leaves playback running, which length mode does not,
// because a swing changes nothing in any pattern and stopping the set to move a groove control
// would make the control useless on a running groove.

// ToggleSwingEntry enters or leaves swing entry.
func (p *PatternBank) ToggleSwingEntry() error {
	return p.setSwingEntry(!p.swingEditActive())
}

func (p *PatternBank) setSwingEntry(active bool) error {
	p.clearPadState()
	// Note editing owns the grid with the palette and length mode owns the readout row and
	// the encoder, so entering swing entry over either would leave a mode whose readout it
	// has taken and whose pads it has taken. The same two calls setLengthMode makes.
	if active && p.noteEditActive() {
		if err := p.setNoteEdit(false); err != nil {
			return err
		}
	}
	if active && p.lengthEditActive() {
		if err := p.setLengthMode(false); err != nil {
			return err
		}
	}
	p.claimMode(active, swingEdit)
	if active {
		// A tempo entry the player abandoned — Shift, some pads, and a mode switch instead
		// of a release — leaves digits behind, and applying them as a swing would be a
		// gesture nobody made.
		p.controller.pending = 0
		if err := p.pads.SetLed(NoteSnap, LEDRed); err != nil {
			return err
		}
		return p.printSwingEntry()
	}
	p.applyPendingSwing()
	if err := p.pads.SetLed(NoteSnap, LEDOff); err != nil {
		return err
	}
	return p.printStepStatus()
}

// ToggleSwingOnOff is the groove control's on and off: straight, or the last swing that was
// not straight. It is deliberately not the same key as swing entry, because entering a mode
// and changing a value are different gestures and a player reaching for the groove while the
// set is running wants the value, not a readout that has to be left again.
//
// Nothing here clears the entry mode, so it can be used from inside swing entry as well as
// from step edit.
func (p *PatternBank) ToggleSwingOnOff() error {
	from := currentSwingPct()
	to := straightSwingPct
	if from == straightSwingPct {
		if remembered := lastGroove.Load(); remembered != 0 {
			to = float64(remembered) / swingTenths
		} else {
			// Never swung in this session, so there is no groove to put back. Saying so
			// is better than answering a toggle with a swing the player never set.
			logger.Debug("swing toggle", "from", from, "to", from, "remembered", "none")
			return nil
		}
	}
	setSwingPct(to)
	logger.Debug("swing toggle", "from", from, "to", currentSwingPct())
	// The value is in two places at once, and redrawing one of them and not the other is how
	// the readout and the separator drift apart.
	return p.redraw()
}

// applyPendingSwing is what leaving swing entry does with a number the pads typed.
//
// Discarded rather than clamped, unlike a value from a file: one stray digit is a mis-hit,
// and turning it into a tenth of a percent would answer a gesture nobody made. Three digits
// cannot be a swing at all either, so they are discarded the same way. The player typed 66,
// watched the readout fill in, and pressed Snap to leave — applying is what they meant.
func (p *PatternBank) applyPendingSwing() {
	pending := p.controller.pending
	p.controller.pending = 0
	if pending < minTypedSwingPct || pending > maxTypedSwingPct {
		return
	}
	from := currentSwingPct()
	setSwingPct(float64(pending))
	logger.Debug("swing typed", "from", from, "to", currentSwingPct(), "typed", pending)
}

// AdjustSwing moves the swing by one detent. It does nothing outside swing entry, so the knob
// keeps whatever meaning it has in that mode.
func (p *PatternBank) AdjustSwing(dir int) error {
	if !p.swingEditActive() {
		return nil
	}
	from := currentSwingPct()
	to := clampSwingPct(from + float64(dir)*swingStepPct)
	if to == from {
		// Already against a bound, so there is nothing to redraw and nothing to say.
		return nil
	}
	setSwingPct(to)
	logger.Debug("swing", "from", from, "to", to)
	// The value is in two places at once, and redrawing one of them and not the other is how
	// the readout and the separator drift apart.
	return p.redraw()
}

// printSwingEntry is the readout row while swing entry owns it, the way printLength owns it
// while length entry does. Column zero, like printLength, because a mode's own readout does
// not indent itself for the tempo entry it briefly shares the row with.
func (p *PatternBank) printSwingEntry() error {
	return p.printText(readoutRow, 0, fmt.Sprintf("Swing: %s", swingLabel(currentSwingPct())), false)
}

// printSwing rewrites the separator row rather than appending to it. There is no partial-row
// clear — displayClear takes a row and a count and nothing finer — so a suffix written only
// while the value was not straight would outlive the value that justified it.
func (p *PatternBank) printSwing() error {
	return p.printText(separatorDisplayRow, 0, separatorDisplayRowText+swingText(currentSwingPct()), false)
}

// typeNumber is the pads-as-keypad gesture both entries use: Shift for the tempo, swing entry
// for the swing. One routine rather than two because the addend, the three-digit cap and the
// roll-over are one set of rules, and a second copy of them is a second thing to keep right.
// Only the label and the column differ, and neither number is applied here — the tempo waits
// for Shift to be released and the swing for swing entry to be left.
func (c *Controller) typeNumber(x, y int, what string, col int, format string) error {
	c.pending *= 10
	if c.pending > 999 {
		c.pending = 0
	}
	c.pending += padDigit(y, x)
	logger.Debug(what, "pending", c.pending, "padX", x, "padY", y)
	return c.patbank.printReadout(col, fmt.Sprintf(format, c.pending))
}

// padDigit is the number a pad contributes to a number being typed: one through nine across
// the first three rows, and zero for the bottom row, which is what lets a leading zero be
// typed rather than the run being started by it. The modulo on x is what makes the leftmost
// pad of each group of four read as one.
func padDigit(y, x int) int {
	if y == padRows-1 {
		return 0
	}
	return (3*y + ((x % 4) % 3)) + 1
}
