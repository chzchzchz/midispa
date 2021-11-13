package main

type VolcaBeats struct {
	KickLevel      int `midicc:"40"`
	SnareLevel     int `midicc:"41"`
	LoTomLevel     int `midicc:"42"`
	HiTomLevel     int `midicc:"43"`
	ClosedHatLevel int `midicc:"44"`
	OpenHatLevel   int `midicc:"45"`
	ClapLevel      int `midicc:"46"`
	ClavesLevel    int `midicc:"47"`
	AgogoLevel     int `midicc:"48"`
	CrashLevel     int `midicc:"49"`
	ClapPCMSpeed   int `midicc:"50"`
	ClavesPCMSpeed int `midicc:"51"`
	AgogoPCMSpeed  int `midicc:"52"`
	CrashPCMSpeed  int `midicc:"53"`
	StutterTime    int `midicc:"54"`
	StutterDepth   int `midicc:"55"`
	TomDecay       int `midicc:"56"`
	ClosedHatDecay int `midicc:"57"`
	OpenHatDecay   int `midicc:"58"`
	HatGrain       int `midicc:"59"`
}

type WorldeEasyControl9 struct {
	SliderAB int `midicc:"9"`
	Slider1  int `midicc:"3"`
	Slider2  int `midicc:"4"`
	Slider3  int `midicc:"5"`
	Slider4  int `midicc:"6"`
	Slider5  int `midicc:"7"`
	Slider6  int `midicc:"8"`
	Slider7  int `midicc:"9"`
	Slider8  int `midicc:"10"`
	Slider9  int `midicc:"11"`

	Knob1 int `midicc:"14"`
	Knob2 int `midicc:"15"`
	Knob3 int `midicc:"16"`
	Knob4 int `midicc:"17"`
	Knob5 int `midicc:"18"`
	Knob6 int `midicc:"19"`
	Knob7 int `midicc:"20"`
	Knob8 int `midicc:"21"`
	Knob9 int `midicc:"22"`

	Button1 int `midicc:"23"`
	Button2 int `midicc:"24"`
	Button3 int `midicc:"25"`
	Button4 int `midicc:"26"`
	Button5 int `midicc:"27"`
	Button6 int `midicc:"28"`
	Button7 int `midicc:"29"`
	Button8 int `midicc:"30"`
	Button9 int `midicc:"31"`

	Repeat    int `midicc:"49"`
	Backwards int `midicc:"47"`
	Forwards  int `midicc:"48"`
	Stop      int `midicc:"46"`
	Play      int `midicc:"45"`
	Record    int `midicc:"44"`

	ButtonLeftProgramKnob  int `midicc:"67"`
	ButtonRightProgramKnob int `midicc:"64"`
}

type VolcaDrum struct {
	Pan            int `midicc:"10"`
	Select1        int `midicc:"14"`
	Select2        int `midicc:"15"`
	Select1m2      int `midicc:"16"`
	Level1         int `midicc:"17"`
	Level2         int `midicc:"18"`
	Level1m2       int `midicc:"19"`
	EGAttack1      int `midicc:"20"`
	EGAttack2      int `midicc:"21"`
	EGAttack1m2    int `midicc:"22"`
	EGRelease1     int `midicc:"23"`
	EGRelease2     int `midicc:"24"`
	EGRelease1m2   int `midicc:"25"`
	Pitch1         int `midicc:"26"`
	Pitch2         int `midicc:"27"`
	Pitch1m2       int `midicc:"28"`
	ModAmount1     int `midicc:"29"`
	ModAmount2     int `midicc:"30"`
	ModAmount1m2   int `midicc:"31"`
	ModRate1       int `midicc:"46"`
	ModRate2       int `midicc:"47"`
	ModRate1m2     int `midicc:"48"`
	BitReduction   int `midicc:"49"`
	Fold           int `midicc:"50"`
	Drive          int `midicc:"51"`
	DryGain        int `midicc:"52"`
	Send           int `midicc:"103"`
	WaveguideModel int `midicc:"116"`
	Decay          int `midicc:"117"`
	Body           int `midicc:"118"`
	Tune           int `midicc:"119"`
}

type UnoSynth struct {
	ModulationWheel           int `midicc:"1"`
	GlideTime                 int `midicc:"5"`
	VCALevel                  int `midicc:"7"`
	Swing                     int `midicc:"9"`
	GlideOnOff                int `midicc:"65"`
	VibratoOnOff              int `midicc:"77"`
	WahOnOff                  int `midicc:"78"`
	TremoloOnOff              int `midicc:"79"`
	DiveOnOff                 int `midicc:"89"`
	DiveRange                 int `midicc:"90"`
	ScoopOnOff                int `midicc:"91"`
	ScoopRange                int `midicc:"92"`
	ModWheelToLFORate         int `midicc:"93"`
	ModWheelToVibrato         int `midicc:"94"`
	ModWheelToWah             int `midicc:"95"`
	ModWheelToTremelo         int `midicc:"96"`
	ModWheelFilterCutoff      int `midicc:"97"`
	PitchBendRange            int `midicc:"101"`
	VelocityToVCA             int `midicc:"102"`
	VelocityToFilterCutoff    int `midicc:"103"`
	VelocityToFilterEnvAmount int `midicc:"104"`
	VelocityToLFORate         int `midicc:"105"`
	FilterCutoffKeytrack      int `midicc:"106"`
	DelayMix                  int `midicc:"80"`
	DelayTime                 int `midicc:"81"`
	OSC1Level                 int `midicc:"12"`
	OSC2Level                 int `midicc:"13"`
	NoiseLevel                int `midicc:"14"`
	OSC1Wave                  int `midicc:"15"`
	OSC2Wave                  int `midicc:"16"`
	OSC1Tune                  int `midicc:"17"`
	OSC2Tune                  int `midicc:"18"`
	AmpAttack                 int `midicc:"24"`
	AmpDecay                  int `midicc:"25"`
	AmpSustain                int `midicc:"26"`
	AmpRelease                int `midicc:"27"`
	FilterMode                int `midicc:"19"`
	FilterCutoff              int `midicc:"20"`
	FilterResonance           int `midicc:"21"`
	FilterDrive               int `midicc:"22"`
	FilterEnv                 int `midicc:"23"`
	FilterAttack              int `midicc:"44"`
	FilterDecay               int `midicc:"45"`
	FilterSustain             int `midicc:"46"`
	FilterRelease             int `midicc:"47"`
	FilterEnvToOSC1PWM        int `midicc:"48"`
	FilterEnvToOSC2PWM        int `midicc:"49"`
	FilterEnvToOSC1Wave       int `midicc:"50"`
	FilterEnvToOSC2Wave       int `midicc:"51"`
	LFOWave                   int `midicc:"66"`
	LFORate                   int `midicc:"67"`
	LFOToPitch                int `midicc:"68"`
	LFOToFilterCutoff         int `midicc:"69"`
	LFOToTremelo              int `midicc:"70"`
	LFOToWah                  int `midicc:"71"`
	LFOToVibr                 int `midicc:"72"`
	LFOToOSC1PWM              int `midicc:"73"`
	LFOToOSC2PWM              int `midicc:"74"`
	LFOToOSC1Waveform         int `midicc:"75"`
	LFOToOSC2Waveform         int `midicc:"76"`
	ArpeggiatorOnOff          int `midicc:"82"`
	ArpeggiatorDirection      int `midicc:"83"`
	ArpeggiatorRange          int `midicc:"84"`
	ArpeggiatorAndSeqGateTime int `midicc:"85"`
	SeqDirection              int `midicc:"86"`
	SeqRange                  int `midicc:"87"`
}

type Skulpt struct {
	// SeqLoad int `midicc:"0"` // [0,63]
	ModulationWheel  int `midicc:"1"`
	Glide            int `midicc:"5"`
	HeadphoneVolume  int `midicc:"7"`
	VoiceMode        int `midicc:"9"`  // 0-42 for mono; 43-85 for duo; 86-127 for poly
	ExpressionPedal  int `midicc:"11"` //,,0,63,,,,,0-based,Min and max values need verification
	Distortion       int `midicc:"12"`
	Delay            int `midicc:"13"`
	DelayTime        int `midicc:"14"` // No sync: 0-250ms. Sync: 8 steps with longest delay time possible divided down.
	DelayFeedback    int `midicc:"15"` // Ranges from 0-90%
	OSC1Wave         int `midicc:"16"` // 0-21 for sine; 22-42 for tri; 43-63 for saw; 64-127 for PWM duty 50%-5%
	OSC2Wave         int `midicc:"17"` // 0-21 for sine; 22-42 for tri; 43-63 for saw; 64-85 for square; 86-127 for white noise
	OSCMix           int `midicc:"18"`
	FMAmount         int `midicc:"19"` // Centered Plus or minus 2 octaves.
	Spread           int `midicc:"20"` // 0-63 for unison; 64-70 for major; 71-77 for minor; 78-84 for major 6th; 85-91 for sus 4th; 92-98 for 5ths; 99-105 for 5th + oct; 106-112 for oct +1+2; 113-119 for oct +1-1; 119-127 for oct-1-2
	ChordMode        int `midicc:"21"` // 0-63 for off; 64-127 for on
	FEGattack        int `midicc:"22"` // 0-4 seconds
	FEGDecay         int `midicc:"23"` // 0-2 seconds
	FEGSustain       int `midicc:"24"` // 0-1 seconds
	FEGRelease       int `midicc:"25"` // 0-4 seconds
	AEGAttack        int `midicc:"26"` // 0-4 seconds
	AEGDecay         int `midicc:"27"` // 0-2 seconds
	AEGSustain       int `midicc:"28"` // 0-1 seconds
	AEGRelease       int `midicc:"29"` // 0-4 seconds
	OSC2CourseDetune int `midicc:"30"` // Plus or minus 4 octaves
	OSC2FineDetune   int `midicc:"31"` // Plus or minus 1 semitone
	FEGAmount        int `midicc:"32"`
	Morph            int `midicc:"33"` // 0 for LP; 64 for BP; 127 for HP
	Cutoff           int `midicc:"34"` // 0Hz to 2kHz
	Reso             int `midicc:"35"`
	LFO1Rate         int `midicc:"36"` // No sync: 0-127 for 0.02Hz - 32Hz. Sync: 0-7 for 1/16; 8-15 for 1/8; 16-23 for 3/16; 24-31 for 1/4; 32-39 for 3/8; 40-47 for 1/2; 48-55 for 3/4; 56-53 for 1; 64-71 for 3/2; 72-79 for 2; 80-87 for 3; 88-95 for 4; 96-103 for 6; 104-111 for 8; 112-119 for 12; 120-127 for 16
	LFO1Depth        int `midicc:"37"`
	LFO1Shape        int `midicc:"39"` // 0-14 for sine; 15-31 for iSine; 32-47 for tri; 48-63 for iTri; 64-79 for ramp up; 80-95 for ramp down; 96-120 for square; 121-127 for iSquare
	OctAve           int `midicc:"40"` // Octaves -2 to +4
	MEGAttack        int `midicc:"43"` //0-4 seconds
	MEGDecay         int `midicc:"44"` // 0-2 seconds
	MEGSustain       int `midicc:"45"` // 0-1 seconds
	MEGRelease       int `midicc:"46"` // 0-4 seconds
	LFO2Rate         int `midicc:"47"` // No sync: 0-63 for 0-32Hz free; 64-71 for root/8; 72-79 for root/4; 80-87 for root/2; 88-95 for root; 96-13 for root*1.5; 104-111 for root*2; 112-119 for root*2.5; 120*127 for root*3. Sync: 0-7 for 1/16; 8-15 for 1/8; 16-23 for 1/4; 24-31 for 1/2; 32-39 for 1; 40-47 for 5/4; 48-55 for 2; 56-63 for 4 (cycles per beat)
	LFO2Depth        int `midicc:"48"`
	MEGAmount        int `midicc:"49"` // Centered
	LFO2Shape        int `midicc:"50"` // 0-14 for sine; 15-31 for iSine; 32-47 for tri; 48-63 for iTri; 64-79 for ramp up; 80-95 for ramp down; 96-120 for square; 121-127 for iSquare
	AEGAmount        int `midicc:"51"`
	LFO1MIDISync     int `midicc:"52"` // 0-63 for off; 64-127 for on
	RingMod          int `midicc:"53"` // "
	LFO2MIDISync     int `midicc:"54"` // "
	DelayMIDISync    int `midicc:"55"` // "
	LFO1Mode         int `midicc:"56"` // 0-41 for retrig; 42-83 for free; 84-127 for single
	LFO2Mode         int `midicc:"57"` // 0-41 for retrig; 42-83 for free; 84-127 for single
	ArpStatus        int `midicc:"58"` // 0-63 for off; 64-127 for on
	ArpOctave        int `midicc:"59"` // 0-31 for 1 oct; 32-63 for 2 oct; 64-95 for 3 oct; 96-127 for 4 oct
	ArpDirection     int `midicc:"60"` // 0-20 for forwards; 21-41 for backwards; 42-62 for pendulum; 63-83 for note forwards; 84-104 for note backwards; 105-127 for note pendulum
	ArpDivision      int `midicc:"61"` // 16 = 1/32nd 1/24th 1/16th 1/12th 1/8th 1/6th 1/4th or 1/2
	VeloDepth        int `midicc:"62"` // Centered
	NoteDepth        int `midicc:"63"` // Centered
	AftertouchDepth  int `midicc:"65"` // Centered
	ExtDepth         int `midicc:"66"` // Centered
	SequenceLength   int `midicc:"67"` // 0-31 for 1 bar; 32-63 for 2 bars; 64-95 for 4 bars; 96-127 for 8 bars
	SequenceHold     int `midicc:"70"` // 0-63 for off; 64-127 for on
	SequenceLoop     int `midicc:"71"` // 0 to set loop stop point; 127 to set loop start point
	Transpose        int `midicc:"75"` // From -24 to +36 sent as (value + 24) * 2
	Swing            int `midicc:"78"`
	//Anim 1 cc,,80,,0,127,,,,,0-based,CC number of new destination
	//Anim 2 cc,,81,,0,127,,,,,0-based,CC number of new destination
	//Anim 3 cc,,82,,0,127,,,,,0-based,CC number of new destination
	//Anim 4 cc,,83,,0,127,,,,,0-based,CC number of new destination
	AllEnvelopeAttack  int `midicc:"84"`  // 0-4 seconds
	AllEnvelopeDecay   int `midicc:"85"`  // 0-2 seconds
	AllEnvelopeSustain int `midicc:"86"`  // 0-1 seconds
	AllEnvelopeRelease int `midicc:"87"`  // 0-4 seconds
	ModSlot1Depth      int `midicc:"88"`  // Centered
	ModSlot2Depth      int `midicc:"89"`  // Centered
	ModSlot3Depth      int `midicc:"90"`  // Centered
	ModSlot4Depth      int `midicc:"91"`  // Centered
	ModSlot5Depth      int `midicc:"92"`  // Centered
	ModSlot6Depth      int `midicc:"93"`  // Centered
	ModSlot7Depth      int `midicc:"94"`  // Centered
	ModSlot8Depth      int `midicc:"95"`  // Centered
	ModWheelDepth      int `midicc:"96"`  // Centered
	ModSlot1Source     int `midicc:"101"` // 0,7
	ModSlot2Source     int `midicc:"102"` // 0,7
	ModSlot3Source     int `midicc:"103"` // 0,7
	ModSlot4Source     int `midicc:"104"` // 0,7
	ModSlot5Source     int `midicc:"105"` // 0,7
	ModSlot6Source     int `midicc:"106"` // 0,7
	ModSlot7Source     int `midicc:"107"` // 0,7
	ModSlot8Source     int `midicc:"108"` // 0,7
	ModSlot1Dest       int `midicc:"111"` // 0,36
	ModSlot2Dest       int `midicc:"112"` // 0,36
	ModSlot3Dest       int `midicc:"113"` // 0,36
	ModSlot4Dest       int `midicc:"114"` // 0,36
	ModSlot5Dest       int `midicc:"115"` // 0,36
	ModSlot6Dest       int `midicc:"116"` // 0,36
	ModSlot7Dest       int `midicc:"117"` // 0,36
	ModSlot8Dest       int `midicc:"118"` // 0,36
	// Randomise patch 121
}

// SoundController represents a general midi 2 sound controller.
type SoundController struct {
	SoundController1  int `midicc:"70"`
	SoundController2  int `midicc:"71"`
	SoundController3  int `midicc:"72"`
	SoundController4  int `midicc:"73"`
	SoundController5  int `midicc:"74"`
	SoundController6  int `midicc:"75"`
	SoundController7  int `midicc:"76"`
	SoundController8  int `midicc:"77"`
	SoundController9  int `midicc:"78"`
	SoundController10 int `midicc:"79"`
}

type GMController struct {
	BankSelect           int `midicc:"0"`
	Modulation           int `midicc:"1"`
	BreathController     int `midicc:"2"`
	FootController       int `midicc:"4"`
	ChannelVolume        int `midicc:"7"`
	ChannelBalance       int `midicc:"8"`
	Pan                  int `midicc:"10"`
	ExpressionController int `midicc:"11"`
}

type Model struct {
	Model            string
	*VolcaBeats      `json:"VolcaBeats,omitempty"`
	*VolcaDrum       `json:"VolcaDrum,omitempty"`
	*UnoSynth        `json:"UnoSynth,omitempty"`
	*Skulpt          `json:"Skulpt,omitempty"`
	*SoundController `json:"SoundController,omitempty"`
	*GMController    `json:"GMController,omitempty"`
}

func (m *Model) MidiParams() interface{} {
	switch m.Model {
	case "Volca Beats":
		return m.VolcaBeats
	case "WorldeEasyControl9":
		return &WorldeEasyControl9{}
	case "Volca Drum":
		return m.VolcaDrum
	case "Uno Synth":
		return m.UnoSynth
	case "Skulpt":
		return m.Skulpt
	case "Sound Controller":
		return m.SoundController
	case "GM Controller":
		return m.GMController
	default:
		panic("unknown model " + m.Model)
	}
}
