package ladspa_test

import (
	"testing"

	"github.com/chzchzchz/midispa/ladspa"
)

func TestFilter(t *testing.T) {
	p, err := ladspa.LowPassFilter(44100)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	hz := float32(11025)
	p.Connect("Cutoff Frequency (Hz)", &hz)

	input, output := make([]float32, 1024), make([]float32, 1024)
	for i := range input {
		input[i] = float32((2.0 * (i % 2)) - 1.0)
	}
	p.Connect("Input", &input[0])
	p.Connect("Output", &output[0])
	p.Run(len(input))
}
