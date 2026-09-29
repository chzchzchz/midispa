package microkorg

import (
	"bytes"
	"testing"

	"github.com/chzchzchz/midispa/sysex"
)

// The four payload sizes the implementation sheet quotes, as MIDI bytes after
// packing. They are the strongest check available on the packing, because a
// seven byte group costing anything other than eight MIDI bytes, or a trailing
// partial group handled any other way, moves all four numbers.
func TestPackedSizeMatchesSheet(t *testing.T) {
	tests := []struct {
		name string
		size int
		want int
	}{
		{"program", programSize, 291},
		{"program bank", programBankSize, 37157},
		{"global", globalSize, 229},
		{"all data", allDataSize, 37386},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := packedSize(test.size); got != test.want {
				t.Fatalf("packedSize(%d) = %d, want %d", test.size, got, test.want)
			}
		})
	}
}

func TestDumpRequestRoundTrip(t *testing.T) {
	tests := []struct {
		function int
		want     []byte
	}{
		{FuncGlobalDumpRequest, []byte{0xf0, 0x42, 0x33, 0x58, 0x0e, 0xf7}},
		{FuncAllDumpRequest, []byte{0xf0, 0x42, 0x33, 0x58, 0x0f, 0xf7}},
		{FuncCurrentProgramDumpRequest, []byte{0xf0, 0x42, 0x33, 0x58, 0x10, 0xf7}},
		{FuncProgramDumpRequest, []byte{0xf0, 0x42, 0x33, 0x58, 0x1c, 0xf7}},
	}
	for _, test := range tests {
		request := &DumpRequest{Channel: 3, Function: test.function}
		got, err := request.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(0x%02x): %v", test.function, err)
		}
		if !bytes.Equal(got, test.want) {
			t.Errorf("MarshalBinary(0x%02x) = % x, want % x", test.function, got, test.want)
		}
		var decoded DumpRequest
		if err := decoded.UnmarshalBinary(got); err != nil {
			t.Fatalf("UnmarshalBinary(0x%02x): %v", test.function, err)
		}
		if decoded != *request {
			t.Errorf("UnmarshalBinary(0x%02x) = %+v, want %+v", test.function, decoded, *request)
		}
	}
}

func TestProgramWriteRequestRoundTrip(t *testing.T) {
	request := &ProgramWriteRequest{Channel: 0, Program: 127}
	want := []byte{0xf0, 0x42, 0x30, 0x58, 0x11, 0x00, 0x7f, 0xf7}
	got, err := request.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("MarshalBinary = % x, want % x", got, want)
	}
	var decoded ProgramWriteRequest
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded != *request {
		t.Errorf("UnmarshalBinary = %+v, want %+v", decoded, *request)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	status := &Status{Channel: 15, Function: FuncDataLoadCompleted}
	want := []byte{0xf0, 0x42, 0x3f, 0x58, 0x23, 0xf7}
	got, err := status.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("MarshalBinary = % x, want % x", got, want)
	}
	var decoded Status
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded != *status {
		t.Errorf("UnmarshalBinary = %+v, want %+v", decoded, *status)
	}
}

// sampleTimbre returns a synth voice with every field set to a distinct value,
// so that a field landing in the wrong byte shows up as a mismatch rather than
// as two copies of the same number.
func sampleTimbre(seed int) Timbre {
	return Timbre{
		Channel:              0,
		AssignMode:           1,
		Eg2Reset:             1,
		Eg1Reset:             1,
		TriggerMode:          1,
		KeyPriority:          0,
		UnisonDetune:         50 + seed,
		Tune:                 40 + seed,
		BendRange:            64,
		Transpose:            70 + seed,
		VibratoInt:           60 + seed,
		Osc1Wave:             5,
		Osc1Ctrl1:            90 + seed,
		Osc1Ctrl2:            30 + seed,
		DwgsWave:             17,
		Osc2ModSelect:        3,
		Osc2Wave:             1,
		Osc2Semitone:         80 + seed,
		Osc2Tune:             60 + seed,
		Portamento:           20 + seed,
		Osc1Level:            100 + seed,
		Osc2Level:            90 + seed,
		NoiseLevel:           10 + seed,
		FilterType:           2,
		Cutoff:               110 + seed,
		Resonance:            45 + seed,
		FilterEgIntensity:    100 + seed,
		FilterVelocitySense:  64,
		FilterKeyboardTrack:  70 + seed,
		AmpLevel:             95 + seed,
		Panpot:               60 + seed,
		AmpSwitch:            1,
		Distortion:           1,
		AmpVelocitySense:     64,
		AmpKeyboardTrack:     80 + seed,
		FilterEgAttack:       1 + seed,
		FilterEgDecay:        2 + seed,
		FilterEgSustain:      3 + seed,
		FilterEgRelease:      4 + seed,
		AmpEgAttack:          5 + seed,
		AmpEgDecay:           6 + seed,
		AmpEgSustain:         7 + seed,
		AmpEgRelease:         8 + seed,
		Lfo1KeySync:          2,
		Lfo1Wave:             3,
		Lfo1Frequency:        120 + seed,
		Lfo1TempoSync:        1,
		Lfo1SyncNote:         7,
		Lfo2KeySync:          1,
		Lfo2Wave:             2,
		Lfo2Frequency:        100 + seed,
		Lfo2TempoSync:        1,
		Lfo2SyncNote:         9,
		Patches: [4]Patch{
			{Source: 4, Destination: 6, Intensity: 20 + seed},
			{Source: 5, Destination: 3, Intensity: 21 + seed},
			{Source: 6, Destination: 7, Intensity: 22 + seed},
			{Source: 7, Destination: 0, Intensity: 23 + seed},
		},
	}
}

func sampleProgram(name string) Program {
	program := Program{
		Name:           name,
		VoiceMode:      VoiceModeLayer,
		ScaleKey:       7,
		ScaleType:      0,
		TriggerLength:  3,
		TriggerPattern: 0x55,
		Arpeggio: Arpeggio{
			Tempo:      240,
			On:         1,
			Latch:      1,
			Target:     2,
			KeySync:    1,
			Type:       4,
			Range:      2,
			GateTime:   90,
			Resolution: 3,
			Swing:      70,
		},
		DelayFx: DelayFx{
			Sync:     1,
			TimeBase: 6,
			Time:     100,
			Depth:    80,
			Type:     1,
		},
		ModFx: ModFx{
			LfoSpeed: 30,
			Depth:    40,
			Type:     2,
		},
		Eq: Eq{
			HighFrequency: 12,
			HighGain:      70,
			LowFrequency:  5,
			LowGain:       58,
		},
		KeyboardOctave: -2,
		Timbre1:        sampleTimbre(0),
		Timbre2:        sampleTimbre(1),
	}
	return program
}

func sampleVocoder() Vocoder {
	vocoder := Vocoder{
		Channel:          1,
		AssignMode:       2,
		Eg2Reset:         1,
		Eg1Reset:         0,
		TriggerMode:      0,
		KeyPriority:      1,
		UnisonDetune:     30,
		Tune:             65,
		BendRange:        66,
		Transpose:        64,
		VibratoInt:       50,
		Wave:             0,
		Ctrl1:            45,
		Ctrl2:            55,
		DwgsWave:         3,
		HpfGate:          1,
		Portamento:       12,
		Osc1Level:        90,
		Ext1Level:        20,
		NoiseLevel:       10,
		HpfLevel:         33,
		GateSense:        44,
		Threshold:        55,
		Shift:            4,
		Cutoff:           68,
		Resonance:        22,
		ModSource:        6,
		Intensity:        90,
		EFSense:          127,
		Level:            100,
		DirectLevel:      30,
		Distortion:       1,
		VelSense:         64,
		KeyTrack:         60,
		FilterEgAttack:   0,
		FilterEgDecay:    0,
		FilterEgSustain:  127,
		FilterEgRelease:  0,
		AmpEgAttack:      3,
		AmpEgDecay:       4,
		AmpEgSustain:     5,
		AmpEgRelease:     6,
		Lfo1KeySync:      1,
		Lfo1Wave:         2,
		Lfo1Frequency:    77,
		Lfo1TempoSync:    1,
		Lfo1SyncNote:     11,
		Lfo2KeySync:      2,
		Lfo2Wave:         0,
		Lfo2Frequency:    88,
		Lfo2TempoSync:    0,
		Lfo2SyncNote:     3,
	}
	for i := range vocoder.BandLevel {
		vocoder.BandLevel[i] = i
		vocoder.BandPan[i] = 64 + i
		for j := range vocoder.EfHoldLevel[i] {
			vocoder.EfHoldLevel[i][j] = i*4 + j
		}
	}
	return vocoder
}

func sampleGlobal() Global {
	global := Global{
		MasterTune:            65,
		Transpose:             60,
		Position:              1,
		VelocityValue:         100,
		VelocityCurve:         4,
		LocalControl:          1,
		MemoryProtect:         1,
		Clock:                 2,
		MidiChannel:           9,
		SyncControlNumber:     90,
		TimbreSelectNumber:    95,
		Midi1ControlNumber:    0,
		Midi2ControlNumber:    3,
		SystemExclusiveFilter: 1,
		NoteReceive:           0,
		PitchBendFilter:       1,
		ControlChangeFilter:   1,
		ProgramChangeFilter:   0,
	}
	// The factory knob bindings, which is what the cc package's MicroKorg
	// model records. Populating them here checks the array survives the
	// round trip with its instrument specific ordering intact.
	copy(global.KnobControlChange[:], []int{
		5, 77, 14, 15, 78, 82, 18, 19, 20, 21, 22, 83, 74, 71, 79, 85, 7, 10,
		73, 23, 24, 25, 26, 75, 70, 72, 87, 27, 88, 76, 28, 29, 30, 31, 12,
		93, 13, 94, 92, ControlChangeUnassigned,
	})
	for i := range global.UserScale {
		global.UserScale[i] = 64
	}
	for i := range global.ProgramChangeMap {
		global.ProgramChangeMap[i] = i
	}
	return global
}

func TestProgramDataRoundTrip(t *testing.T) {
	want := ProgramData{Channel: 5, Program: sampleProgram("INIT")}

	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(encoded) != headerSize+1+packedSize(programSize)+1 {
		t.Fatalf("encoded length = %d, want %d", len(encoded), headerSize+1+packedSize(programSize)+1)
	}

	var got ProgramData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Channel != want.Channel {
		t.Errorf("Channel = %d, want %d", got.Channel, want.Channel)
	}
	if got.Program.Name != want.Program.Name {
		t.Errorf("Name = %q, want %q", got.Program.Name, want.Program.Name)
	}
	// The two timbres carry the fields that share packed bytes, so comparing
	// them whole covers the bit placement as well as the plain bytes.
	if got.Program.Timbre1 != want.Program.Timbre1 {
		t.Errorf("Timbre1 = %+v, want %+v", got.Program.Timbre1, want.Program.Timbre1)
	}
	if got.Program.Timbre2 != want.Program.Timbre2 {
		t.Errorf("Timbre2 = %+v, want %+v", got.Program.Timbre2, want.Program.Timbre2)
	}
	if got.Program.Arpeggio != want.Program.Arpeggio {
		t.Errorf("Arpeggio = %+v, want %+v", got.Program.Arpeggio, want.Program.Arpeggio)
	}
	if got.Program.DelayFx != want.Program.DelayFx {
		t.Errorf("DelayFx = %+v, want %+v", got.Program.DelayFx, want.Program.DelayFx)
	}
	if got.Program.ModFx != want.Program.ModFx {
		t.Errorf("ModFx = %+v, want %+v", got.Program.ModFx, want.Program.ModFx)
	}
	if got.Program.Eq != want.Program.Eq {
		t.Errorf("Eq = %+v, want %+v", got.Program.Eq, want.Program.Eq)
	}
	if got.Program.VoiceMode != want.Program.VoiceMode {
		t.Errorf("VoiceMode = %d, want %d", got.Program.VoiceMode, want.Program.VoiceMode)
	}
	if got.Program.KeyboardOctave != want.Program.KeyboardOctave {
		t.Errorf("KeyboardOctave = %d, want %d", got.Program.KeyboardOctave, want.Program.KeyboardOctave)
	}
}

func TestVocoderProgramRoundTrip(t *testing.T) {
	want := ProgramData{Channel: 0, Program: sampleProgram("VOC")}
	want.Program.VoiceMode = VoiceModeVocoder
	want.Program.Vocoder = sampleVocoder()

	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var got ProgramData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Program.Vocoder != want.Program.Vocoder {
		t.Errorf("Vocoder = %+v, want %+v", got.Program.Vocoder, want.Program.Vocoder)
	}
	if got.Program.VoiceMode != VoiceModeVocoder {
		t.Errorf("VoiceMode = %d, want %d", got.Program.VoiceMode, VoiceModeVocoder)
	}
}

func TestProgramBankRoundTrip(t *testing.T) {
	want := ProgramBank{Channel: 1}
	for i := range want.Programs {
		want.Programs[i] = sampleProgram("BANK")
		want.Programs[i].KeyboardOctave = -2 + i%4
	}
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(encoded) != headerSize+1+37157+1 {
		t.Fatalf("encoded length = %d, want %d", len(encoded), headerSize+1+37157+1)
	}
	var got ProgramBank
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Channel != want.Channel {
		t.Errorf("Channel = %d, want %d", got.Channel, want.Channel)
	}
	for i := range want.Programs {
		if got.Programs[i].Timbre1 != want.Programs[i].Timbre1 {
			t.Fatalf("program %d timbre 1 = %+v, want %+v", i, got.Programs[i].Timbre1, want.Programs[i].Timbre1)
		}
	}
}

func TestGlobalDataRoundTrip(t *testing.T) {
	want := GlobalData{Channel: 2, Global: sampleGlobal()}
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(encoded) != headerSize+1+229+1 {
		t.Fatalf("encoded length = %d, want %d", len(encoded), headerSize+1+229+1)
	}
	var got GlobalData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Global != want.Global {
		t.Fatalf("Global = %+v, want %+v", got.Global, want.Global)
	}
}

func TestAllDataRoundTrip(t *testing.T) {
	want := AllData{Channel: 3, Global: sampleGlobal()}
	for i := range want.Programs {
		want.Programs[i] = sampleProgram("ALL")
	}
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if len(encoded) != headerSize+1+37386+1 {
		t.Fatalf("encoded length = %d, want %d", len(encoded), headerSize+1+37386+1)
	}
	var got AllData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Global != want.Global {
		t.Errorf("Global = %+v, want %+v", got.Global, want.Global)
	}
	if got.Programs[0].Timbre1 != want.Programs[0].Timbre1 {
		t.Errorf("program 0 = %+v, want %+v", got.Programs[0].Timbre1, want.Programs[0].Timbre1)
	}
}

// The packed payload is what actually travels, so a dump whose payload has an
// eighth bit set is not a Korg message and must be refused rather than decoded
// into plausible looking parameters.
func TestUnpackRejectsEightBitPayload(t *testing.T) {
	data, err := (&ProgramData{Channel: 0, Program: sampleProgram("X")}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	data[headerSize+1] |= 0x80
	var got ProgramData
	if err := got.UnmarshalBinary(data); err == nil {
		t.Fatal("decoded a message with an eighth bit set in its payload")
	}
}

func TestUnmarshalRejectsWrongFunction(t *testing.T) {
	data, err := (&ProgramData{Channel: 0, Program: sampleProgram("X")}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	data[4] = FuncProgramDump
	var got ProgramData
	if err := got.UnmarshalBinary(data); err == nil {
		t.Fatal("accepted a program dump where a program bank dump was expected")
	}
}

func TestUnmarshalRejectsWrongManufacturer(t *testing.T) {
	data, err := (&ProgramData{Channel: 0, Program: sampleProgram("X")}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	data[1] = 0x43
	var got ProgramData
	if err := got.UnmarshalBinary(data); err == nil {
		t.Fatal("accepted a Yamaha message")
	}
}

func TestUnmarshalRejectsWrongLength(t *testing.T) {
	data, err := (&GlobalData{Channel: 0, Global: sampleGlobal()}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var got GlobalData
	if err := got.UnmarshalBinary(data[:len(data)-1]); err == nil {
		t.Fatal("accepted a truncated global dump")
	}
}

// A dump the instrument would clamp is worse than a rejected one, because the
// clamped value comes back on the next read and the edit looks like it stuck.
func TestEncodeRejectsOutOfRangeParameters(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProgramData)
	}{
		{"arpeggio tempo", func(p *ProgramData) { p.Program.Arpeggio.Tempo = 301 }},
		{"filter wave", func(p *ProgramData) { p.Program.Timbre1.Osc1Wave = 8 }},
		{"osc2 mod", func(p *ProgramData) { p.Program.Timbre2.Osc2ModSelect = 4 }},
		{"equaliser gain", func(p *ProgramData) { p.Program.Eq.HighGain = 51 }},
		{"keyboard octave", func(p *ProgramData) { p.Program.KeyboardOctave = 4 }},
		{"patch source", func(p *ProgramData) { p.Program.Timbre1.Patches[0].Source = 8 }},
		{"lfo sync note", func(p *ProgramData) { p.Program.Timbre2.Lfo2SyncNote = 15 }},
		{"program name too long", func(p *ProgramData) { p.Program.Name = "THIRTEENCHARS" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := &ProgramData{Channel: 0, Program: sampleProgram("X")}
			test.mutate(data)
			if _, err := data.MarshalBinary(); err == nil {
				t.Error("encoded a program the instrument cannot hold")
			}
		})
	}
}

func TestEncodeRejectsOutOfRangeChannel(t *testing.T) {
	if _, err := (&ProgramData{Channel: 16, Program: sampleProgram("X")}).MarshalBinary(); err == nil {
		t.Fatal("encoded a channel above 15")
	}
	if _, err := (&DumpRequest{Channel: 16, Function: FuncAllDumpRequest}).MarshalBinary(); err == nil {
		t.Fatal("encoded a dump request on a channel above 15")
	}
}

// The strongest statement a round trip can make without an instrument to hand:
// every message kind, encoded from a program that exercises all of its fields,
// decodes and re-encodes to the very same bytes. Anything that shifted an
// offset, dropped a reserved byte or disagreed with itself about a packed bit
// would change the message here.
func TestMessagesRoundTripByteExact(t *testing.T) {
	layer := sampleProgram("LAYER")
	single := sampleProgram("SNGLE")
	single.VoiceMode = VoiceModeSingle
	vocoder := sampleProgram("VOCODR")
	vocoder.VoiceMode = VoiceModeVocoder
	vocoder.Vocoder = sampleVocoder()

	bank := &ProgramBank{Channel: 7}
	for i := range bank.Programs {
		bank.Programs[i] = sampleProgram("BANK")
		bank.Programs[i].KeyboardOctave = -2 + i%4
	}
	all := &AllData{Channel: 1, Global: sampleGlobal()}
	all.Programs = bank.Programs

	tests := []struct {
		name   string
		encode func() ([]byte, error)
		decode func([]byte) (func() ([]byte, error), error)
	}{
		{
			name:   "current program",
			encode: (&ProgramData{Channel: 5, Program: layer}).MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got ProgramData
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
		{
			name:   "single voice program",
			encode: (&ProgramData{Channel: 0, Program: single}).MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got ProgramData
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
		{
			name:   "vocoder program",
			encode: (&ProgramData{Channel: 15, Program: vocoder}).MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got ProgramData
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
		{
			name:   "program bank",
			encode: bank.MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got ProgramBank
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
		{
			name:   "all data",
			encode: all.MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got AllData
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
		{
			name:   "global",
			encode: (&GlobalData{Channel: 2, Global: sampleGlobal()}).MarshalBinary,
			decode: func(b []byte) (func() ([]byte, error), error) {
				var got GlobalData
				err := got.UnmarshalBinary(b)
				return got.MarshalBinary, err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.encode()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			reencode, err := test.decode(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			again, err := reencode()
			if err != nil {
				t.Fatalf("re-encode: %v", err)
			}
			if !bytes.Equal(again, encoded) {
				for i := range encoded {
					if encoded[i] != again[i] {
						t.Fatalf("re-encoded %d bytes differ at %d: %#02x against %#02x",
							len(encoded), i, again[i], encoded[i])
					}
				}
			}
		})
	}
}

// The errors the decoders return belong to the sysex package, so that a caller
// can tell a malformed message from an instrument that simply refused a write.
func TestErrorsComeFromTheSysexPackage(t *testing.T) {
	var got ProgramData
	if err := got.UnmarshalBinary([]byte{0xf0, 0x42, 0x30, 0x58, 0x40, 0xf7}); err != sysex.ErrBadRange {
		t.Fatalf("UnmarshalBinary of a truncated dump = %v, want %v", err, sysex.ErrBadRange)
	}
}

// A dump arrives from a cable, so every decoder has to survive a message that
// is the wrong length, the wrong function, or simply not a Korg message at all,
// without walking off the end of the buffer.
func TestDecodersSurviveArbitraryInput(t *testing.T) {
	seeds := [][]byte{}
	for _, encode := range []func() ([]byte, error){
		(&ProgramData{Channel: 0, Program: sampleProgram("X")}).MarshalBinary,
		(&GlobalData{Channel: 0, Global: sampleGlobal()}).MarshalBinary,
		(&DumpRequest{Channel: 0, Function: FuncAllDumpRequest}).MarshalBinary,
		(&ProgramWriteRequest{Channel: 0, Program: 3}).MarshalBinary,
		(&Status{Channel: 0, Function: FuncDataLoadCompleted}).MarshalBinary,
	} {
		data, err := encode()
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		seeds = append(seeds, data)
	}
	decoders := []func() interface{ UnmarshalBinary([]byte) error }{
		func() interface{ UnmarshalBinary([]byte) error } { return &ProgramData{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &ProgramBank{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &GlobalData{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &AllData{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &DumpRequest{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &ProgramWriteRequest{} },
		func() interface{ UnmarshalBinary([]byte) error } { return &Status{} },
	}
	// Every prefix of a real message, and every single-byte corruption of the
	// first one, are the cases most likely to walk off the end of a buffer.
	for _, seed := range seeds {
		for length := range seed {
			for _, newDecoder := range decoders {
				_ = newDecoder().UnmarshalBinary(seed[:length])
			}
		}
	}
	for i := range seeds[0] {
		corrupt := make([]byte, len(seeds[0]))
		copy(corrupt, seeds[0])
		corrupt[i] ^= 0xff
		for _, newDecoder := range decoders {
			_ = newDecoder().UnmarshalBinary(corrupt)
		}
	}
}

// A factory vocoder program stores a filter envelope sustain of 3 where the
// sheet says the envelope is fixed at 127, so the value is carried rather than
// pinned. This guards that decision, which is otherwise invisible in the
// samples and is the kind of thing a later reader would "correct".
func TestVocoderFilterEnvelopeIsNotPinned(t *testing.T) {
	program := sampleProgram("VOC")
	program.VoiceMode = VoiceModeVocoder
	program.Vocoder = sampleVocoder()
	program.Vocoder.FilterEgSustain = 3

	data := &ProgramData{Channel: 0, Program: program}
	encoded, err := data.MarshalBinary()
	if err != nil {
		t.Fatalf("refused a vocoder whose filter envelope sustain is 3: %v", err)
	}
	var got ProgramData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Program.Vocoder.FilterEgSustain != 3 {
		t.Errorf("FilterEgSustain = %d, want 3", got.Program.Vocoder.FilterEgSustain)
	}
}

// A byte exact round trip cannot tell four distinct patches from one patch
// written four times, so each of the four is checked to occupy its own two
// bytes. Changing one must move exactly one byte of the decoded record, and it
// must be the one the record says.
func TestEachPatchOccupiesItsOwnRecord(t *testing.T) {
	base := &ProgramData{Channel: 0, Program: sampleProgram("X")}
	before, err := base.MarshalBinary()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	beforePayload, err := unpackPayload(before[headerSize+1:len(before)-1], programSize)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}

	for i := range base.Program.Timbre1.Patches {
		changed := &ProgramData{Channel: 0, Program: sampleProgram("X")}
		// Sources run 0 to 7, so step to a value that differs from every
		// sample without colliding with the one already there.
		changed.Program.Timbre1.Patches[i].Source = (base.Program.Timbre1.Patches[i].Source + 3) % 8
		after, err := changed.MarshalBinary()
		if err != nil {
			t.Fatalf("encode patch %d: %v", i, err)
		}
		afterPayload, err := unpackPayload(after[headerSize+1:len(after)-1], programSize)
		if err != nil {
			t.Fatalf("unpack: %v", err)
		}
		moved := []int{}
		for b := range beforePayload {
			if beforePayload[b] != afterPayload[b] {
				moved = append(moved, b)
			}
		}
		// The record's first timbre starts at vocoderOffset and its first
		// virtual patch at 44, sharing a byte with its destination.
		want := vocoderOffset + 44 + i*2
		if len(moved) != 1 || moved[0] != want {
			t.Errorf("patch %d changed record bytes %v, want exactly [%d]", i, moved, want)
		}
	}
}

// A write request is a different message, two bytes longer and carrying a
// destination program, so it has to be refused by the dump request decoder
// rather than half accepted by it.
func TestWriteRequestIsNotADumpRequest(t *testing.T) {
	write := &ProgramWriteRequest{Channel: 2, Program: 77}
	encoded, err := write.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var request DumpRequest
	if err := request.UnmarshalBinary(encoded); err == nil {
		t.Fatalf("a write request decoded as %+v", request)
	}
	var decoded ProgramWriteRequest
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded != *write {
		t.Errorf("UnmarshalBinary = %+v, want %+v", decoded, *write)
	}
}
