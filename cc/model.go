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

type Model struct {
	Model               string
	*GMController       `json:"GMController,omitempty"`
	*CraftSynth2        `json:"CraftSynth2,omitempty"`
	*MeeblipSE          `json:"MeeblipSE,omitempty"`
	*MeeblipTriode      `json:"MeeblipTriode,omitempty"`
	*MidiMix            `json:"MidiMix,omitempty"`
	*MicrokorgXL        `json:"MicrokorgXL,omitempty"`
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
	"Perform-VE",
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
	case "Perform-VE":
		if m.PerformVE == nil {
			m.PerformVE = &PerformVE{}
		}
		return m.PerformVE
	default:
		panic("unknown model " + m.Model)
	}
}
