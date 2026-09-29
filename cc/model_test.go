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
		"microKORG XL",
		"MicroKorg",
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

// TestMicroKorgFactoryCCAssignment pins the microKORG's factory control change
// bindings from the manual's "Messages transmitted and received by the
// microKORG" table. They are a user setting rather than a constant, so this
// only guards the transcription; the uniqueness check is what the rest of the
// package depends on, since MidiControls maps a controller number to a single
// field name.
func TestMicroKorgFactoryCCAssignment(t *testing.T) {
	want := map[string]int{
		"Portamento":      5,
		"AmpLevel":        7,
		"Panpot":          10,
		"ModFxLfoSpeed":   12,
		"DelayTime":       13,
		"Osc1Control1":    14,
		"Osc1Control2":    15,
		"Osc2Semitone":    18,
		"AudioIn1Tune":    19,
		"Osc1Level":       20,
		"Osc2Level":       21,
		"NoiseLevel":      22,
		"FilterEgAttack":  23,
		"FilterEgDecay":   24,
		"FilterEgSustain": 25,
		"FilterEgRelease": 26,
		"Lfo1Frequency":   27,
		"Patch1Intensity": 28,
		"Patch2Intensity": 29,
		"Patch3Intensity": 30,
		"Patch4Intensity": 31,
		"AmpEgSustain":    70,
		"Resonance":       71,
		"AmpEgRelease":    72,
		"AmpEgAttack":     73,
		"Cutoff":          74,
		"AmpEgDecay":      75,
		"Lfo2Frequency":   76,
		"Osc1Wave":        77,
		"Osc2Wave":        78,
		"FilterEgInt":     79,
		"Osc2OscMod":      82,
		"FilterType":      83,
		"KbdTrack":        85,
		"Lfo1Wave":        87,
		"Lfo2Wave":        88,
		"SyncCtrl":        90,
		"Distortion":      92,
		"ModFxDepth":      93,
		"DelayDepth":      94,
		"TimbreSelect":    95,
	}

	params, err := NewModelParams("MicroKorg")
	if err != nil {
		t.Fatalf("NewModelParams: %v", err)
	}
	fields, err := ControlFields(params)
	if err != nil {
		t.Fatalf("ControlFields: %v", err)
	}
	if len(fields) != len(want) {
		t.Fatalf("ControlFields returned %d fields, want %d", len(fields), len(want))
	}

	seen := make(map[int]string, len(fields))
	for _, field := range fields {
		controller, ok := want[field.Name]
		if !ok {
			t.Errorf("unexpected field %s", field.Name)
			continue
		}
		if field.Controller != controller {
			t.Errorf("%s is CC#%d, want CC#%d", field.Name, field.Controller, controller)
		}
		if other, dup := seen[field.Controller]; dup {
			t.Errorf("CC#%d claimed by both %s and %s", field.Controller, other, field.Name)
		}
		seen[field.Controller] = field.Name
	}
}
