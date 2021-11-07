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
}

type Model struct {
	Model       string
	*VolcaBeats `json:"VolcaBeats,omitempty"`
}

func (m *Model) MidiParams() interface{} {
	switch m.Model {
	case "Volca Beats":
		return m.VolcaBeats
	case "WorldeEasyControl9":
		return &WorldeEasyControl9{}
	default:
		panic("unknown model " + m.Model)
	}
}
