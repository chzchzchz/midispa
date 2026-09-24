package sysex

import (
	"bytes"
	"testing"
)

func TestFileDumpDataPacketRoundTrip(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	pkt := &FileDumpDataPacket{DeviceId: 1, Packet: 0, Data: data}
	b, err := pkt.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	// MIDI_Specification.pdf, File Dump: the parser receives data after the F0 7E frame, as Decode does.
	decoded := FileDumpDataPacketFromSysEx(b[2:])
	if decoded == nil {
		t.Fatal("FileDumpDataPacketFromSysEx returned nil")
	}
	if decoded.DeviceId != pkt.DeviceId || decoded.Packet != pkt.Packet {
		t.Errorf("DeviceId/Packet mismatch: got %+v, want %+v", decoded, pkt)
	}
	if !bytes.Equal(decoded.Data, pkt.Data) {
		t.Errorf("Data mismatch: got %v, want %v", decoded.Data, pkt.Data)
	}
}

func TestFileDumpDataPacketFromSysExBadChecksum(t *testing.T) {
	// MIDI_Specification.pdf, File Dump: build a valid packet before corrupting its checksum.
	data := []byte{0x01, 0x02, 0x03}
	pkt := &FileDumpDataPacket{DeviceId: 1, Packet: 0, Data: data}
	b, err := pkt.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	// MIDI_Specification.pdf, File Dump: the checksum is the byte immediately before F7.
	b[len(b)-2] ^= 0xff
	decoded := FileDumpDataPacketFromSysEx(b[2:])
	if decoded != nil {
		t.Error("expected nil for bad checksum")
	}
}
