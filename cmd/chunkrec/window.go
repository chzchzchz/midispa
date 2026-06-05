package main

type Window []float32

var SilenceCutoff = float32(0.001)

func NewWindow(samples int) Window {
	return make([]float32, 0, samples)
}

func (w Window) full() bool { return len(w) == cap(w) }

func (w Window) silent() bool {
	sum := float32(0.0)
	for _, v := range w {
		if v < 0 {
			v = -v
		}
		sum = sum + v
	}
	avg := sum / float32(len(w))
	return avg < SilenceCutoff
}

func (w Window) reset() {}
