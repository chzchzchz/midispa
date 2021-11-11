package main

import (
	"time"
)

type ADSR struct {
	Attack  time.Duration
	Decay   time.Duration
	Sustain float32
	Release time.Duration
}

type adsrCycles struct {
	Attack  int
	Decay   int
	Sustain float32 // [0, 1.0)
	Release int

	// add to scaling each attack cycle
	attackStep float32
	// add to scaling each decay cycle
	decayStep float32
}

type adsrCycleState struct {
	*adsrCycles

	attackCycles  int
	decayCycles   int
	releaseCycles int
	on            bool

	releaseStep   float32 // computed based on current scale
	currentScale  float32
	velocityScale float32
}

func (a *ADSR) Cycles(hz float64) adsrCycles {
	return adsrCycles{
		Attack:  int(a.Attack.Seconds() * hz),
		Decay:   int(a.Decay.Seconds() * hz),
		Sustain: a.Sustain,
		Release: int(a.Release.Seconds() * hz),

		attackStep: float32(a.Attack.Seconds() / hz),
		decayStep:  -float32((1.0 - float64(a.Sustain)) * a.Decay.Seconds() / hz),
	}
}

func (a *adsrCycleState) Off() {
	if a.on && a.releaseCycles == 0 {
		a.attackCycles, a.decayCycles, a.releaseCycles = 0, 0, a.Release
		a.releaseStep = -a.currentScale / float32(a.Release)
	}
}

func (a *adsrCycleState) Apply(samp float32) float32 {
	if !a.on {
		return 0
	} else if a.attackCycles > 0 {
		a.attackCycles--
		a.currentScale += a.attackStep
	} else if a.decayCycles > 0 {
		a.decayCycles--
		a.currentScale += a.decayStep
	} else if a.releaseCycles > 0 {
		a.releaseCycles--
		a.currentScale += a.releaseStep
		if a.releaseCycles == 0 {
			a.on = false
		}
	} else {
		a.currentScale = a.Sustain
	}
	return samp * a.velocityScale * a.currentScale
}

func (a *adsrCycles) Press(vel float32) adsrCycleState {
	if vel > 1.0 || vel < 0.0 {
		panic("bad velocity")
	}
	aa := adsrCycleState{
		adsrCycles:    a,
		attackCycles:  a.Attack,
		decayCycles:   a.Decay,
		releaseCycles: 0,
		on:            true,
		currentScale:  0,
		velocityScale: vel,
	}
	if a.Attack == 0 {
		aa.currentScale = 1.0
		if a.Decay == 0 {
			aa.currentScale = a.Sustain
			if a.Release == 0 {
				aa.on = false
			}
		}
	}
	return aa
}
