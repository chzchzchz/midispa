package cc

import "fmt"

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

type ProVSMini struct {
	Modulation       Control `cc:"1"`   // 0-127
	Portamento       Control `cc:"5"`   // 0-127
	VoiceAWave       Control `cc:"24"`  // 0-127
	VoiceBWave       Control `cc:"25"`  // 0-127
	VoiceCWave       Control `cc:"26"`  // 0-127
	VoiceDWave       Control `cc:"27"`  // 0-127
	LFO2Amount       Control `cc:"28"`  // 0-99  (Pitch)
	FilterEnvAmount  Control `cc:"47"`  // 0-127
	LFO1Waveform     Control `cc:"54"`  // 0-127 (triangle, square, saw)
	LFO2Waveform     Control `cc:"55"`  // 0-127 (triangle, square, saw)
	LFO1Destination  Control `cc:"56"`  // 0-127 (VCF OSC)
	LFO1Amount       Control `cc:"70"`  // 0-99  (VCF)
	FilterResonance  Control `cc:"71"`  // 0-99
	LFO1Rate         Control `cc:"72"`  // 0-99
	LFO2Rate         Control `cc:"73"`  // 0-99
	FilterCutoff     Control `cc:"74"`  // 0-99
	AmpEnvAttack     Control `cc:"81"`  // 0-99
	AmpEnvDecay      Control `cc:"82"`  // 0-99
	AmpEnvSustain    Control `cc:"83"`  // 0-99
	AmpEnvRelease    Control `cc:"84"`  // 0-99
	FilterEnvAttack  Control `cc:"85"`  // 0-99
	FilterEnvDecay   Control `cc:"86"`  // 0-99
	FilterEnvSustain Control `cc:"87"`  // 0-99
	FilterEnvRelease Control `cc:"88"`  // 0-99
	ChorusDepth      Control `cc:"91"`  // 0-99
	ChorusRate       Control `cc:"92"`  // 0-99
	VoiceAFine       Control `cc:"111"` // 0-99
	VoiceBFine       Control `cc:"112"` // 0-99
	VoiceCFine       Control `cc:"113"` // 0-99
	VoiceDFine       Control `cc:"114"` // 0-99
	VoiceACoarse     Control `cc:"115"` // 0-99
	VoiceBCoarse     Control `cc:"116"` // 0-99
	VoiceCCoarse     Control `cc:"117"` // 0-99
	VoiceDCoarse     Control `cc:"118"` // 0-99
}

// MicrokorgXL is the Korg microKORG XL. The instrument addresses its front
// panel controls two different ways, so this struct mixes both tags.
//
// cc:  a direct control change; one message reaches the parameter. Only the
// parameters the manual lists in "Front panel knob/button control change
// assignments" (p. 90) are reachable this way out of the box, and the numbers
// below are the factory CC MAP assignments. CC MAP is user editable
// (p. 61): any of CC#00-CC#95 and CC#102-CC#119 can be moved to a different
// parameter, so these numbers describe the default patch, not the hardware.
// A control the user has remapped away will not respond to the number here.
//
// nrpn: a non-registered parameter, addressed as "<msb>.<lsb>" in decimal.
// Reaching one takes three messages: CC#99 with the MSB, CC#98 with the LSB,
// then CC#6 data entry MSB with the value. The microKORG XL implements data
// entry MSB only, so every NRPN value is 0...127. Unlike CC MAP, the NRPN map
// is fixed in the firmware and cannot be remapped.
//
// Both mechanisms are transmitted and received on the global MIDI channel
// (p. 86), except that in MULTI voice mode timbre 2 uses its own channel.
//
// Several parameters are asymmetric: the value the instrument transmits when
// a knob moves is not always the value it accepts from a knob, so the two are
// noted separately where they differ.
//
// This is the microKORG XL, which is a different instrument from the
// microKORG modelled as MicroKorg below. The two share 32 of their control
// change numbers while meaning different parameters on each, so the numbers
// are not interchangeable between them.
type MicrokorgXL struct {
	// Unison stacks up to five detuned oscillators inside a single oscillator
	// (manual p. 35).
	UnisonMode Control `cc:"3"` // 0-31 OFF, 32-63 2 VOICE, 64-95 3 VOICE, 96-127 4 VOICE

	Portamento Control `cc:"5"` // 0 = no portamento, 127 = slowest glide

	// Oscillator 1 selects its algorithm with OSC1Mod and then gives that
	// algorithm a shape with OSC1Control1 and a second dimension with
	// OSC1Control2. What the two controls actually do depends on the
	// waveform and modulation type, so consult manual p. 37-40 before
	// interpreting a value.
	OSC1Wave     Control `cc:"8"`  // 0-15 SAW, 16-31 PULSE, 32-47 TRIANGLE, 48-63 SINE, 64-79 FORMANT, 80-95 NOISE, 96-111 PCM/DWGS, 112-127 AUDIO IN
	OSC1Mod      Control `cc:"9"`  // 0-31 WAVEFORM, 32-63 CROSS, 64-95 UNISON, 96-127 VPM
	OSC1Control1 Control `cc:"15"` // 0-127
	OSC1Control2 Control `cc:"17"` // Transmitted 0-127; received 0-127, or 1-32 mapped to 0-127 when OSC1Mod is VPM

	// Oscillator 2 can be ringed, synced or both against oscillator 1.
	OSC2Wave Control `cc:"18"` // 0-31 SAW, 32-63 PULSE, 64-95 TRIANGLE, 96-127 SINE
	OSC2Mod  Control `cc:"19"` // 0-31 OFF, 32-63 RING, 64-95 SYNC, 96-127 RING.SYNC
	// Semitone is -24...+24, spread over only 49 of the 128 steps; manual
	// p. 92 gives the value mapping.
	OSC2Semitone Control `cc:"20"`
	OSC2Tune     Control `cc:"21"` // 0-127, +/- 2 octaves relative to oscillator 1

	// The mixer feeds the filters, so a level here also sets how hard the
	// DRIVE waveshaper clips.
	OSC1Level  Control `cc:"23"`
	OSC2Level  Control `cc:"24"`
	NoiseLevel Control `cc:"25"`

	// Filter 1 is the continuously variable filter; filter 2 is a simpler
	// three-way filter that only participates when routing says so. Filter
	// 1's TypeBalance sweeps across the filter types rather than selecting
	// one; manual p. 92 maps 0-127 onto the LPF24...THRU continuum.
	Filter1Routing      Control `cc:"26"` // 0-31 SINGLE, 32-63 SERIAL, 64-95 PARALLEL, 96-127 INDIV
	Filter1TypeBalance  Control `cc:"27"`
	Filter1KeyTrack     Control `cc:"28"` // 0/1 -2, 64 0, 127 +2 octaves per key
	Filter1Cutoff       Control `cc:"74"`
	Filter1Resonance    Control `cc:"71"`
	Filter1EG1Intensity Control `cc:"79"` // 0/1 -63, 64 0, 127 +63

	Filter2Type         Control `cc:"29"` // 0-42 LPF, 43-83 HPF, 85-127 BPF
	Filter2Cutoff       Control `cc:"30"`
	Filter2KeyTrack     Control `cc:"82"` // 0/1 -2, 64 0, 127 +2 octaves per key
	Filter2Resonance    Control `cc:"68"`
	Filter2EG1Intensity Control `cc:"69"` // 0/1 -63, 64 0, 127 +63

	AmpLevel  Control `cc:"7"`  // Also the standard MIDI volume controller
	AmpPanpot Control `cc:"10"` // 0/1 far left, 64 centre, 127 far right

	// DriveWaveShapeDepth is the mix between the untouched signal and the
	// waveshaped one; which waveshaper is selected is an edit parameter that
	// has no CC.
	DriveWaveShapeDepth Control `cc:"83"`

	// EG1 is the filter EG, EG2 the amplifier EG. EG3 exists on the panel but
	// is not bound to a knob or button, so it has no CC here.
	EG1Attack  Control `cc:"85"`
	EG1Decay   Control `cc:"86"`
	EG1Sustain Control `cc:"87"`
	EG1Release Control `cc:"88"`

	EG2Attack  Control `cc:"73"`
	EG2Decay   Control `cc:"75"`
	EG2Sustain Control `cc:"70"`
	EG2Release Control `cc:"72"`

	LFO1Wave Control `cc:"89"` // 0-25 SAW, 26-50 SQUARE, 51-76 TRIANGLE, 77-101 S/H, 102-127 RANDOM
	// LFO1Freq doubles as the sync note when the LFO's BPM SYNC is on; the
	// two share one knob, so the same CC value means different things
	// depending on that setting (manual p. 93).
	LFO1Freq Control `cc:"90"`

	LFO2Wave Control `cc:"102"` // 0-25 SAW, 26-50 SQUARE, 51-76 SINE, 77-101 S/H, 102-127 RANDOM
	LFO2Freq Control `cc:"76"`  // See LFO1Freq for the BPM SYNC caveat

	// The six virtual patches. A patch is a source, a destination and an
	// intensity; only the intensity is bound to a knob, and a patch with
	// nothing assigned to it can still be driven over NRPN (see PatchNSource
	// and PatchNDest below).
	Patch1Intensity Control `cc:"103"` // 0/1 -63, 64 0, 127 +63
	Patch2Intensity Control `cc:"104"` // 0/1 -63, 64 0, 127 +63
	Patch3Intensity Control `cc:"105"` // 0/1 -63, 64 0, 127 +63
	Patch4Intensity Control `cc:"106"` // 0/1 -63, 64 0, 127 +63
	Patch5Intensity Control `cc:"107"` // 0/1 -63, 64 0, 127 +63
	Patch6Intensity Control `cc:"108"` // 0/1 -63, 64 0, 127 +63

	// Two-band shelving EQ, +/- 15 dB. The value mapping is non-linear at
	// the top end (manual p. 93).
	EQLowGain  Control `cc:"110"`
	EQHighGain Control `cc:"109"`

	// Each master effect has a dry/wet balance plus two control inputs whose
	// meaning depends on the effect type, so CTRL-1/2 are only meaningful
	// once the effect is known.
	FX1DryWet   Control `cc:"115"` // Transmitted 0-127; received 0 dry, 1-126 mixed, 127 wet
	FX1Control1 Control `cc:"12"`
	FX1Control2 Control `cc:"112"`
	FX2DryWet   Control `cc:"116"` // Transmitted 0-127; received 0 dry, 1-126 mixed, 127 wet
	FX2Control1 Control `cc:"13"`
	FX2Control2 Control `cc:"113"`

	// Arpeggiator, NRPN MSB 0 (manual p. 86). These are the front panel
	// buttons and knobs that CC MAP cannot cover.
	ArpOnOff        Control `nrpn:"0.2"`  // Transmitted 0 off / 127 on; received 0-63 off, 64-127 on
	ArpLatch        Control `nrpn:"0.4"`  // Transmitted 0 off / 127 on; received 0-63 off, 64-127 on
	ArpType         Control `nrpn:"0.7"`  // 0-21 UP, 22-42 DOWN, 43-63 ALT1, 64-85 ALT2, 86-106 RANDOM, 107-127 TRIGGER
	ArpGate         Control `nrpn:"0.10"` // Gate time; the 0-127 value is a compressed percent scale (manual p. 87)
	ArpTimbreSelect Control `nrpn:"0.11"` // 0-42 TIMBRE1, 43-85 TIMBRE2, 86-127 TIMBRE1+2

	// The six virtual patches again, now over NRPN, MSB 4 (manual p. 87).
	// Source and destination share one 0-127 scale; the ranges are listed in
	// the manual. The sources are EG1 0-10, EG2 11-20, EG3 21-31, LFO1
	// 32-42, LFO2 43-52, VELOCITY 53-63, PITCH BEND 64-74, MOD WHEEL
	// 75-84, KEY TRACK 85-95, MIDI1 96-106, MIDI2 107-116, MIDI3 117-127.
	//
	// NRPN 4.0 is this field in a synth program and the vocoder's
	// FC.MOD.SRC in a vocoder program, so it carries one name. The microKORG
	// XL manual states the address twice, once under "Controlling the Timbre
	// parameters" (p. 87) and once under "Controlling the vocoder
	// parameters" (p. 88), without noting that they collide. The original
	// microKORG's MIDI implementation is explicit: it heads a block of rows
	// with "(Synth Mode / Vocoder Mode)" and gives 04 00 the single row
	// "Patch1 Source/Fc Mod Source". The device therefore has one active
	// meaning per address, chosen by the loaded program, not two parameters
	// that can be set together.
	Patch1Source Control `nrpn:"4.0"`
	Patch2Source Control `nrpn:"4.1"`
	Patch3Source Control `nrpn:"4.2"`
	Patch4Source Control `nrpn:"4.3"`
	Patch5Source Control `nrpn:"4.4"`
	Patch6Source Control `nrpn:"4.5"`

	// Destinations are grouped in fours: Pitch 0-2, OSC2 Tune 3-5, OSC1
	// Control 1 6-9, OSC1 Level 10-12, OSC2 Level 13-15, Noise Level
	// 16-18, Filter 1 Type Balance 19-21, Filter 1 Cutoff 22-25, Filter 1
	// Resonance 26-28, Filter 2 Cutoff 29-31, Drive/WS Depth 32-34, AMP
	// Level 35-37, Panpot 38-41, LFO1 Frequency 42-44, LFO2 Frequency
	// 45-47, Portamento 48-50, OSC1 Control 2 51-53, Filter 1 EG1 Int
	// 54-57, Filter 1 Key Track 58-60, Filter 2 Resonance 61-63, Filter 2
	// EG1 Int 64-66, Filter 2 Key Track 67-69, EG1 ADSR 70-82, EG2 ADSR
	// 83-95, EG3 ADSR 96-108, Patch 1-6 intensity 109-127.
	Patch1Dest Control `nrpn:"4.8"`
	Patch2Dest Control `nrpn:"4.9"`
	Patch3Dest Control `nrpn:"4.10"`
	Patch4Dest Control `nrpn:"4.11"`
	Patch5Dest Control `nrpn:"4.12"`
	Patch6Dest Control `nrpn:"4.13"`

	// The voice mode decides how many timbres a program has and which MIDI
	// channel they live on (manual p. 32).
	VoiceMode Control `nrpn:"5.0"` // 0-31 SINGLE, 32-63 LAYER, 64-95 SPLIT, 96-127 MULTI

	// Vocoder on/off. The manual's summary table prints the LSB as 04(00)
	// while the prose above it gives the message as [Bn, 62, 04]; the prose
	// is used here.
	VocoderSwitch Control `nrpn:"5.4"` // Transmitted 0 off / 127 on; received 0-63 off, 64-127 on

	// The vocoder's sixteen carrier band-pass filters each have a level and a
	// pan, MSB 4, LSB 0x40...0x5F (manual p. 88). Sending a level or pan to
	// one microKORG XL makes it track on every other, so both units need the
	// same program loaded. These addresses are only live in a vocoder program;
	// a synth program ignores them, the same way the original microKORG leaves
	// its band rows blank on the synth side of that column.
	VocoderBandLevel1  Control `nrpn:"4.64"` // 0-127
	VocoderBandLevel2  Control `nrpn:"4.65"`
	VocoderBandLevel3  Control `nrpn:"4.66"`
	VocoderBandLevel4  Control `nrpn:"4.67"`
	VocoderBandLevel5  Control `nrpn:"4.68"`
	VocoderBandLevel6  Control `nrpn:"4.69"`
	VocoderBandLevel7  Control `nrpn:"4.70"`
	VocoderBandLevel8  Control `nrpn:"4.71"`
	VocoderBandLevel9  Control `nrpn:"4.72"`
	VocoderBandLevel10 Control `nrpn:"4.73"`
	VocoderBandLevel11 Control `nrpn:"4.74"`
	VocoderBandLevel12 Control `nrpn:"4.75"`
	VocoderBandLevel13 Control `nrpn:"4.76"`
	VocoderBandLevel14 Control `nrpn:"4.77"`
	VocoderBandLevel15 Control `nrpn:"4.78"`
	VocoderBandLevel16 Control `nrpn:"4.79"`

	VocoderBandPan1  Control `nrpn:"4.80"` // 0/1 far left, 64 centre, 127 far right
	VocoderBandPan2  Control `nrpn:"4.81"`
	VocoderBandPan3  Control `nrpn:"4.82"`
	VocoderBandPan4  Control `nrpn:"4.83"`
	VocoderBandPan5  Control `nrpn:"4.84"`
	VocoderBandPan6  Control `nrpn:"4.85"`
	VocoderBandPan7  Control `nrpn:"4.86"`
	VocoderBandPan8  Control `nrpn:"4.87"`
	VocoderBandPan9  Control `nrpn:"4.88"`
	VocoderBandPan10 Control `nrpn:"4.89"`
	VocoderBandPan11 Control `nrpn:"4.90"`
	VocoderBandPan12 Control `nrpn:"4.91"`
	VocoderBandPan13 Control `nrpn:"4.92"`
	VocoderBandPan14 Control `nrpn:"4.93"`
	VocoderBandPan15 Control `nrpn:"4.94"`
	VocoderBandPan16 Control `nrpn:"4.95"`
}

// MicroKorg describes the control change surface of the Korg microKORG: the
// patch parameters that can be reached over MIDI control changes, bound to the
// control change numbers the instrument ships with from the factory (manual
// p.56, "Messages transmitted and received by the microKORG").
//
// Three properties of the instrument explain most of the field comments below,
// and they are the difference between a model that sounds right and one that
// lands every edit on the wrong parameter.
//
// The parameter to control change binding is a setting rather than a constant.
// SHIFT > CONTROL CHANGE rebinds any parameter here to a different number in
// 0...95, so these numbers record a factory default instead of a guarantee.
// GLOBAL > MIDI > "MIDI Filter" > CONTROL CHANGE additionally has to be set to
// Enable, or the instrument ignores all of them.
//
// Transmitted and received values are encoded differently. Moving a knob makes
// the microKORG send the rest position of a stepped parameter on a few sparse
// 7-bit anchors: OSC 1 WAVE transmits 0, 18, 36, 54, 72, 90, 108 and 126. A
// control change sent into the instrument instead spreads the steps evenly
// across 0...127, so the same waveform arrives as 0...15, 16...31, 32...47 and
// so on. Values captured in the transmitted scheme do not address the received
// scheme correctly, so sequencing this instrument has to speak the received
// scheme, which is what the field comments below record.
//
// Bipolar parameters are encoded centre-zero rather than signed: 0 and 1 are
// -63, 2 is -62, rising through 63 as -1, 64 as 0 and 127 as +63. The same
// encoding covers filter envelope intensity, keyboard tracking and the
// modulation effect rate.
//
// Where a parameter means something different in a vocoder program than in a
// synth program, both meanings are noted, because the same control change
// reaches whichever kind of program happens to be selected.
//
// This is the microKORG, not the microKORG XL, which is modelled separately as
// MicrokorgXL above. The two instruments share 32 of their control change
// numbers while meaning different parameters on each: control change 23 is this
// instrument's filter envelope attack and the XL's oscillator 1 level, and
// control change 18 is this one's oscillator 2 semitone and the XL's
// oscillator 2 waveform. Despite the similar field names, the numbers are not
// interchangeable.
type MicroKorg struct {
	// PITCH

	// Portamento is the glide time between notes. 0...127; the manual gives
	// the control range without the time range it covers.
	Portamento Control `cc:"5"`

	// OSC 1

	// Osc1Wave selects the oscillator 1 waveform, the patch's main source of
	// character. Transmitted: 0 Saw, 18 Square, 36 Tri, 54 Sin, 72 Vox Wave,
	// 90 DWGS, 108 Noise, 126 Audio In. Received: 0...15 Saw, 16...31 Square,
	// 32...47 Tri, 48...63 Sin, 64...79 Vox Wave, 80...95 DWGS, 96...111
	// Noise, 112...127 Audio In.
	Osc1Wave Control `cc:"77"`

	// Osc1Control1 morphs the shape selected by Osc1Wave within a single
	// family: it sets pulse width on the square wave and progressively
	// reshapes the other waveforms towards the next family along. 0...127,
	// and it has no effect at all while Osc1Wave is DWGS.
	Osc1Control1 Control `cc:"14"`

	// Osc1Control2 is the modulation of Osc1Control1 by LFO 1, which is what
	// turns a static shape into a PWM-like sweep. 0...127. While Osc1Wave is
	// DWGS it means something else entirely: it selects DWGS waveform 1...64,
	// two received values per waveform, so waveform n sits at 2n-2 and 2n-1
	// and the oscillator is only 64 steps deep rather than 128.
	Osc1Control2 Control `cc:"15"`

	// OSC 2

	// Osc2Wave selects the oscillator 2 waveform. Transmitted: 0 Saw, 64
	// Squ, 127 Tri. Received: 0...42 Saw, 43...85 Squ, 86...127 Tri. The
	// three transmitted anchors are a poor fit for three received bands,
	// which is a good illustration of the encoding split above.
	Osc2Wave Control `cc:"78"`

	// Osc2OscMod chooses how the two oscillators interact.
	// Transmitted: 0 OFF, 43 Ring, 85 Sync, 127 RingSync. Received:
	// 0...31 OFF, 32...63 Ring, 64...95 Sync, 96...127 RingSync. Ring and
	// RingSync are the only two ways to get the classic Korg metallic
	// cross-modulated tone out of this instrument.
	Osc2OscMod Control `cc:"82"`

	// Osc2Semitone detunes oscillator 2 against oscillator 1, from -24 to
	// +24 semitones. The received scale spends three values per semitone:
	// 0...2 is -24, 63...65 is unison, 126 and 127 are +24. In a vocoder
	// program the same control change drives HPF Level instead.
	Osc2Semitone Control `cc:"18"`

	// AUDIO IN 1

	// AudioIn1Tune trims the input on the AUDIO IN 1 jack, which feeds the
	// oscillator ring as a modulation source. Centre-zero over -63...+63. In
	// a vocoder program it drives the Threshold of the envelope follower
	// that detects the modulator signal, so it decides how loudly the
	// vocoder responds to speech.
	AudioIn1Tune Control `cc:"19"`

	// MIXER

	// Osc1Level sets the level of oscillator 1. 0...127.
	Osc1Level Control `cc:"20"`

	// Osc2Level sets the level of oscillator 2. 0...127. In a vocoder
	// program the same control change is labelled "Inst Level", which does
	// not match either carrier level the vocoder record names, so what it
	// drives there is not stated by the manual.
	Osc2Level Control `cc:"21"`

	// NoiseLevel sets the level of the noise generator, which is also one of
	// the destinations a virtual patch can modulate. 0...127.
	NoiseLevel Control `cc:"22"`

	// FILTER

	// FilterType selects the filter response, which is a large part of what
	// the patch sounds like. Synth: transmitted 0 -24LPF, 43 -12LPF, 85
	// -12BPF, 127 -12HPF; received 0...31 -24LPF, 32...63 -12LPF, 64...95
	// -12BPF, 96...127 -12HPF. In a vocoder program it becomes Formant
	// Shift: received 0...25 is 0, 26...51 is +1, 52...76 is +2, 77...102 is
	// -1 and 103...127 is -2, transposed onto the carrier bandpass filters
	// to make the voice sound taller or shorter.
	FilterType Control `cc:"83"`

	// Cutoff is the filter cutoff frequency, the single most important
	// parameter on a subtractive voice. Synth: 0...127. Vocoder: centre-zero
	// over -63...+63, offsetting every synthesis bandpass filter at once.
	Cutoff Control `cc:"74"`

	// Resonance boosts the cutoff frequency itself rather than any harmonic,
	// which is what gives a patch its acidic edge. 0...127.
	Resonance Control `cc:"71"`

	// FilterEgInt is how far the filter envelope swings the cutoff, negative
	// values pulling it down for a percussive decay and positive values
	// pushing it up for a swelling filter. Centre-zero over -63...+63. In a
	// vocoder program it becomes Mod Int, the depth of the modulator
	// envelope applied to the carrier.
	FilterEgInt Control `cc:"79"`

	// KbdTrack is keyboard tracking, the amount by which the cutoff follows
	// the note you play, so that high notes stay bright across the range.
	// Centre-zero over -63...+63. In a vocoder program it becomes E.F.Sense,
	// the sensitivity of the analysis filter to the modulator, which is
	// plain 0...127 there and whose top value freezes the detected spectrum.
	KbdTrack Control `cc:"85"`

	// F.EG, the filter envelope. The microKORG always runs it as a decaying
	// envelope, so Sustain is the level the cutoff rests at once the decay
	// finishes rather than a hold gate.

	// FilterEgAttack is the filter envelope attack time. 0...127.
	FilterEgAttack Control `cc:"23"`

	// FilterEgDecay is the filter envelope decay time. 0...127.
	FilterEgDecay Control `cc:"24"`

	// FilterEgSustain is the level the filter envelope settles to. 0...127.
	FilterEgSustain Control `cc:"25"`

	// FilterEgRelease is the filter envelope release time. 0...127.
	FilterEgRelease Control `cc:"26"`

	// AMP, the amplifier that all of the above is summed into.

	// AmpLevel is the output level of the amplifier, plain 0...127 in both
	// program kinds: in a synth program it is the level itself, and in a
	// vocoder program the same control change is the vocoder's own output
	// level. The pan encoding belongs to Panpot, not to this.
	AmpLevel Control `cc:"7"`

	// Panpot is the stereo position of the output, and the patch's only pan
	// control as well as a destination for the virtual patches. Synth:
	// centre-zero over L63...R63, with 0 and 1 both L63 and 64 centre.
	// Vocoder: plain 0...127, where it is the direct level of the carrier
	// that bypasses the vocoder.
	Panpot Control `cc:"10"`

	// Distortion is the amp stage's drive. Transmitted 0 OFF and 127 ON;
	// received 0...63 OFF and 64...127 ON, so the switch arrives on the
	// lower half of the received range rather than at the top.
	Distortion Control `cc:"92"`

	// A.EG, the amplifier envelope.

	// AmpEgAttack is the amplifier envelope attack time. 0...127.
	AmpEgAttack Control `cc:"73"`

	// AmpEgDecay is the amplifier envelope decay time. 0...127.
	AmpEgDecay Control `cc:"75"`

	// AmpEgSustain is the level the amplifier envelope settles to, which
	// is the loudest the voice will play. 0...127.
	AmpEgSustain Control `cc:"70"`

	// AmpEgRelease is the amplifier envelope release time. 0...127.
	AmpEgRelease Control `cc:"72"`

	// LFO 1, which the manual also nominates as the modulator for the
	// dedicated vibrato, tremolo and wah amounts.

	// Lfo1Wave selects the LFO 1 waveform. Transmitted: 0 Saw, 43 Squ1,
	// 85 Tri, 127 S/H. Received: 0...31 Saw, 32...63 Squ1, 64...95 Tri,
	// 96...127 S/H. Sample and hold is what makes LFO 1 usable as a
	// stepped random source rather than a smooth sweep.
	Lfo1Wave Control `cc:"87"`

	// Lfo1Frequency sets the LFO 1 rate. 0...127, or a synced note
	// division when LFO TEMPO SYNC is on, in which case the received scale
	// runs 0...8 as 1/1, 9...17 as 3/4, 18...25 as 2/3 and so on down to
	// 120...127 as 1/32.
	Lfo1Frequency Control `cc:"27"`

	// LFO 2, the modulator the manual nominates for pitch vibrato.

	// Lfo2Wave selects the LFO 2 waveform. Transmitted: 0 Saw, 43 Squ2,
	// 85 Sin, 127 S/H. Received: 0...31 Saw, 32...63 Squ2, 64...95 Sin,
	// 96...127 S/H. LFO 2 has a sine where LFO 1 has a triangle.
	Lfo2Wave Control `cc:"88"`

	// Lfo2Frequency sets the LFO 2 rate, and doubles as a virtual patch
	// destination. 0...127, or a synced note division with the same scale as
	// Lfo1Frequency when LFO TEMPO SYNC is on.
	Lfo2Frequency Control `cc:"76"`

	// PATCH 1 to PATCH 4, the four virtual patch routes. Each picks a
	// modulation source and a destination, and the intensities below set
	// how far the source drives it. The source and destination selections
	// are not control changes at all; they are made with the front panel
	// knobs and live outside this model, so only the depth of each route is
	// reachable from here.

	// Patch1Intensity is the depth of virtual patch 1. Centre-zero over
	// -63...+63, and the sign is what decides whether the source lifts or
	// drops the destination, which is how two patches are set up to fight
	// each other.
	Patch1Intensity Control `cc:"28"`

	// Patch2Intensity is the depth of virtual patch 2. Centre-zero over
	// -63...+63.
	Patch2Intensity Control `cc:"29"`

	// Patch3Intensity is the depth of virtual patch 3. Centre-zero over
	// -63...+63.
	Patch3Intensity Control `cc:"30"`

	// Patch4Intensity is the depth of virtual patch 4. Centre-zero over
	// -63...+63.
	Patch4Intensity Control `cc:"31"`

	// MOD FX, the modulation effect, which is where the chorus, flange and
	// ensemble of the three effect types live.

	// ModFxLfoSpeed is the rate of the modulation effect's internal LFO,
	// and is the knob the manual describes as changing timbre over time
	// rather than pitch. Centre-zero over -63...+63.
	ModFxLfoSpeed Control `cc:"12"`

	// ModFxDepth is how deep the modulation effect runs. 0...127.
	ModFxDepth Control `cc:"93"`

	// DELAY, downstream of the modulation effect.

	// DelayTime is the delay time, 0...127, or a synced note division when
	// DELAY TEMPO SYNC is on. The synced scale runs the opposite way to the
	// LFOs: 0...8 is 1/32, 9...17 is 1/24, up to 120...127 as 1/1.
	DelayTime Control `cc:"13"`

	// DelayDepth is the delay output level against the dry signal.
	// 0...127.
	DelayDepth Control `cc:"94"`

	// MIDI section. These are perform controls rather than patch
	// parameters, but they are the two remaining entries in the manual's
	// control change table and both sit on otherwise unused numbers.

	// TimbreSelect switches the vocoder between its two carrier
	// timbres, or between the two synchronised carriers together.
	// Transmitted: 0 Timbre1, 1 Timbre1&2 (Sync), 127 Timbre2. Received:
	// 0 Timbre1, 1 Timbre1&2 (Sync), 2...127 Timbre2, so unlike every other
	// switch here it is a three-state control with a wide third state.
	TimbreSelect Control `cc:"95"`

	// SyncCtrl starts and stops the arpeggiator in sync with the external
	// clock. Transmitted 0 OFF and 127 ON; received 0...63 OFF and
	// 64...127 ON.
	SyncCtrl Control `cc:"90"`
}

// Microkorg2 is the Korg microKORG2 synthesizer/vocoder: the parameters it
// exposes over MIDI, bound to the control change and non-registered parameter
// numbers the instrument answers to.
//
// Source: "microKORG2 Owner's Manual", KORG INC., document En 1a, carrying a
// MIDI Implementation Chart dated 2024.5.15, Ver: 1.0.0 (microKORG2_OM_En1.pdf).
// Every page number below is that manual's own printed number, which is also
// how the chart on p. 133 is numbered.
//
// cc:  a direct control change; one message reaches the parameter. The
// numbering is the "Control Change" row of the implementation chart (p. 133),
// and each of the body sections repeats the same numbers against its own
// parameters (OSC 1 on p. 56, MIXER on p. 61, FILTER on p. 63, the envelopes
// on p. 64-66, the LFOs on p. 68-69, the effects on p. 98-110 and the EQ on
// p. 112). The two agree throughout, so a chart transcription slip would
// show up as a disagreement with the body.
//
// The manual documents no way to rebind these numbers on this instrument,
// which is worth saying because it is not true of either sibling: the
// microKORG moves any parameter under SHIFT > CONTROL CHANGE and the
// microKORG XL under its CC MAP, so the numbers recorded for those two models
// describe a factory default rather than the hardware. Nothing in the
// microKORG2 manual suggests an equivalent, but the absence of a procedure is
// weaker evidence than a statement that the map is fixed, so these are still
// only what the instrument answers out of the box.
//
// Reception is gated. Every row of the chart's control change block carries the
// *C flag, which its notes tie to GLOBAL > MIDI FILTER, and the manual's own
// wording is that the "Control Change (CC)" setting must be Enable or the
// instrument neither transmits nor receives any of them (p. 87).
//
// Timbre 1's parameters travel on the Global Ch channel and timbre 2's on the
// channel set as Timbre 2 Ch (p. 84); the chart's note *1 says the same. A Dual
// program therefore has two independent sets of these addresses and a message
// sent on one channel does not reach the other.
//
// nrpn: a non-registered parameter, addressed as "<msb>.<lsb>" in decimal,
// the form the manual itself writes them in ("NRPN 4, 0...5"). Reaching one
// takes three messages: CC#99 with the MSB, CC#98 with the LSB, then CC#6
// data entry MSB with the value. The chart lists those three as "98, 99" for
// NRPN (LSB, MSB) and "6, 38" for data entry (MSB, LSB) (p. 133), which is the
// MIDI standard order. The instrument also implements data entry LSB, but
// nothing below needs more than the 0...127 a data entry MSB carries.
//
// The General MIDI plumbing the chart lists alongside them is deliberately
// absent: bank select MSB and LSB (CC#0 and CC#32), data entry MSB and LSB
// (CC#6 and CC#38) and the NRPN selectors themselves (CC#98 and CC#99). None
// of them sets a parameter, and a model that claimed them would make the tail
// of an NRPN's select-and-set look like an edit of the sound.
//
// Unlike the original microKORG, this manual documents no split between the
// values the instrument transmits when a knob moves and the values it accepts
// as a control change. The ranges in the field comments are therefore the
// manual's own parameter ranges and nothing more, with one exception: the
// filter's type anchors, which the manual does pin to specific values
// (p. 62).
//
// Several parameters a reader might expect here have no address, and in every
// case it is the manual that lists none rather than the chart claiming a
// number it does not mean. Unison Number, which decides whether Unison Detune
// and Unison Spread do anything at all (p. 52); OSC 3's OSC Mod Type, the
// switch that turns ring, sync and VPM on in the first place (p. 58); each
// oscillator's Ratio/Fixed keytrack (p. 60); each LFO's Mode and Key Sync
// (p. 67, p. 69); the virtual patches' Connect on/off (p. 71); the effect
// types, which are what give the effect control fields below their meaning
// (p. 97, p. 105, p. 108); and the arpeggiator's ON button and tempo, the
// latter set by tapping or by the incoming MIDI clock rather than by a
// parameter message (p. 74).
type Microkorg2 struct {
	// The chart pairs CC#7 and CC#10 under the single name "Timbre Level"
	// and describes neither parameter anywhere in its body text. They are
	// kept apart here because these are the MIDI standard's own numbers for
	// volume and pan, which is what makes the pairing readable: 7 is the
	// timbre's level and 10 its stereo position.
	TimbreLevel Control `cc:"7"`
	TimbrePan   Control `cc:"10"`

	// VOICE. The unison stack is up to eight voices of one note, and
	// UnisonDetune and UnisonSpread are inert while it is off.
	UnisonDetune Control `cc:"33"` // Parameter range 0...127 (manual p. 52)
	UnisonSpread Control `cc:"34"` // Parameter range 0...127 (manual p. 52)

	// PITCH. Transpose and Fine Tune are shared by all three oscillators
	// (manual p. 53), which is what lets a patch set one detune for the
	// whole voice rather than per oscillator.
	//
	// The manual's PITCH section calls these Transpose and Fine Tune, while
	// its PATCH 1-6 destination list offers TIMBRE P.Time and TIMBRE Pitch
	// as destinations (p. 129). It does not say how those four names pair
	// up, so the two spellings are not reconciled here.
	PortamentoTime Control `cc:"5"`  // Parameter range 0...127, 0 = no portamento (manual p. 53)
	PortamentoMode Control `cc:"65"` // Fingered, Always (manual p. 53)
	Transpose      Control `cc:"35"` // Parameter range -24...0...+24 semitones (manual p. 53)
	FineTune       Control `cc:"36"` // Parameter range -50...0...+50 cents (manual p. 53)

	// OSC 1. Shape is the parameter that gives the chosen waveform its
	// character, and what it means depends entirely on the waveform: a
	// morph within the Saw-to-Sine family for the four basic waves, but a
	// sample selector once DWGS or OneShot is chosen (manual p. 56).
	OSC1Wave      Control `cc:"8"`  // Saw, Square, Triangle, Sine, DWGS, OneShot (manual p. 55)
	OSC1Shape     Control `cc:"9"`  // Saw-Sine 0...127; DWGS 1...64; OneShot 1...32 (manual p. 56)
	OSC1ModAmount Control `cc:"15"` // Off, then 1...127 of oscillator 3's modulation (manual p. 56)
	OSC1Semitones Control `cc:"16"` // Parameter range -24...0...+24 semitones (manual p. 56)
	OSC1FineTune  Control `cc:"17"` // Parameter range -50...0...+50 cents (manual p. 56)

	// OSC 2. Same parameters as oscillator 1, and the manual says so
	// rather than repeating their descriptions (p. 57).
	OSC2Wave      Control `cc:"18"` // See OSC1Wave
	OSC2Shape     Control `cc:"19"` // See OSC1Shape
	OSC2ModAmount Control `cc:"20"` // Off, then 1...127 (manual p. 57)
	OSC2Semitones Control `cc:"21"` // Parameter range -24...0...+24 semitones (manual p. 57)
	OSC2FineTune  Control `cc:"22"` // Parameter range -50...0...+50 cents (manual p. 57)

	// OSC 3 is the modulator: oscillator 3 modulates oscillators 1 and 2
	// and those two output the modulated result (manual p. 58). Turning
	// that modulation on is OSC 3's Mod Type parameter, which is ring, sync,
	// ring plus sync or VPM, and which has no address of its own, so
	// OSC1ModAmount and OSC2ModAmount only do something once it is set.
	OSC3Wave      Control `cc:"48"` // See OSC1Wave (manual p. 58)
	OSC3Shape     Control `cc:"49"` // See OSC1Shape (manual p. 58)
	OSC3Semitones Control `cc:"51"` // Parameter range -24...0...+24 semitones (manual p. 59)
	OSC3FineTune  Control `cc:"52"` // Parameter range -50...0...+50 cents (manual p. 59)

	// NOISE. NoiseColor is the noise generator's own filter rather than a
	// timbre colour control, and which filter it adjusts is decided by
	// NoiseType: a low pass or high pass cutoff, a band pass peak, or a
	// sample rate under the decimator (manual p. 60).
	NoiseType  Control `cc:"29"` // LPF, HPF, BPF, Deci (manual p. 60)
	NoiseColor Control `cc:"30"` // Parameter range 0...127, meaning set by NoiseType (manual p. 60)

	// MIXER. These are the input levels to the filter, so a level set here
	// also sets how hard the filter and its drive stage work (manual p. 61).
	OSC1Level  Control `cc:"23"` // Parameter range 0...127
	OSC2Level  Control `cc:"24"` // Parameter range 0...127
	OSC3Level  Control `cc:"25"` // Parameter range 0...127
	NoiseLevel Control `cc:"26"` // Parameter range 0...127

	// FILTER. Type is the one parameter here whose 7-bit values the manual
	// pins down, and it is a morph rather than a selector: the four named
	// types sit at 0, 32, 64, 96 and 127 and the values between them are
	// mixtures of the types either side, so there is no way to have pure
	// 24 dB/octave filtering without also having some of the next type in it
	// (manual p. 62).
	FilterType     Control `cc:"27"` // 0 LP4 (-24 dB/oct LPF), 32 LP2 (-12 dB/oct LPF), 64 BP2 (-12 dB/oct BPF), 96 HP2 (-12 dB/oct HPF), 127 HP4 (-24 dB/oct HPF)
	Cutoff         Control `cc:"74"` // Parameter range 0...127 (manual p. 63)
	Resonance      Control `cc:"71"` // Parameter range 0...127 (manual p. 63)
	FilterDrive    Control `cc:"83"` // Parameter range 0...127 (manual p. 63)
	FilterKeytrack Control `cc:"28"` // Parameter range -200.0...0.0...200.0 % of the pitch change, measured from C4 (manual p. 63)

	// AMP EG. The velocity sensitivity is on this envelope rather than on
	// the filter, so a patch can follow how hard a key is played in level
	// while the timbre stays fixed.
	AmpEgAttack   Control `cc:"73"` // Parameter range 0...127 (manual p. 64)
	AmpEgDecay    Control `cc:"75"` // Parameter range 0...127 (manual p. 64)
	AmpEgSustain  Control `cc:"70"` // Parameter range 0...127 (manual p. 64)
	AmpEgRelease  Control `cc:"72"` // Parameter range 0...127 (manual p. 64)
	AmpEgVelocity Control `cc:"79"` // Parameter range 0...127 (manual p. 64)

	// FILTER EG. Sustain is the cutoff frequency held once the decay has
	// run out, so it is a cutoff offset rather than a level (p. 65).
	FilterEgAttack    Control `cc:"85"` // Parameter range 0...127 (manual p. 65)
	FilterEgDecay     Control `cc:"86"` // Parameter range 0...127 (manual p. 65)
	FilterEgSustain   Control `cc:"87"` // Cutoff frequency held after the decay (manual p. 65)
	FilterEgRelease   Control `cc:"88"` // Parameter range 0...127 (manual p. 65)
	FilterEgIntensity Control `cc:"84"` // Parameter range -63...0...63 (manual p. 66)

	// LFO 1. Frequency does double duty as the sync division, because the
	// LFO's Mode parameter has no address and is what decides which of the
	// two the same control change means: in Tempo mode the manual lists ten
	// note values from 1/1 down to 1/32, and in Free or One Shot mode it is
	// a plain 0...127 rate (p. 68).
	LFO1Wave      Control `cc:"89"` // Triangle, Saw Down, Saw Up, Square, Sample & Hold (manual p. 67)
	LFO1Frequency Control `cc:"90"` // Tempo mode: 1/1 ... 1/32; Free or One Shot: 0...127 (manual p. 68)
	LFO1Smooth    Control `cc:"91"` // Parameter range 0...127 (manual p. 68)

	// LFO 2. Where LFO 1 has a Smooth control, LFO 2 has a Delay that holds
	// the LFO off for a moment after its phase resets, which is how a patch
	// gets an LFO that restarts on every note without the modulation
	// jumping straight back to full depth (manual p. 69).
	LFO2Wave      Control `cc:"102"` // See LFO1Wave (manual p. 69)
	LFO2Frequency Control `cc:"76"`  // See LFO1Frequency for the two modes (manual p. 69)
	LFO2Delay     Control `cc:"92"`  // Parameter range 0...127 (manual p. 69)

	// PATCH 1-6. Each virtual patch is two modulation sources, a
	// destination and an intensity, and the intensity is the only part of
	// the route reachable over a control change; the sources and
	// destination are over NRPN, below. The manual's parameter range is
	// -63...0...+63, and the sign is what decides whether the source lifts
	// or drops the destination, so two patches set to opposite intensities
	// against one destination work against each other (manual p. 71).
	Patch1Intensity Control `cc:"103"` // Parameter range -63...0...+63 (manual p. 71)
	Patch2Intensity Control `cc:"104"` // Parameter range -63...0...+63
	Patch3Intensity Control `cc:"105"` // Parameter range -63...0...+63
	Patch4Intensity Control `cc:"106"` // Parameter range -63...0...+63
	Patch5Intensity Control `cc:"107"` // Parameter range -63...0...+63
	Patch6Intensity Control `cc:"108"` // Parameter range -63...0...+63

	// MOD effect. The three controls below are the ones the effect's own
	// page binds to a knob after the effect type and its sub type, so none
	// of them has a fixed meaning: ModEffectControl1 is Speed for the four
	// modulation types and for Tremolo, Wow Depth for LoFi, Time for the
	// compressor and Tone for both distortion types and the amp simulator;
	// ModEffectControl2 is Depth everywhere except LoFi, where it is the
	// isolator's Intensity, and the compressor, where it is the threshold;
	// ModEffectControl3 exists only for LoFi, Distortion, Comp and Amp
	// Simulator, as Saturation or as the output level or mix. The four
	// modulation types and Tremolo have no third control at all, their
	// remaining parameters living on the MOD EXTRA page (manual p. 97-104).
	ModEffectControl1 Control `cc:"12"`
	ModEffectControl2 Control `cc:"111"`
	ModEffectControl3 Control `cc:"112"`

	// DELAY effect. The first control is the effect's type-specific
	// parameter, which the manual lists only for four of the six delay
	// types: Ping Pong's Width, Tape Echo's Instability, Pitch Shift's mix
	// between the shifted and unshifted signal, and LoRes's sample rate
	// reduction. Stereo and Reverse have none. Time then splits between an
	// absolute 0...127 and, when the delay's BPM Sync is on, a set of
	// fifteen tempo subdivisions from 1/64 to 1/1 (manual p. 106-107).
	DelayEffectControl1 Control `cc:"115"` // Meaning set by the delay type
	DelayEffectControl2 Control `cc:"13"`  // Time: 0...127, or 1/64...1/1 when BPM synced
	DelayEffectControl3 Control `cc:"113"` // Input level, 0...127
	DelayEffectControl4 Control `cc:"114"` // High pass cutoff, 0...127

	// REVERB effect. As with the delay, the first control is the
	// type-specific one, and again only some types have one: Rust's Age,
	// Pitch Shift's shifted signal mix and LoRes's sample rate reduction.
	// Hall, Room and Spring have none. The manual lists Pitch Shift's mix
	// as -63...63 rather than the -63...0...+63 it uses elsewhere, which
	// reads as the same bipolar control without the centre marked (p. 109).
	ReverbEffectControl1 Control `cc:"118"` // Meaning set by the reverb type
	ReverbEffectControl2 Control `cc:"14"`  // Time, 0...127
	ReverbEffectControl3 Control `cc:"116"` // Input trim, 0...127
	ReverbEffectControl4 Control `cc:"117"` // Damping filter cutoff, 0...127

	// EQ. A two-band equalizer, and the last stage of the effect chain, so
	// it colours the effects as well as the dry sound. The output feedback
	// feeds the EQ's own output back into its input and is a bipolar
	// control, which is how this instrument makes a resonant sound out of
	// the equalizer alone (manual p. 112).
	EQLowFrequency   Control `cc:"95"`  // Parameter range 40...1000 Hz
	EQLowGain        Control `cc:"110"` // Parameter range -63...0...+63
	EQHighFrequency  Control `cc:"94"`  // Parameter range 1.0...18.0 kHz
	EQHighGain       Control `cc:"109"` // Parameter range -63...0...+63
	EQOutputFeedback Control `cc:"93"`  // Parameter range -63...0...+63; negative inverts the phase fed back

	// Performance controls. The mod wheel has no destination of its own:
	// it is a virtual patch source in its own right, so a patch has to
	// name Mod.W as a source for the wheel to do anything (manual p. 70).
	//
	// The damper pedal's job is set by a global setting rather than by the
	// program, and one of that setting's four positions turns the pedal
	// into the loop recorder's record control instead of a damper
	// (manual p. 82). The instrument has no half damper.
	ModulationWheel Control `cc:"1"`
	DamperPedal     Control `cc:"64"`

	// The arpeggiator, NRPN MSB 0. Its ON button and its tempo are not
	// here: the manual gives them no address, and the tempo is set by
	// tapping or by the incoming MIDI clock (p. 74). Everything else the
	// arpeggiator page exposes is below, including the latch that keeps it
	// running after the keyboard is released.
	ArpSwing        Control `nrpn:"0.5"`  // Parameter range -100%...0...+100% (manual p. 76)
	ArpResolution   Control `nrpn:"0.6"`  // 1/32, 1/24, 1/16, 1/12, 1/8, 1/6, 1/4 (manual p. 76)
	ArpTargetTimbre Control `nrpn:"0.11"` // Both Timbre, Timbre 1, Timbre 2; only live in Dual mode (manual p. 77)
	ArpLatch        Control `nrpn:"0.4"`  // Off, On (manual p. 77)
	ArpType         Control `nrpn:"0.7"`  // Up, Down, UpDown, DownUp, Converge, Diverge, Manual, Random 1, Random 2, Trigger (manual p. 77-78)
	ArpOctave       Control `nrpn:"0.8"`  // 1...4 octaves (manual p. 78)
	ArpGate         Control `nrpn:"0.10"` // Parameter range 0%...100% (manual p. 79)
	ArpLastStep     Control `nrpn:"0.9"`  // 1...8 steps (manual p. 79)
	ArpKeySync      Control `nrpn:"0.12"` // Off, On (manual p. 79)

	// The virtual patches' sources and destinations, NRPN MSB 4. Each of
	// the six patches has three separate addresses, laid out as three runs
	// of six: Source 1 at LSB 0...5, Source 2 at 16...21 and Destination at
	// 32...37 (manual p. 70-71).
	//
	// Both sources and the destination share one 0-127 selection scale.
	// The source scale runs 0 NoAssign, 1 Velocity, 2 KbdTrk, 3 Pitch Bend,
	// 4 Mod.W, 5 Flt EG, 6 Amp EG, 7 LFO1, 8 LFO2, 9 Noise, 10 Analog, and
	// the destination scale runs 0 NoAssign through 75 EQ Feedback
	// (manual p. 129). The two sources are multiplied rather than summed,
	// so assigning the same source to both gives an exponential curve, and
	// leaving one of them as NoAssign is how a single source is used
	// (p. 70).
	Patch1Source  Control `nrpn:"4.0"`
	Patch2Source  Control `nrpn:"4.1"`
	Patch3Source  Control `nrpn:"4.2"`
	Patch4Source  Control `nrpn:"4.3"`
	Patch5Source  Control `nrpn:"4.4"`
	Patch6Source  Control `nrpn:"4.5"`
	Patch1Source2 Control `nrpn:"4.16"`
	Patch2Source2 Control `nrpn:"4.17"`
	Patch3Source2 Control `nrpn:"4.18"`
	Patch4Source2 Control `nrpn:"4.19"`
	Patch5Source2 Control `nrpn:"4.20"`
	Patch6Source2 Control `nrpn:"4.21"`
	Patch1Dest    Control `nrpn:"4.32"`
	Patch2Dest    Control `nrpn:"4.33"`
	Patch3Dest    Control `nrpn:"4.34"`
	Patch4Dest    Control `nrpn:"4.35"`
	Patch5Dest    Control `nrpn:"4.36"`
	Patch6Dest    Control `nrpn:"4.37"`

	// The vocoder, NRPN MSB 5. The instrument's own synthesis runs as the
	// carrier while the mic input is analysed as the modulator, so these
	// are mostly about how the sixteen band-pass filters sound once the
	// detected spectrum has been mapped onto them.
	VocoderMicDirect       Control `nrpn:"5.1"` // Mic input level bypassing the modulator, 0...127 (manual p. 90)
	VocoderSynthDryWet     Control `nrpn:"5.2"` // 0...100% balance between the synth sound and the vocoder (manual p. 90)
	VocoderFormant         Control `nrpn:"5.3"` // Parameter range -63...0...+63, shifting every carrier band at once (manual p. 90)
	VocoderResonance       Control `nrpn:"5.4"` // Resonance of every carrier band, 0...127 (manual p. 90)
	VocoderEnvFollowerSens Control `nrpn:"5.5"` // Parameter range 0...126, plus a topmost Hold that freezes the detected spectrum (manual p. 90)

	// The sixteen carrier bands' levels and pans, one NRPN per band at each
	// of two consecutive runs of sixteen (manual p. 91). Band Select, which
	// picks which band the panel's knobs are editing, is explicitly not
	// saved in the program and resets to band 1 at power on, so the address
	// reaches a specific band directly rather than going through it.
	VocoderBandLevel1  Control `nrpn:"5.16"` // 0...127
	VocoderBandLevel2  Control `nrpn:"5.17"`
	VocoderBandLevel3  Control `nrpn:"5.18"`
	VocoderBandLevel4  Control `nrpn:"5.19"`
	VocoderBandLevel5  Control `nrpn:"5.20"`
	VocoderBandLevel6  Control `nrpn:"5.21"`
	VocoderBandLevel7  Control `nrpn:"5.22"`
	VocoderBandLevel8  Control `nrpn:"5.23"`
	VocoderBandLevel9  Control `nrpn:"5.24"`
	VocoderBandLevel10 Control `nrpn:"5.25"`
	VocoderBandLevel11 Control `nrpn:"5.26"`
	VocoderBandLevel12 Control `nrpn:"5.27"`
	VocoderBandLevel13 Control `nrpn:"5.28"`
	VocoderBandLevel14 Control `nrpn:"5.29"`
	VocoderBandLevel15 Control `nrpn:"5.30"`
	VocoderBandLevel16 Control `nrpn:"5.31"`

	VocoderBandPan1  Control `nrpn:"5.32"` // L63...L1, Centre, R1...R63
	VocoderBandPan2  Control `nrpn:"5.33"`
	VocoderBandPan3  Control `nrpn:"5.34"`
	VocoderBandPan4  Control `nrpn:"5.35"`
	VocoderBandPan5  Control `nrpn:"5.36"`
	VocoderBandPan6  Control `nrpn:"5.37"`
	VocoderBandPan7  Control `nrpn:"5.38"`
	VocoderBandPan8  Control `nrpn:"5.39"`
	VocoderBandPan9  Control `nrpn:"5.40"`
	VocoderBandPan10 Control `nrpn:"5.41"`
	VocoderBandPan11 Control `nrpn:"5.42"`
	VocoderBandPan12 Control `nrpn:"5.43"`
	VocoderBandPan13 Control `nrpn:"5.44"`
	VocoderBandPan14 Control `nrpn:"5.45"`
	VocoderBandPan15 Control `nrpn:"5.46"`
	VocoderBandPan16 Control `nrpn:"5.47"`

	// HARD TUNE, NRPN MSB 6. A single part that snaps the vocal pitch to
	// the nearest scale note or played key, so Intensity decides how much
	// of the correction is heard and Speed decides how quickly it settles.
	// The harmonizer and hard tune can be used together, but the vocoder
	// cannot be used with either (manual p. 90).
	HardTuneIntensity Control `nrpn:"6.1"` // 0...127 (manual p. 92)
	HardTuneSpeed     Control `nrpn:"6.2"` // 0 corrects instantly, larger values slow it (manual p. 92)
	HardTuneFormant   Control `nrpn:"6.3"` // Parameter range -63...0...+63 (manual p. 92)

	// HARMONIZER, NRPN MSB 7. Up to two harmony voices are added to the
	// vocal, and the level, spread, formant, detune and delay below apply to
	// the harmonies as a group while the two pitches below are per voice.
	HarmonizerLevel        Control `nrpn:"7.1"`  // Harmony output level, 0...127 (manual p. 93)
	HarmonizerStereoSpread Control `nrpn:"7.2"`  // Parameter range -63...0...+63 (manual p. 93)
	HarmonizerFormant      Control `nrpn:"7.3"`  // Parameter range -63...0...+63 (manual p. 93)
	HarmonizerDetune       Control `nrpn:"7.4"`  // Parameter range -63...0...+63 (manual p. 93)
	HarmonizerDelay        Control `nrpn:"7.5"`  // Parameter range 0...150 mSec behind the vocal (manual p. 93)
	HarmonizerNumber       Control `nrpn:"7.16"` // 1 or 2 harmonies (manual p. 94)
	HarmonizerPitch1       Control `nrpn:"7.32"` // -2 Oct...Unison...+2 Oct in scale steps (manual p. 94)
	HarmonizerPitch2       Control `nrpn:"7.48"` // -2 Oct...Unison...+2 Oct in scale steps; live only with 2 harmonies (manual p. 94)

	// LOOP RECORDER, NRPN MSB 8. The stutter settings are the recorder's
	// own parameters, not the arpeggiator's, and the offset shifts the
	// stutter's loop point by up to one measure. The manual's item
	// numbering on this page skips from 3 to 5, because the Count Level it
	// numbers 4 gives no address, so the last run here is play level only.
	LoopStutter       Control `nrpn:"8.16"` // Off, On Forward, On Reverse (manual p. 116)
	LoopStutterLength Control `nrpn:"8.17"` // 1/1, 1/2, 1/4, 1/6, 1/8, 1/12, 1/16, 1/24, 1/32, 1/64, 1/128 (manual p. 116)
	LoopStutterOffset Control `nrpn:"8.18"` // Parameter range -63...0...+63; reset to zero while the stutter is off (manual p. 116)
	LoopPlayLevel     Control `nrpn:"8.19"` // 0...127 (manual p. 116)
}

// PerformVE is the TC-Helicon Perform-VE vocal manipulator. The unit answers
// Control Change, Program Change and MIDI Tempo (Appendix B, manual p. 27) but
// never sends MIDI itself, so unlike the microKORG XL there is no separate
// transmit map to describe here. Presets are Program Change 0, 1 and 2, and
// incoming MIDI Tempo retunes the Tap Tempo, but neither is a continuous
// control, so neither has a field here.
//
// Most fields belong to one of the six effect processors (DOUBLE, MORPH,
// HARDTUNE, XFX, ECHO, FILTER) or to the LOOPER and SAMPLE sections, so the
// fields are grouped that way instead of sorted by controller number. The few
// that belong to none of them, the mod wheel, the top mix, the shared
// envelope and the sustain pedal, sit at the ends. The controller numbers come
// from the "MIDI CC List" of Appendix B, the parameter behaviour from
// section 5, and the style enumerations from Appendix A. Where those three
// pages disagree, the disagreement is noted on the field.
//
// Three things a reader might expect here are absent, each for a reason. The
// Control Knob has no CC of its own: it edits the top mix, which the device
// addresses as two, see TopMixLeadLevel below. The LED ring is the display
// for whichever parameter is being edited, so it has no address. And the
// unit's own settings, the mic gain, the MIDI channel and the split point,
// are reached by holding SET and playing a note rather than by a CC
// (p. 7, p. 24).
type PerformVE struct {
	// CC 1 is the standard MIDI modulation wheel, which the manual names
	// "Vibrato (Mod Wheel)" in Appendix B. Along with the sustain pedal at
	// the end of this struct, it is one of only two controllers here that are
	// a MIDI standard rather than a Perform-VE specific parameter.
	Vibrato Control `cc:"1"` // 0-127

	// Top mix: the balance between the processed singer, the LEAD voice, and
	// the up to eight note-triggered MIDI Voices (manual p. 6). It is the
	// Control Knob's default job, applies to all three presets, and is not
	// stored when the unit powers down. Appendix B notes that LEAD is "set
	// independent of MIDI via CC" and MIDI is "set independent of LEAD via
	// CC", which is the decoupling the unit's single knob does not have.
	TopMixLeadLevel Control `cc:"41"` // 0-127
	TopMixMidiLevel Control `cc:"42"` // 0-127

	// DOUBLE, on the LEAD voice. Simulates the "double tracked" studio
	// vocal that is common across genres (manual section 5.1, p. 9).
	DoubleStyle  Control `cc:"17"` // 0 UNISON, 1 OCTAVE DOWN, 2 OCTAVE UP, 3 OCTAVE UP/DOWN
	DoubleLevel  Control `cc:"45"` // 0-127, printed on the unit as "Off, -10 to 0 dB"
	DoubleEnable Control `cc:"51"` // 0-63 OFF, 64-127 ON

	// MORPH is the one processor with a split personality (manual p. 9):
	// Shift and Gender act on the LEAD voice while Mode and Style act on
	// the MIDI Voices. Switching MORPH on replaces the MIDI Voices' plain
	// notes harmony with whatever Style selects, so Mode and Style only
	// become audible once MorphEnable is on.
	//
	// Mode is the mono/poly switch for the Notes, Vocoder and Sample
	// voices. It reads as one 26 position knob made of two halves: the
	// first 13 positions are Poly and set the note release time, the last
	// 13 are Mono and set the portamento time. The manual shows this as a
	// green half and a red half of the LED ring (p. 11), and Appendix B
	// states the split as 0-12 for Poly Release and 13-25 for Mono
	// Portamento. Poly voices need the singer to keep voicing after the
	// note release to hear the fade out.
	MorphMode Control `cc:"23"` // 0-12 POLY RELEASE, 13-25 MONO PORTAMENTO

	// The eleven positions are the two harmony treatments and the nine
	// vocoder patches. Appendix A (p. 26) enumerates 1 Natural Shift, 2 Warp
	// Shift and 3 to 11 as the vocoder patches, which matches the 0-10 range
	// in Appendix B, but section 5.2 (p. 9) instead says there are "eight
	// different Synth Vocoder styles". The two pages disagree by one and the
	// longer Appendix A list is the one used here. Section 5.2 names only
	// the first two; the patches are described as synthesizer presets "named
	// accordingly" in the unit's own display, so their individual names are
	// not reproducible from the manual.
	MorphStyle Control `cc:"24"` // 0 NATURAL SHIFT, 1 WARP SHIFT, 2-10 ANALOG SYNTH MODELED VOCODER

	// Shift transposes the LEAD voice. The knob on the unit only reaches
	// plus or minus 12 semitones, but the manual calls out that MIDI widens
	// this to plus or minus 36 "for even more extreme effects and automated
	// sweeps" (p. 11). The 0-72 working range is linear with 36 as centre,
	// so 0 is -36, 36 is 0 and 72 is +36 semitones.
	MorphShift Control `cc:"43"` // 0 = -36, 36 = 0, 72 = +36 semitones

	// Gender expands (positive) or compresses (negative) the formant
	// signature of the LEAD voice, which is what makes it read as more
	// female or more male (p. 11).
	MorphGender Control `cc:"44"` // 0-127

	// Notes Voice Smoothing appears only in Appendix B; section 5 never
	// describes it, so what it smooths and over what range is undocumented.
	// It is grouped with MORPH because the "Notes" in its name matches the
	// notes-harmony voices that Mode and Style select, but the manual does
	// not confirm that is what it acts on.
	NotesVoiceSmoothing Control `cc:"26"` // 0-127

	MorphEnable Control `cc:"52"` // 0-63 OFF, 64-127 ON

	// HARDTUNE, on the LEAD voice. Pitch correction that varies from subtle
	// to T-Pain style (manual section 5.3, p. 12).
	//
	// Key picks the scale to correct toward. The manual describes it as
	// "NaturalPlay Pop Major Scale, Pop Major Scale in all 12 keys, or
	// Chromatic scale" (p. 12), which is 1 + 12 + 1 = 14 positions and
	// matches the 0-13 range in Appendix B. That sentence is the only place
	// the enumeration appears, so the order below is the natural reading of
	// it rather than something the manual states outright.
	HardTuneKey Control `cc:"19"` // 0 NATURALPLAY, 1-12 POP MAJOR IN C..B, 13 CHROMATIC

	// NaturalPlay is the entry that makes the unit track the key itself
	// from the chords in the incoming MIDI or on the AUX input, instead of
	// holding a key the user picked. Getting it right matters most for the
	// Pop Major scale, which looks odd once transposed away from C, so the
	// unit has to be set to the composition's real key.
	//
	// Amount scales how hard correction pulls the voice in; the unit prints
	// it as "Natural to Slammed!".
	HardTuneAmount Control `cc:"20"` // 0-127

	HardTuneEnable Control `cc:"53"` // 0-63 OFF, 64-127 ON

	// XFX, short for "EXTREME EFFECTS" (manual section 5.4, p. 13). It is the
	// only processor that reaches beyond the vocal: Flange, SideChain
	// Pumping and LPF/HPF also act on the looper's recorded audio, and
	// Flange and LPF/HPF additionally reach the looper's drums.
	//
	// Style is a mini-preset that resets a batch of internal parameters
	// which have no CC of their own, and the two Mods then act as the
	// "tweaks" matched to that style (manual p. 6). Neither Mod has a
	// meaning of its own: for the rhythmic styles Mod1 is the division and
	// Mod2 the level or depth, but for SideChain Pumping they are the
	// compressor threshold and release time instead. Appendix A (p. 26)
	// lists the mapping for each style.
	XFXStyle Control `cc:"16"` // 0 STUTTER, 1 MONO CHOPPER, 2 STEREO CHOPPER, 3 RING MOD, 4 STEREO FLANGER, 5 MONO FLANGER, 6 SIDE CHAIN PUMPING
	XFXMod1  Control `cc:"21"` // 0-127, meaning set by XFXStyle
	XFXMod2  Control `cc:"22"` // 0-127, meaning set by XFXStyle

	// Selecting Stutter arms it to sample the LEAD voice immediately, and
	// turning XFX off puts the stutter on hold without losing that sample
	// (p. 13). Ring Mod multiplies the voice by an internal sine wave, and
	// SideChain Pumping compresses everything but the drums whenever the
	// kick crosses a threshold, which is why it is meant to be used with
	// the looper's built-in drum sequencer (p. 15).
	XFXEnable Control `cc:"54"` // 0-63 OFF, 64-127 ON

	// ECHO, on the LEAD vocal and the MIDI Voices. A combined delay and
	// reverb processor: either on its own, or both together (manual
	// section 5.5, p. 16).
	//
	// Style selects the reverb, and is the one style list the manual names
	// in prose: two natural reverbs, Hall and Arena, and two
	// electromechanical ones, Spring and Plate (p. 16). The order below
	// follows the order that prose lists them in; the LED ring's own
	// ordering is not documented. The range is 0-3 even though Appendix B
	// annotates the row "See Style List", because that note covers every
	// style parameter in the unit rather than this one in particular.
	EchoStyle Control `cc:"28"` // 0 HALL, 1 ARENA, 2 SPRING, 3 PLATE

	// Div is one knob doing two jobs at once: it picks the delay's rhythmic
	// division and its stereo placement (p. 16). The divisions themselves
	// exist in the manual only as a picture of the LED ring, so the 0-12
	// range is taken from Appendix B and the individual values cannot be
	// recovered from the text. What the text does pin down is the placement
	// per colour: blue positions ping-pong the echoes between left and
	// right, green positions play them on both sides, a single red position
	// is a short single-repeat slapback, and a white LED marks a straight
	// quarter note.
	EchoDiv Control `cc:"27"` // 0-12

	// Delay and Reverb are master parameters rather than single controls.
	// Delay edits the feedback and the output level together, so raising it
	// adds both level and repeat count, and at 100% the delay feeds back
	// forever. Reverb edits the decay time and the output level together,
	// so raising it makes the reverb sound bigger rather than simply
	// louder (p. 16).
	Delay  Control `cc:"46"` // 0-127
	Reverb Control `cc:"47"` // 0-127

	EchoEnable Control `cc:"55"` // 0-63 OFF, 64-127 ON

	// FILTER, on the LEAD vocal and the MIDI Voices. Everything that
	// equalises, filters or distorts the voice (manual section 5.6, p. 17).
	//
	// Style 0 is a plain sweepable low pass/high pass pair for shaping
	// musical audio, and it is the only Filter style that reaches the
	// looper's drums. The other four are transducer emulations that colour
	// the signal as though it came through a particular amp or enclosure
	// (Appendix A, p. 26), and those leave the drums alone.
	FilterStyle Control `cc:"18"` // 0 LPF/HPF, 1 RADIO, 2 MEGAPHONE, 3 GUITAR AMP, 4 DISTORTION

	// Mod is the transducer's cutoff frequency for the four transducer
	// styles, and the cutoff of whichever of the low pass or high pass
	// filters is active under the LPF/HPF style (p. 17). The transducer
	// styles rework the EQ hard enough to feed back more readily than the
	// other effects, and the manual singles out Megaphone as the one to
	// watch in front of a loud PA.
	FilterMod Control `cc:"48"` // 0-127

	FilterEnable Control `cc:"56"` // 0-63 OFF, 64-127 ON

	// SAMPLE. VS Synthesis, short for Vocal SampleSynthesis: record a short
	// phrase and play it back at the pitch of incoming MIDI notes, with the
	// nuance of the performance intact (manual section 7, p. 22).
	//
	// Mode is how the recording responds to note input. Re-trigger starts
	// the sample over on every new note, Legato syncs each new note to the
	// one already playing, and Looped keeps the sample going past its end
	// while the key is held (p. 22, Appendix A p. 26).
	SampleMode Control `cc:"25"` // 0 RE-TRIGGER, 1 LEGATO, 2 LOOPED RE-TRIGGER, 3 LOOPED LEGATO

	// Enable turns the sample-backed voices on, and the two switches arm
	// recording and start playback of the recording as it was performed.
	// Enabling SAMPLE also changes how the vocoder runs: the recording
	// becomes the modulator for up to eight synth voices and the lead
	// vocal passes through to the rest of the chain, instead of the lead
	// vocal acting as the modulator (p. 10).
	SampleEnable Control `cc:"80"` // 0-63 OFF, 64-127 ON
	SampleRecord Control `cc:"58"` // 0-63 OFF, 64-127 ON
	SamplePlay   Control `cc:"59"` // 0-63 OFF, 64-127 ON

	// LOOPER drum triggers. The looper acts as both a drum machine and an
	// audio looper (manual section 6, p. 19), but only the three drum
	// triggers are addressable. Arming, recording, playing back and erasing
	// the audio loop are front panel button gestures, and the manual's
	// looper cookbook (p. 21) describes them as presses with no CC
	// equivalent.
	//
	// These are edge triggers, not levels, and they are the one place in
	// this struct where the threshold is 0 against everything above rather
	// than 64: every positive value fires the drum, not just 127. The kick
	// and snare can be sounded at any time regardless of looper activity,
	// but the manual notes the hi-hat only triggers while the looper button
	// is lit or pulsing (p. 19). Selecting a different snare sample selects
	// a matching hi-hat alongside it (p. 19).
	LooperKickTrigger  Control `cc:"81"` // 0 OFF, 1-127 ON
	LooperSnareTrigger Control `cc:"82"` // 0 OFF, 1-127 ON
	LooperHiHatTrigger Control `cc:"83"` // 0 OFF, 1-127 ON

	// Appendix B names these only "Envelope Release" and "Envelope Attack".
	// The manual gives them no section, no range, and never says what they
	// act on. They sit after the LOOPER triggers because the drum samples are
	// the only envelopes the unit is described as having anywhere in the
	// manual, but that is an inference, not something the manual states. The
	// table also lists release before attack, which is why the field order
	// here follows attack first.
	EnvelopeAttack  Control `cc:"73"` // 0-127
	EnvelopeRelease Control `cc:"72"` // 0-127

	// CC 64 is the MIDI standard sustain pedal. The manual lists it in the
	// CC table but never says what the unit holds while it is down.
	SustainPedal Control `cc:"64"` // 0-63 OFF, 64-127 ON
}

// MiniNova is the Novation MiniNova, a 16-voice analogue polysynth. The
// parameter map below is transcribed from Novation's "MiniNova Extended CC &
// NRPN Map" (version 0002).
//
// The instrument exposes almost everything over non-registered parameters and
// reserves direct control changes for the handful of parameters the front
// panel can reach, so this struct mixes both tags.
//
// cc:   a direct control change; one message reaches the parameter. These are
// the Global-mode front panel assignments. They are fixed in firmware and
// cannot be remapped.
//
// The chart also lists the General MIDI plumbing under controller numbers of
// its own, and none of it is a patch parameter: data entry MSB and LSB
// (CC#6 and CC#38), data increment and decrement (CC#96 and CC#97), NRPN MSB
// and LSB (CC#98 and CC#99), and the channel mode messages all sounds off,
// local control and all notes off (CC#120, CC#122, CC#123). Three of those
// silence the instrument outright and the rest drive the select-and-set an
// NRPN is written with rather than the sound, so a generated patch that
// randomized them would either mute the MiniNova or corrupt the parameter
// selection it was midway through. They are left out for that reason. The
// NRPN path does not depend on the omitted fields: it writes CC#99, CC#98 and
// CC#6 from its own controller constants, so every nrpn field below still
// transmits.
//
// nrpn: a non-registered parameter, addressed as "<msb>.<lsb>" in decimal.
// Reaching one takes three messages: CC#99 with the MSB, CC#98 with the LSB,
// then CC#6 data entry MSB with the value. The chart's header note states
// that the instrument is driven with data entry MSB, so every NRPN value is
// 0...127 and the mapped range of a field can exceed that: TempoRate lists
// 40-240 BPM but no single data byte reaches past 127.
//
// Two chart conventions are worth calling out because they look like typos
// and are not:
//
//   - Category, Genre, the patch and bank commands and every Global-mode
//     parameter print a number in the CC column that is identical to their
//     NRPN MSB. That number is the MSB restated, not a data channel: taking
//     it literally would address CC#64 (sustain pedal) for all thirteen
//     Global parameters and CC#63 (filter 1 drive) for the patch commands.
//     They are tagged nrpn here for that reason.
//
// The chart also gives several unrelated parameters the same NRPN address.
// The hardware cannot separate those by value the way it separates the switch
// block below, so on a unit with all of them live a write to one address
// changes more than the field it names. They are kept as printed rather than
// renumbered, because the addresses are what the instrument listens on:
//
//   - NRPN 0.0 to 0.7 is both the eight AnimateNHold fields and the first
//     eight Envelope2 parameters, and NRPN 0.16 is both AnimateHoldButton and
//     Env3Decay.
//   - NRPN 1.123 is the three LFO fade modes, the two vocal tune fields and
//     ModMatrix9Source1.
//
// One address is a deliberate block rather than a collision: NRPN 0.122
// carries thirty switches, told apart only by the value written to it, and
// the ranges are on the individual fields. NRPN 1.123 mixes that scheme with
// the collisions above.
//
// The address maps behind MidiControls hold one field per address, so a
// lookup by address returns only one of any group. The ordered field and
// message sequences still cover every field, which is what a patch needs.
type MiniNova struct {
	Modulation           Control `cc:"1"`
	BreathController     Control `cc:"2"` // Received only; routed to the expression pedal
	PolyphonyMode        Control `cc:"3"` // 0 MONO, 1 MONO AG, 2 POLY 1, 3 POLY 2, 4 MONO 2
	PortamentoRate       Control `cc:"5"`
	WetLevel             Control `cc:"8"`
	PreGlide             Control `cc:"9"`  // 52 = -12 semitones, 64 = none, 76 = +12 semitones
	PanPosition          Control `cc:"10"` // 0 far left, 64 centre, 127 far right
	ExpressionController Control `cc:"11"`
	PortamentoMode       Control `cc:"12"` // 0 exponential, 1 linear
	KeyboardOctave       Control `cc:"13"` // 0 = 0 octaves, 1-4 = +1..+4, 124-127 = -4..-1
	Unison               Control `cc:"14"` // 0 off, then the number of stacked voices
	UnisonDetune         Control `cc:"15"`
	Drift                Control `cc:"16"`
	Phase                Control `cc:"17"` // 0-119 in 3 degree steps up to 357 degrees, 120 free
	FixedTranspose       Control `cc:"18"`
	SustainPedal         Control `cc:"64"` // 0-63 off, 64-127 on

	// Osc1Wave and Osc2Wave/Osc3Wave index the chart's 72-entry waveform
	// table, whose first entries are sine, triangle, sawtooth and a family of
	// pulse-width saws. The table also holds the two audio-input waveforms at
	// 70 and 71.
	Osc1Wave             Control `cc:"19"` // 0-71, waveform table index
	Osc1WaveInterpolate  Control `cc:"20"`
	Osc1PulseWidthIndex  Control `cc:"21"`
	Osc1VirtualSyncDepth Control `cc:"22"`
	Osc1Hardness         Control `cc:"23"`
	Osc1Density          Control `cc:"24"`
	Osc1DensityDetune    Control `cc:"25"`
	Osc1Semitones        Control `cc:"26"` // 64 = 0 semitones, mapped onto -64..+63
	Osc1Cents            Control `cc:"27"` // 64 = 0 cents, mapped onto -64..+63
	Osc1PitchBend        Control `cc:"28"` // 52 = -12 semitones, 76 = +12 semitones

	Osc2Wave             Control `cc:"29"` // 0-71, waveform table index
	Osc2WaveInterpolate  Control `cc:"30"`
	Osc2PulseWidthIndex  Control `cc:"31"`
	BankLSB              Control `cc:"32"` // Only banks 1-3 exist: 1 A, 2 B, 3 C
	Osc2VirtualSyncDepth Control `cc:"33"`
	Osc2Hardness         Control `cc:"34"`
	Osc2Density          Control `cc:"35"`
	Osc2DensityDetune    Control `cc:"36"`
	Osc2Semitones        Control `cc:"37"` // 64 = 0 semitones, mapped onto -64..+63
	Osc2Cents            Control `cc:"39"` // 64 = 0 cents, mapped onto -64..+63
	Osc2PitchBend        Control `cc:"40"` // 52 = -12 semitones, 76 = +12 semitones

	Osc3Wave             Control `cc:"41"` // 0-71, waveform table index
	Osc3WaveInterpolate  Control `cc:"42"`
	Osc3PulseWidthIndex  Control `cc:"43"`
	Osc3VirtualSyncDepth Control `cc:"44"`
	Osc3Hardness         Control `cc:"45"`
	Osc3Density          Control `cc:"46"`
	Osc3DensityDetune    Control `cc:"47"`
	Osc3Semitones        Control `cc:"48"` // 64 = 0 semitones, mapped onto -64..+63
	Osc3Cents            Control `cc:"49"` // 64 = 0 cents, mapped onto -64..+63
	Osc3PitchBend        Control `cc:"50"` // 52 = -12 semitones, 76 = +12 semitones

	Osc1Level      Control `cc:"51"`
	Osc2Level      Control `cc:"52"`
	Osc3Level      Control `cc:"53"`
	RingModLevel13 Control `cc:"54"` // Oscillator 1 ring-modulated by oscillator 3
	RingModLevel23 Control `cc:"55"` // Oscillator 2 ring-modulated by oscillator 3
	NoiseLevel     Control `cc:"56"`
	NoiseColour    Control `cc:"57"` // 0 white, 1 high, 2 band, 3 high band
	PreFXLevel     Control `cc:"58"` // 52 = -12 dB, 82 = +18 dB
	PostFXLevel    Control `cc:"59"` // 52 = -12 dB, 82 = +18 dB

	FilterRouting     Control `cc:"60"` // 0 bypass, 1 single, 2 series, 3 parallel, 4 parallel 2, 5 drum
	FilterBalance     Control `cc:"61"` // 64 = 0, mapped onto -64..+63
	Filter1Drive      Control `cc:"63"`
	Filter1DriveType  Control `cc:"65"` // 0 diode, 1 valve, 2 clipper, 3 crossover, 4 rectify, 5 bits down, 6 rate down
	Filter1Type       Control `cc:"68"` // 0-13, from 6 dB/octave low pass to 24 dB/octave high pass
	Filter1Track      Control `cc:"69"`
	Filter1Resonance  Control `cc:"71"`
	Filter1Frequency  Control `cc:"74"`
	Filter1QNormalise Control `cc:"78"`
	Filter1Env2ToFreq Control `cc:"79"` // 64 = 0, mapped onto -64..+63

	Filter2Drive      Control `cc:"80"`
	Filter2DriveType  Control `cc:"81"` // See Filter1DriveType
	Filter2Type       Control `cc:"82"` // See Filter1Type
	Filter2Frequency  Control `cc:"83"`
	Filter2Track      Control `cc:"84"`
	Filter2Resonance  Control `cc:"85"`
	Filter2QNormalise Control `cc:"86"`
	Filter2Env2ToFreq Control `cc:"87"` // 64 = 0, mapped onto -64..+63

	VibratoSpeed Control `cc:"76"`
	VibratoDepth Control `cc:"77"`
	PanRate      Control `cc:"88"`
	PanSync      Control `cc:"89"` // 0-35, sync division table
	PanModDepth  Control `cc:"90"`

	// The five effect returns. What a level does depends on the effect
	// selected in Fx1Select and friends.
	Fx1Level Control `cc:"91"`
	Fx2Level Control `cc:"92"`
	Fx3Level Control `cc:"93"`
	Fx4Level Control `cc:"94"`
	Fx5Level Control `cc:"95"`

	// Envelope 1 is the amplifier envelope and envelope 2 the filter
	// envelope. EnvelopeTrackCentre applies to all six.
	EnvelopeTrackCentre Control `cc:"106"`
	Env1Sustain         Control `cc:"70"`
	Env1Release         Control `cc:"72"`
	Env1Attack          Control `cc:"73"`
	Env1Decay           Control `cc:"75"`
	Env1Velocity        Control `cc:"108"` // 64 = 0, mapped onto -64..+63
	Env1SustainRate     Control `cc:"109"` // 64 = 0, mapped onto -64..+63
	Env1SustainTime     Control `cc:"110"`
	Env1ADRepeats       Control `cc:"111"`
	Env1AttackTrack     Control `cc:"112"` // 64 = 0, mapped onto -64..+63
	Env1DecayTrack      Control `cc:"113"` // 64 = 0, mapped onto -64..+63
	Env1LevelTrack      Control `cc:"114"` // 64 = 0, mapped onto -64..+63
	Env1AttackSlope     Control `cc:"115"`
	Env1DecaySlope      Control `cc:"116"`
	Env1AnimTrigger     Control `cc:"117"` // 0-8, retrigger and enable an animate source

	// The two fields the save menu offers. Both are listed with a CC that
	// repeats their NRPN MSB; CC#2 is the breath controller elsewhere in the
	// same chart, so only the NRPN address is usable.
	PatchCategory Control `nrpn:"2.64"` // 0 none, 1 arp, 2 bass, 3 bell, 4 classic, 5 drum, 6 keyboard,
	PatchGenre    Control `nrpn:"2.65"` // 0 none, 1 classic, 2 D&B/breaks, 3 house, 4 industrial, 5 jazz,
	// 7 lead, 8 movement, 9 pad, 10 poly, 11 SFX, 12 string, 13 ext input, 14 vocoder/tune
	// 6 R&B/hip hop, 7 rock/pop, 8 techno, 9 dubstep

	// The eight animate sources hold the envelope loops that the animate
	// parameters elsewhere trigger. Only the extremes are used. The chart
	// prints CC 60 in the CC column for all of them, but CC 60 is filter
	// routing in the same chart, so only the NRPN address is usable. These
	// also collide with the first eight envelope 2 parameters below, and
	// AnimateHoldButton with Env3Decay.
	Animate1Hold      Control `nrpn:"0.0"` // 0 no hold, 127 hold
	Animate2Hold      Control `nrpn:"0.1"`
	Animate3Hold      Control `nrpn:"0.2"`
	Animate4Hold      Control `nrpn:"0.3"`
	Animate5Hold      Control `nrpn:"0.4"`
	Animate6Hold      Control `nrpn:"0.5"`
	Animate7Hold      Control `nrpn:"0.6"`
	Animate8Hold      Control `nrpn:"0.7"`
	AnimateHoldButton Control `nrpn:"0.16"` // 0 animate hold off, 1 animate hold on

	// The trigger mode of each envelope. All six share NRPN 0.122 and are
	// separated only by the value written to it: 0 single shot, 1 multi.
	Env1Trigger Control `nrpn:"0.122"` // 0 single, 1 multi
	Env2Trigger Control `nrpn:"0.122"` // 2 single, 3 multi
	Env3Trigger Control `nrpn:"0.122"` // 4 single, 5 multi
	Env4Trigger Control `nrpn:"0.122"` // 6 single, 7 multi
	Env5Trigger Control `nrpn:"0.122"` // 8 single, 9 multi
	Env6Trigger Control `nrpn:"0.122"` // 10 single, 11 multi

	// Envelopes 2 to 6 have no control change of their own; envelope 2 is
	// the filter envelope and 3 to 6 are modulation envelopes, each with a
	// delay the amplifier envelope lacks. The fourteen parameters repeat at
	// fourteen consecutive LSBs per envelope, starting at 0.0 for envelope 2.
	// The chart overlaps that run with the AnimateNHold addresses above.
	Env2Velocity    Control `nrpn:"0.0"` // 64 = 0, mapped onto -64..+63
	Env2Attack      Control `nrpn:"0.1"`
	Env2Decay       Control `nrpn:"0.2"`
	Env2Sustain     Control `nrpn:"0.3"`
	Env2Release     Control `nrpn:"0.4"`
	Env2SustainRate Control `nrpn:"0.5"` // 64 = 0, mapped onto -64..+63
	Env2SustainTime Control `nrpn:"0.6"`
	Env2ADRepeats   Control `nrpn:"0.7"`
	Env2AttackTrack Control `nrpn:"0.8"`  // 64 = 0, mapped onto -64..+63
	Env2DecayTrack  Control `nrpn:"0.9"`  // 64 = 0, mapped onto -64..+63
	Env2LevelTrack  Control `nrpn:"0.10"` // 64 = 0, mapped onto -64..+63
	Env2AttackSlope Control `nrpn:"0.11"`
	Env2DecaySlope  Control `nrpn:"0.12"`
	Env2AnimTrigger Control `nrpn:"0.13"` // 0-24, see Env6AnimTrigger for the code list

	Env3Delay       Control `nrpn:"0.14"`
	Env3Attack      Control `nrpn:"0.15"`
	Env3Decay       Control `nrpn:"0.16"`
	Env3Sustain     Control `nrpn:"0.17"`
	Env3Release     Control `nrpn:"0.18"`
	Env3SustainRate Control `nrpn:"0.19"` // 64 = 0, mapped onto -64..+63
	Env3SustainTime Control `nrpn:"0.20"`
	Env3ADRepeats   Control `nrpn:"0.21"`
	Env3AttackTrack Control `nrpn:"0.22"` // 64 = 0, mapped onto -64..+63
	Env3DecayTrack  Control `nrpn:"0.23"` // 64 = 0, mapped onto -64..+63
	Env3LevelTrack  Control `nrpn:"0.24"` // 64 = 0, mapped onto -64..+63
	Env3AttackSlope Control `nrpn:"0.25"`
	Env3DecaySlope  Control `nrpn:"0.26"`
	Env3AnimTrigger Control `nrpn:"0.27"` // 0-24, see Env6AnimTrigger for the code list

	Env4Delay       Control `nrpn:"0.28"`
	Env4Attack      Control `nrpn:"0.29"`
	Env4Decay       Control `nrpn:"0.30"`
	Env4Sustain     Control `nrpn:"0.31"`
	Env4Release     Control `nrpn:"0.32"`
	Env4SustainRate Control `nrpn:"0.33"` // 64 = 0, mapped onto -64..+63
	Env4SustainTime Control `nrpn:"0.34"`
	Env4ADRepeats   Control `nrpn:"0.35"`
	Env4AttackTrack Control `nrpn:"0.36"` // 64 = 0, mapped onto -64..+63
	Env4DecayTrack  Control `nrpn:"0.37"` // 64 = 0, mapped onto -64..+63
	Env4LevelTrack  Control `nrpn:"0.38"` // 64 = 0, mapped onto -64..+63
	Env4AttackSlope Control `nrpn:"0.39"`
	Env4DecaySlope  Control `nrpn:"0.40"`
	Env4AnimTrigger Control `nrpn:"0.41"` // 0-24, see Env6AnimTrigger for the code list

	Env5Delay       Control `nrpn:"0.42"`
	Env5Attack      Control `nrpn:"0.43"`
	Env5Decay       Control `nrpn:"0.44"`
	Env5Sustain     Control `nrpn:"0.45"`
	Env5Release     Control `nrpn:"0.46"`
	Env5SustainRate Control `nrpn:"0.47"` // 64 = 0, mapped onto -64..+63
	Env5SustainTime Control `nrpn:"0.48"`
	Env5ADRepeats   Control `nrpn:"0.49"`
	Env5AttackTrack Control `nrpn:"0.50"` // 64 = 0, mapped onto -64..+63
	Env5DecayTrack  Control `nrpn:"0.51"` // 64 = 0, mapped onto -64..+63
	Env5LevelTrack  Control `nrpn:"0.52"` // 64 = 0, mapped onto -64..+63
	Env5AttackSlope Control `nrpn:"0.53"`
	Env5DecaySlope  Control `nrpn:"0.54"`
	Env5AnimTrigger Control `nrpn:"0.55"` // 0-24, see Env6AnimTrigger for the code list

	Env6Delay       Control `nrpn:"0.56"`
	Env6Attack      Control `nrpn:"0.57"`
	Env6Decay       Control `nrpn:"0.58"`
	Env6Sustain     Control `nrpn:"0.59"`
	Env6Release     Control `nrpn:"0.60"`
	Env6SustainRate Control `nrpn:"0.61"` // 64 = 0, mapped onto -64..+63
	Env6SustainTime Control `nrpn:"0.62"`
	Env6ADRepeats   Control `nrpn:"0.63"`
	Env6AttackTrack Control `nrpn:"0.64"` // 64 = 0, mapped onto -64..+63
	Env6DecayTrack  Control `nrpn:"0.65"` // 64 = 0, mapped onto -64..+63
	Env6LevelTrack  Control `nrpn:"0.66"` // 64 = 0, mapped onto -64..+63
	Env6AttackSlope Control `nrpn:"0.67"`
	Env6DecaySlope  Control `nrpn:"0.68"`
	Env6AnimTrigger Control `nrpn:"0.69"` // 0 off, 1-8 retrigger animate 1-8, 9-16 trigger them,
	// 17-24 enable them

	// The two filter links decide whether filter 2 follows filter 1's
	// frequency and resonance. Like the envelope triggers they share one
	// address and differ only in value.
	FilterFreqLink Control `nrpn:"0.122"` // 42 off, 43 on
	FilterResLink  Control `nrpn:"0.122"` // 44 off, 45 on

	// The three LFOs. Animate and the modulation matrix are separate sources
	// even though they share the LFO numbers on the panel.
	//
	// All three Waveform fields share one 0-37 scale. The chart only names
	// the entries once, on the LFO2 row: 0 sine, 1 triangle, 2 sawtooth,
	// 3 square, 4 random sample and hold, 5 time sample and hold, 6 piano
	// envelope, 7-13 the seven sequence tables, 14-21 the eight alternators,
	// 22 chromatic, 23 chromatic over 16, 24-37 a set of scale and interval
	// tables the chart names by number only (1625 major, 1625 minor, 2511
	// and so on). The chart does not say what those last names denote.
	//
	// The LFO1..LFO3 switches below all live on NRPN 0.122. LFO1FadeMode,
	// LFO2FadeMode and LFO3FadeMode share NRPN 1.123 with the two vocal tune
	// fields and ModMatrix9Source1; the chart prints the LFO3 range as 4-7, a
	// repeat of LFO2's, where the 0-3/4-7/8-11 progression of the first two
	// makes 8-11 the consistent reading.
	LFO1Waveform     Control `nrpn:"0.70"` // 0-37, see the waveform list above
	LFO1PhaseOffset  Control `nrpn:"0.71"` // 0-119 in 3 degree steps up to 357 degrees
	LFO1SlewRate     Control `nrpn:"0.72"`
	LFO1Delay        Control `nrpn:"0.74"`
	LFO1DelaySync    Control `nrpn:"0.75"` // 0-35, sync division table
	LFO1Rate         Control `nrpn:"0.76"`
	LFO1RateSync     Control `nrpn:"0.77"`  // 0-35, sync division table
	LFO1OneShot      Control `nrpn:"0.122"` // 12 normal, 13 one shot
	LFO1KeySync      Control `nrpn:"0.122"` // 14 free running, 15 restarted by each key
	LFO1CommonSync   Control `nrpn:"0.122"` // 16 normal, 17 locked to the other LFOs
	LFO1DelayTrigger Control `nrpn:"0.122"` // 18 single, 19 multi
	LFO1FadeMode     Control `nrpn:"1.123"` // 0 fade in, 1 fade out, 2 gate in, 3 gate out

	LFO2Waveform     Control `nrpn:"0.79"`
	LFO2PhaseOffset  Control `nrpn:"0.80"`
	LFO2SlewRate     Control `nrpn:"0.81"`
	LFO2Delay        Control `nrpn:"0.83"`
	LFO2DelaySync    Control `nrpn:"0.84"`
	LFO2Rate         Control `nrpn:"0.85"`
	LFO2RateSync     Control `nrpn:"0.86"`
	LFO2OneShot      Control `nrpn:"0.122"` // 22 normal, 23 one shot
	LFO2KeySync      Control `nrpn:"0.122"` // 24 free running, 25 restarted by each key
	LFO2CommonSync   Control `nrpn:"0.122"` // 26 normal, 27 locked to the other LFOs
	LFO2DelayTrigger Control `nrpn:"0.122"` // 28 single, 29 multi
	LFO2FadeMode     Control `nrpn:"1.123"` // 4 fade in, 5 fade out, 6 gate in, 7 gate out

	LFO3Waveform     Control `nrpn:"0.88"`
	LFO3PhaseOffset  Control `nrpn:"0.89"`
	LFO3SlewRate     Control `nrpn:"0.90"`
	LFO3Delay        Control `nrpn:"0.92"`
	LFO3DelaySync    Control `nrpn:"0.93"`
	LFO3Rate         Control `nrpn:"0.94"`
	LFO3RateSync     Control `nrpn:"0.95"`
	LFO3OneShot      Control `nrpn:"0.122"` // 32 normal, 33 one shot
	LFO3KeySync      Control `nrpn:"0.122"` // 34 free running, 35 restarted by each key
	LFO3CommonSync   Control `nrpn:"0.122"` // 36 normal, 37 locked to the other LFOs
	LFO3DelayTrigger Control `nrpn:"0.122"` // 38 single, 39 multi
	LFO3FadeMode     Control `nrpn:"1.123"` // 8 fade in, 9 fade out, 10 gate in, 11 gate out

	// The effects chain. Select names the effect in a slot; the fields below
	// are only live once the corresponding slot selects the effect they
	// belong to, so a patch leaves most of them inert.
	FxRouting  Control `nrpn:"0.97"` // 0-7, effect routing order
	FxFeedback Control `nrpn:"0.98"`
	Fx1Select  Control `nrpn:"0.99"` // 0 bypass, 1 EQ, 2-3 compressor, 4-5 distortion,
	Fx2Select  Control `nrpn:"0.100"`
	Fx3Select  Control `nrpn:"0.101"`
	Fx4Select  Control `nrpn:"0.102"`
	Fx5Select  Control `nrpn:"0.103"`
	// 6-7 delay, 8-9 reverb, 10-13 chorus, 14 gator

	EqBassFrequency   Control `nrpn:"0.104"`
	EqBassLevel       Control `nrpn:"0.105"` // 64 = 0 dB, mapped onto -64..+63
	EqMidFrequency    Control `nrpn:"0.106"`
	EqMidLevel        Control `nrpn:"0.107"` // 64 = 0 dB, mapped onto -64..+63
	EqTrebleFrequency Control `nrpn:"0.108"`
	EqTrebleLevel     Control `nrpn:"0.109"` // 64 = 0 dB, mapped onto -64..+63

	Compressor1Ratio     Control `nrpn:"0.110"`
	Compressor1Threshold Control `nrpn:"0.111"` // 0 = -60 dB, 60 = 0 dB
	Compressor1Attack    Control `nrpn:"0.112"`
	Compressor1Release   Control `nrpn:"0.113"`
	Compressor1Hold      Control `nrpn:"0.114"`
	Compressor1Gain      Control `nrpn:"0.115"`
	Compressor2Ratio     Control `nrpn:"0.116"`
	Compressor2Threshold Control `nrpn:"0.117"` // 0 = -60 dB, 60 = 0 dB
	Compressor2Attack    Control `nrpn:"0.118"`
	Compressor2Release   Control `nrpn:"0.119"`
	Compressor2Hold      Control `nrpn:"0.120"`
	Compressor2Gain      Control `nrpn:"0.121"`

	Distortion1Type         Control `nrpn:"1.0"` // See Filter1DriveType for the distortion shapes
	Distortion1Compensation Control `nrpn:"1.1"`
	Distortion1Level        Control `nrpn:"1.2"` // 52 = -12 dB, 82 = +18 dB
	Distortion2Type         Control `nrpn:"1.3"`
	Distortion2Compensation Control `nrpn:"1.4"`
	Distortion2Level        Control `nrpn:"1.5"` // 52 = -12 dB, 82 = +18 dB

	Delay1Time     Control `nrpn:"1.6"`
	Delay1TimeSync Control `nrpn:"1.7"` // 0-35, sync division table
	Delay1Feedback Control `nrpn:"1.8"`
	Delay1Width    Control `nrpn:"1.9"`
	Delay1LRRatio  Control `nrpn:"1.10"` // 0-12, left/right time ratio
	Delay1SlewRate Control `nrpn:"1.11"`
	Delay2Time     Control `nrpn:"1.12"`
	Delay2TimeSync Control `nrpn:"1.13"`
	Delay2Feedback Control `nrpn:"1.14"`
	Delay2Width    Control `nrpn:"1.15"`
	Delay2LRRatio  Control `nrpn:"1.16"`
	Delay2SlewRate Control `nrpn:"1.17"`

	Reverb1Type    Control `nrpn:"1.18"` // 0 chamber, 1 small room, 2 large room, 3 small hall, 4 large hall, 5 great hall
	Reverb1Decay   Control `nrpn:"1.19"`
	Reverb1Damping Control `nrpn:"1.20"`
	Reverb2Type    Control `nrpn:"1.21"`
	Reverb2Decay   Control `nrpn:"1.22"`
	Reverb2Damping Control `nrpn:"1.23"`

	// Chorus1Type is 0 phaser or 1 chorus, so a chorus slot in phaser mode
	// ignores the depth and delay fields below.
	Chorus1Type     Control `nrpn:"1.24"` // 0 phaser, 1 chorus
	Chorus1Rate     Control `nrpn:"1.25"`
	Chorus1RateSync Control `nrpn:"1.26"` // 0-35, sync division table
	Chorus1Feedback Control `nrpn:"1.27"` // 64 = 0, mapped onto -64..+63
	Chorus1ModDepth Control `nrpn:"1.28"`
	Chorus1Delay    Control `nrpn:"1.29"`
	Chorus2Type     Control `nrpn:"1.30"`
	Chorus2Rate     Control `nrpn:"1.31"`
	Chorus2RateSync Control `nrpn:"1.32"`
	Chorus2Feedback Control `nrpn:"1.33"`
	Chorus2ModDepth Control `nrpn:"1.34"`
	Chorus2Delay    Control `nrpn:"1.35"`
	Chorus3Type     Control `nrpn:"1.36"`
	Chorus3Rate     Control `nrpn:"1.37"`
	Chorus3RateSync Control `nrpn:"1.38"`
	Chorus3Feedback Control `nrpn:"1.39"`
	Chorus3ModDepth Control `nrpn:"1.40"`
	Chorus3Delay    Control `nrpn:"1.41"`
	Chorus4Type     Control `nrpn:"1.42"`
	Chorus4Rate     Control `nrpn:"1.43"`
	Chorus4RateSync Control `nrpn:"1.44"`
	Chorus4Feedback Control `nrpn:"1.45"`
	Chorus4ModDepth Control `nrpn:"1.46"`
	Chorus4Delay    Control `nrpn:"1.47"`

	// The gator is a step arpeggiator. Its three switches share NRPN 0.122
	// with the envelope and LFO switches; 0.122 LSB 51 is the one gap in
	// that block.
	GatorOn       Control `nrpn:"0.122"` // 52 off, 53 on
	GatorKeySync  Control `nrpn:"0.122"` // 54 free running, 55 restarted by each key
	GatorKeyLatch Control `nrpn:"0.122"` // 56 latch off, 57 latch on
	GatorRateSync Control `nrpn:"1.49"`  // 0-35, sync division table
	GatorMode     Control `nrpn:"1.50"`  // 0-5, mono or stereo crossed with 16 note or two alternates
	GatorEdgeSlew Control `nrpn:"1.52"`
	GatorHold     Control `nrpn:"1.53"`
	GatorLRDelay  Control `nrpn:"1.54"` // 64 = 0, mapped onto -64..+63

	// The thirty two gator step levels, 0-7 each.
	GatorLevel1  Control `nrpn:"5.0"`
	GatorLevel2  Control `nrpn:"5.1"`
	GatorLevel3  Control `nrpn:"5.2"`
	GatorLevel4  Control `nrpn:"5.3"`
	GatorLevel5  Control `nrpn:"5.4"`
	GatorLevel6  Control `nrpn:"5.5"`
	GatorLevel7  Control `nrpn:"5.6"`
	GatorLevel8  Control `nrpn:"5.7"`
	GatorLevel9  Control `nrpn:"5.8"`
	GatorLevel10 Control `nrpn:"5.9"`
	GatorLevel11 Control `nrpn:"5.10"`
	GatorLevel12 Control `nrpn:"5.11"`
	GatorLevel13 Control `nrpn:"5.12"`
	GatorLevel14 Control `nrpn:"5.13"`
	GatorLevel15 Control `nrpn:"5.14"`
	GatorLevel16 Control `nrpn:"5.15"`
	GatorLevel17 Control `nrpn:"5.16"`
	GatorLevel18 Control `nrpn:"5.17"`
	GatorLevel19 Control `nrpn:"5.18"`
	GatorLevel20 Control `nrpn:"5.19"`
	GatorLevel21 Control `nrpn:"5.20"`
	GatorLevel22 Control `nrpn:"5.21"`
	GatorLevel23 Control `nrpn:"5.22"`
	GatorLevel24 Control `nrpn:"5.23"`
	GatorLevel25 Control `nrpn:"5.24"`
	GatorLevel26 Control `nrpn:"5.25"`
	GatorLevel27 Control `nrpn:"5.26"`
	GatorLevel28 Control `nrpn:"5.27"`
	GatorLevel29 Control `nrpn:"5.28"`
	GatorLevel30 Control `nrpn:"5.29"`
	GatorLevel31 Control `nrpn:"5.30"`
	GatorLevel32 Control `nrpn:"5.31"`

	// The vocoder. Its five switches share NRPN 0.122 with the envelope, LFO,
	// gator and arpeggiator switches; the range starts where the gator's
	// ends.
	VocoderOn             Control `nrpn:"0.122"` // 58 off, 59 on
	VocoderSibilanceType  Control `nrpn:"0.122"` // 60 high pass, 61 noise
	VocoderFreeze         Control `nrpn:"0.122"` // 62 running, 63 frozen
	VocoderAllMax         Control `nrpn:"0.122"` // 64 off, 65 on
	VocoderInput          Control `nrpn:"0.122"` // 66 audio in, 67 vocoder output
	VocoderWidth          Control `nrpn:"1.57"`
	VocoderSibilance      Control `nrpn:"1.58"`
	VocoderSpecShift      Control `nrpn:"1.59"` // 64 = 0, mapped onto -64..+63
	VocoderSpecSpread     Control `nrpn:"1.60"` // 64 = 0, mapped onto -64..+63
	VocoderLevel          Control `nrpn:"1.71"`
	VocoderCarrierLevel   Control `nrpn:"1.72"`
	VocoderModulatorLevel Control `nrpn:"1.73"`
	VocoderResonance      Control `nrpn:"1.74"`
	VocoderDecay          Control `nrpn:"1.75"`
	VocoderGateThreshold  Control `nrpn:"1.76"` // 0 = -96 dB, 96 = 0 dB
	VocoderGateRelease    Control `nrpn:"1.77"`

	// The thirty two vocal tune spectrum band levels, which the resample
	// command below fills from the audio input.
	VocoderSpectrumLevel1  Control `nrpn:"6.0"`
	VocoderSpectrumLevel2  Control `nrpn:"6.1"`
	VocoderSpectrumLevel3  Control `nrpn:"6.2"`
	VocoderSpectrumLevel4  Control `nrpn:"6.3"`
	VocoderSpectrumLevel5  Control `nrpn:"6.4"`
	VocoderSpectrumLevel6  Control `nrpn:"6.5"`
	VocoderSpectrumLevel7  Control `nrpn:"6.6"`
	VocoderSpectrumLevel8  Control `nrpn:"6.7"`
	VocoderSpectrumLevel9  Control `nrpn:"6.8"`
	VocoderSpectrumLevel10 Control `nrpn:"6.9"`
	VocoderSpectrumLevel11 Control `nrpn:"6.10"`
	VocoderSpectrumLevel12 Control `nrpn:"6.11"`
	VocoderSpectrumLevel13 Control `nrpn:"6.12"`
	VocoderSpectrumLevel14 Control `nrpn:"6.13"`
	VocoderSpectrumLevel15 Control `nrpn:"6.14"`
	VocoderSpectrumLevel16 Control `nrpn:"6.15"`
	VocoderSpectrumLevel17 Control `nrpn:"6.16"`
	VocoderSpectrumLevel18 Control `nrpn:"6.17"`
	VocoderSpectrumLevel19 Control `nrpn:"6.18"`
	VocoderSpectrumLevel20 Control `nrpn:"6.19"`
	VocoderSpectrumLevel21 Control `nrpn:"6.20"`
	VocoderSpectrumLevel22 Control `nrpn:"6.21"`
	VocoderSpectrumLevel23 Control `nrpn:"6.22"`
	VocoderSpectrumLevel24 Control `nrpn:"6.23"`
	VocoderSpectrumLevel25 Control `nrpn:"6.24"`
	VocoderSpectrumLevel26 Control `nrpn:"6.25"`
	VocoderSpectrumLevel27 Control `nrpn:"6.26"`
	VocoderSpectrumLevel28 Control `nrpn:"6.27"`
	VocoderSpectrumLevel29 Control `nrpn:"6.28"`
	VocoderSpectrumLevel30 Control `nrpn:"6.29"`
	VocoderSpectrumLevel31 Control `nrpn:"6.30"`
	VocoderSpectrumLevel32 Control `nrpn:"6.31"`
	// This one is a command rather than a stored value: the chart gives it a
	// single legal value, 1.
	VocoderSpectrumResample Control `nrpn:"6.32"`

	// The arpeggiator. Only patterns 0-18 are implemented, so the chart's
	// 0-32 range overstates it.
	ArpOn       Control `nrpn:"0.122"` // 46 off, 47 on
	ArpKeyLatch Control `nrpn:"0.122"` // 50 latch off, 51 latch on
	ArpOctaves  Control `nrpn:"1.62"`  // 0-3, mapped onto 1-4 octaves
	ArpRateSync Control `nrpn:"1.63"`  // 0-18, a subset of the sync table
	ArpGate     Control `nrpn:"1.64"`  // 1-127
	ArpMode     Control `nrpn:"1.65"`  // 0 up, 1 down, 2 up/down, 3 up/down 2, 4 played, 5 random, 6 chord
	ArpPattern  Control `nrpn:"1.66"`  // 0-18, the implemented subset of the chart's 0-32
	ArpSwing    Control `nrpn:"1.68"`  // 1-99
	// A compatibility flag the UltraNova shares, not a MiniNova sound
	// parameter.
	ArpMininova Control `nrpn:"127.127"` // 0-1

	// The eight arpeggiator step gates, plus the step count that decides how
	// many of them are read.
	ArpLength Control `nrpn:"60.40"` // 2-8
	Arp1Step  Control `nrpn:"60.32"` // 0 step 1 off, 1 step 1 on
	Arp2Step  Control `nrpn:"60.33"`
	Arp3Step  Control `nrpn:"60.34"`
	Arp4Step  Control `nrpn:"60.35"`
	Arp5Step  Control `nrpn:"60.36"`
	Arp6Step  Control `nrpn:"60.37"`
	Arp7Step  Control `nrpn:"60.38"`
	Arp8Step  Control `nrpn:"60.39"`

	// The chorder plays a chord of fixed intervals. ChorderKey1 is the root
	// and is implicit; the nine fields below are keys 2 to 10, each a number
	// of semitones from the root.
	ChorderTranspose Control `nrpn:"1.78"` // 53 = -11 semitones, 75 = +11 semitones
	ChorderOn        Control `nrpn:"1.79"` // 0 off, 1 on
	ChorderCount     Control `nrpn:"7.16"` // 0-10
	ChorderKey1      Control `nrpn:"7.17"`
	ChorderKey2      Control `nrpn:"7.18"`
	ChorderKey3      Control `nrpn:"7.19"`
	ChorderKey4      Control `nrpn:"7.20"`
	ChorderKey5      Control `nrpn:"7.21"`
	ChorderKey6      Control `nrpn:"7.22"`
	ChorderKey7      Control `nrpn:"7.23"`
	ChorderKey8      Control `nrpn:"7.24"`
	ChorderKey9      Control `nrpn:"7.25"`

	// Vocal tune. Its mode and insert point share NRPN 1.123 with the three
	// LFO fade modes and ModMatrix9Source1, and the chart prints the insert
	// range as 25-27 while mapping it onto 20-22; the two are kept as printed
	// because the receive range is what a sender has to produce.
	VocalTuneShift           Control `nrpn:"1.80"`  // 40 = -24 semitones, 88 = +24 semitones
	VocalTuneBend            Control `nrpn:"1.81"`  // 40 = -24 semitones, 88 = +24 semitones
	VocalTuneMode            Control `nrpn:"1.123"` // 16 off, 17 scale correction, 18 keyboard control, 19 pitch
	VocalTuneInsert          Control `nrpn:"1.123"` // 20-22 as received, mapped onto 20 pre filter, 21 post filter, 22 pre effects
	VocalTuneScaleType       Control `nrpn:"2.56"`  // 0 played, 1 chromatic, 2 major, 3 natural minor, 4 harmonic minor, 5 melodic minor
	VocalTuneScaleKey        Control `nrpn:"2.57"`  // 0 C ... 11 B
	VocalTuneCorrectionTime  Control `nrpn:"2.58"`
	VocalTuneLevel           Control `nrpn:"2.59"`
	VocalTuneVibrato         Control `nrpn:"2.60"`
	VocalTuneVibratoModWheel Control `nrpn:"2.61"`
	VocalTuneVibratoRate     Control `nrpn:"2.62"`

	// The twenty slot modulation matrix. Slot 1 starts at NRPN 1.83 and
	// slots 2 to 9 continue to 1.127; slot 10 wraps to MSB 2 and slots 10 to
	// 20 run from 2.0 to 2.54. Each slot is a source, a second source, an
	// animate trigger, a bipolar depth and a destination. The chart puts
	// slot 9's first source on 1.123, where the LFO fade modes already sit.
	ModMatrix1Source1      Control `nrpn:"1.83"` // 0-18, see ModMatrix20Destination for the shared codes
	ModMatrix1Source2      Control `nrpn:"1.84"`
	ModMatrix1AnimTrigger  Control `nrpn:"1.85"` // 0-8, retrigger or enable an animate source
	ModMatrix1Depth        Control `nrpn:"1.86"` // 64 = 0, mapped onto -64..+63
	ModMatrix1Destination  Control `nrpn:"1.87"` // 0-69, destination table
	ModMatrix2Source1      Control `nrpn:"1.88"`
	ModMatrix2Source2      Control `nrpn:"1.89"`
	ModMatrix2AnimTrigger  Control `nrpn:"1.90"`
	ModMatrix2Depth        Control `nrpn:"1.91"`
	ModMatrix2Destination  Control `nrpn:"1.92"`
	ModMatrix3Source1      Control `nrpn:"1.93"`
	ModMatrix3Source2      Control `nrpn:"1.94"`
	ModMatrix3AnimTrigger  Control `nrpn:"1.95"`
	ModMatrix3Depth        Control `nrpn:"1.96"`
	ModMatrix3Destination  Control `nrpn:"1.97"`
	ModMatrix4Source1      Control `nrpn:"1.98"`
	ModMatrix4Source2      Control `nrpn:"1.99"`
	ModMatrix4AnimTrigger  Control `nrpn:"1.100"` // 0-8, retrigger or enable an animate source
	ModMatrix4Depth        Control `nrpn:"1.101"` // 64 = 0, mapped onto -64..+63
	ModMatrix4Destination  Control `nrpn:"1.102"` // 0-69, destination table
	ModMatrix5Source1      Control `nrpn:"1.103"`
	ModMatrix5Source2      Control `nrpn:"1.104"`
	ModMatrix5AnimTrigger  Control `nrpn:"1.105"`
	ModMatrix5Depth        Control `nrpn:"1.106"`
	ModMatrix5Destination  Control `nrpn:"1.107"`
	ModMatrix6Source1      Control `nrpn:"1.108"`
	ModMatrix6Source2      Control `nrpn:"1.109"`
	ModMatrix6AnimTrigger  Control `nrpn:"1.110"`
	ModMatrix6Depth        Control `nrpn:"1.111"`
	ModMatrix6Destination  Control `nrpn:"1.112"`
	ModMatrix7Source1      Control `nrpn:"1.113"`
	ModMatrix7Source2      Control `nrpn:"1.114"`
	ModMatrix7AnimTrigger  Control `nrpn:"1.115"`
	ModMatrix7Depth        Control `nrpn:"1.116"`
	ModMatrix7Destination  Control `nrpn:"1.117"`
	ModMatrix8Source1      Control `nrpn:"1.118"`
	ModMatrix8Source2      Control `nrpn:"1.119"`
	ModMatrix8AnimTrigger  Control `nrpn:"1.120"`
	ModMatrix8Depth        Control `nrpn:"1.121"`
	ModMatrix8Destination  Control `nrpn:"1.122"`
	ModMatrix9Source1      Control `nrpn:"1.123"`
	ModMatrix9Source2      Control `nrpn:"1.124"`
	ModMatrix9AnimTrigger  Control `nrpn:"1.125"`
	ModMatrix9Depth        Control `nrpn:"1.126"`
	ModMatrix9Destination  Control `nrpn:"1.127"`
	ModMatrix10Source1     Control `nrpn:"2.0"`
	ModMatrix10Source2     Control `nrpn:"2.1"`
	ModMatrix10AnimTrigger Control `nrpn:"2.2"`
	ModMatrix10Depth       Control `nrpn:"2.3"`
	ModMatrix10Destination Control `nrpn:"2.4"`
	ModMatrix11Source1     Control `nrpn:"2.5"`
	ModMatrix11Source2     Control `nrpn:"2.6"`
	ModMatrix11AnimTrigger Control `nrpn:"2.7"`
	ModMatrix11Depth       Control `nrpn:"2.8"`
	ModMatrix11Destination Control `nrpn:"2.9"`
	ModMatrix12Source1     Control `nrpn:"2.10"`
	ModMatrix12Source2     Control `nrpn:"2.11"`
	ModMatrix12AnimTrigger Control `nrpn:"2.12"`
	ModMatrix12Depth       Control `nrpn:"2.13"`
	ModMatrix12Destination Control `nrpn:"2.14"`
	ModMatrix13Source1     Control `nrpn:"2.15"`
	ModMatrix13Source2     Control `nrpn:"2.16"`
	ModMatrix13AnimTrigger Control `nrpn:"2.17"`
	ModMatrix13Depth       Control `nrpn:"2.18"`
	ModMatrix13Destination Control `nrpn:"2.19"`
	ModMatrix14Source1     Control `nrpn:"2.20"`
	ModMatrix14Source2     Control `nrpn:"2.21"`
	ModMatrix14AnimTrigger Control `nrpn:"2.22"`
	ModMatrix14Depth       Control `nrpn:"2.23"`
	ModMatrix14Destination Control `nrpn:"2.24"`
	ModMatrix15Source1     Control `nrpn:"2.25"`
	ModMatrix15Source2     Control `nrpn:"2.26"`
	ModMatrix15AnimTrigger Control `nrpn:"2.27"`
	ModMatrix15Depth       Control `nrpn:"2.28"`
	ModMatrix15Destination Control `nrpn:"2.29"`
	ModMatrix16Source1     Control `nrpn:"2.30"`
	ModMatrix16Source2     Control `nrpn:"2.31"`
	ModMatrix16AnimTrigger Control `nrpn:"2.32"`
	ModMatrix16Depth       Control `nrpn:"2.33"`
	ModMatrix16Destination Control `nrpn:"2.34"`
	ModMatrix17Source1     Control `nrpn:"2.35"`
	ModMatrix17Source2     Control `nrpn:"2.36"`
	ModMatrix17AnimTrigger Control `nrpn:"2.37"`
	ModMatrix17Depth       Control `nrpn:"2.38"`
	ModMatrix17Destination Control `nrpn:"2.39"`
	ModMatrix18Source1     Control `nrpn:"2.40"`
	ModMatrix18Source2     Control `nrpn:"2.41"`
	ModMatrix18AnimTrigger Control `nrpn:"2.42"`
	ModMatrix18Depth       Control `nrpn:"2.43"`
	ModMatrix18Destination Control `nrpn:"2.44"`
	ModMatrix19Source1     Control `nrpn:"2.45"`
	ModMatrix19Source2     Control `nrpn:"2.46"`
	ModMatrix19AnimTrigger Control `nrpn:"2.47"`
	ModMatrix19Depth       Control `nrpn:"2.48"`
	ModMatrix19Destination Control `nrpn:"2.49"`
	ModMatrix20Source1     Control `nrpn:"2.50"`
	ModMatrix20Source2     Control `nrpn:"2.51"`
	ModMatrix20AnimTrigger Control `nrpn:"2.52"`
	ModMatrix20Depth       Control `nrpn:"2.53"`
	ModMatrix20Destination Control `nrpn:"2.54"`

	// The eight tweak slots, each naming one patch parameter that the front
	// panel knobs then control. The chart's tweak table numbers them 0-125.
	TweakAssignment1 Control `nrpn:"4.0"`
	TweakAssignment2 Control `nrpn:"4.1"`
	TweakAssignment3 Control `nrpn:"4.2"`
	TweakAssignment4 Control `nrpn:"4.3"`
	TweakAssignment5 Control `nrpn:"4.4"`
	TweakAssignment6 Control `nrpn:"4.5"`
	TweakAssignment7 Control `nrpn:"4.6"`
	TweakAssignment8 Control `nrpn:"4.7"`

	// The chart lists 40-240 BPM here, past what one 7-bit data entry can
	// carry, so only the bottom of the range is reachable this way.
	TempoRate Control `nrpn:"2.63"`

	// Both of these act on the instrument when they arrive rather than
	// storing a value: 1 asks for the current patch as a program change and
	// 2 steps to the next patch.
	PatchSelect Control `nrpn:"63.0"` // 0 decrement patch, 1 get program change, 2 increment patch
	BankSelect  Control `nrpn:"63.1"` // 1 bank A, 2 bank B, 3 bank C

	// Global mode. These take effect only while the instrument is in its
	// global menu, which is where the chart's header note about editing
	// with data entry MSB applies.
	GlobalProtect          Control `nrpn:"64.0"`  // 0 protect off, 1 protect on
	GlobalMIDIChannel      Control `nrpn:"64.4"`  // 0-15
	GlobalTuningCents      Control `nrpn:"64.6"`  // 15 = -50 cents, 114 = +50 cents
	GlobalTranspose        Control `nrpn:"64.7"`  // 40 = -24 semitones, 88 = +24 semitones
	GlobalVelocityCurve    Control `nrpn:"64.9"`  // 0 low, 1 medium, 2 high, 3 switch, 4-127 fixed velocity
	GlobalClockSource      Control `nrpn:"64.11"` // 0 internal, 1 USB, 2 DIN, 3 auto
	GlobalSustainPedalMode Control `nrpn:"64.14"` // 0 auto, 1 normally open, 2 normally closed
	GlobalWheelLights      Control `nrpn:"64.20"` // 0 off, 1 on
	GlobalPotPickup        Control `nrpn:"64.21"` // 0 off, 1 on
	GlobalStandbyMode      Control `nrpn:"64.22"` // 0 off, 1 on, 2 after ten minutes
	GlobalArpMIDI          Control `nrpn:"64.23"` // 0 MIDI into arpeggiator, 1 arpeggiator out to MIDI
	GlobalAudioInputGain   Control `nrpn:"64.28"` // 21 off, 22-97 as -10 dB to +65 dB in 1 dB steps
	GlobalAudioInputFX     Control `nrpn:"64.30"`
}

// Pro800 is the Behringer Pro 800, the analogue polysynth whose control change
// surface this struct records. The bindings are transcribed from the
// instrument's control change implementation notes, which list every number it
// answers to and what that number means.
//
// Those notes are a reconstruction from probing the instrument rather than a
// manufacturer specification, and they contradict each other in a few places.
// Each contradiction is recorded next to the field it affects rather than
// resolved by quietly picking a side.
//
// Five properties of the instrument shape the struct.
//
// Almost every continuous parameter is a pair of control changes rather than
// one 7-bit value: a coarse half in 8...42 and a fine half in 80...117, each
// of which is itself 0...127 while the panel shows the parameter as 0...999.
// The notes do not say how the two halves combine, so they are carried as
// separate fields and a caller that needs the panel's number has to work the
// mapping out. The two parameters with no fine half say so.
//
// The switches are not General MIDI switches. An on or off parameter is 0x00
// or 0x40, not 0...63 and 64...127, and a stepped parameter has values of its
// own again: the LFO's shape sits at 0x00, 0x16, 0x2c, 0x42, 0x58 and 0x6e.
// Every stepped field below says which values it takes.
//
// One number does two jobs. The instrument transmits its sync subdivision on
// control change 80 while it receives oscillator A's fine frequency on the
// same number, so moving the subdivision from outside also moves oscillator
// A. The notes record that as a firmware bug rather than as an ambiguity in
// the table, and it is why the subdivision has no field here.
//
// The instrument also implements the General MIDI plumbing, and the notes list
// it: data entry MSB and LSB, the non-registered parameter selectors, and the
// two channel voice messages. None of them sets a parameter, so they are left
// unclaimed, and the test checks that they stay free.
//
// Five further numbers are left unclaimed because the notes record no
// parameter on them: 4, 5, 43 to 47 and 119. That is the notes' silence
// rather than a measured silence from the instrument, so it says nothing
// about whether a given firmware answers them, and it is not a reason to
// treat them as free.
//
// The panel calls the two signal stages VCF and VCA; the fields spell those
// out as Filter and Amp and keep the panel's own labels everywhere else.
type Pro800 struct {
	// Performance controllers. BankSelect names the four banks of a hundred
	// patches that the panel shows as A to D.
	BankSelect   Control `cc:"0"`
	ModWheel     Control `cc:"1"`
	Breath       Control `cc:"2"`
	MasterTune   Control `cc:"3"` // Coarse only; the notes give it no fine half
	MainVolume   Control `cc:"7"` // Coarse only
	SustainPedal Control `cc:"64"`

	// Oscillator A. The three waveform switches are separate controls, so
	// an oscillator mixes its shapes rather than choosing one.
	OscAFrequencyMSB  Control `cc:"8"`
	OscAFrequencyLSB  Control `cc:"80"`
	OscAVolumeMSB     Control `cc:"9"`
	OscAVolumeLSB     Control `cc:"81"`
	OscAPulseWidthMSB Control `cc:"10"`
	OscAPulseWidthLSB Control `cc:"82"`
	OscASaw           Control `cc:"48"`
	OscATriangle      Control `cc:"49"`
	OscASquare        Control `cc:"50"`

	// Oscillator B. Fine detunes it against oscillator A, and Sync drives
	// it from oscillator A's frequency.
	OscBFrequencyMSB  Control `cc:"11"`
	OscBFrequencyLSB  Control `cc:"83"`
	OscBVolumeMSB     Control `cc:"12"`
	OscBVolumeLSB     Control `cc:"84"`
	OscBPulseWidthMSB Control `cc:"13"`
	OscBPulseWidthLSB Control `cc:"85"`
	OscBFineMSB       Control `cc:"14"`
	OscBFineLSB       Control `cc:"86"`
	OscBSaw           Control `cc:"51"`
	OscBTriangle      Control `cc:"52"`
	OscBSquare        Control `cc:"53"`
	OscBSync          Control `cc:"54"`

	// Filter. The envelope amount is what decides whether the envelope
	// sweeps the cutoff up or pulls it down, so it is the parameter that
	// turns the envelope from a percussive decay into a swelling tone.
	FilterCutoffMSB        Control `cc:"15"`
	FilterCutoffLSB        Control `cc:"87"`
	FilterResonanceMSB     Control `cc:"16"`
	FilterResonanceLSB     Control `cc:"88"`
	FilterEnvAmountMSB     Control `cc:"17"`
	FilterEnvAmountLSB     Control `cc:"89"`
	FilterEnvReleaseMSB    Control `cc:"18"`
	FilterEnvReleaseLSB    Control `cc:"90"`
	FilterEnvSustainMSB    Control `cc:"19"`
	FilterEnvSustainLSB    Control `cc:"91"`
	FilterEnvDecayMSB      Control `cc:"20"`
	FilterEnvDecayLSB      Control `cc:"92"`
	FilterEnvAttackMSB     Control `cc:"21"`
	FilterEnvAttackLSB     Control `cc:"93"`
	FilterEnvCurve         Control `cc:"61"`
	FilterEnvSpeed         Control `cc:"62"`
	FilterKeyboardTracking Control `cc:"60"` // 0x00 off, 0x2b half, 0x56 full
	FilterVelocityMSB      Control `cc:"32"`
	FilterVelocityLSB      Control `cc:"108"`
	FilterAftertouchMSB    Control `cc:"40"`
	FilterAftertouchLSB    Control `cc:"115"`

	// Amp.
	AmpEnvReleaseMSB Control `cc:"22"`
	AmpEnvReleaseLSB Control `cc:"94"`
	AmpEnvSustainMSB Control `cc:"23"`
	AmpEnvSustainLSB Control `cc:"95"`
	AmpEnvDecayMSB   Control `cc:"24"`
	AmpEnvDecayLSB   Control `cc:"100"`
	AmpEnvAttackMSB  Control `cc:"25"`
	AmpEnvAttackLSB  Control `cc:"101"`
	AmpEnvCurve      Control `cc:"63"`
	AmpEnvSpeed      Control `cc:"72"`
	AmpVelocityMSB   Control `cc:"31"`
	AmpVelocityLSB   Control `cc:"107"`
	AmpAftertouchMSB Control `cc:"39"`
	AmpAftertouchLSB Control `cc:"114"`

	// Poly Mod turns one oscillator into a modulator, which is how this
	// instrument gets the cross-modulated and filter-sweeping tones that
	// two plain oscillators cannot make on their own.
	PolyModFilterEnvMSB  Control `cc:"26"`
	PolyModFilterEnvLSB  Control `cc:"102"`
	PolyModOscBAmountMSB Control `cc:"27"`
	PolyModOscBAmountLSB Control `cc:"103"`
	PolyModFreqA         Control `cc:"55"`
	PolyModFilter        Control `cc:"56"`

	// LFO. The panel also calls this LFO 2, because the vibrato is the third
	// thing the mod wheel can be routed to and shares this section's
	// modulation delay.
	LfoFrequencyMSB  Control `cc:"28"`
	LfoFrequencyLSB  Control `cc:"104"`
	LfoAmountMSB     Control `cc:"29"`
	LfoAmountLSB     Control `cc:"105"`
	LfoShape         Control `cc:"57"` // 0x00 pulse, 0x16 triangle, 0x2c random, 0x42 sine, 0x58 noise, 0x6e saw
	LfoSpeed         Control `cc:"58"`
	LfoModDelayMSB   Control `cc:"33"`
	LfoModDelayLSB   Control `cc:"109"`
	LfoAftertouchMSB Control `cc:"41"`
	LfoAftertouchLSB Control `cc:"116"`

	// The LFO's routing is described twice and the two descriptions do not
	// obviously agree. The patch record stores it as six independent bits,
	// one per destination, while this switch is documented as four
	// positions at 0x00, 0x21, 0x42 and 0x63, which is a two bit selector
	// with the rest of the value scaled to match: both oscillators, one,
	// the other, or the amplifier. The three switches after it are the
	// panel's own routing knobs, and the notes mark those same three
	// destinations as the hardware-controlled ones.
	LfoTargets       Control `cc:"59"`
	LfoDestFrequency Control `cc:"74"`
	LfoDestFilter    Control `cc:"75"`
	LfoDestPWM       Control `cc:"76"`

	// Vibrato, the modulation the instrument applies when the mod wheel is
	// aimed at it. It has a rate and a depth of its own rather than
	// borrowing the LFO's, so LfoFrequencyMSB is not the vibrato rate.
	VibratoFrequencyMSB Control `cc:"34"`
	VibratoFrequencyLSB Control `cc:"110"`
	VibratoAmountMSB    Control `cc:"35"`
	VibratoAmountLSB    Control `cc:"111"`
	// ModWheelTarget and VibratoTarget are the two switches that aim the mod
	// wheel, and what the wheel modulates is therefore a choice between
	// them rather than a pair of independent amounts.
	//
	// They do not rest on the same evidence. The notes that list the
	// instrument's control changes mark control change 71 as having no
	// parameter on it at all, while a separate chart of the same instrument
	// names this one the vibrato target; the patch record holds only the mod
	// wheel target, so this switch has no stored counterpart. How the two
	// combine is not stated by any of the three.
	ModWheelTarget Control `cc:"70"` // 0x00 LFO, 0x40 vibrato
	VibratoTarget  Control `cc:"71"`
	ModWheelRange  Control `cc:"67"` // 0x00 minimum, 0x20 low, 0x40 high, 0x60 full

	// Glide and the unison stack. UnisonDetune spreads the stack around the
	// detune, so the two only mean anything to each other.
	GlideMSB        Control `cc:"30"`
	GlideLSB        Control `cc:"106"`
	Unison          Control `cc:"65"`
	UnisonDetuneMSB Control `cc:"36"`
	UnisonDetuneLSB Control `cc:"112"`
	VoiceSpread     Control `cc:"77"`

	NoiseLevelMSB Control `cc:"37"`
	NoiseLevelLSB Control `cc:"113"`

	// PitchBendTarget chooses what the bend wheel acts on, so a bend can be
	// turned into a filter sweep or a level change rather than a note
	// change. The amount and range are a coarse and fine pair like every
	// other continuous parameter.
	PitchBendTarget    Control `cc:"66"` // 0x00 off, 0x20 oscillators, 0x40 filter, 0x60 level
	PitchBendAmountMSB Control `cc:"42"`
	PitchBendAmountLSB Control `cc:"117"`

	// OscAPitchMode and OscBPitchMode are how far the bend wheel moves each
	// oscillator: free running, in semitones, in octaves, or not at all.
	OscAPitchMode Control `cc:"68"` // 0x00 free, 0x20 semitones, 0x40 octaves, 0x60 fixed
	OscBPitchMode Control `cc:"69"`

	// The bend is measured from a reference key, and glide is either a
	// fixed time or a fixed rate, neither of which is the same on both.
	TrackingReference Control `cc:"78"` // 0x00 C1, 0x20 C2, 0x40 C3, 0x60 C4
	GlideMode         Control `cc:"79"` // 0x00 time, 0x40 speed

	// ArpMode is the arpeggiator's pattern. The instrument's table gives
	// the last two positions as assigned order then random, while its patch
	// record gives them as random then assigned order; the patch record is
	// followed here, and the two cannot both be right. Nothing in either
	// starts the arpeggiator, so that is left to the panel.
	ArpMode Control `cc:"73"` // 0x00 off, 0x13 up, 0x25 down, 0x37 up then down, 0x4a up and down together, 0x5c random, 0x6e assigned order

	// AbandonedParameter is the one number the notes mark as a possible bug
	// rather than naming: the instrument accepts it and shows it on the
	// display like any other parameter, but nothing it does changes the
	// sound, which the notes take to mean it is left over from an earlier
	// firmware or was a test function.
	AbandonedParameter Control `cc:"118"` // 0x00 to 0x1f
}

// Liven8BitWarps is the Sonicware LIVEN 8bit warps, a groovebox with a
// keyboard, a step sequencer and a four track looper as well as the synth
// below. Its control change surface reaches the synth nearly end to end and
// the sequencer only where a setting outlives the step being edited: the gate
// time, the swing, the sequencer mode and whether recorded parameter locks
// are applied at all.
//
// Two documents supply it, and neither is complete on its own. The
// instrument's "LIVEN 8bit warps MIDI implementation chart"
// (8bw_manual_MIDI_en_r1.pdf) gives every controller number it answers to, and
// nothing else: it states for each one that it is both transmitted and
// recognized, and it never says what a value means. The user's manual,
// document LVN-010-UM-01-EN (8bw_manual_en_r2.pdf), says what each parameter
// does and what range it covers but never mentions a controller number. The
// numbers below are the chart's and the meanings are the manual's, joined
// through the parameter names the chart prints in its Remarks column, which
// match the panel labels the manual describes.
//
// Every field is a direct control change. The chart lists no non-registered
// parameter, so there is nothing for the nrpn tag to address here and a caller
// that builds NRPN controls for this model gets nil back.
//
// The chart records no value encoding for anything, which shapes every
// comment below. The enumerations are written in the order the manual lists
// their entries starting at 0: that is the panel's own ordering, but nothing in
// either document states that the wire format numbers them that way, so it is
// an assumption the two documents together support rather than one either of
// them makes. The ranges are the manual's own, and a parameter narrower than
// 0...127 says so on its field. The switches below are on and off and the
// chart does not say whether that is 0 and 1, 0...63 and 64...127, or
// something else, so their fields say only what they switch.
//
// Three properties of the instrument decide what a value means, and each of
// them puts several parameters behind the same physical control.
//
// Every knob on the panel carries two labels, an uppercase one and a lowercase
// one, and which of the two a knob is editing is decided by a mode button held
// down while it is turned. Nine of the panel's knobs are each a pair of
// parameters, which is 18 of the fields below; the four main knobs are three
// more pairs plus main knob 4, another two. The remaining 11 fields are the
// buttons, which have no knob to hide a second parameter behind.
//
// SynthParameter1 to 3 are the unshifted main knobs 1 to 3 and
// SynthParameter4 to 6 are the same knobs shifted; Detune and MemoryLevel are
// main knob 4 the same way. The remaining pairs are named for what they select
// once the engine is known, which is why SynthParameter1 to 6 carry no such
// name.
//
// SynthParameter1 to 6 mean something different in each of the four synth
// engines, and SynthParameter4 to 6 mean nothing at all in two of them. The
// per-engine table is in the comments on the fields; the engine it applies to
// is whichever SynthEngine names.
//
// SynthEngine's own selection decides what the rest of the patch can be
// playing, so the six synth parameters and the FM engine's own LFO among them
// are inert under the wrong engine rather than merely differently scaled.
//
// The instrument answers none of the General MIDI controllers the other models
// here carry. Its chart marks the modulation wheel, the channel volume, the
// expression pedal, the sustain pedal, bank select, the NRPN selectors and all
// three channel voice messages as neither transmitted nor recognized, and it
// marks aftertouch, active sensing and all-notes-off the same way, so none of
// them is a field here and sending one does nothing to the instrument.
//
// Two General MIDI numbers are spent on parameters of its own anyway: CC#32,
// which General MIDI gives to bank select LSB, is this instrument's filter
// type, and CC#38, which it gives to data entry LSB, is the envelope
// generator's attack. The reuse is unambiguous only because the chart says
// the instrument never implemented either message, so a sender who meant the
// General MIDI one had no way to reach it. The same reasoning covers the two
// non-control-change routes the instrument does answer, a Program Change that
// selects a patch memory or a pattern and a System Exclusive dump carrying
// patch, pattern and waveform data: both are real, and neither is something
// this package's tags can address.
//
// The numbers the chart does not mention at all, 0 to 4, 6 to 19 and 56 to 127,
// are the chart's silence rather than a measured silence from the instrument.
// That is not evidence that the firmware ignores them, and it is not a reason
// to treat them as free.
type Liven8BitWarps struct {
	// The engine is selected rather than morphed through: SynthEngine picks
	// which of the six synth parameters below are live and what they mean.
	// The four engines are WARP, which crossfades between two waveforms,
	// ATTACK, which switches waveforms a set time after a key is played,
	// MORPH, which morphs through three in order, and FM (manual p. 8).
	SynthEngine Control `cc:"20"` // 0 WARP, 1 ATTACK, 2 MORPH, 3 FM

	// The octave shifts the whole instrument's pitch and is the one
	// performance control here with no knob behind it: the panel drives it
	// with two buttons whose colours mark how far it has moved, and the
	// manual gives no adjustment range for the control change (manual p. 7).
	Octave Control `cc:"21"` // Plus or minus 3 octaves from standard

	// Velocity is the level every note is played at, including notes arriving
	// over MIDI, so it is a patch parameter rather than a keyboard
	// performance control. 0...127 (manual p. 7).
	Velocity Control `cc:"22"` // 0-127

	// Detune is main knob 4 unshifted, and is the width of the detuning
	// applied to the voice. Centre-zero over -16...+16 (manual p. 8).
	Detune Control `cc:"23"` // -16...0...+16

	// The six synth parameters are main knobs 1 to 3, unshifted and then
	// shifted, so what one of them selects is decided twice over: by the
	// engine and by whether shift is held. The engine parameter lists below
	// are the manual's appendix Table 1 (p. 30).
	//
	// The three morphing engines pick waveforms with main knobs 1 and 2.
	// Each of those knobs selects MEM, a preset waveform numbered 1 to 63,
	// or a user waveform numbered U-01 to U-64, in that order. The list
	// totals 128 entries, which is exactly the controller's own 0...127
	// range, though neither document says how the two are lined up. FM picks
	// a frequency ratio and an output level instead, and the engine's own
	// LFO rate and depth sit on main knob 3 and on the shifted knob 1.
	SynthParameter1 Control `cc:"24"` // WARP/ATTACK/MORPH: Waveform 1; FM: Ratio 0.5-32
	SynthParameter2 Control `cc:"25"` // WARP/ATTACK/MORPH: Waveform 2; FM: Level 0-127
	// Main knob 3 unshifted, which is where the three engines put the one
	// time constant each of them is built around. Only WARP's is bipolar.
	SynthParameter3 Control `cc:"26"` // WARP: Crossfade -63...0...+63; ATTACK: waveform switching time 0-127; MORPH: morphing time 0-127; FM: LFO rate 0-127
	// Main knobs 1 to 3 shifted. Table 1 leaves WARP and ATTACK with no
	// second layer, so for those two engines these three do nothing at all
	// and a patch that randomizes them leaves them inert rather than wrong.
	SynthParameter4 Control `cc:"27"` // MORPH: Waveform 3; FM: Ratio depth 0-127; unused on WARP and ATTACK
	// Morphing mode is a count of waveforms in the cycle rather than a
	// choice of a pattern, so 2 cycles between waveforms 1 and 2 and 3
	// cycles through all three.
	SynthParameter5 Control `cc:"28"` // MORPH: Morphing mode, 2 or 3 waveforms per cycle; FM: Level depth 0-127; unused on WARP and ATTACK
	// The FM engine's LFO has eight shapes where the instrument's single
	// global LFO has one, which is why this is a list and LfoRate below is
	// not.
	SynthParameter6 Control `cc:"29"` // FM: Waveform, 0 SINE, 1 SQAR, 2 TRI, 3 SAW, 4 R.SAW, 5 RAND, 6 LOG, 7 R.LOG; unused on WARP, ATTACK and MORPH

	// AliasNoise mixes the aliasing artefacts of the 8-bit conversion back
	// into the output. On an engine built from short waveforms it is what
	// turns the raw digital edge into a deliberate character rather than a
	// defect (manual p. 8).
	AliasNoise Control `cc:"30"` // On or off

	// The filter is switched on by one button and its type chosen by the
	// filter buttons, which is why these are two controller numbers rather
	// than one selection with an off position. FilterActive off leaves
	// FilterType, FilterCutoff and FilterResonance inert (manual p. 9).
	FilterActive Control `cc:"31"` // On or off
	// BPF is selected by pressing the LPF and HPF buttons together, and the
	// manual notes both of their LEDs light red for it, so it is a
	// combination rather than a button of its own.
	FilterType Control `cc:"32"` // 0 LPF, 1 HPF, 2 BPF

	// FilterCutoff is 70 Hz to 21.6 kHz over the controller's 0...127
	// (manual p. 9).
	FilterCutoff Control `cc:"33"` // 0-127

	// Resonance reads as bandwidth rather than emphasis when the filter is
	// in BPF, and the two are not the same control: the resonance itself
	// runs 0.1 to 10 while the BPF bandwidth runs 0.1 to 2.0 octaves
	// (manual p. 9).
	FilterResonance Control `cc:"34"` // 0-127

	// The chart calls this FILTER CO and the panel calls the knob FLTR CO,
	// which is the amount of the global LFO applied to the filter cutoff.
	// It is a destination rather than a filter control: raising it makes
	// the cutoff move at the LFO's rate (manual p. 9).
	LfoToFilterCutoff Control `cc:"35"` // 0-127

	// The instrument has one LFO and it produces a sine wave at all times,
	// which is why there is no LFO wave field. 0.1 to 30 Hz (manual p. 9).
	LfoRate Control `cc:"36"` // 0-127

	// LfoToPitch is the depth of the same LFO applied to pitch, up to two
	// octaves at full (manual p. 9).
	LfoToPitch Control `cc:"37"` // 0-127

	// The single envelope generator drives the amplifier and is always a
	// decaying envelope: there is no hold gate, so sustain is the level the
	// voice rests at once the decay runs out rather than a switch. Attack,
	// decay and release are each a 0 to 5000 ms time and sustain is a level
	// of 0 to 100% (manual p. 9).
	EgAttack  Control `cc:"38"` // 0-127
	EgDecay   Control `cc:"39"` // 0-127
	EgSustain Control `cc:"40"` // 0-127
	EgRelease Control `cc:"41"` // 0-127

	// Sweep runs the notes being played up or down past the keys, and it is
	// held rather than latched: the panel needs shift down while a sweep
	// button is pressed, and the sweep ends when neither is (manual p. 7).
	// A pattern recorded with sweep active sweeps during playback too.
	//
	// The panel has a button for each direction and the chart gives them one
	// number between them, so a single value has to carry both which way the
	// sweep runs and whether it is running. The chart does not say how it
	// divides the controller's range between the two.
	Sweep Control `cc:"42"`

	// The time one sweep step takes is a stepped parameter rather than a
	// continuous one, seven speeds from 7.8 ms to 54.7 ms (manual p. 10).
	SweepSpeed Control `cc:"43"` // 1-7: 7.8, 15.6, 23.4, 31.3, 39.1, 46.9, 54.7 ms per step

	// How far past the played notes a sweep travels (manual p. 10).
	SweepShift Control `cc:"44"` // 0-7

	// The effect is a single slot chosen by a button rather than a chain of
	// slots, so FxSpeed and FxAmount below are meaningless until this names
	// one of the four active types (manual p. 10).
	FxType Control `cc:"45"` // 0 OFF, 1 CHORUS, 2 FLANGER, 3 DELAY, 4 CRUSH

	// FxSpeed sets the effect's time and FxAmount its strength, so neither
	// means anything without FxType. 0...127 (manual p. 10).
	FxSpeed  Control `cc:"46"` // 0-127
	FxAmount Control `cc:"47"` // 0-127

	// The reverb is chosen by a button and has its own slot, running
	// alongside the effect rather than after it (manual p. 10).
	ReverbType Control `cc:"48"` // 0 OFF, 1 HALL, 2 ROOM, 3 ARENA, 4 PLAT, 5 TUNNEL, 6 INFINITY, 7 TAPE

	// ReverbAmount is the reverb's mix against the dry sound, except under
	// the TAPE reverb where it is the noise and wow/flutter mix instead,
	// which is what makes that type sound like a tape rather than like a
	// room. 0...127 (manual p. 10).
	ReverbAmount Control `cc:"49"` // 0-127

	// GateTime is shared by the step sequencer and the arpeggiator, so it
	// sets both note lengths from one control, and it applies under every
	// sequencer mode including STUTTER. 10 to 90% (manual p. 18).
	GateTime Control `cc:"50"` // 10-90%

	// Swing delays every other step, and unlike GateTime the panel reaches it
	// only with the sequencer's mode button held down. The manual documents
	// it solely under the step sequencer and never says whether the
	// arpeggiator's notes are swung as well (manual p. 18).
	Swing Control `cc:"51"` // 0-75%

	// VoiceMode decides how held keys are played, and it is what makes the
	// arpeggiator exist at all: in ARP mode the held keys are played one at
	// a time, and GlideOrArpType below is read as an arpeggiator type
	// rather than as a glide time (manual p. 11).
	VoiceMode Control `cc:"52"` // 0 POLY (6 voices), 1 MONO (1 voice), 2 ARPEGGIATOR

	// SeqMode changes how the step sequencer plays its steps back and can be
	// changed mid-playback. Changing or reloading a pattern resets it to
	// NORMAL (manual p. 18).
	SeqMode Control `cc:"53"` // 0 NORMAL, 1 SLICE, 2 RANDOM, 3 STUTTER

	// MemoryLevel is main knob 4 shifted: the level of the selected patch
	// memory against the sound being played, which is how a performance
	// ducks one patch under another. Centre-zero, from -inf dB to +12 dB
	// (manual p. 11).
	MemoryLevel Control `cc:"54"` // -inf...0...+12 dB

	// ParameterLock decides whether the parameter changes recorded against
	// the steps of a pattern are applied while that pattern plays: on, they
	// are applied, and off, they are not (manual p. 16).
	ParameterLock Control `cc:"55"` // On or off

	// GlideOrArpType is one knob with two jobs, and which job it is doing
	// is decided by VoiceMode above. In MONO it is the glide time between
	// notes, 0 to 10000 ms over the controller's own 0...127. In
	// ARPEGGIATOR it selects which of twelve orders the arpeggiator plays
	// held keys in, and at that point a value of 0 to 11 is an index rather
	// than a time. Nothing on the panel names one parameter that is two, so
	// the two readings are kept on one field (manual p. 11).
	GlideOrArpType Control `cc:"5"` // MONO: glide 0-10000 ms; ARPEGGIATOR: 0 UP, 1 DOWN, 2 UP DOWN, 3 DOWN UP, 4 UP&DOWN, 5 DOWN&UP, 6 RANDOM, 7 UP+1, 8 UP+2, 9 DOWN-1, 10 DOWN-2, 11 PLAY ORDER
}

type Model struct {
	Model               string
	*GMController       `json:"GMController,omitempty"`
	*CraftSynth2        `json:"CraftSynth2,omitempty"`
	*MeeblipSE          `json:"MeeblipSE,omitempty"`
	*MeeblipTriode      `json:"MeeblipTriode,omitempty"`
	*MidiMix            `json:"MidiMix,omitempty"`
	*MicrokorgXL        `json:"MicrokorgXL,omitempty"`
	*Microkorg2         `json:"Microkorg2,omitempty"`
	*MiniNova           `json:"MiniNova,omitempty"`
	*PerformVE          `json:"PerformVE,omitempty"`
	*Skulpt             `json:"Skulpt,omitempty"`
	*SoundController    `json:"SoundController,omitempty"`
	*VolcaBass          `json:"VolcaBass,omitempty"`
	*VolcaBeats         `json:"VolcaBeats,omitempty"`
	*VolcaKeys          `json:"VolcaKeys,omitempty"`
	*VolcaKick          `json:"VolcaKick,omitempty"`
	*VolcaDrum          `json:"VolcaDrum,omitempty"`
	*UnoSynth           `json:"UnoSynth,omitempty"`
	*WorldeEasyControl9 `json:"WorldeEasyControl9,omitempty"`
	*ProVSMini          `json:"ProVSMini,omitempty"`
	*MicroKorg          `json:"MicroKorg,omitempty"`
	*Pro800             `json:"Pro800,omitempty"`
	*Liven8BitWarps     `json:"Liven8BitWarps,omitempty"`
}

var modelNames = []string{
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

// ModelNames returns the canonical model names accepted by NewModelParams.
func ModelNames() []string {
	return append([]string(nil), modelNames...)
}

// NewModelParams initializes model-specific storage without exposing MidiParams'
// panic-based unknown-model path to callers.
func NewModelParams(name string) (any, error) {
	for _, modelName := range modelNames {
		if name == modelName {
			model := &Model{Model: name}
			return model.MidiParams(), nil
		}
	}
	return nil, fmt.Errorf("unknown model %q", name)
}

func (m *Model) MidiParams() any {
	switch m.Model {
	case "Craft Synth 2":
		if m.CraftSynth2 == nil {
			m.CraftSynth2 = &CraftSynth2{}
		}
		return m.CraftSynth2
	case "Meeblip SE":
		if m.MeeblipSE == nil {
			m.MeeblipSE = &MeeblipSE{}
		}
		return m.MeeblipSE
	case "Meeblip Triode":
		if m.MeeblipTriode == nil {
			m.MeeblipTriode = &MeeblipTriode{}
		}
		return m.MeeblipTriode
	case "MidiMix":
		if m.MidiMix == nil {
			m.MidiMix = &MidiMix{}
		}
		return m.MidiMix
	case "Skulpt":
		if m.Skulpt == nil {
			m.Skulpt = &Skulpt{}
		}
		return m.Skulpt
	case "Sound Controller":
		if m.SoundController == nil {
			m.SoundController = &SoundController{}
		}
		return m.SoundController
	case "Volca Bass":
		if m.VolcaBass == nil {
			m.VolcaBass = &VolcaBass{}
		}
		return m.VolcaBass
	case "Volca Beats":
		if m.VolcaBeats == nil {
			m.VolcaBeats = &VolcaBeats{}
		}
		return m.VolcaBeats
	case "Volca Keys":
		if m.VolcaKeys == nil {
			m.VolcaKeys = &VolcaKeys{}
		}
		return m.VolcaKeys
	case "Volca Kick":
		if m.VolcaKick == nil {
			m.VolcaKick = &VolcaKick{}
		}
		return m.VolcaKick
	case "Volca Drum":
		if m.VolcaDrum == nil {
			m.VolcaDrum = &VolcaDrum{}
		}
		return m.VolcaDrum
	case "GM Controller":
		if m.GMController == nil {
			m.GMController = &GMController{}
		}
		return m.GMController
	case "Uno Synth":
		if m.UnoSynth == nil {
			m.UnoSynth = &UnoSynth{}
		}
		return m.UnoSynth
	case "WorldeEasyControl9":
		if m.WorldeEasyControl9 == nil {
			m.WorldeEasyControl9 = &WorldeEasyControl9{}
		}
		return m.WorldeEasyControl9
	case "Pro VS Mini":
		if m.ProVSMini == nil {
			m.ProVSMini = &ProVSMini{}
		}
		return m.ProVSMini
	case "microKORG XL":
		if m.MicrokorgXL == nil {
			m.MicrokorgXL = &MicrokorgXL{}
		}
		return m.MicrokorgXL
	case "MicroKorg":
		if m.MicroKorg == nil {
			m.MicroKorg = &MicroKorg{}
		}
		return m.MicroKorg
	case "microKORG2":
		if m.Microkorg2 == nil {
			m.Microkorg2 = &Microkorg2{}
		}
		return m.Microkorg2
	case "Perform-VE":
		if m.PerformVE == nil {
			m.PerformVE = &PerformVE{}
		}
		return m.PerformVE
	case "MiniNova":
		if m.MiniNova == nil {
			m.MiniNova = &MiniNova{}
		}
		return m.MiniNova
	case "Pro 800":
		if m.Pro800 == nil {
			m.Pro800 = &Pro800{}
		}
		return m.Pro800
	case "Liven 8bit warps":
		if m.Liven8BitWarps == nil {
			m.Liven8BitWarps = &Liven8BitWarps{}
		}
		return m.Liven8BitWarps
	default:
		panic("unknown model " + m.Model)
	}
}
