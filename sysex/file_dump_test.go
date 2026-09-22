package sysex

import (
	"testing"
)

func TestFileDumpDataPacketRoundTrip(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	pkt := &FileDumpDataPacket{DeviceId: 1, Packet: 0, Data: data}
	b, err := pkt.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	decoded := FileDumpDataPacketFromSysEx(b)
	if decoded == nil {
		t.Fatal("FileDumpDataPacketFromSysEx returned nil")
	}
	if decoded.DeviceId != pkt.DeviceId || decoded.Packet != pkt.Packet {
		t.Errorf("DeviceId/Packet mismatch: got %+v, want %+v", decoded, pkt)
	}
	if len(decoded.Data) != len(pkt.Data) {
		t.Errorf("Data length mismatch: got %d, want %d", len(decoded.Data), len(pkt.Data))
	}
}

func TestFileDumpDataPacketFromSysExBadChecksum(t *testing.T) {
	// Construct valid packet then corrupt the checksum byte
	data := []byte{0x01, 0x02, 0x03}
	pkt := &FileDumpDataPacket{DeviceId: 1, Packet: 0, Data: data}
	b, err := pkt.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	// Corrupt the checksum byte (second-to-last byte before F7)
	b[len(b)-2] ^= 0xff
	decoded := FileDumpDataPacketFromSysEx(b)
	if decoded != nil {
		t.Error("expected nil for bad checksum")
	}
}
