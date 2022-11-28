package main

const (
	VolumeCC          = 7
	AttackTimeCC      = 73
	DecayTimeCC       = 75
	SustainLevelCC    = 79
	ReleaseTimeCC     = 72
	ReverbSendLevelCC = 91
	ChorusSendLevelCC = 93

	AllSoundOffCC = 120
	AllNotesOffCC = 123

	// From spacline keyboard.
	RecordCC      = 20
	PlayCC        = 19
	StopCC        = 18
	SeekForwardCC = 17
	SeekBackCC    = 16
	RepeatCC      = 15
)

type Controls struct {
	Volume int `cc:7`

	AttackTime   int `cc:73`
	DecayTime    int `cc:75`
	SustainLevel int `cc:79` // sound control 10; no default
	ReleaseTime  int `cc:72`

	ReverbSendLevel int `cc:91`
	ChorusSendLevel int `cc:93`

	AllSoundOff int `cc:120` // mute
	AllNotesOff int `cc:123` // panic

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
