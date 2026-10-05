package jack

import "testing"

// The process callback runs on the thread JACK calls back into, once per cycle, so the
// property worth guarding is not how fast it is but that it does not allocate. A
// collection on that thread is a pause in the audio that no functional test would
// notice, and the ports slice is indexed there precisely so that no lock is needed.
//
// The absolute numbers mean little here: against the fake, the cost is the mutex in
// AudioBuffer rather than the cgo call it stands in for. The allocs are the point.
func BenchmarkProcessAudio(b *testing.B) {
	tests := []struct {
		name string
		open func(PortConfig) (*Port, error)
		pc   PortConfig
	}{
		{
			name: "mono",
			open: NewWritePort,
			pc:   PortConfig{AudioCallback: func([]float32) int { return 0 }},
		},
		{
			name: "stereo",
			open: NewStereoWritePort,
			pc:   PortConfig{StereoCallback: func(l, r []float32) int { return 0 }},
		},
	}
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			c := newFakeClient(testClientName)
			c.existing = []*fakePort{newFakePort(playbackPort)}
			pc := tc.pc
			pc.MatchName = []string{playbackMatch}

			p, err := tc.open(testConfig(c, pc))
			if err != nil {
				b.Fatalf("opening port: %v", err)
			}
			defer p.Close()

			// One cycle first, so the fake's buffer has been grown to full size and is
			// not charged to the measurement.
			c.cycle(b, testFrames)

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				c.cycle(b, testFrames)
			}
		})
	}
}
