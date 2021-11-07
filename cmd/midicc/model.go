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

type Model struct {
	Model       string
	*VolcaBeats `json:"VolcaBeats,omitempty"`
	*VolcaDrum  `json:"VolcaDrum,omitempty"`
	*UnoSynth   `json:"UnoSynth,omitempty"`
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
	default:
		panic("unknown model " + m.Model)
	}
}
