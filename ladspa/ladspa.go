package ladspa

func LowPassFilter(sampleRate int) (*Plugin, error) {
	return NewPlugin("/usr/lib64/ladspa/filter.so", sampleRate)
}

func Reverb(sampleRate int) (*Plugin, error) {
	return NewPlugin("/usr/lib64/ladspa/revdelay_1605.so", sampleRate)
}

func Chorus(sampleRate int) (*Plugin, error) {
	return NewPlugin("/usr/lib64/ladspa/multivoice_chorus_1201.so", sampleRate)
}
