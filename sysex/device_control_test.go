package sysex

import (
	"bytes"
	"testing"
)

func TestChorusModDepthRoundTrip(t *testing.T) {
	// MIDI_Specification.pdf, System Exclusive Messages: exercise representable 7-bit values.
	for _, value := range []byte{0x01, 0x0b, 0x65, 0xc9} {
		testChorusRoundTrip(t, chorusMessage(ChorusParameterModDepth, value))
	}
}

func TestChorusFeedbackRoundTrip(t *testing.T) {
	// MIDI_Specification.pdf, System Exclusive Messages: feedback uses a 7-bit data byte.
	for _, value := range []byte{0, 1, 50, 100} {
		testChorusRoundTrip(t, chorusMessage(ChorusParameterFeedback, value))
	}
}

func TestChorusSendToReverbRoundTrip(t *testing.T) {
	// MIDI_Specification.pdf, System Exclusive Messages: reverb send uses a 7-bit data byte.
	for _, value := range []byte{0, 1, 50, 100} {
		testChorusRoundTrip(t, chorusMessage(ChorusParameterSendToReverb, value))
	}
}

func chorusMessage(parameter, value byte) []byte {
	return []byte{
		0xf0, IdRealTime, 0x10,
		SubIdDeviceControl, DeviceControlIdGlobalParameterControl,
		1, 1, 1,
		1, 2,
		parameter, value,
		0xf7,
	}
}

func testChorusRoundTrip(t *testing.T, message []byte) {
	t.Helper()
	decoded := Decode(message)
	if decoded == nil {
		t.Fatalf("Decode returned nil for %v", message)
	}
	marshaler, ok := decoded.(interface{ MarshalBinary() ([]byte, error) })
	if !ok {
		t.Fatalf("decoded value %T does not implement MarshalBinary", decoded)
	}
	encoded, err := marshaler.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(encoded, message) {
		t.Errorf("round-trip = %v, want %v", encoded, message)
	}
}
