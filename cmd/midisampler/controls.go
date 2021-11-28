package main

const (
	VolumeCC          = 7
	AttackTimeCC      = 73
	DecayTimeCC       = 75
	SustainLevelCC    = 79
	ReleaseTimeCC     = 72
	ReverbSendLevelCC = 91
	ChorusSendLevelCC = 93
	AllSoundOffCC     = 120
	AllNotesOffCC     = 123
)

type Controls struct {
	Volume int `midicc:7`

	AttackTime   int `midicc:73`
	DecayTime    int `midicc:75`
	SustainLevel int `midicc:79` // sound control 10; no default
	ReleaseTime  int `midicc:72`

	ReverbSendLevel int `midicc:91`
	ChorusSendLevel int `midicc:93`

	AllSoundOff int `midicc:120` // mute
	AllNotesOff int `midicc:123` // panic

	updated bool
}

func (c *Controls) Set(cc, val int) bool {
	switch cc {
	case VolumeCC:
		c.Volume = val
	case AttackTimeCC:
		c.AttackTime = val
	case DecayTimeCC:
		c.DecayTime = val
	case SustainLevelCC:
		c.SustainLevel = val
	case ReleaseTimeCC:
		c.ReleaseTime = val
	case ReverbSendLevelCC:
		c.ReverbSendLevel = val
	case ChorusSendLevelCC:
		c.ChorusSendLevel = val
	default:
		return false
	}
	c.updated = true
	return true
}
