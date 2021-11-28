package ladspa_test

import (
	"testing"

	"github.com/chzchzchz/midispa/ladspa"
)

func TestLowPassFilter(t *testing.T) {
	p, err := ladspa.LowPassFilter(44100)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if p.Label != "lpf" {
		t.Fatalf("expected lpf label, got %s", p.Label)
	}

	*p.Control("Cutoff Frequency (Hz)") = 11025.0

	input, output := make([]float32, 1024), make([]float32, 1024)
	for i := range input {
		input[i] = float32((2.0 * (i % 2)) - 1.0)
	}
	p.Connect("Input", &input[0])
	p.Connect("Output", &output[0])
	p.Run(len(input))
}

func TestReverb(t *testing.T) {
	p, err := ladspa.Reverb(44100)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Label != "revdelay" {
		t.Fatalf("expected revdelay label, got %s", p.Label)
	}
}

func TestChorus(t *testing.T) {
	p, err := ladspa.Chorus(44100)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Label != "multivoiceChorus" {
		t.Fatalf("expected revdelay label, got %s", p.Label)
	}
}
