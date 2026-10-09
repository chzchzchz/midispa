package main

import (
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// The Volume knob, which is the one control that sets a step's dynamics for either kind of
// voice. It lives apart from the palette because the two do different jobs on a step: the
// palette decides which pitch a note has, and this decides how hard it is struck.

// defaultStepVelocity is where the Volume knob starts, and what a chromatic step takes
// when the knob has not been turned yet.
const defaultStepVelocity = midi.DataMax

// velocityStep is how far one detent of the Volume knob moves a step velocity
// before the turn's own speed is counted.
const velocityStep = 1

// velocityQuiescence is how long the Volume knob must rest before the step it
// has been trimming is auditioned. A knob turned quickly sends its detents
// closer together than a note is worth hearing apart, so the detents of a
// fast turn are accumulated and the step is heard once, at the value the
// turn landed on, rather than machine-gunned along the way. A turn slow
// enough to leave gaps this wide still hears every detent, this long after
// each one, which is what keeps a single careful detent audible.
//
// It is a variable rather than a constant so a test can widen the window,
// which is what makes a held-back audition tellable from a fired one.
var velocityQuiescence = 20 * time.Millisecond

// velocityFastWindow is how close together two detents must arrive for the
// later one to count as part of the same fast turn. Detents further apart
// than this are a careful turn, and each one counts as one step however
// long the knob took between them.
const velocityFastWindow = 50 * time.Millisecond

// velocityStepMax caps how far one detent of a fast turn counts. Acceleration
// is what makes a long sweep cheap, and the cap is what keeps one flick of
// the knob from throwing a velocity across the whole range.
const velocityStepMax = 8

// knobNow is the clock the Volume knob's timing is measured against. It is a
// variable rather than a call to time.Now so a test can set it and land
// detents on either side of velocityFastWindow without sleeping.
var knobNow = time.Now

// AdjustVelocity moves the Volume encoder and writes the result to the step
// under the cursor on the selected track, so a step's dynamics can be set
// without playing the step again to get them.
//
// The two kinds of voice count from different places. A chromatic step counts from the
// encoder's own carried value rather than from the step, so the value survives moving
// between steps and a step picked after the encoder was set takes it on the next detent. A
// percussive step counts from the velocity it holds: that is the hit the step was played
// with, and the knob is there to trim a hit rather than to replace it with whatever value
// the encoder happened to be left at.
//
// The detent is applied at once, so the readout follows the knob, but the audition is
// held back until the knob rests; see armAudition. A preview that fails to write reaches
// the log rather than the caller, because nothing on the event loop is waiting for an
// audition that has not happened yet.
func (p *PatternBank) AdjustVelocity(aseq alsa.EventWriter, encoderValue int) error {
	if p.readoutIsOwned() {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil {
		return nil
	}
	direction, turning := encoderDirection(encoderValue)
	if !turning {
		return nil
	}
	from := p.velocityBase(voice)
	value := clampStepVelocity(voice, from+p.velocityIncrement(direction))
	// The chromatic encoder keeps its own value whether or not the step holds a note,
	// because a step chosen afterwards is meant to take it rather than its own.
	if voice.IsChromatic() {
		p.chromaticVelocity = value
	}
	event, updated := p.writeStepVelocity(voice, value)
	// The step is in the log so a velocity change can always be traced to one step rather
	// than guessed at from the sound. It is logged before the audition, so the change is
	// recorded even if writing the preview fails.
	logger.Debug("volume knob", "step", p.stepCursor, "from", from, "to", value, "noteUpdated", updated)
	if updated {
		p.armAudition(aseq, event)
	}
	// A percussive step carries its velocity as its brightness, so a trim
	// repaints the rows the voice is on; a chromatic step keeps its pitch
	// colour whatever its dynamics, so only its readout moves. The repaint
	// rides the same gate as the audition: over an empty step the knob does
	// nothing, and a repaint would send pad rows nothing changed.
	if updated && !voice.IsChromatic() {
		if err := p.redrawRowsWithVoice(voice); err != nil {
			return err
		}
	}
	// Only the palette is repainted here, and only while note editing owns the
	// grid, which is the one other place a velocity is shown.
	if p.noteEditActive() {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	}
	return p.printStepStatus()
}

// velocityIncrement is how far one detent counts at the speed the knob is
// turning at. A detent that arrives within velocityFastWindow of the one
// before it is part of a fast turn, and every detent of that turn counts one
// step more than the last, up to velocityStepMax, so the faster the knob
// turns the further a detent reaches. A detent after a longer gap starts the
// count again at one, so a careful turn always moves exactly one step.
//
// The state this reads and writes belongs to the event loop, which is the only
// caller, so it needs no lock of its own.
func (p *PatternBank) velocityIncrement(direction int) int {
	now := knobNow()
	if now.Sub(p.lastDetent) > velocityFastWindow {
		p.detentStep = velocityStep
	} else {
		p.detentStep = min(p.detentStep+velocityStep, velocityStepMax)
	}
	p.lastDetent = now
	return direction * p.detentStep
}

// armAudition holds a step's audition back until the knob has rested for
// velocityQuiescence. Each detent replaces the audition the one before it
// armed, so a fast turn hears the step once, at the value the sweep landed
// on, instead of once per detent.
//
// The event is copied into the callback rather than read back when the timer
// fires, because the fire happens on the timer's own goroutine while the
// banks belong to the event loop: the preview carries everything it needs and
// reads nothing back. The audition is counted so a shutdown can wait for one
// in flight, the way it waits for the playback worker, before it closes the
// sequencer.
func (p *PatternBank) armAudition(aseq alsa.EventWriter, event Event) {
	if p.audition != nil {
		p.audition.Stop()
	}
	step := p.stepCursor
	p.auditions.Add(1)
	p.audition = time.AfterFunc(velocityQuiescence, func() {
		defer p.auditions.Done()
		if err := p.auditionEvent(aseq, event); err != nil {
			logger.Error("volume audition failed",
				"error", err, "step", step, "voice", voiceLabel(event.Voice))
		}
	})
}

// stopAuditions cancels the audition a turn is holding back and waits for one
// already in flight to finish. Shutdown calls it before it closes the
// sequencer, for the same reason it joins the playback worker first: a
// preview written to a client that is gone would be a write to nothing, and
// waiting here means no audition can land after the close.
func (p *PatternBank) stopAuditions() {
	if p.audition != nil {
		p.audition.Stop()
	}
	p.auditions.Wait()
}

// velocityBase is the value a detent starts from: the encoder's carried value for a
// chromatic voice, and the selected step's own velocity for a percussive one.
func (p *PatternBank) velocityBase(voice *Voice) int {
	if voice.IsChromatic() {
		return p.chromaticVelocity
	}
	if pattern := p.CurrentPattern(); pattern != nil {
		if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			return event.Velocity
		}
	}
	// An empty step has no dynamics to trim, so there is nothing to write either way.
	return defaultStepVelocity
}

// writeStepVelocity stores the value on the step under the cursor, reporting whether that
// step held a note to write it to.
func (p *PatternBank) writeStepVelocity(voice *Voice, velocity int) (Event, bool) {
	pattern := p.CurrentPattern()
	if pattern == nil {
		return Event{}, false
	}
	return pattern.SetVelocity(p.stepCursor, voice, velocity)
}
