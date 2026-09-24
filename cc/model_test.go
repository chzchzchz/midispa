package cc

import (
	"reflect"
	"testing"
)

func TestModelNames(t *testing.T) {
	want := []string{
		"Craft Synth 2",
		"Meeblip SE",
		"Meeblip Triode",
		"MidiMix",
		"Skulpt",
		"Sound Controller",
		"Volca Bass",
		"Volca Beats",
		"Volca Keys",
		"Volca Kick",
		"Volca Drum",
		"GM Controller",
		"Uno Synth",
		"WorldeEasyControl9",
		"Pro VS Mini",
	}
	got := ModelNames()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ModelNames() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if ModelNames()[0] != want[0] {
		t.Fatal("ModelNames exposed its backing storage")
	}
}

func TestNewModelParamsInitializesEveryModel(t *testing.T) {
	for _, name := range ModelNames() {
		t.Run(name, func(t *testing.T) {
			params, err := NewModelParams(name)
			if err != nil {
				t.Fatalf("NewModelParams(%q): %v", name, err)
			}
			if params == nil {
				t.Fatalf("NewModelParams(%q) returned nil", name)
			}
		})
	}
}

func TestMidiParamsInitializesSelectedModel(t *testing.T) {
	model := &Model{Model: "Craft Synth 2"}
	first := model.MidiParams()
	second := model.MidiParams()
	if first == nil || first != second {
		t.Fatalf("MidiParams returned %#v then %#v", first, second)
	}
}

func TestNewModelParamsRejectsUnknownModel(t *testing.T) {
	if _, err := NewModelParams("unknown model"); err == nil {
		t.Fatal("accepted an unknown model")
	}
}
