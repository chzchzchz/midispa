package microkorg

import (
	"github.com/chzchzchz/midispa/sysex"
)

// UnmarshalBinary decodes a current program data dump.
func (p *ProgramData) UnmarshalBinary(data []byte) error {
	ch, payload, err := unpack(data, FuncCurrentProgramDump, programSize)
	if err != nil {
		return err
	}
	var program Program
	if err := program.decode(payload); err != nil {
		return err
	}
	p.Channel = ch
	p.Program = program
	return nil
}

// UnmarshalBinary decodes a program data dump.
func (b *ProgramBank) UnmarshalBinary(data []byte) error {
	ch, payload, err := unpack(data, FuncProgramDump, programBankSize)
	if err != nil {
		return err
	}
	var programs [programCount]Program
	for i := range programs {
		start := i * programSize
		if err := programs[i].decode(payload[start : start+programSize]); err != nil {
			return err
		}
	}
	b.Channel = ch
	b.Programs = programs
	return nil
}

// UnmarshalBinary decodes a global data dump.
func (g *GlobalData) UnmarshalBinary(data []byte) error {
	ch, payload, err := unpack(data, FuncGlobalDump, globalSize)
	if err != nil {
		return err
	}
	var global Global
	if err := global.decode(payload); err != nil {
		return err
	}
	g.Channel = ch
	g.Global = global
	return nil
}

// UnmarshalBinary decodes an all data dump.
func (a *AllData) UnmarshalBinary(data []byte) error {
	ch, payload, err := unpack(data, FuncAllDump, allDataSize)
	if err != nil {
		return err
	}
	var programs [programCount]Program
	for i := range programs {
		start := i * programSize
		if err := programs[i].decode(payload[start : start+programSize]); err != nil {
			return err
		}
	}
	var global Global
	if err := global.decode(payload[programBankSize:]); err != nil {
		return err
	}
	a.Channel = ch
	a.Programs = programs
	a.Global = global
	return nil
}

// decode reads a program out of the 254 bytes of the sheet's table 1.
func (p *Program) decode(data []byte) error {
	if len(data) != programSize {
		return sysex.ErrBadRange
	}
	program := Program{
		Name:           decodeName(data[0:programNameSize]),
		VoiceMode:      (int(data[16]) >> 4) & 0x03,
		ScaleKey:       (int(data[17]) >> 4) & 0x0f,
		ScaleType:      int(data[17]) & 0x0f,
		TriggerLength:  int(data[14]) & 0x07,
		TriggerPattern: int(data[15]),
		Reserved:       [3]int{int(data[12]), int(data[13]), int(data[18])},
		DelayFx: DelayFx{
			Sync:     (int(data[19]) >> 7) & 0x01,
			TimeBase: int(data[19]) & 0x0f,
			Time:     int(data[20]),
			Depth:    int(data[21]),
			Type:     int(data[22]),
		},
		ModFx: ModFx{
			LfoSpeed: int(data[23]),
			Depth:    int(data[24]),
			Type:     int(data[25]),
		},
		Eq: Eq{
			HighFrequency: int(data[26]),
			HighGain:      int(data[27]),
			LowFrequency:  int(data[28]),
			LowGain:       int(data[29]),
		},
		Arpeggio: Arpeggio{
			// The tempo is the only field that spans two bytes, and it is
			// a plain sixteen bit value, most significant byte first.
			Tempo:      int(data[30])<<8 | int(data[31]),
			On:         (int(data[32]) >> 7) & 0x01,
			Latch:      (int(data[32]) >> 6) & 0x01,
			Target:     (int(data[32]) >> 4) & 0x03,
			KeySync:    int(data[32]) & 0x01,
			Type:       int(data[33]) & 0x0f,
			Range:      (int(data[33]) >> 4) & 0x0f,
			GateTime:   int(data[34]),
			Resolution: int(data[35]),
			Swing:      int(data[36]),
		},
		// The keyboard octave is a signed byte, so the factory bank's 0xFF
		// and 0xFE are -1 and -2 rather than out of range values.
		KeyboardOctave: int(int8(data[37])),
	}
	// Bytes 12, 13 and 18 hold firmware data, and 38 onwards holds either
	// two synth timbres or a vocoder.
	switch program.VoiceMode {
	case VoiceModeVocoder:
		if err := program.Vocoder.decode(data[38 : 38+vocoderSize]); err != nil {
			return err
		}
	case VoiceModeSingle, VoiceModeLayer:
		if err := program.Timbre1.decode(data[38 : 38+timbreSize]); err != nil {
			return err
		}
		if err := program.Timbre2.decode(data[38+timbreSize:]); err != nil {
			return err
		}
	default:
		return sysex.ErrBadRange
	}
	if err := sysex.CheckTaggedFields(&program); err != nil {
		return err
	}
	*p = program
	return nil
}

// decodeName trims the padding the fixed width program name carries. Both
// spaces and NULs appear in dumps depending on how the name was written.
func decodeName(data []byte) string {
	end := len(data)
	for end > 0 && (data[end-1] == ' ' || data[end-1] == 0) {
		end--
	}
	return string(data[:end])
}

// decode reads a synth voice out of the 108 bytes of the sheet's table 2.
func (t *Timbre) decode(data []byte) error {
	if len(data) != timbreSize {
		return sysex.ErrBadRange
	}
	timbre := Timbre{
		Channel:              int(data[0]),
		AssignMode:           (int(data[1]) >> 6) & 0x03,
		Eg2Reset:             (int(data[1]) >> 5) & 0x01,
		Eg1Reset:             (int(data[1]) >> 4) & 0x01,
		TriggerMode:          (int(data[1]) >> 3) & 0x01,
		KeyPriority:          int(data[1]) & 0x03,
		UnisonDetune:         int(data[2]),
		Tune:                 int(data[3]),
		BendRange:            int(data[4]),
		Transpose:            int(data[5]),
		VibratoInt:           int(data[6]),
		Osc1Wave:             int(data[7]),
		Osc1Ctrl1:            int(data[8]),
		Osc1Ctrl2:            int(data[9]),
		DwgsWave:             int(data[10]),
		Osc2ModSelect:        (int(data[12]) >> 4) & 0x03,
		Osc2Wave:             int(data[12]) & 0x03,
		Osc2Semitone:         int(data[13]),
		Osc2Tune:             int(data[14]),
		Portamento:           int(data[15]) & 0x7f,
		Osc1Level:            int(data[16]),
		Osc2Level:            int(data[17]),
		NoiseLevel:           int(data[18]),
		FilterType:           int(data[19]),
		Cutoff:               int(data[20]),
		Resonance:            int(data[21]),
		FilterEgIntensity:    int(data[22]),
		FilterVelocitySense:  int(data[23]),
		FilterKeyboardTrack:  int(data[24]),
		AmpLevel:             int(data[25]),
		Panpot:               int(data[26]),
		AmpSwitch:            (int(data[27]) >> 6) & 0x01,
		Distortion:           int(data[27]) & 0x01,
		AmpVelocitySense:     int(data[28]),
		AmpKeyboardTrack:     int(data[29]),
		FilterEgAttack:       int(data[30]),
		FilterEgDecay:        int(data[31]),
		FilterEgSustain:      int(data[32]),
		FilterEgRelease:      int(data[33]),
		AmpEgAttack:          int(data[34]),
		AmpEgDecay:           int(data[35]),
		AmpEgSustain:         int(data[36]),
		AmpEgRelease:         int(data[37]),
		Lfo1KeySync:          (int(data[38]) >> 4) & 0x03,
		Lfo1Wave:             int(data[38]) & 0x03,
		Lfo1Frequency:        int(data[39]),
		Lfo1TempoSync:        (int(data[40]) >> 7) & 0x01,
		Lfo1SyncNote:         int(data[40]) & 0x0f,
		Lfo2KeySync:          (int(data[41]) >> 4) & 0x03,
		Lfo2Wave:             int(data[41]) & 0x03,
		Lfo2Frequency:        int(data[42]),
		Lfo2TempoSync:        (int(data[43]) >> 7) & 0x01,
		Lfo2SyncNote:         int(data[43]) & 0x0f,
	}
	// +44..+51: the four virtual patches, source and destination sharing a
	// byte and the intensity taking the next.
	for i := range timbre.Patches {
		timbre.Patches[i] = Patch{
			Source:      int(data[44+i*2]) & 0x07,
			Destination: int(data[44+i*2]) >> 4 & 0x07,
			Intensity:   int(data[45+i*2]),
		}
	}
	// +52..+107: firmware data, carried through untouched.
	for i := range timbre.Reserved {
		timbre.Reserved[i] = int(data[52+i])
	}
	if err := sysex.CheckTaggedFields(&timbre); err != nil {
		return err
	}
	*t = timbre
	return nil
}

// decode reads a vocoder program out of the 104 bytes of the sheet's table 3.
func (v *Vocoder) decode(data []byte) error {
	if len(data) != vocoderSize {
		return sysex.ErrBadRange
	}
	vocoder := Vocoder{
		Channel:     int(data[0]),
		AssignMode:  (int(data[1]) >> 6) & 0x03,
		Eg2Reset:    (int(data[1]) >> 5) & 0x01,
		Eg1Reset:    (int(data[1]) >> 4) & 0x01,
		TriggerMode: (int(data[1]) >> 3) & 0x01,
		KeyPriority: int(data[1]) & 0x03,
		UnisonDetune: int(data[2]),
		Tune:         int(data[3]),
		BendRange:    int(data[4]),
		Transpose:    int(data[5]),
		VibratoInt:   int(data[6]),
		Wave:         int(data[7]),
		Ctrl1:        int(data[8]),
		Ctrl2:        int(data[9]),
		DwgsWave:     int(data[10]),
		HpfGate:      int(data[12]) & 0x01,
		Portamento:   int(data[14]) & 0x7f,
		Osc1Level:    int(data[15]),
		Ext1Level:    int(data[16]),
		NoiseLevel:   int(data[17]),
		HpfLevel:     int(data[18]),
		GateSense:    int(data[19]),
		Threshold:    int(data[20]),
		Shift:        int(data[21]),
		Cutoff:       int(data[22]),
		Resonance:    int(data[23]),
		ModSource:    int(data[24]),
		Intensity:    int(data[25]),
		EFSense:      int(data[26]),
		Level:        int(data[27]),
		DirectLevel:  int(data[28]),
		Distortion:   int(data[29]) & 0x01,
		VelSense:     int(data[30]),
		KeyTrack:     int(data[31]),

		FilterEgAttack:  int(data[32]),
		FilterEgDecay:   int(data[33]),
		FilterEgSustain: int(data[34]),
		FilterEgRelease: int(data[35]),
		AmpEgAttack:     int(data[36]),
		AmpEgDecay:      int(data[37]),
		AmpEgSustain:    int(data[38]),
		AmpEgRelease:    int(data[39]),

		Lfo1KeySync:   (int(data[40]) >> 4) & 0x03,
		Lfo1Wave:      int(data[40]) & 0x03,
		Lfo1Frequency: int(data[41]),
		Lfo1TempoSync: (int(data[42]) >> 7) & 0x01,
		Lfo1SyncNote:  int(data[42]) & 0x0f,
		Lfo2KeySync:   (int(data[43]) >> 4) & 0x03,
		Lfo2Wave:      int(data[43]) & 0x03,
		Lfo2Frequency: int(data[44]),
		Lfo2TempoSync: (int(data[45]) >> 7) & 0x01,
		Lfo2SyncNote:  int(data[45]) & 0x0f,
	}
	// The sixteen analysis channels carry their level and pan interleaved.
	for i := 0; i < 16; i++ {
		vocoder.BandLevel[i] = int(data[46+i*2])
		vocoder.BandPan[i] = int(data[46+i*2+1])
	}
	// The held envelope level of each channel follows, four bytes apiece.
	for i := 0; i < 16; i++ {
		for j := 0; j < 4; j++ {
			vocoder.EfHoldLevel[i][j] = int(data[78+i*4+j])
		}
	}
	if err := sysex.CheckTaggedFields(&vocoder); err != nil {
		return err
	}
	*v = vocoder
	return nil
}

// decode reads the global block out of the 200 bytes of the sheet's table 6.
func (g *Global) decode(data []byte) error {
	if len(data) != globalSize {
		return sysex.ErrBadRange
	}
	global := Global{
		MasterTune:             int(data[0]),
		Transpose:              int(data[1]),
		Position:               int(data[2]) & 0x01,
		VelocityValue:          int(data[3]),
		VelocityCurve:          int(data[4]),
		LocalControl:           (int(data[5]) >> 2) & 0x01,
		MemoryProtect:          int(data[5]) & 0x01,
		UnnamedFlags:           int(data[5]) & unnamedFlagMask,
		Clock:                  int(data[8]) & 0x03,
		MidiChannel:            int(data[9]) & 0x0f,
		SyncControlNumber:      int(data[10]),
		TimbreSelectNumber:     int(data[11]),
		Midi1ControlNumber:     int(data[14]),
		Midi2ControlNumber:     int(data[15]),
		SystemExclusiveFilter:  (int(data[16]) >> 7) & 0x01,
		NoteReceive:            int(data[16]) & 0x03,
		PitchBendFilter:        (int(data[17]) >> 6) & 0x01,
		ControlChangeFilter:    (int(data[17]) >> 2) & 0x01,
		ProgramChangeFilter:    int(data[17]) & 0x01,
		Reserved:               [2]int{int(data[6]), int(data[7])},
	}
	// 18..59: the control change number bound to each panel knob, 60..71 the
	// user scale and 72..199 the program change map.
	for i := 0; i < 42; i++ {
		global.KnobControlChange[i] = int(data[18+i])
	}
	for i := 0; i < 12; i++ {
		global.UserScale[i] = int(data[60+i])
	}
	for i := 0; i < 128; i++ {
		global.ProgramChangeMap[i] = int(data[72+i])
	}
	if err := sysex.CheckTaggedFields(&global); err != nil {
		return err
	}
	*g = global
	return nil
}
