package main

type Channel struct {
	*Program
	Controls

	Volume float32
	FxLevel
}

func scale(n int) float32 { return float32(n) / 127.0 }

func (c *Channel) UpdateControls() bool {
	if !c.updated {
		return false
	}
	c.Volume = scale(c.Controls.Volume)
	c.FxLevel.ChorusSendLevel = scale(c.Controls.ChorusSendLevel)
	c.FxLevel.ReverbSendLevel = scale(c.Controls.ReverbSendLevel)
	c.updated = false
	return true
}

func NewChannel() *Channel {
	return &Channel{
		Volume: 1.0,
		// TODO: load controls from programs
		Controls: Controls{
			Volume:       127,
			AttackTime:   10,
			DecayTime:    10,
			SustainLevel: 64,
			ReleaseTime:  20,
		},
	}
}
