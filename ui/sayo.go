package ui

import (
	"context"
	"time"

	sayo "github.com/chzchzchz/sayo-rgb"
)

type RgbCmd struct {
	color [3]byte
	idx   int
}

type RgbQueue struct {
	c     chan RgbCmd
	donec chan struct{}
	dev   *sayo.Device
}

func newRgbQueue(dev *sayo.Device) *RgbQueue {
	return &RgbQueue{c: make(chan RgbCmd, 16), donec: make(chan struct{}), dev: dev}
}

func (q *RgbQueue) Write(idx int, color [3]byte) {
	q.c <- RgbCmd{idx: idx, color: color}
}

func (q *RgbQueue) loop(ctx context.Context) {
	defer close(q.donec)
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-q.c:
			r, g, b := cmd.color[0], cmd.color[1], cmd.color[2]
			q.dev.Write(sayo.ModeSwitchOnce, cmd.idx, r, g, b)
		}
		// Macropad does not like rapid changes from bank toggles.
		time.Sleep(5 * time.Millisecond)
	}
}

type Sayo struct {
	d *sayo.Device
	*RgbQueue
}

func NewSayo(ctx context.Context, hidPath string) (*Sayo, error) {
	d, err := sayo.NewDevice(hidPath)
	if err != nil {
		return nil, err
	}
	rgbq := newRgbQueue(d)
	go rgbq.loop(context.Background())
	return &Sayo{d, rgbq}, nil
}

func (s *Sayo) Off() {
	for i := 0; i < 24; i++ {
		s.Write(i, [3]byte{0, 0, 0})
	}
}
