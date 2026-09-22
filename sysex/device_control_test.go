package sysex

import (
	"testing"
	"time"
)

func TestChorusModDepthRoundTrip(t *testing.T) {
	for _, ms := range []float64{0, 1, 10, 100, 1000} {
		orig := time.Duration(ms * float64(time.Millisecond))
		cm := &ChorusModDepth{ModDepth: orig}
		b, err := cm.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}
		// The value is stored at byte index 11 (after the header)
		val := int(b[11])
		decoded := time.Duration(float64(time.Millisecond) * (float64(val) - 1.0/3.2))
		if decoded != orig {
			t.Errorf("ModDepth round-trip failed: orig=%v, decoded=%v", orig, decoded)
		}
	}
}

func TestChorusFeedbackRoundTrip(t *testing.T) {
	for _, fb := range []float32{0, 0.5, 0.763, 1.0, 10.0} {
		orig := fb
		cf := &ChorusFeedback{Feedback: orig}
		b, err := cf.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}
		val := int(b[11])
		decoded := float32(val) * 0.763
		if decoded != orig {
			t.Errorf("Feedback round-trip failed: orig=%v, decoded=%v", orig, decoded)
		}
	}
}

func TestChorusSendToReverbRoundTrip(t *testing.T) {
	for _, send := range []float32{0, 0.5, 0.763, 1.0, 10.0} {
		orig := send
		cs := &ChorusSendToReverb{SendToReverb: orig}
		b, err := cs.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}
		val := int(b[11])
		decoded := float32(val) / 100.0 * 0.763
		if decoded != orig {
			t.Errorf("SendToReverb round-trip failed: orig=%v, decoded=%v", orig, decoded)
		}
	}
}
