package cc

type CraftSynth2 struct {
	ModulationWheel Control `cc:"1"`
	Glide           Control `cc:"5"` // 0 - 2.5 seconds, exponential
	HeadphoneVolume Control `cc:"7"` // Silence - full volume
	ExpressionPedal Control `cc:"11"`
	Distortion      Control `cc:"12"` // Dry - Wet
	Delay           Control `cc:"13"` // Dry - Wet

	// No Sync: 0 - 250 milliseconds
	// Sync: 8 steps
	// Longest delay time possible divided down
	DelayTime     Control `cc:"14"`
	DelayFeedback Control `cc:"15"` // 0% - 90%
	Osc1Wave      Control `cc:"16"`
	Osc2Wave      Control `cc:"17"`
	OscMix        Control `cc:"18"` // Osc1 - Osc2
	OscModAmount  Control `cc:"19"` // 0 - Full

	// 0 - 63 Unison / 64 - 70 Major / 71 - 77 Minor / 78 - 84 Major 6th
	// / 85 - 91 Sus 4th / 92 - 98 5ths / 99 - 105 5th + Oct
	// / 106 - 112 Oct + 1 + 2/ 113 - 119 Oct + 1 -1 / 119 - 127 Oct -1 -2
	Spread Control `cc:"20"`

	FegAttack        Control `cc:"22"` // 0 - 4 Seconds
	FegDecay         Control `cc:"23"` // 0 - 4 Seconds
	FegSustain       Control `cc:"24"` // 0 - 1
	FegRelease       Control `cc:"25"` // 0 - 4 Seconds
	AegAttack        Control `cc:"26"` // 0 - 4 Seconds
	AegDecay         Control `cc:"27"` // 0 - 4 Seconds
	AegSustain       Control `cc:"28"` // 0 - 1
	AegRelease       Control `cc:"29"` // 0 - 4 Seconds
	Osc2CourseDetune Control `cc:"30"` // +/- 4 Octaves
	Osc2FineDetune   Control `cc:"31"` // -/+ 1 Semitone
	FegAmount        Control `cc:"32"` // 63 (0) +/- 63
	Morph            Control `cc:"33"` // 0 = LP / 64 = BP / 127 = HP
	Cutoff           Control `cc:"34"` // 0Hz - 22kHz
	Reso             Control `cc:"35"` // None - Full

	/*
		NO SYNC: 0-127 = 0.02Hz - 32Hz
		SYNC: 0-7 = 1/16 / 8-15 = 1/8 / 16-23 = 3/16 / 24-31 = 1/4 /
		32-39 = 3/8 / 40-47 = 1/2 / 48-55 = 3/4 / 56-63 = 1 / 64-71 = 3/2
		/ 72-79 = 2 / 80-87 = 3 / 88-95 = 4 / 96-103 = 6 /104-111 = 8 /
		112-119 = 12 / 120-127 = 16
	*/
	Lfo1Rate  Control `cc:"36"`
	Lfo1Depth Control `cc:"37"` // 63 (0) +/- 63

	//  0-32 Sine to Triangle / 33-64 - Triangle to Sawtooth
	// / 65-96 - Sawtooth to Square / 97-127 - Square to Sample and Hold
	Lfo1Shape  Control `cc:"39"`
	Octave     Control `cc:"40"` // Octaves -2 to +4
	OscModMode Control `cc:"41"` // 0 - 127 (16 Modes)
	MegAttack  Control `cc:"43"` // 0 - 4 Seconds
	MegDecay   Control `cc:"44"` // 0 - 4 Seconds
	MegSustain Control `cc:"45"` // 0 - 1
	MegRelease Control `cc:"46"` // 0 - 4 Seconds
	Lfo2Rate   Control `cc:"47"`

	/*
		NO SYNC: 0-63 = 0-32Hz Free / 64-71 Root/8 / 72-79 Root/4 /
		80-87 Root/2 / 88-95 Root / 96-103 Root*1.5 /104-111 Root*2 /
		112-119 Root*2.5 / 120-127 Root*3
		SYNC: 0-7 = 1/16 / 8-15 = 1/8 / 16-23 =1/4 / 24-31 =1/2 / 32-39
		= 1 / 40-47 = 5/4 / 48-55 =2 / 56-63 = 4 (Cycles per beat)
	*/
	Lfo2Depth Control `cc:"48"` // 63 (0) +/- 63

	// 0-32 Sine to Triangle / 33-64 - Triangle to Sawtooth
	// / 65-96 - Sawtooth to Square / 97-127 - Square to Sample and Hold
	Lfo2Shape Control `cc:"50"`

	AegAmount          Control `cc:"51"`  // 63 (0) +/- 63
	Lfo1MidiSync       Control `cc:"52"`  // 0 - 63 = OFF / 64 - 127 = ON
	Lfo2MidiSync       Control `cc:"54"`  // 0 - 63 = OFF / 64 - 127 = ON
	DelayMidiSync      Control `cc:"55"`  // 0 - 63 = OFF / 64 - 127 = ON
	Lfo1Mode           Control `cc:"56"`  // 0-41 Retrig / 42-83 Free / 84-127 Single
	Lfo2Mode           Control `cc:"57"`  // 0-41 Retrig / 42-83 Free / 84-127 Single
	ArpStatus          Control `cc:"58"`  // 0 - 63 = OFF / 64 - 127 = ON
	SustainPedal       Control `cc:"64"`  // 0 - 63 = OFF / 64 - 127 = ON
	Scale              Control `cc:"73"`  // 0 - 7
	RootNote           Control `cc:"79"`  // 0 - 127
	AllEnvelopeAttack  Control `cc:"84"`  // 0 - 4 Seconds
	AllEnvelopeDecay   Control `cc:"85"`  // 0 - 4 Seconds
	AllEnvelopeSustain Control `cc:"86"`  // 0 - 1
	AllEnvelopeRelease Control `cc:"87"`  // 0 - 4 Seconds
	ModSlot1Depth      Control `cc:"88"`  // 63 (0) +/- 63
	ModSlot2Depth      Control `cc:"89"`  // 63 (0) +/- 63
	ModSlot3Depth      Control `cc:"90"`  // 63 (0) +/- 63
	ModSlot4Depth      Control `cc:"91"`  // 63 (0) +/- 63
	ModSlot5Depth      Control `cc:"92"`  // 63 (0) +/- 63
	ModSlot6Depth      Control `cc:"93"`  // 63 (0) +/- 63
	ModSlot7Depth      Control `cc:"94"`  // 63 (0) +/- 63
	ModSlot8Depth      Control `cc:"95"`  // 63 (0) +/- 63
	ModSlot1Dest       Control `cc:"101"` // 0 - 36
	ModSlot2Dest       Control `cc:"102"` // 0 - 36
	ModSlot3Dest       Control `cc:"103"` // 0 - 36
	ModSlot4Dest       Control `cc:"104"` // 0 - 36
	ModSlot5Dest       Control `cc:"105"` // 0 - 36
	ModSlot6Dest       Control `cc:"106"` // 0 - 36
	ModSlot7Dest       Control `cc:"107"` // 0 - 36
	ModSlot8Dest       Control `cc:"108"` // 0 - 36
	RandomisePatch     Control `cc:"121"`
}

type VolcaBass struct {
	SlideTime         Control `cc:"5"`
	Expression        Control `cc:"11"`
	Octave            Control `cc:"40"`
	LfoRate           Control `cc:"41"`
	LfoIntensity      Control `cc:"42"`
	VcoPitch1         Control `cc:"43"`
	VcoPitch2         Control `cc:"44"`
	VcoPitch3         Control `cc:"45"`
	EgAttack          Control `cc:"46"`
	EgDecayRelease    Control `cc:"47"`
	CutoffEgIntensity Control `cc:"48"`
	GateTime          Control `cc:"49"`
}

type VolcaBeats struct {
	KickLevel      Control `cc:"40"`
	SnareLevel     Control `cc:"41"`
	LoTomLevel     Control `cc:"42"`
	HiTomLevel     Control `cc:"43"`
	ClosedHatLevel Control `cc:"44"`
	OpenHatLevel   Control `cc:"45"`
	ClapLevel      Control `cc:"46"`
	ClavesLevel    Control `cc:"47"`
	AgogoLevel     Control `cc:"48"`
	CrashLevel     Control `cc:"49"`
	ClapPCMSpeed   Control `cc:"50"`
	ClavesPCMSpeed Control `cc:"51"`
	AgogoPCMSpeed  Control `cc:"52"`
	CrashPCMSpeed  Control `cc:"53"`
	StutterTime    Control `cc:"54"`
	StutterDepth   Control `cc:"55"`
	TomDecay       Control `cc:"56"`
	ClosedHatDecay Control `cc:"57"`
	OpenHatDecay   Control `cc:"58"`
	HatGrain       Control `cc:"59"`
}

type VolcaKeys struct {
	Portamento     Control `cc:"5"`
	Detune         Control `cc:"42"`
	VcoEGintensity Control `cc:"43"`
	Expression     Control `cc:"11"`
	// 0-12: Poly;
	// 13-37: Unison;
	// 38-62: Octave;
	// 63-87: Fifth;
	// 88-112: Unison Ring;
	// 113-127: Poly Ring
	Voice Control `cc:"40"`
	// 0-21: 32'; 22-43: 16'; 44-65: 8'; 66-87: 4'; 88-109: 2'; 110-127: 1'
	Octave         Control `cc:"41"`
	Attack         Control `cc:"49"`
	DecayRelease   Control `cc:"50"`
	Sustain        Control `cc:"51"`
	VcfCutoff      Control `cc:"44"`
	VcfEGintensity Control `cc:"45"`
	LfoRate        Control `cc:"46"`
	LfoPitch       Control `cc:"47"`
	LfoCutoff      Control `cc:"48"`
	DelayTime      Control `cc:"52"`
	DelayFeedback  Control `cc:"53"`
}

type VolcaKick struct {
	PulseColor     Control `cc:"40"`
	PulseLevel     Control `cc:"41"`
	AmpAttack      Control `cc:"42"`
	AmpDecay       Control `cc:"43"`
	Drive          Control `cc:"44"`
	Tone           Control `cc:"45"`
	ResonatorPitch Control `cc:"46"`
	ResonatorBend  Control `cc:"47"`
	ResonatorTime  Control `cc:"48"`
	Accent         Control `cc:"49"`
}

type VolcaDrum struct {
	Pan            Control `cc:"10"`
	Select1        Control `cc:"14"`
	Select2        Control `cc:"15"`
	Select1m2      Control `cc:"16"`
	Level1         Control `cc:"17"`
	Level2         Control `cc:"18"`
	Level1m2       Control `cc:"19"`
	EGAttack1      Control `cc:"20"`
	EGAttack2      Control `cc:"21"`
	EGAttack1m2    Control `cc:"22"`
	EGRelease1     Control `cc:"23"`
	EGRelease2     Control `cc:"24"`
	EGRelease1m2   Control `cc:"25"`
	Pitch1         Control `cc:"26"`
	Pitch2         Control `cc:"27"`
	Pitch1m2       Control `cc:"28"`
	ModAmount1     Control `cc:"29"`
	ModAmount2     Control `cc:"30"`
	ModAmount1m2   Control `cc:"31"`
	ModRate1       Control `cc:"46"`
	ModRate2       Control `cc:"47"`
	ModRate1m2     Control `cc:"48"`
	BitReduction   Control `cc:"49"`
	Fold           Control `cc:"50"`
	Drive          Control `cc:"51"`
	DryGain        Control `cc:"52"`
	Send           Control `cc:"103"`
	WaveguideModel Control `cc:"116"`
	Decay          Control `cc:"117"`
	Body           Control `cc:"118"`
	Tune           Control `cc:"119"`
}

type MeeblipTriode struct {
	LfoDepth   Control `cc:"48"`
	LfoRate    Control `cc:"49"`
	Detune     Control `cc:"50"`
	Glide      Control `cc:"51"`
	PulseWidth Control `cc:"58"`

	Resonance          Control `cc:"52"`
	Cutoff             Control `cc:"53"`
	FilterAccent       Control `cc:"56"`
	EnvelopeModulation Control `cc:"57"`
	FilterAttack       Control `cc:"59"`
	FilterDecay        Control `cc:"54"`
	AmplitudeDecay     Control `cc:"55"`
	AmplitudeAttack    Control `cc:"60"`

	// Buttons
	LfoNoteRetrigger Control `cc:"70"`
	SubOscillator    Control `cc:"65"`
	PWMSweep         Control `cc:"66"`
	WavePulseSaw     Control `cc:"68"`
	Sustain          Control `cc:"64"`
	LfoRandomize     Control `cc:"69"`
	LfoDestination   Control `cc:"67"`
}

type MeeblipSE struct {
	FilterResonance      Control `cc:"48"`
	FilterCutoff         Control `cc:"49"`
	LfoFrequency         Control `cc:"50"`
	LfoLevel             Control `cc:"51"`
	FilterEnvelopeAmount Control `cc:"52"`
	Portamento           Control `cc:"53"`
	PulseWidthPWMRate    Control `cc:"54"`
	OscillatorDetune     Control `cc:"55"`
	FilterDecay          Control `cc:"58"`
	FilterAttack         Control `cc:"59"`
	AmplitudeDecay       Control `cc:"60"`
	AmplitudeAttack      Control `cc:"61"`

	// switches; 0-63 = off, 64-127 = on
	KnobShift         Control `cc:"64"`
	FM                Control `cc:"65"`
	LfoRandom         Control `cc:"66"`
	LfoWave           Control `cc:"67"` // (Triangle/Square)
	FilterMode        Control `cc:"68"` // (Low/High)
	Distortion        Control `cc:"69"`
	LfoEnable         Control `cc:"70"`
	LfoDestination    Control `cc:"71"` // (Filter/Oscillator)
	AntiAlias         Control `cc:"72"`
	OscillatorBOctave Control `cc:"73"` // (Normal/Up)
	OscillatorBEnable Control `cc:"74"`
	OscillatorBWave   Control `cc:"75"` // (Triangle/Square)
	EnvelopeSustain   Control `cc:"76"`
	OscillatorANoise  Control `cc:"77"`
	PWMSweep          Control `cc:"78"` // (Pulse/PWM)
	OscillatorAWave   Control `cc:"79"` //(Sawtooth/PWM)
}

type UnoSynth struct {
	ModulationWheel           Control `cc:"1"`
	GlideTime                 Control `cc:"5"`
	VCALevel                  Control `cc:"7"`
	Swing                     Control `cc:"9"`
	GlideOnOff                Control `cc:"65"`
	VibratoOnOff              Control `cc:"77"`
	WahOnOff                  Control `cc:"78"`
	TremoloOnOff              Control `cc:"79"`
	DiveOnOff                 Control `cc:"89"`
	DiveRange                 Control `cc:"90"`
	ScoopOnOff                Control `cc:"91"`
	ScoopRange                Control `cc:"92"`
	ModWheelToLFORate         Control `cc:"93"`
	ModWheelToVibrato         Control `cc:"94"`
	ModWheelToWah             Control `cc:"95"`
	ModWheelToTremelo         Control `cc:"96"`
	ModWheelFilterCutoff      Control `cc:"97"`
	PitchBendRange            Control `cc:"101"`
	VelocityToVCA             Control `cc:"102"`
	VelocityToFilterCutoff    Control `cc:"103"`
	VelocityToFilterEnvAmount Control `cc:"104"`
	VelocityToLFORate         Control `cc:"105"`
	FilterCutoffKeytrack      Control `cc:"106"`
	DelayMix                  Control `cc:"80"`
	DelayTime                 Control `cc:"81"`
	OSC1Level                 Control `cc:"12"`
	OSC2Level                 Control `cc:"13"`
	NoiseLevel                Control `cc:"14"`
	OSC1Wave                  Control `cc:"15"`
	OSC2Wave                  Control `cc:"16"`
	OSC1Tune                  Control `cc:"17"`
	OSC2Tune                  Control `cc:"18"`
	AmpAttack                 Control `cc:"24"`
	AmpDecay                  Control `cc:"25"`
	AmpSustain                Control `cc:"26"`
	AmpRelease                Control `cc:"27"`
	FilterMode                Control `cc:"19"`
	FilterCutoff              Control `cc:"20"`
	FilterResonance           Control `cc:"21"`
	FilterDrive               Control `cc:"22"`
	FilterEnv                 Control `cc:"23"`
	FilterAttack              Control `cc:"44"`
	FilterDecay               Control `cc:"45"`
	FilterSustain             Control `cc:"46"`
	FilterRelease             Control `cc:"47"`
	FilterEnvToOSC1PWM        Control `cc:"48"`
	FilterEnvToOSC2PWM        Control `cc:"49"`
	FilterEnvToOSC1Wave       Control `cc:"50"`
	FilterEnvToOSC2Wave       Control `cc:"51"`
	LFOWave                   Control `cc:"66"`
	LFORate                   Control `cc:"67"`
	LFOToPitch                Control `cc:"68"`
	LFOToFilterCutoff         Control `cc:"69"`
	LFOToTremelo              Control `cc:"70"`
	LFOToWah                  Control `cc:"71"`
	LFOToVibr                 Control `cc:"72"`
	LFOToOSC1PWM              Control `cc:"73"`
	LFOToOSC2PWM              Control `cc:"74"`
	LFOToOSC1Waveform         Control `cc:"75"`
	LFOToOSC2Waveform         Control `cc:"76"`
	ArpeggiatorOnOff          Control `cc:"82"`
	ArpeggiatorDirection      Control `cc:"83"`
	ArpeggiatorRange          Control `cc:"84"`
	ArpeggiatorAndSeqGateTime Control `cc:"85"`
	SeqDirection              Control `cc:"86"`
	SeqRange                  Control `cc:"87"`
}

type Skulpt struct {
	// SeqLoad Control `cc:"0"` // [0,63]
	ModulationWheel  Control `cc:"1"`
	Glide            Control `cc:"5"`
	HeadphoneVolume  Control `cc:"7"`
	VoiceMode        Control `cc:"9"`  // 0-42 for mono; 43-85 for duo; 86-127 for poly
	ExpressionPedal  Control `cc:"11"` //,,0,63,,,,,0-based,Min and max values need verification
	Distortion       Control `cc:"12"`
	Delay            Control `cc:"13"`
	DelayTime        Control `cc:"14"` // No sync: 0-250ms. Sync: 8 steps with longest delay time possible divided down.
	DelayFeedback    Control `cc:"15"` // Ranges from 0-90%
	OSC1Wave         Control `cc:"16"` // 0-21 for sine; 22-42 for tri; 43-63 for saw; 64-127 for PWM duty 50%-5%
	OSC2Wave         Control `cc:"17"` // 0-21 for sine; 22-42 for tri; 43-63 for saw; 64-85 for square; 86-127 for white noise
	OSCMix           Control `cc:"18"`
	FMAmount         Control `cc:"19"` // Centered Plus or minus 2 octaves.
	Spread           Control `cc:"20"` // 0-63 for unison; 64-70 for major; 71-77 for minor; 78-84 for major 6th; 85-91 for sus 4th; 92-98 for 5ths; 99-105 for 5th + oct; 106-112 for oct +1+2; 113-119 for oct +1-1; 119-127 for oct-1-2
	ChordMode        Control `cc:"21"` // 0-63 for off; 64-127 for on
	FEGattack        Control `cc:"22"` // 0-4 seconds
	FEGDecay         Control `cc:"23"` // 0-2 seconds
	FEGSustain       Control `cc:"24"` // 0-1 seconds
	FEGRelease       Control `cc:"25"` // 0-4 seconds
	AEGAttack        Control `cc:"26"` // 0-4 seconds
	AEGDecay         Control `cc:"27"` // 0-2 seconds
	AEGSustain       Control `cc:"28"` // 0-1 seconds
	AEGRelease       Control `cc:"29"` // 0-4 seconds
	OSC2CourseDetune Control `cc:"30"` // Plus or minus 4 octaves
	OSC2FineDetune   Control `cc:"31"` // Plus or minus 1 semitone
	FEGAmount        Control `cc:"32"`
	Morph            Control `cc:"33"` // 0 for LP; 64 for BP; 127 for HP
	Cutoff           Control `cc:"34"` // 0Hz to 2kHz
	Reso             Control `cc:"35"`
	LFO1Rate         Control `cc:"36"` // No sync: 0-127 for 0.02Hz - 32Hz. Sync: 0-7 for 1/16; 8-15 for 1/8; 16-23 for 3/16; 24-31 for 1/4; 32-39 for 3/8; 40-47 for 1/2; 48-55 for 3/4; 56-53 for 1; 64-71 for 3/2; 72-79 for 2; 80-87 for 3; 88-95 for 4; 96-103 for 6; 104-111 for 8; 112-119 for 12; 120-127 for 16
	LFO1Depth        Control `cc:"37"`
	LFO1Shape        Control `cc:"39"` // 0-14 for sine; 15-31 for iSine; 32-47 for tri; 48-63 for iTri; 64-79 for ramp up; 80-95 for ramp down; 96-120 for square; 121-127 for iSquare
	OctAve           Control `cc:"40"` // Octaves -2 to +4
	MEGAttack        Control `cc:"43"` //0-4 seconds
	MEGDecay         Control `cc:"44"` // 0-2 seconds
	MEGSustain       Control `cc:"45"` // 0-1 seconds
	MEGRelease       Control `cc:"46"` // 0-4 seconds
	LFO2Rate         Control `cc:"47"` // No sync: 0-63 for 0-32Hz free; 64-71 for root/8; 72-79 for root/4; 80-87 for root/2; 88-95 for root; 96-13 for root*1.5; 104-111 for root*2; 112-119 for root*2.5; 120*127 for root*3. Sync: 0-7 for 1/16; 8-15 for 1/8; 16-23 for 1/4; 24-31 for 1/2; 32-39 for 1; 40-47 for 5/4; 48-55 for 2; 56-63 for 4 (cycles per beat)
	LFO2Depth        Control `cc:"48"`
	MEGAmount        Control `cc:"49"` // Centered
	LFO2Shape        Control `cc:"50"` // 0-14 for sine; 15-31 for iSine; 32-47 for tri; 48-63 for iTri; 64-79 for ramp up; 80-95 for ramp down; 96-120 for square; 121-127 for iSquare
	AEGAmount        Control `cc:"51"`
	LFO1MIDISync     Control `cc:"52"` // 0-63 for off; 64-127 for on
	RingMod          Control `cc:"53"` // "
	LFO2MIDISync     Control `cc:"54"` // "
	DelayMIDISync    Control `cc:"55"` // "
	LFO1Mode         Control `cc:"56"` // 0-41 for retrig; 42-83 for free; 84-127 for single
	LFO2Mode         Control `cc:"57"` // 0-41 for retrig; 42-83 for free; 84-127 for single
	ArpStatus        Control `cc:"58"` // 0-63 for off; 64-127 for on
	ArpOctave        Control `cc:"59"` // 0-31 for 1 oct; 32-63 for 2 oct; 64-95 for 3 oct; 96-127 for 4 oct
	ArpDirection     Control `cc:"60"` // 0-20 for forwards; 21-41 for backwards; 42-62 for pendulum; 63-83 for note forwards; 84-104 for note backwards; 105-127 for note pendulum
	ArpDivision      Control `cc:"61"` // 16 = 1/32nd 1/24th 1/16th 1/12th 1/8th 1/6th 1/4th or 1/2
	VeloDepth        Control `cc:"62"` // Centered
	NoteDepth        Control `cc:"63"` // Centered
	AftertouchDepth  Control `cc:"65"` // Centered
	ExtDepth         Control `cc:"66"` // Centered
	SequenceLength   Control `cc:"67"` // 0-31 for 1 bar; 32-63 for 2 bars; 64-95 for 4 bars; 96-127 for 8 bars
	SequenceHold     Control `cc:"70"` // 0-63 for off; 64-127 for on
	SequenceLoop     Control `cc:"71"` // 0 to set loop stop point; 127 to set loop start point
	Transpose        Control `cc:"75"` // From -24 to +36 sent as (value + 24) * 2
	Swing            Control `cc:"78"`
	//Anim 1 cc,,80,,0,127,,,,,0-based,CC number of new destination
	//Anim 2 cc,,81,,0,127,,,,,0-based,CC number of new destination
	//Anim 3 cc,,82,,0,127,,,,,0-based,CC number of new destination
	//Anim 4 cc,,83,,0,127,,,,,0-based,CC number of new destination
	AllEnvelopeAttack  Control `cc:"84"`  // 0-4 seconds
	AllEnvelopeDecay   Control `cc:"85"`  // 0-2 seconds
	AllEnvelopeSustain Control `cc:"86"`  // 0-1 seconds
	AllEnvelopeRelease Control `cc:"87"`  // 0-4 seconds
	ModSlot1Depth      Control `cc:"88"`  // Centered
	ModSlot2Depth      Control `cc:"89"`  // Centered
	ModSlot3Depth      Control `cc:"90"`  // Centered
	ModSlot4Depth      Control `cc:"91"`  // Centered
	ModSlot5Depth      Control `cc:"92"`  // Centered
	ModSlot6Depth      Control `cc:"93"`  // Centered
	ModSlot7Depth      Control `cc:"94"`  // Centered
	ModSlot8Depth      Control `cc:"95"`  // Centered
	ModWheelDepth      Control `cc:"96"`  // Centered
	ModSlot1Source     Control `cc:"101"` // 0,7
	ModSlot2Source     Control `cc:"102"` // 0,7
	ModSlot3Source     Control `cc:"103"` // 0,7
	ModSlot4Source     Control `cc:"104"` // 0,7
	ModSlot5Source     Control `cc:"105"` // 0,7
	ModSlot6Source     Control `cc:"106"` // 0,7
	ModSlot7Source     Control `cc:"107"` // 0,7
	ModSlot8Source     Control `cc:"108"` // 0,7
	ModSlot1Dest       Control `cc:"111"` // 0,36
	ModSlot2Dest       Control `cc:"112"` // 0,36
	ModSlot3Dest       Control `cc:"113"` // 0,36
	ModSlot4Dest       Control `cc:"114"` // 0,36
	ModSlot5Dest       Control `cc:"115"` // 0,36
	ModSlot6Dest       Control `cc:"116"` // 0,36
	ModSlot7Dest       Control `cc:"117"` // 0,36
	ModSlot8Dest       Control `cc:"118"` // 0,36
	// Randomise patch 121
}

// SoundController represents a general midi 2 sound controller.
type SoundController struct {
	SoundController1  Control `cc:"70"`
	SoundController2  Control `cc:"71"`
	SoundController3  Control `cc:"72"`
	SoundController4  Control `cc:"73"`
	SoundController5  Control `cc:"74"`
	SoundController6  Control `cc:"75"`
	SoundController7  Control `cc:"76"`
	SoundController8  Control `cc:"77"`
	SoundController9  Control `cc:"78"`
	SoundController10 Control `cc:"79"`
}

type GMController struct {
	BankSelect           Control `cc:"0"`
	Modulation           Control `cc:"1"`
	BreathController     Control `cc:"2"`
	FootController       Control `cc:"4"`
	ChannelVolume        Control `cc:"7"`
	ChannelBalance       Control `cc:"8"`
	Pan                  Control `cc:"10"`
	ExpressionController Control `cc:"11"`

	SoundVariation  Control `cc:"70"`
	FilterResonance Control `cc:"71"`
	ReleaseTime     Control `cc:"72"`
	AttackTime      Control `cc:"73"`
	Brightness      Control `cc:"74"`
	DecayTime       Control `cc:"75"`
	VibratoRate     Control `cc:"76"`
	VibratoDepth    Control `cc:"77"`
	VibratoDelay    Control `cc:"78"`

	ReverbSendLevel Control `cc:"91"`
	ChorusSendLevel Control `cc:"93"`
}

type WorldeEasyControl9 struct {
	SliderAB Control `cc:"9"`
	Slider1  Control `cc:"3"`
	Slider2  Control `cc:"4"`
	Slider3  Control `cc:"5"`
	Slider4  Control `cc:"6"`
	Slider5  Control `cc:"7"`
	Slider6  Control `cc:"8"`
	Slider7  Control `cc:"9"`
	Slider8  Control `cc:"10"`
	Slider9  Control `cc:"11"`

	Knob1 Control `cc:"14"`
	Knob2 Control `cc:"15"`
	Knob3 Control `cc:"16"`
	Knob4 Control `cc:"17"`
	Knob5 Control `cc:"18"`
	Knob6 Control `cc:"19"`
	Knob7 Control `cc:"20"`
	Knob8 Control `cc:"21"`
	Knob9 Control `cc:"22"`

	Button1 Control `cc:"23"`
	Button2 Control `cc:"24"`
	Button3 Control `cc:"25"`
	Button4 Control `cc:"26"`
	Button5 Control `cc:"27"`
	Button6 Control `cc:"28"`
	Button7 Control `cc:"29"`
	Button8 Control `cc:"30"`
	Button9 Control `cc:"31"`

	Repeat    Control `cc:"49"`
	Backwards Control `cc:"47"`
	Forwards  Control `cc:"48"`
	Stop      Control `cc:"46"`
	Play      Control `cc:"45"`
	Record    Control `cc:"44"`

	ButtonLeftProgramKnob  Control `cc:"67"`
	ButtonRightProgramKnob Control `cc:"64"`
}

type MidiMix struct {
	// Knob_row_column

	Knob1x1 Control `cc:"16"`
	Knob2x1 Control `cc:"17"`
	Knob3x1 Control `cc:"18"`

	Knob1x2 Control `cc:"20"`
	Knob2x2 Control `cc:"21"`
	Knob3x2 Control `cc:"22"`

	Knob1x3 Control `cc:"24"`
	Knob2x3 Control `cc:"25"`
	Knob3x3 Control `cc:"26"`

	Knob1x4 Control `cc:"28"`
	Knob2x4 Control `cc:"29"`
	Knob3x4 Control `cc:"30"`

	Knob1x5 Control `cc:"46"`
	Knob2x5 Control `cc:"47"`
	Knob3x5 Control `cc:"48"`

	Knob1x6 Control `cc:"50"`
	Knob2x6 Control `cc:"51"`
	Knob3x6 Control `cc:"52"`

	Knob1x7 Control `cc:"54"`
	Knob2x7 Control `cc:"55"`
	Knob3x7 Control `cc:"56"`

	Knob1x8 Control `cc:"58"`
	Knob2x8 Control `cc:"59"`
	Knob3x8 Control `cc:"60"`

	Slider1      Control `cc:"19"`
	Slider2      Control `cc:"23"`
	Slider3      Control `cc:"27"`
	Slider4      Control `cc:"31"`
	Slider5      Control `cc:"49"`
	Slider6      Control `cc:"53"`
	Slider7      Control `cc:"57"`
	Slider8      Control `cc:"61"`
	SliderMaster Control `cc:"62"`

	BankLeft  Control `note:"25"`
	BankRight Control `note:"26"`
	Solo      Control `note:"27"`

	Mute1 Control `note:"1"`
	Mute2 Control `note:"4"`
	Mute3 Control `note:"7"`
	Mute4 Control `note:"10"`
	Mute5 Control `note:"13"`
	Mute6 Control `note:"16"`
	Mute7 Control `note:"19"`
	Mute8 Control `note:"22"`

	RecArm1 Control `note:"3"`
	RecArm2 Control `note:"6"`
	RecArm3 Control `note:"9"`
	RecArm4 Control `note:"12"`
	RecArm5 Control `note:"15"`
	RecArm6 Control `note:"18"`
	RecArm7 Control `note:"21"`
	RecArm8 Control `note:"24"`
}

type Model struct {
	Model            string
	*GMController    `json:"GMController,omitempty"`
	*CraftSynth2     `json:"CraftSynth2,omitempty"`
	*MeeblipSE       `json:"MeeblipSE,omitempty"`
	*MeeblipTriode   `json:"MeeblipTriode,omitempty"`
	*Skulpt          `json:"Skulpt,omitempty"`
	*SoundController `json:"SoundController,omitempty"`
	*VolcaBass       `json:"VolcaBass,omitempty"`
	*VolcaBeats      `json:"VolcaBeats,omitempty"`
	*VolcaKeys       `json:"VolcaKeys,omitempty"`
	*VolcaKick       `json:"VolcaKick,omitempty"`
	*VolcaDrum       `json:"VolcaDrum,omitempty"`
	*UnoSynth        `json:"UnoSynth,omitempty"`
}

func (m *Model) MidiParams() interface{} {
	switch m.Model {
	case "Craft Synth 2":
		return m.CraftSynth2
	case "Meeblip SE":
		return m.MeeblipSE
	case "Meeblip Triode":
		return m.MeeblipTriode
	case "MidiMix":
		return &MidiMix{}
	case "Skulpt":
		return m.Skulpt
	case "Sound Controller":
		return m.SoundController
	case "Volca Bass":
		return m.VolcaBass
	case "Volca Beats":
		return m.VolcaBeats
	case "Volca Keys":
		return m.VolcaKeys
	case "Volca Kick":
		return m.VolcaKick
	case "Volca Drum":
		return m.VolcaDrum
	case "GM Controller":
		return m.GMController
	case "Uno Synth":
		return m.UnoSynth
	case "WorldeEasyControl9":
		return &WorldeEasyControl9{}
	default:
		panic("unknown model " + m.Model)
	}
}
