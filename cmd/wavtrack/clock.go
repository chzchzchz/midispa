package main

import (
	"time"
)

type Clock struct {
	position SampleTick
	start    time.Time
	rate     float64
}

func (c *Clock) Start() {
	c.start = time.Now()
}

func (c *Clock) Stop() {}

func (c *Clock) Update() {
	now := time.Now()
	dur := now.Sub(c.start)
	c.start = now
	c.position += c.Ticks(dur)
}

func (c *Clock) Seek(samples SampleTick) {
	c.position += samples
	if c.position < 0 {
		c.position = 0
	}
}

func (c *Clock) SetPosition(d time.Duration) {
	c.position = SampleTick(d.Seconds() * c.rate)
	if c.position < 0 {
		c.position = 0
	}
}

func (c *Clock) Reset() { c.position = 0 }

func (c *Clock) Position() time.Duration {
	seconds := float64(c.position) / c.rate
	return time.Duration(float64(time.Second) * seconds)
}

func (c *Clock) Sample() SampleTick { return c.position }

func (c *Clock) Ticks(d time.Duration) SampleTick {
	return SampleTick(c.rate * d.Seconds())
}
