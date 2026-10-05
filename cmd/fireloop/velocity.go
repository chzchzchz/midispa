package main

import (
	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// The Volume knob, which is the one control that sets a step's dynamics for either kind of
// voice. It lives apart from the palette because the two do different jobs on a step: the
// palette decides which pitch a note has, and this decides how hard it is struck.

// defaultStepVelocity is where the Volume knob starts, and what a chromatic step takes
// when the knob has not been turned yet.
const defaultStepVelocity = midi.DataMax

// velocityStep is how far one detent of the Volume knob moves a step velocity.
const velocityStep = 1

// AdjustVelocity moves the Volume encoder by one detent and writes the result to the step
// under the cursor on the selected track, so a step's dynamics can be set without playing
// the step again to get them.
//
// The two kinds of voice count from different places. A chromatic step counts from the
// encoder's own carried value rather than from the step, so the value survives moving
// between steps and a step picked after the encoder was set takes it on the next detent. A
// percussive step counts from the velocity it holds: that is the hit the step was played
// with, and the knob is there to trim a hit rather than to replace it with whatever value
// the encoder happened to be left at.
func (p *PatternBank) AdjustVelocity(aseq alsa.EventWriter, encoderValue int) error {
	if p.lengthEditActive() {
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
	value := clampStepVelocity(voice, from+direction*velocityStep)
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
		if err := p.auditionEvent(aseq, event); err != nil {
			return err
		}
	}
	// Only the palette is repainted here, and only while note editing owns the grid: a
	// velocity is not part of what a step row shows, so repainting the rows would send
	// four pad messages for a change none of them would show.
	if p.noteEditActive() {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	}
	return p.printStepStatus()
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
