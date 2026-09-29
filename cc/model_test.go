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
		"Perform-VE",
		"MiniNova",
		"Pro 800",
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

// TestPro800ControlChangeChart pins the Pro 800's control change bindings as
// transcribed from the instrument's chart. Every parameter there has its own
// number, so the uniqueness check is again what the rest of the package needs:
// MidiControls maps a controller number to a single field name.
//
// The chart also lists the instrument's General MIDI plumbing: data entry MSB
// and LSB, the non-registered parameter selectors, and the two channel voice
// messages. None of them sets a parameter, so the model must not claim them;
// declaring one would make an incoming CC#6 or CC#123 look like an edit of the
// instrument rather than the standard message it is.
func TestPro800ControlChangeChart(t *testing.T) {
	want := map[string]int{
		"BankSelect":             0,
		"ModWheel":               1,
		"Breath":                 2,
		"MasterTune":             3,
		"MainVolume":             7,
		"OscAFrequencyMSB":       8,
		"OscAVolumeMSB":          9,
		"OscAPulseWidthMSB":      10,
		"OscBFrequencyMSB":       11,
		"OscBVolumeMSB":          12,
		"OscBPulseWidthMSB":      13,
		"OscBFineMSB":            14,
		"FilterCutoffMSB":        15,
		"FilterResonanceMSB":     16,
		"FilterEnvAmountMSB":     17,
		"FilterEnvReleaseMSB":    18,
		"FilterEnvSustainMSB":    19,
		"FilterEnvDecayMSB":      20,
		"FilterEnvAttackMSB":     21,
		"AmpEnvReleaseMSB":       22,
		"AmpEnvSustainMSB":       23,
		"AmpEnvDecayMSB":         24,
		"AmpEnvAttackMSB":        25,
		"PolyModFilterEnvMSB":    26,
		"PolyModOscBAmountMSB":   27,
		"LfoFrequencyMSB":        28,
		"LfoAmountMSB":           29,
		"GlideMSB":               30,
		"AmpVelocityMSB":         31,
		"FilterVelocityMSB":      32,
		"LfoModDelayMSB":         33,
		"VibratoFrequencyMSB":    34,
		"VibratoAmountMSB":       35,
		"UnisonDetuneMSB":        36,
		"NoiseLevelMSB":          37,
		"AmpAftertouchMSB":       39,
		"FilterAftertouchMSB":    40,
		"LfoAftertouchMSB":       41,
		"PitchBendAmountMSB":     42,
		"OscASaw":                48,
		"OscATriangle":           49,
		"OscASquare":             50,
		"OscBSaw":                51,
		"OscBTriangle":           52,
		"OscBSquare":             53,
		"OscBSync":               54,
		"PolyModFreqA":           55,
		"PolyModFilter":          56,
		"LfoShape":               57,
		"LfoSpeed":               58,
		"LfoTargets":             59,
		"FilterKeyboardTracking": 60,
		"FilterEnvCurve":         61,
		"FilterEnvSpeed":         62,
		"AmpEnvCurve":            63,
		"SustainPedal":           64,
		"Unison":                 65,
		"PitchBendTarget":        66,
		"ModWheelRange":          67,
		"OscAPitchMode":          68,
		"OscBPitchMode":          69,
		"ModWheelTarget":         70,
		"VibratoTarget":          71,
		"AmpEnvSpeed":            72,
		"ArpMode":                73,
		"LfoDestFrequency":       74,
		"LfoDestFilter":          75,
		"LfoDestPWM":             76,
		"VoiceSpread":            77,
		"TrackingReference":      78,
		"GlideMode":              79,
		"OscAFrequencyLSB":       80,
		"OscAVolumeLSB":          81,
		"OscAPulseWidthLSB":      82,
		"OscBFrequencyLSB":       83,
		"OscBVolumeLSB":          84,
		"OscBPulseWidthLSB":      85,
		"OscBFineLSB":            86,
		"FilterCutoffLSB":        87,
		"FilterResonanceLSB":     88,
		"FilterEnvAmountLSB":     89,
		"FilterEnvReleaseLSB":    90,
		"FilterEnvSustainLSB":    91,
		"FilterEnvDecayLSB":      92,
		"FilterEnvAttackLSB":     93,
		"AmpEnvReleaseLSB":       94,
		"AmpEnvSustainLSB":       95,
		"AmpEnvDecayLSB":         100,
		"AmpEnvAttackLSB":        101,
		"PolyModFilterEnvLSB":    102,
		"PolyModOscBAmountLSB":   103,
		"LfoFrequencyLSB":        104,
		"LfoAmountLSB":           105,
		"GlideLSB":               106,
		"AmpVelocityLSB":         107,
		"FilterVelocityLSB":      108,
		"LfoModDelayLSB":         109,
		"VibratoFrequencyLSB":    110,
		"VibratoAmountLSB":       111,
		"UnisonDetuneLSB":        112,
		"NoiseLevelLSB":          113,
		"AmpAftertouchLSB":       114,
		"FilterAftertouchLSB":    115,
		"LfoAftertouchLSB":       116,
		"PitchBendAmountLSB":     117,
		"AbandonedParameter":     118,
	}

	params, err := NewModelParams("Pro 800")
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
	for _, plumbing := range []int{6, 38, 96, 97, 98, 99, 120, 123} {
		if name, ok := seen[plumbing]; ok {
			t.Errorf("CC#%d is claimed by %s, want it left to the MIDI protocol", plumbing, name)
		}
	}
}
