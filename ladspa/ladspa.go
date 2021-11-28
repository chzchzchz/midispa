package ladspa

func LowPassFilter(sampleRate int) (*Plugin, error) {
	return NewPlugin("/usr/lib64/ladspa/filter.so", sampleRate)
}
