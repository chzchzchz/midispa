package sysex

import (
	"testing"
)

func TestHandshakeFromSysExValid(t *testing.T) {
	data := []byte{0xF0, 0x7e, 0x10, 0x7c, 0x01, 0xf7}
	// MIDI_Specification.pdf, Generic Handshaking Messages: the parser receives data after the F0 7E frame.
	hs, err := HandshakeFromSysEx(data[2:])
	if err != nil {
		t.Fatalf("HandshakeFromSysEx: %v", err)
	}
	if hs.DeviceId != 0x10 || hs.SubId != 0x7c || hs.Packet != 0x01 {
		t.Errorf("unexpected values: %+v", hs)
	}
}

func TestHandshakeFromSysExMissingEndSysEx(t *testing.T) {
	data := []byte{0x10, 0x7c, 0x01}
	_, err := HandshakeFromSysEx(data)
	if err == nil {
		t.Error("expected error for missing EndSysEx")
	}
}

func TestHandshakeFromSysExTooShort(t *testing.T) {
	data := []byte{0x10, 0x7c}
	_, err := HandshakeFromSysEx(data)
	if err == nil {
		t.Error("expected error for too-short data")
	}
}
