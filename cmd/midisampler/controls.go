package main

const (
	AttackTimeCC   = 73
	DecayTimeCC    = 75
	SustainLevelCC = 79
	ReleaseTimeCC  = 72
	AllSoundOffCC  = 120
	AllNotesOffCC  = 123
)

type Controls struct {
	AttackTime   int `midicc:73`
	DecayTime    int `midicc:75`
	SustainLevel int `midicc:79` // sound control 10; no default
	ReleaseTime  int `midicc:72`

	AllSoundOff int `midicc:120` // mute
	AllNotesOff int `midicc:123` // panic
}
