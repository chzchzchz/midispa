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
		"microKORG2",
		"Perform-VE",
		"MiniNova",
		"Pro 800",
		"Liven 8bit warps",
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

// TestMicrokorg2ImplementationChart pins the microKORG2's control change and
// non-registered parameter bindings, transcribed from its owner's manual
// (microKORG2_OM_En1.pdf): the Control Change row of the MIDI Implementation
// Chart on p. 133, the NRPN addresses printed beside the parameters in the
// body on p. 70-79, 90-94 and 116, and the parameter ranges and enumerations
// those pages give.
//
// Both sets of numbers are unique in the manual, and that uniqueness is what
// the rest of the package depends on: MidiControls maps an address to a single
// field name. The General MIDI plumbing the chart also lists is not a patch
// parameter, so the test checks that it stays unclaimed; otherwise an incoming
// CC#6 or CC#99 would be reported as an edit of the sound rather than as the
// protocol message it is.
func TestMicrokorg2ImplementationChart(t *testing.T) {
	params, err := NewModelParams("microKORG2")
	if err != nil {
		t.Fatalf("NewModelParams: %v", err)
	}
	ccFields, err := ControlFields(params)
	if err != nil {
		t.Fatalf("ControlFields: %v", err)
	}
	nrpnFields, err := NRPNFields(params)
	if err != nil {
		t.Fatalf("NRPNFields: %v", err)
	}

	wantCC := map[string]int{
		"ModulationWheel":      1,
		"TimbreLevel":          7,
		"TimbrePan":            10,
		"PortamentoTime":       5,
		"PortamentoMode":       65,
		"UnisonDetune":         33,
		"UnisonSpread":         34,
		"Transpose":            35,
		"FineTune":             36,
		"OSC1Wave":             8,
		"OSC1Shape":            9,
		"OSC1ModAmount":        15,
		"OSC1Semitones":        16,
		"OSC1FineTune":         17,
		"OSC2Wave":             18,
		"OSC2Shape":            19,
		"OSC2ModAmount":        20,
		"OSC2Semitones":        21,
		"OSC2FineTune":         22,
		"OSC3Wave":             48,
		"OSC3Shape":            49,
		"OSC3Semitones":        51,
		"OSC3FineTune":         52,
		"NoiseType":            29,
		"NoiseColor":           30,
		"OSC1Level":            23,
		"OSC2Level":            24,
		"OSC3Level":            25,
		"NoiseLevel":           26,
		"FilterType":           27,
		"FilterKeytrack":       28,
		"FilterDrive":          83,
		"Resonance":            71,
		"Cutoff":               74,
		"AmpEgAttack":          73,
		"AmpEgDecay":           75,
		"AmpEgSustain":         70,
		"AmpEgRelease":         72,
		"AmpEgVelocity":        79,
		"FilterEgAttack":       85,
		"FilterEgDecay":        86,
		"FilterEgSustain":      87,
		"FilterEgRelease":      88,
		"FilterEgIntensity":    84,
		"LFO1Wave":             89,
		"LFO1Frequency":        90,
		"LFO1Smooth":           91,
		"LFO2Wave":             102,
		"LFO2Frequency":        76,
		"LFO2Delay":            92,
		"ModEffectControl1":    12,
		"ModEffectControl2":    111,
		"ModEffectControl3":    112,
		"DelayEffectControl1":  115,
		"DelayEffectControl2":  13,
		"DelayEffectControl3":  113,
		"DelayEffectControl4":  114,
		"ReverbEffectControl1": 118,
		"ReverbEffectControl2": 14,
		"ReverbEffectControl3": 116,
		"ReverbEffectControl4": 117,
		"EQLowFrequency":       95,
		"EQLowGain":            110,
		"EQHighFrequency":      94,
		"EQHighGain":           109,
		"EQOutputFeedback":     93,
		"DamperPedal":          64,
		"Patch1Intensity":      103,
		"Patch2Intensity":      104,
		"Patch3Intensity":      105,
		"Patch4Intensity":      106,
		"Patch5Intensity":      107,
		"Patch6Intensity":      108,
	}
	if len(ccFields) != len(wantCC) {
		t.Fatalf("ControlFields returned %d fields, want %d", len(ccFields), len(wantCC))
	}
	seen := make(map[int]string, len(ccFields))
	for _, field := range ccFields {
		controller, ok := wantCC[field.Name]
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
	// Bank select, data entry and the NRPN selectors drive the protocol
	// rather than the sound. The NRPN path does not need fields for them:
	// it writes CC#99, CC#98 and CC#6 from its own controller constants.
	for _, plumbing := range []int{0, 32, 6, 38, 98, 99} {
		if name, ok := seen[plumbing]; ok {
			t.Errorf("CC#%d is claimed by %s, want it left to the MIDI protocol", plumbing, name)
		}
	}

	wantNRPN := map[string][2]int{
		// Arpeggiator, MSB 0.
		"ArpLatch":        {0, 4},
		"ArpSwing":        {0, 5},
		"ArpResolution":   {0, 6},
		"ArpType":         {0, 7},
		"ArpOctave":       {0, 8},
		"ArpLastStep":     {0, 9},
		"ArpGate":         {0, 10},
		"ArpTargetTimbre": {0, 11},
		"ArpKeySync":      {0, 12},
		// The six virtual patches, MSB 4, in three runs of six.
		"Patch1Source":  {4, 0},
		"Patch2Source":  {4, 1},
		"Patch3Source":  {4, 2},
		"Patch4Source":  {4, 3},
		"Patch5Source":  {4, 4},
		"Patch6Source":  {4, 5},
		"Patch1Source2": {4, 16},
		"Patch2Source2": {4, 17},
		"Patch3Source2": {4, 18},
		"Patch4Source2": {4, 19},
		"Patch5Source2": {4, 20},
		"Patch6Source2": {4, 21},
		"Patch1Dest":    {4, 32},
		"Patch2Dest":    {4, 33},
		"Patch3Dest":    {4, 34},
		"Patch4Dest":    {4, 35},
		"Patch5Dest":    {4, 36},
		"Patch6Dest":    {4, 37},
		// Vocoder, MSB 5.
		"VocoderMicDirect":       {5, 1},
		"VocoderSynthDryWet":     {5, 2},
		"VocoderFormant":         {5, 3},
		"VocoderResonance":       {5, 4},
		"VocoderEnvFollowerSens": {5, 5},
		"VocoderBandLevel1":      {5, 16},
		"VocoderBandLevel2":      {5, 17},
		"VocoderBandLevel3":      {5, 18},
		"VocoderBandLevel4":      {5, 19},
		"VocoderBandLevel5":      {5, 20},
		"VocoderBandLevel6":      {5, 21},
		"VocoderBandLevel7":      {5, 22},
		"VocoderBandLevel8":      {5, 23},
		"VocoderBandLevel9":      {5, 24},
		"VocoderBandLevel10":     {5, 25},
		"VocoderBandLevel11":     {5, 26},
		"VocoderBandLevel12":     {5, 27},
		"VocoderBandLevel13":     {5, 28},
		"VocoderBandLevel14":     {5, 29},
		"VocoderBandLevel15":     {5, 30},
		"VocoderBandLevel16":     {5, 31},
		"VocoderBandPan1":        {5, 32},
		"VocoderBandPan2":        {5, 33},
		"VocoderBandPan3":        {5, 34},
		"VocoderBandPan4":        {5, 35},
		"VocoderBandPan5":        {5, 36},
		"VocoderBandPan6":        {5, 37},
		"VocoderBandPan7":        {5, 38},
		"VocoderBandPan8":        {5, 39},
		"VocoderBandPan9":        {5, 40},
		"VocoderBandPan10":       {5, 41},
		"VocoderBandPan11":       {5, 42},
		"VocoderBandPan12":       {5, 43},
		"VocoderBandPan13":       {5, 44},
		"VocoderBandPan14":       {5, 45},
		"VocoderBandPan15":       {5, 46},
		"VocoderBandPan16":       {5, 47},
		// Hard tune, MSB 6.
		"HardTuneIntensity": {6, 1},
		"HardTuneSpeed":     {6, 2},
		"HardTuneFormant":   {6, 3},
		// Harmonizer, MSB 7.
		"HarmonizerLevel":        {7, 1},
		"HarmonizerStereoSpread": {7, 2},
		"HarmonizerFormant":      {7, 3},
		"HarmonizerDetune":       {7, 4},
		"HarmonizerDelay":        {7, 5},
		"HarmonizerNumber":       {7, 16},
		"HarmonizerPitch1":       {7, 32},
		"HarmonizerPitch2":       {7, 48},
		// Loop recorder, MSB 8.
		"LoopStutter":       {8, 16},
		"LoopStutterLength": {8, 17},
		"LoopStutterOffset": {8, 18},
		"LoopPlayLevel":     {8, 19},
	}
	if len(nrpnFields) != len(wantNRPN) {
		t.Fatalf("NRPNFields returned %d fields, want %d", len(nrpnFields), len(wantNRPN))
	}
	nrpnSeen := make(map[[2]int]string, len(nrpnFields))
	for _, field := range nrpnFields {
		address, ok := wantNRPN[field.Name]
		if !ok {
			t.Errorf("unexpected NRPN field %s", field.Name)
			continue
		}
		if field.MSB != address[0] || field.LSB != address[1] {
			t.Errorf("%s addresses %d.%d, want %d.%d", field.Name, field.MSB, field.LSB, address[0], address[1])
		}
		if other, dup := nrpnSeen[address]; dup {
			t.Errorf("NRPN %d.%d claimed by both %s and %s", address[0], address[1], other, field.Name)
		}
		nrpnSeen[address] = field.Name
	}

	// The instrument reaches each of these as three messages, so a patch has
	// to select the parameter before it can write it.
	controls := NewMidiControlsNRPN(&Microkorg2{ArpType: new(int)})
	if controls == nil {
		t.Fatal("NewMidiControlsNRPN returned nil")
	}
	if !controls.Set(controls.CC("ArpType"), 100) {
		t.Fatal("Set rejected a valid NRPN")
	}
	want := []ControlGroup{{
		Name: "ArpType",
		Messages: [][]byte{
			{0xb0, 99, 0},  // NRPN MSB selects the arpeggiator block
			{0xb0, 98, 7},  // NRPN LSB selects the arpeggio type
			{0xb0, 6, 100}, // data entry MSB carries the value
		},
	}}
	if got := controls.ControlCodes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ArpType sends %v, want %v", got, want)
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

// TestLiven8BitWarpsControlChangeChart pins the LIVEN 8bit warps' control
// change bindings as transcribed from the instrument's MIDI implementation
// chart (8bw_manual_MIDI_en_r1.pdf). The chart lists one number per parameter
// and no number twice, so the uniqueness check is what the rest of the package
// depends on: MidiControls maps a controller number to a single field name.
//
// The chart is also the whole of the instrument's control change surface, so
// the test checks that in both directions rather than only that the fields the
// model declares land where the chart says. Declaring a number the chart does
// not list would claim a control the instrument never promised to answer;
// leaving out a number the chart does list would drop a parameter the
// instrument does answer to.
func TestLiven8BitWarpsControlChangeChart(t *testing.T) {
	// CC#5 is the only number the chart lists below 20, and the panel's legato
	// and arpeggiator knob is the one control behind it, read two different ways.
	want := map[string]int{
		"GlideOrArpType":    5,
		"SynthEngine":       20,
		"Octave":            21,
		"Velocity":          22,
		"Detune":            23,
		"SynthParameter1":   24,
		"SynthParameter2":   25,
		"SynthParameter3":   26,
		"SynthParameter4":   27,
		"SynthParameter5":   28,
		"SynthParameter6":   29,
		"AliasNoise":        30,
		"FilterActive":      31,
		"FilterType":        32,
		"FilterCutoff":      33,
		"FilterResonance":   34,
		"LfoToFilterCutoff": 35,
		"LfoRate":           36,
		"LfoToPitch":        37,
		"EgAttack":          38,
		"EgDecay":           39,
		"EgSustain":         40,
		"EgRelease":         41,
		"Sweep":             42,
		"SweepSpeed":        43,
		"SweepShift":        44,
		"FxType":            45,
		"FxSpeed":           46,
		"FxAmount":          47,
		"ReverbType":        48,
		"ReverbAmount":      49,
		"GateTime":          50,
		"Swing":             51,
		"VoiceMode":         52,
		"SeqMode":           53,
		"MemoryLevel":       54,
		"ParameterLock":     55,
	}

	params, err := NewModelParams("Liven 8bit warps")
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

	// The chart's control change block is CC#5 and then CC#20 to CC#55 with
	// no gaps, so every number the model claims has to be one of those.
	for controller := 0; controller <= 127; controller++ {
		listed := controller == 5 || controller >= 20 && controller <= 55
		if _, claimed := seen[controller]; claimed != listed {
			t.Errorf("CC#%d claimed=%t, want claimed=%t (chart %s)",
				controller, claimed, listed, claimedName(seen, controller))
		}
	}

	// The chart marks the General MIDI controllers as neither transmitted nor
	// recognized, so none of them may be claimed: a field here would report an
	// incoming CC#1 or CC#64 as an edit of the instrument when the instrument
	// ignores it.
	//
	// CC#32 and CC#38 are deliberately absent from this list. The instrument
	// spends both of them on parameters of its own, the filter type and the
	// envelope attack, and the two claims are not up for checking against the
	// General MIDI meaning because the chart says the instrument never
	// implemented that meaning in the first place.
	for _, ignored := range []int{0, 1, 2, 4, 6, 7, 10, 11, 64, 96, 97, 98, 99, 120, 121, 123} {
		if name, ok := seen[ignored]; ok {
			t.Errorf("CC#%d is claimed by %s, want it left unclaimed", ignored, name)
		}
	}

	// The chart has no non-registered parameter rows at all, so a caller that
	// builds NRPN controls for this model must be told there are none rather
	// than handed an empty set it cannot distinguish from a broken one.
	if fields, err := NRPNFields(params); err != nil {
		t.Fatalf("NRPNFields: %v", err)
	} else if len(fields) != 0 {
		t.Errorf("NRPNFields returned %d fields, want 0", len(fields))
	}
	if controls := NewMidiControlsNRPN(params); controls != nil {
		t.Error("NewMidiControlsNRPN built controls from a model with no NRPN fields")
	}
}

// claimedName names the field holding a controller, or reports the number bare
// when nothing does, so a failure above says which claim is wrong.
func claimedName(seen map[int]string, controller int) string {
	if name, ok := seen[controller]; ok {
		return "claimed by " + name
	}
	return "not listed"
}
