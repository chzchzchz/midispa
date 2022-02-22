package main

type Voice struct {
	Name    string
	Note    int
	Channel int // [1,16] if defined; use device channel it not set

	device *Device // backpointer
}

type VoiceBank struct {
	voices []*Voice
}

func NewVoiceBank(devs []Device) *VoiceBank {
	vb := &VoiceBank{}
	for _, d := range devs {
		for i := range d.Voices {
			vv := &d.Voices[i]
			vv.device = &d
			vb.voices = append(vb.voices, vv)
		}
	}
	return vb
}
