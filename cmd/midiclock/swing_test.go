package main

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T1: straight-time equivalence with swingPct=50.
// Every interval must equal the base interval.
func TestStraightTimeEquivalence(t *testing.T) {
	const baseInterval = 100.0

	for pulseIdx := 0; pulseIdx < 24; pulseIdx++ {
		interval := computeInterval(baseInterval, pulseIdx, 50.0)
		assert.Equal(t, baseInterval, interval,
			"pulse %d should produce a straight-time interval", pulseIdx)
	}
}

// T2: swing ratio with swingPct=66.
// Pulse 12 (the off-beat) must land at 66.0% of the beat.
func TestSwingRatioPulse12At66(t *testing.T) {
	beatDur := float64(500 * time.Millisecond)
	baseInterval := beatDur / 24.0

	heartbeat := 0.0
	for pulseIdx := 0; pulseIdx < 12; pulseIdx++ {
		heartbeat += computeInterval(baseInterval, pulseIdx, 66.0)
	}
	pulsePosition := heartbeat / beatDur * 100.0

	assert.InDelta(t, 66.0, pulsePosition, 1.0,
		"pulse 12 should land near 66%% of the beat; got %.2f%%", pulsePosition)
}

// T3: reverse swing with swingPct=40.
// Same as T2 but inverted: pulse 12 must land at 40.0% of the beat.
func TestReverseSwingPulse12At40(t *testing.T) {
	beatDur := float64(500 * time.Millisecond)
	baseInterval := beatDur / 24.0

	heartbeat := 0.0
	for pulseIdx := 0; pulseIdx < 12; pulseIdx++ {
		heartbeat += computeInterval(baseInterval, pulseIdx, 40.0)
	}
	pulsePosition := heartbeat / beatDur * 100.0

	assert.InDelta(t, 40.0, pulsePosition, 1.0,
		"pulse 12 should land near 40%% of the beat; got %.2f%%", pulsePosition)
}

// Total of all 24 intervals must equal one beat regardless of swing
// percentage; otherwise the tempo would drift over time.
func TestTotalBeatDurationInvariant(t *testing.T) {
	beatDur := float64(500 * time.Millisecond)
	baseInterval := beatDur / 24.0

	for _, swingPct := range []float64{50.0, 66.0, 40.0} {
		swingPct := swingPct
		t.Run("", func(t *testing.T) {
			heartbeat := 0.0
			for pulseIdx := 0; pulseIdx < 24; pulseIdx++ {
				heartbeat += computeInterval(baseInterval, pulseIdx, swingPct)
			}
			assert.InDelta(t, beatDur, heartbeat, float64(time.Millisecond),
				"swingPct=%v: total of 24 intervals should equal beatDur", swingPct)
		})
	}
}

// T4: randpct layered on swing.
// With randpct=10%, each interval must stay within ±10% of base, never
// negative, and the mean over many samples must approach base. Uses a
// deterministic local RNG so the test is reproducible.
func TestRandPctLayeredOnSwing(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const (
		swingPct     = 66.0
		randPct      = 10.0
		baseInterval = 100.0
		samples      = 200
	)
	base := computeInterval(baseInterval, 0, swingPct)

	var sum float64
	for i := 0; i < samples; i++ {
		interval := applyJitterWith(base, randPct, rng)
		require.Greater(t, interval, 0.0, "interval must stay positive (i=%d)", i)
		assert.InDelta(t, base, interval, base*randPct/100.0,
			"interval %v outside ±%v%% envelope from base %v", interval, randPct, base)
		sum += interval
	}
	mean := sum / float64(samples)
	assert.InDelta(t, base, mean, base*0.05,
		"mean of %d samples = %.2f should approach base %.2f", samples, mean, base)
}

// T5: start/continue resets the pulse counter.
// After reset, the next call to computeInterval uses the on-beat formula,
// identical to pulse 0. We pre-set the counter to 23 (off-beat), reset,
// then verify both slots produce the on-beat (group 1) interval.
func TestPulseCounterReset(t *testing.T) {
	const (
		baseInterval = 100.0
		swingPct     = 66.0
	)
	s := &Sequencer{swingPct: swingPct, pulseInBeat: 23}
	require.Equal(t, 23, s.pulseInBeat, "setup: counter should be at 23")

	s.pulseInBeat = 0
	first := computeInterval(baseInterval, s.pulseInBeat, s.swingPct)
	s.pulseInBeat = (s.pulseInBeat + 1) % ppqn
	second := computeInterval(baseInterval, s.pulseInBeat, s.swingPct)

	assert.Equal(t, first, second,
		"after reset, interval[0] should equal interval[1] (both on-beat)")
}

// T6: validation rejects out-of-range swingPct via validSwingPct.
// Values in [minSwingPct, maxSwingPct] are accepted; everything else is rejected.
func TestValidationRejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name   string
		value  float64
		expect bool
	}{
		{"negative", -1, false},
		{"zero", 0, false},
		{"below-floor", 0.001, false},
		{"min-edge", minSwingPct, true},
		{"mid", 50, true},
		{"max-edge", maxSwingPct, true},
		{"above-ceiling", 100, false},
		{"way-over", maxSwingPct + 0.1, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, validSwingPct(tc.value),
				"validSwingPct(%v) should be %v", tc.value, tc.expect)
		})
	}
}

// CC 0..127 should map to swingPct roughly in [minSwingPct, maxSwingPct],
// always clamped into the same range regardless of how the underlying
// formula extrapolates.
func TestCCToSwingPctMapping(t *testing.T) {
	cases := []struct {
		cc   byte
		want float64
	}{
		{0, minSwingPct},
		{64, 50.0},
		{127, 99.2},
	}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			got := ccSwingToSwingPct(c.cc)
			assert.InDelta(t, c.want, got, 0.1,
				"CC %d -> got %.2f, want %.2f", c.cc, got, c.want)
			assert.GreaterOrEqual(t, got, minSwingPct,
				"CC %d -> %.2f escaped lower bound", c.cc, got)
			assert.LessOrEqual(t, got, maxSwingPct,
				"CC %d -> %.2f escaped upper bound", c.cc, got)
		})
	}
}
