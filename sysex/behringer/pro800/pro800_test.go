package pro800

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/sysex"
)

// The four dumps below were taken off a real Pro 800 and are the only test
// data here that came off an instrument rather than out of the notes. They
// cover a dump whose payload is a whole number of groups, one whose trailing
// group is five bytes, and three name lengths, so a packing that handled only
// the easy case or a name that assumed a fixed width would fail here.
var capturedDumps = []struct {
	name  string
	bytes []byte
	want  string
}{
	{
		name: "12TET",
		bytes: []byte{
			0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x78, 0x64, 0x00, 0x41, 0x25, 0x16, 0x61, 0x00,
			0x6e, 0x00, 0x00, 0x3a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x5b, 0x4c, 0x00, 0x12, 0x3b, 0x23, 0x00, 0x58, 0x7f, 0x21, 0x7f, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x52, 0x03, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x01,
			0x02, 0x02, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x78, 0x01, 0x00, 0x00, 0x7f,
			0x7f, 0x7f, 0x7f, 0x07, 0x7f, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x31, 0x32, 0x54, 0x45, 0x00, 0x54, 0x00, 0xf7,
		},
		want: "12TET",
	},
	{
		name: "Meantone",
		bytes: []byte{
			0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x78, 0x65, 0x00, 0x41, 0x25, 0x16, 0x61, 0x00,
			0x6e, 0x00, 0x00, 0x3a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x5b, 0x4c, 0x00, 0x12, 0x3b, 0x23, 0x00, 0x58, 0x7f, 0x21, 0x7f, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x52, 0x03, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x01,
			0x02, 0x02, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x78, 0x01, 0x00, 0x00, 0x7f,
			0x7f, 0x7f, 0x7f, 0x07, 0x7f, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x6b, 0x2e, 0x7f, 0x7c, 0x3e,
			0x62, 0x56, 0x1d, 0x09, 0x3d, 0x2b, 0x0c, 0x57, 0x3d, 0x1d, 0x3d, 0x06, 0x07, 0x3e, 0x67, 0x5a,
			0x03, 0x3d, 0x56, 0x4c, 0x47, 0x5e, 0x3e, 0x67, 0x5a, 0x03, 0x3d, 0x4f, 0x7c, 0x5b, 0x0d, 0x3e,
			0x2b, 0x0c, 0x57, 0x2d, 0x3d, 0x62, 0x56, 0x1d, 0x3d, 0x18, 0x51, 0x02, 0x35, 0x3e, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x4d, 0x65, 0x61, 0x6e, 0x00, 0x74, 0x6f, 0x6e, 0x65,
			0x00, 0xf7,
		},
		want: "Meantone",
	},
	{
		name: "Pythagorean",
		bytes: []byte{
			0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x78, 0x66, 0x00, 0x41, 0x25, 0x16, 0x61, 0x00,
			0x6e, 0x00, 0x00, 0x3a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x5b, 0x4c, 0x00, 0x12, 0x3b, 0x23, 0x00, 0x58, 0x7f, 0x21, 0x7f, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x52, 0x03, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x01,
			0x02, 0x02, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x78, 0x01, 0x00, 0x00, 0x7f,
			0x7f, 0x7f, 0x7f, 0x07, 0x7f, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x1c, 0x2b, 0x0c, 0x57, 0x3d,
			0x2a, 0x50, 0x2c, 0x3a, 0x3d, 0x08, 0x5b, 0x09, 0x3d, 0x53, 0x13, 0x39, 0x28, 0x3d, 0x4a, 0x4a,
			0x34, 0x3c, 0x65, 0x34, 0x1e, 0x02, 0x3e, 0x4a, 0x4a, 0x34, 0x3c, 0x5d, 0x53, 0x13, 0x28, 0x3d,
			0x08, 0x5b, 0x09, 0x12, 0x3d, 0x2a, 0x50, 0x2c, 0x3d, 0x2b, 0x0c, 0x01, 0x57, 0x3d, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x50, 0x79, 0x74, 0x68, 0x00, 0x61, 0x67, 0x6f, 0x72,
			0x65, 0x61, 0x6e, 0x00, 0x00, 0xf7,
		},
		want: "Pythagorean",
	},
	{
		name: "Werckmeister",
		bytes: []byte{
			0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x78, 0x67, 0x00, 0x41, 0x25, 0x16, 0x61, 0x00,
			0x6e, 0x00, 0x00, 0x3a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x1c, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x5b, 0x4c, 0x00, 0x12, 0x3b, 0x23, 0x00, 0x58, 0x7f, 0x21, 0x7f, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x52, 0x03, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x00, 0x7f, 0x7f, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x01,
			0x02, 0x02, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x78, 0x01, 0x00, 0x00, 0x7f,
			0x7f, 0x7f, 0x7f, 0x07, 0x7f, 0x7f, 0x7f, 0x00, 0x00, 0x00, 0x00, 0x5c, 0x2b, 0x0c, 0x57, 0x3d,
			0x53, 0x13, 0x28, 0x1b, 0x3d, 0x08, 0x5b, 0x09, 0x3d, 0x2b, 0x0c, 0x3b, 0x57, 0x3d, 0x4a, 0x4a,
			0x34, 0x3c, 0x65, 0x4c, 0x1e, 0x02, 0x3e, 0x2a, 0x50, 0x2c, 0x3d, 0x0d, 0x53, 0x13, 0x28, 0x3d,
			0x65, 0x1e, 0x02, 0x33, 0x3e, 0x2a, 0x50, 0x2c, 0x3d, 0x53, 0x13, 0x03, 0x28, 0x3d, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x57, 0x65, 0x72, 0x63, 0x00, 0x6b, 0x6d, 0x65, 0x69,
			0x73, 0x74, 0x65, 0x00, 0x72, 0x00, 0xf7,
		},
		want: "Werckmeister",
	},
}

// A dump taken off an instrument and encoded again has to come back as the
// same bytes. Every field is read out of the dump and written back, so this
// fails on a wrong offset, a swapped byte order, a name terminator handled as
// a fixed width and a packing that mishandles the trailing group alike.
func TestCapturedDumpsRoundTrip(t *testing.T) {
	for _, dump := range capturedDumps {
		t.Run(dump.name, func(t *testing.T) {
			var data PatchData
			if err := data.UnmarshalBinary(dump.bytes); err != nil {
				t.Fatalf("UnmarshalBinary: %v", err)
			}
			if data.Patch.Name != dump.want {
				t.Errorf("name is %q, want %q", data.Patch.Name, dump.want)
			}
			got, err := data.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}
			if !bytes.Equal(got, dump.bytes) {
				t.Errorf("re-encoded to\n% x\nwant\n% x", got, dump.bytes)
			}
		})
	}
}

// The address in the dump response has to come back as the bank and program
// the panel shows. The four captures are patches 100 to 103, which is the
// first patch of bank B onwards.
func TestCapturedDumpAddresses(t *testing.T) {
	for i, dump := range capturedDumps {
		var data PatchData
		if err := data.UnmarshalBinary(dump.bytes); err != nil {
			t.Fatalf("%s: UnmarshalBinary: %v", dump.name, err)
		}
		if want := 100 + i; data.Address != want {
			t.Errorf("%s: address is %d, want %d", dump.name, data.Address, want)
		}
	}
}

// The tuning table of the meantone capture is byte for byte the quarter-comma
// example the notes publish, which pins the offset of the table, its four
// byte width, its little endian order and the sign bit against a table this
// implementation did not derive from.
func TestTuningTableMatchesPublishedExample(t *testing.T) {
	var data PatchData
	if err := data.UnmarshalBinary(capturedDumps[1].bytes); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	want := [12]int32{
		0,
		-1099104338,
		-1113729438,
		1037503531,
		-1106821859,
		1023630055,
		-1101117610,
		-1123853593,
		-1097999364,
		-1109980117,
		1033754210,
		-1103801960,
	}
	if data.Patch.Tuning != want {
		t.Errorf("tuning is %v, want %v", data.Patch.Tuning, want)
	}
}

// A 12TET tuning table is all zeros, and the two dumps either side of it
// differ only in the table, which is what makes them usable as a pair.
func TestTwelveToneTableIsUndetuned(t *testing.T) {
	var data PatchData
	if err := data.UnmarshalBinary(capturedDumps[0].bytes); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	for i, value := range data.Patch.Tuning {
		if value != 0 {
			t.Errorf("note %d is tuned by %d, want 0", i, value)
		}
	}
}

// The patch name starts at byte 150, which the captures confirm: every one of
// them decodes its name correctly only if the parameters are read to that
// offset and no further.
func TestNameStartsAtParameterEnd(t *testing.T) {
	for _, dump := range capturedDumps {
		var data PatchData
		if err := data.UnmarshalBinary(dump.bytes); err != nil {
			t.Fatalf("%s: UnmarshalBinary: %v", dump.name, err)
		}
		if data.Patch.Name != dump.want {
			t.Errorf("%s: name is %q, want %q", dump.name, data.Patch.Name, dump.want)
		}
	}
}

// A request and its response share one address written low byte first. The
// notes give 7E 03 for the system block, which is 510, and the captures give
// the patches that follow A-00.
func TestAddressEncoding(t *testing.T) {
	tests := []struct {
		address int
		want    []byte
	}{
		{0, []byte{0x00, 0x00}},
		{1, []byte{0x01, 0x00}},
		{100, []byte{0x64, 0x00}},
		{103, []byte{0x67, 0x00}},
		{399, []byte{0x0f, 0x03}},
		{510, []byte{0x7e, 0x03}},
	}
	for _, test := range tests {
		got, err := (&DumpRequest{Address: test.address}).MarshalBinary()
		if err != nil {
			t.Fatalf("address %d: %v", test.address, err)
		}
		want := append([]byte{0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x77}, test.want...)
		want = append(want, 0xf7)
		if !bytes.Equal(got, want) {
			t.Errorf("address %d encoded to % x, want % x", test.address, got, want)
		}
		var decoded DumpRequest
		if err := decoded.UnmarshalBinary(got); err != nil {
			t.Fatalf("address %d: UnmarshalBinary: %v", test.address, err)
		}
		if decoded.Address != test.address {
			t.Errorf("address %d decoded as %d", test.address, decoded.Address)
		}
	}
}

func TestRequestRoundTrip(t *testing.T) {
	request := &VersionRequest{}
	got, err := request.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	want := []byte{0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x08, 0x00, 0xf7}
	if !bytes.Equal(got, want) {
		t.Errorf("version request is % x, want % x", got, want)
	}
	var decoded VersionRequest
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
}

// The instrument's answer to a version request is 09 00 followed by three
// decimal numbers, sent unpacked unlike every other payload in the protocol.
func TestVersionResponseRoundTrip(t *testing.T) {
	version := VersionResponse{Major: 1, Minor: 4, Patch: 6}
	got, err := version.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	want := []byte{0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x09, 0x00, 0x01, 0x04, 0x06, 0xf7}
	if !bytes.Equal(got, want) {
		t.Errorf("version response is % x, want % x", got, want)
	}
	var decoded VersionResponse
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded != version {
		t.Errorf("decoded %+v, want %+v", decoded, version)
	}
}

func TestUnsupportedAddressRoundTrip(t *testing.T) {
	got, err := (&UnsupportedAddress{}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	want := []byte{0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, 0x01, 0x00, 0x01, 0xf7}
	if !bytes.Equal(got, want) {
		t.Errorf("refusal is % x, want % x", got, want)
	}
	var decoded UnsupportedAddress
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
}

// The system settings block is carried rather than modelled, so it still has
// to survive a round trip: archiving a settings block and writing it back is
// the reason a dump is wanted in the first place.
func TestSystemDataRoundTrip(t *testing.T) {
	raw := []byte{0x25, 0x16, 0x61, 0x00, 0x6f, 0x7c, 0x00, 0x44, 0x02, 0x02, 0x7f}
	system := SystemData{Address: SystemAddress, Raw: raw}
	got, err := system.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var decoded SystemData
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded.Address != SystemAddress {
		t.Errorf("address is %d, want %d", decoded.Address, SystemAddress)
	}
	if !bytes.Equal(decoded.Raw, raw) {
		t.Errorf("payload is % x, want % x", decoded.Raw, raw)
	}
	again, err := decoded.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(again, got) {
		t.Errorf("re-encoded to % x, want % x", again, got)
	}
}

// The packing is the one piece of the format with no field to check it
// against, so it is checked against the sizes the four captures imply: a 156,
// 159, 162 and 163 byte patch has to pack into 179, 182, 186 and 187 bytes.
func TestPackedSizeMatchesCaptures(t *testing.T) {
	tests := []struct {
		size int
		want int
	}{
		{156, 179},
		{159, 182},
		{162, 186},
		{163, 187},
		{173, 198},
		{154, 176},
		{0, 0},
	}
	for _, test := range tests {
		if got := packedSize(test.size); got != test.want {
			t.Errorf("packedSize(%d) = %d, want %d", test.size, got, test.want)
		}
		if got := decodedSize(test.want); got != test.size {
			t.Errorf("decodedSize(%d) = %d, want %d", test.want, got, test.size)
		}
	}
}

// Every byte of a packed payload has to survive the round trip, at every
// offset within its group, because a packing that loses one bit only shows up
// on the values that happen to use it.
func TestPayloadPackingIsLossless(t *testing.T) {
	for size := range 200 {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i*37 + 11)
		}
		packed := packPayload(payload)
		if len(packed) != packedSize(size) {
			t.Fatalf("size %d packed to %d bytes, want %d", size, len(packed), packedSize(size))
		}
		unpacked, err := unpackPayload(packed)
		if err != nil {
			t.Fatalf("size %d: unpackPayload: %v", size, err)
		}
		if !bytes.Equal(unpacked, payload) {
			t.Fatalf("size %d did not survive the round trip", size)
		}
	}
}

// The 6F layout pads the name to sixteen bytes and adds four settings, so a
// patch in it is a fixed length whatever its name.
func TestVersion6FRoundTrip(t *testing.T) {
	patch := Patch{
		Version: Version6F,
		Name:    "Poly Six F1A",
		OscA:    Oscillator{Frequency: 0x7c00, Volume: 0x7fff, Saw: 1, Triangle: 1, PitchMode: 2},
		OscB:    Oscillator{Frequency: 0x3d7f, Square: 1, Sync: 1, PitchMode: 2},
		Filter: Filter{
			Cutoff:    0x5778,
			Resonance: 0x772b,
			KeyTrack:  1,
			Envelope:  Envelope{Attack: 0x0e75, Decay: 0x6411, Sustain: 0x7738, Release: 0x7100, Speed: 1},
		},
		Amp: Amplifier{
			Velocity: 0x0000,
			Envelope: Envelope{Attack: 0x7f00, Decay: 0x1600, Sustain: 0x2100, Release: 0x0e00},
		},
		ArpMode:              2,
		Glide:                0,
		Unison:               0,
		NoiseLevel:           0,
		LfoAftertouch:        0x3f00,
		LfoAftertouchPresent: true,
		VoiceSpread:          1,
		TrackingReference:    3,
		GlideMode:            1,
		PitchBend:            PitchBend{Target: 1, Range: 0x6000},
	}
	data := PatchData{Address: 2, Patch: patch}
	got, err := data.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var decoded PatchData
	if err := decoded.UnmarshalBinary(got); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded.Patch != patch {
		t.Errorf("decoded %+v, want %+v", decoded.Patch, patch)
	}
	again, err := decoded.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(again, got) {
		t.Errorf("re-encoded to % x, want % x", again, got)
	}
}

// A 6E patch may or may not carry the LFO aftertouch amount after its name,
// and which it is has to come back out of the message rather than out of a
// default, or a dump taken from one instrument would be rewritten to a
// length the other one did not send.
func TestVersion6EAftertouchAmountIsOptional(t *testing.T) {
	for _, present := range []bool{false, true} {
		patch := Patch{
			Version:              Version6E,
			Name:                 "Short",
			LfoAftertouchPresent: present,
		}
		if present {
			patch.LfoAftertouch = 0x1234
		}
		data := PatchData{Address: 0, Patch: patch}
		got, err := data.MarshalBinary()
		if err != nil {
			t.Fatalf("present %t: MarshalBinary: %v", present, err)
		}
		var decoded PatchData
		if err := decoded.UnmarshalBinary(got); err != nil {
			t.Fatalf("present %t: UnmarshalBinary: %v", present, err)
		}
		if decoded.Patch.LfoAftertouchPresent != present {
			t.Errorf("present %t decoded as %t", present, decoded.Patch.LfoAftertouchPresent)
		}
		if present && decoded.Patch.LfoAftertouch != 0x1234 {
			t.Errorf("aftertouch amount is %#x, want 0x1234", decoded.Patch.LfoAftertouch)
		}
	}
}

// A name is sixteen bytes wide in the 6F layout, and a longer one would be
// written over the settings that follow it.
func TestRejectsNamesThatDoNotFit(t *testing.T) {
	patch := Patch{Version: Version6F, Name: "seventeen chars!!"}
	if _, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary(); err == nil {
		t.Error("accepted a name longer than the field")
	}
	patch.Name = "café"
	if _, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary(); err == nil {
		t.Error("accepted a name that is not eight bit text")
	}
}

// A parameter outside the range the instrument accepts would be silently
// clamped on the way in, so it is refused here instead.
func TestRejectsOutOfRangeParameters(t *testing.T) {
	tests := []struct {
		name  string
		patch Patch
	}{
		{"version", Patch{Version: 0x6d}},
		{"oscillator level", Patch{Version: Version6E, OscA: Oscillator{Volume: 65536}}},
		{"oscillator shape", Patch{Version: Version6E, OscA: Oscillator{Saw: 2}}},
		{"envelope", Patch{Version: Version6E, Filter: Filter{Envelope: Envelope{Attack: 70000}}}},
		{"key track", Patch{Version: Version6E, Filter: Filter{KeyTrack: 3}}},
		{"pitch bend target", Patch{Version: Version6E, PitchBend: PitchBend{Target: 4}}},
		{"arp mode", Patch{Version: Version6E, ArpMode: 7}},
		{"chord note", Patch{Version: Version6E, Chord: [8]int{256}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&PatchData{Address: 0, Patch: test.patch}).MarshalBinary(); err == nil {
				t.Error("accepted a parameter outside its range")
			}
		})
	}
}

// A message with a different manufacturer, a different device ID or a
// different function ID is some other instrument's, and a truncated one is
// not a message at all.
func TestUnmarshalRejectsMalformedMessages(t *testing.T) {
	valid := capturedDumps[0].bytes
	replace := func(at int, values ...byte) []byte {
		out := append([]byte(nil), valid...)
		copy(out[at:], values)
		return out
	}
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"too short", valid[:6], sysex.ErrBadRange},
		{"not a sysex", replace(0, 0xf1), sysex.ErrBadHeader},
		{"no end", valid[:len(valid)-1], sysex.ErrBadHeader},
		{"another manufacturer", replace(1, 0x40), sysex.ErrBadHeader},
		{"another device", replace(4, 0x00, 0x01, 0x25, 0x00), sysex.ErrBadHeader},
		{"another function", replace(8, 0x77), sysex.ErrBadHeader},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var data PatchData
			if err := data.UnmarshalBinary(test.data); err != test.want {
				t.Errorf("UnmarshalBinary = %v, want %v", err, test.want)
			}
		})
	}
}

// A payload carrying an eighth bit set is not packed at all, so decoding it
// would build a patch out of bytes the instrument never sent.
func TestRejectsUnpackedPayload(t *testing.T) {
	broken := append([]byte(nil), capturedDumps[0].bytes...)
	broken[11] |= 0x80
	var data PatchData
	if err := data.UnmarshalBinary(broken); err != sysex.ErrBadRange {
		t.Errorf("UnmarshalBinary = %v, want %v", err, sysex.ErrBadRange)
	}
}

// A dump whose version byte is neither of the two the notes describe is not a
// patch, and its length would not tell a reader where the name ends.
func TestRejectsUnknownVersion(t *testing.T) {
	patch := Patch{Version: 0x6d, Name: "Short"}
	if _, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary(); err == nil {
		t.Error("encoded a patch whose version is not one of the two known layouts")
	}
}

// A value the dump layout has nowhere to put must be refused, because
// dropping it hands back a message that says something the caller never
// asked for. These four are the cases: a setting the older layout lacks, and
// the two fields that belong to oscillator B alone.
func TestRejectsValuesWithNoPlaceInTheLayout(t *testing.T) {
	tests := []struct {
		name  string
		patch Patch
	}{
		{"oscillator A fine", Patch{Version: Version6E, Name: "x", OscA: Oscillator{Fine: 1}}},
		{"oscillator A sync", Patch{Version: Version6E, Name: "x", OscA: Oscillator{Sync: 1}}},
		{"voice spread in 6E", Patch{Version: Version6E, Name: "x", VoiceSpread: 1}},
		{"tracking reference in 6E", Patch{Version: Version6E, Name: "x", TrackingReference: 3}},
		{"glide mode in 6E", Patch{Version: Version6E, Name: "x", GlideMode: 1}},
		{"pitch bend range in 6E", Patch{Version: Version6E, Name: "x", PitchBend: PitchBend{Target: 1, Range: 0x6000}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (&PatchData{Address: 0, Patch: test.patch}).MarshalBinary()
			if err != ErrNotRepresentable {
				t.Errorf("MarshalBinary = %v, want %v", err, ErrNotRepresentable)
			}
		})
	}
}

// The same fields are fine in the layout that has room for them, so the
// check above is about the layout rather than about the values.
func TestVersion6FCarriesTheSettingsTheOlderLayoutLacks(t *testing.T) {
	patch := Patch{
		Version:              Version6F,
		Name:                 "x",
		VoiceSpread:          1,
		TrackingReference:    3,
		GlideMode:            1,
		PitchBend:            PitchBend{Target: 1, Range: 0x6000},
		LfoAftertouch:        0x1000,
		LfoAftertouchPresent: true,
	}
	if _, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary(); err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
}

// A patch dump and the settings dump share one function ID, so the address is
// the only thing that tells them apart. Reading one as the other must fail
// rather than produce a settings block made of a patch.
func TestPatchAndSystemDumpsAreNotInterchangeable(t *testing.T) {
	var system SystemData
	if err := system.UnmarshalBinary(capturedDumps[0].bytes); !errors.Is(err, sysex.ErrBadRange) {
		t.Errorf("a patch dump read as settings = %v, want %v", err, sysex.ErrBadRange)
	}
	settings, err := (&SystemData{Address: SystemAddress, Raw: []byte{0x25, 0x16, 0x61, 0x00, 0x6f, 0x7c, 0x00, 0x44, 0x02, 0x02, 0x7f}}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var patch PatchData
	if err := patch.UnmarshalBinary(settings); !errors.Is(err, sysex.ErrBadRange) {
		t.Errorf("a settings dump read as a patch = %v, want %v", err, sysex.ErrBadRange)
	}
}

// A payload one byte past a whole group is an eighth-bits byte with nothing
// after it, which is not a group. Accepting it would drop a byte the
// instrument claims to have sent.
func TestRejectsOrphanedEighthBitsByte(t *testing.T) {
	valid := packPayload(make([]byte, groupSize))
	orphan := append(append([]byte(nil), valid...), 0x00)
	if _, err := unpackPayload(orphan); err != sysex.ErrBadRange {
		t.Errorf("unpackPayload = %v, want %v", err, sysex.ErrBadRange)
	}
	if _, err := unpackPayload(valid); err != nil {
		t.Errorf("unpackPayload on a valid payload = %v", err)
	}
}

// A caller reading from a port needs the dispatcher, because the function ID
// alone does not say which of the two dump responses a message is.
func TestDecodeRoutesByFunctionAndAddress(t *testing.T) {
	request, err := (&DumpRequest{Address: SystemAddress}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	version, err := (&VersionResponse{Major: 1, Minor: 4, Patch: 6}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	refusal, err := (&UnsupportedAddress{}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	tests := []struct {
		name string
		data []byte
		want any
	}{
		{"dump request", request, &DumpRequest{}},
		{"version response", version, &VersionResponse{}},
		{"refusal", refusal, &UnsupportedAddress{}},
		{"patch dump", capturedDumps[0].bytes, &PatchData{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Decode(test.data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got == nil {
				t.Fatal("Decode returned nothing")
			}
			if reflect.TypeOf(got) != reflect.TypeOf(test.want) {
				t.Errorf("Decode returned %T, want %T", got, test.want)
			}
		})
	}
	settings, err := (&SystemData{Address: SystemAddress, Raw: []byte{0x25, 0x16, 0x61, 0x00, 0x6f, 0x7c, 0x00}}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	got, err := Decode(settings)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, ok := got.(*SystemData); !ok {
		t.Errorf("Decode returned %T, want *SystemData", got)
	}
}

// A message the instrument sends that this package does not model is not a
// fault, so the dispatcher passes it over rather than failing.
func TestDecodeIgnoresUnmodelledMessages(t *testing.T) {
	for _, function := range []int{0x0a, 0x0b, 0x38} {
		data := []byte{0xf0, 0x00, 0x20, 0x32, 0x00, 0x01, 0x24, 0x00, byte(function), 0x00, 0x00, 0xf7}
		got, err := Decode(data)
		if err != nil || got != nil {
			t.Errorf("Decode(0x%02x) = %v, %v, want nil, nil", function, got, err)
		}
	}
}

// PatchCount is what a caller uses to ask whether an address is a patch, and
// the range tag on PatchData is what enforces it. Nothing makes the two
// agree except this.
func TestPatchCountMatchesTheAddressRangeTag(t *testing.T) {
	last := &PatchData{Address: PatchCount - 1, Patch: Patch{Version: Version6E, Name: "x"}}
	if _, err := last.MarshalBinary(); err != nil {
		t.Errorf("address %d was refused: %v", PatchCount-1, err)
	}
	past := &PatchData{Address: PatchCount, Patch: Patch{Version: Version6E, Name: "x"}}
	if _, err := past.MarshalBinary(); err == nil {
		t.Errorf("address %d was accepted, want it past the last patch", PatchCount)
	}
}

// A 6E dump carries the aftertouch amount only when it has one, so a value
// set without the flag saying so would be written nowhere at all. Zero is a
// legitimate amount, which is why the flag exists rather than a sentinel.
func TestRejectsAftertouchAmountWithoutTheFlag(t *testing.T) {
	patch := Patch{Version: Version6E, Name: "x", LfoAftertouch: 0x1234}
	_, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary()
	if err != ErrNotRepresentable {
		t.Errorf("MarshalBinary = %v, want %v", err, ErrNotRepresentable)
	}
	patch.LfoAftertouchPresent = true
	encoded, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var decoded PatchData
	if err := decoded.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if decoded.Patch.LfoAftertouch != 0x1234 {
		t.Errorf("aftertouch amount is %#x, want 0x1234", decoded.Patch.LfoAftertouch)
	}
}

// fullPatch returns a patch with every field the given layout can carry set
// to a value of its own, so that a field the encoder writes nowhere or the
// decoder never reads back cannot hide.
func fullPatch(version int) Patch {
	patch := Patch{
		Version: version,
		OscA: Oscillator{
			Frequency:  1001,
			Volume:     1002,
			PulseWidth: 1003,
			Saw:        1,
			Triangle:   1,
			PitchMode:  2,
		},
		OscB: Oscillator{
			Frequency:  2001,
			Volume:     2002,
			PulseWidth: 2003,
			Fine:       2004,
			Saw:        1,
			Triangle:   1,
			Square:     1,
			Sync:       1,
			PitchMode:  3,
		},
		Filter: Filter{
			Envelope: Envelope{
				Attack: 3001, Decay: 3002, Sustain: 3003, Release: 3004,
				Speed: 1, Shape: 1,
			},
			Cutoff:     3005,
			Resonance:  3006,
			Amount:     3007,
			KeyTrack:   2,
			Velocity:   3008,
			Aftertouch: 3009,
		},
		Amp: Amplifier{
			Envelope: Envelope{
				Attack: 4001, Decay: 4002, Sustain: 4003, Release: 4004,
				Speed: 1, Shape: 1,
			},
			Velocity:   4005,
			Aftertouch: 4006,
		},
		PolyMod: PolyMod{
			FilterEnvAmount:   5001,
			OscillatorBAmount: 5002,
			SourceFrequencyA:  1,
			DestinationFilter: 1,
		},
		Lfo: Lfo{
			Frequency: 6001,
			Amount:    6002,
			Shape:     5,
			Speed:     1,
			Target:    0x3f,
			ModDelay:  6003,
		},
		Vibrato:              Vibrato{Speed: 7001, Amount: 7002},
		ModWheel:             ModWheel{Range: 3, Target: 1},
		PitchBend:            PitchBend{Target: 3},
		Glide:                8001,
		Unison:               1,
		UnisonDetune:         8002,
		Reserved:             0xab,
		NoiseLevel:           8003,
		ArpMode:              6,
		Name:                 "Every Field",
		LfoAftertouch:        0xbeef,
		LfoAftertouchPresent: true,
	}
	for i := range patch.Chord {
		patch.Chord[i] = 0xff - i
	}
	for i := range patch.Tuning {
		// Alternating sign, so a sign mishandled on either side shows up.
		patch.Tuning[i] = int32(9001+i) * int32(1-2*(i%2))
	}
	if version == Version6F {
		patch.PitchBend.Range = 0x6000
		patch.VoiceSpread = 1
		patch.TrackingReference = 3
		patch.GlideMode = 1
	}
	return patch
}

// A field the encoder writes nowhere, or that the decoder never reads back,
// makes a fully populated patch come back different. This is the check that
// the two halves of the model agree, and it is what caught the velocities
// being decoded but not written.
func TestEveryFieldSurvivesTheRoundTrip(t *testing.T) {
	for _, version := range []int{Version6E, Version6F} {
		t.Run(fmt.Sprintf("version %#x", version), func(t *testing.T) {
			want := fullPatch(version)
			encoded, err := (&PatchData{Address: 3, Patch: want}).MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}
			var got PatchData
			if err := got.UnmarshalBinary(encoded); err != nil {
				t.Fatalf("UnmarshalBinary: %v", err)
			}
			if got.Patch != want {
				t.Errorf("round trip changed the patch:\n got %+v\nwant %+v", got.Patch, want)
			}
			again, err := got.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}
			if !bytes.Equal(again, encoded) {
				t.Error("encoding is not idempotent")
			}
		})
	}
}

// A patch name of every length the field allows has to survive, because the
// older layout's name is what decides the length of the whole message.
func TestNameLengthsSurviveTheRoundTrip(t *testing.T) {
	for _, version := range []int{Version6E, Version6F} {
		for length := range nameSize + 1 {
			want := fullPatch(version)
			want.Name = strings.Repeat("n", length)
			encoded, err := (&PatchData{Address: 0, Patch: want}).MarshalBinary()
			if err != nil {
				t.Fatalf("version %#x length %d: MarshalBinary: %v", version, length, err)
			}
			var got PatchData
			if err := got.UnmarshalBinary(encoded); err != nil {
				t.Fatalf("version %#x length %d: UnmarshalBinary: %v", version, length, err)
			}
			if got.Patch.Name != want.Name {
				t.Errorf("version %#x length %d: name is %q, want %q", version, length, got.Patch.Name, want.Name)
			}
		}
	}
}

// Every envelope value is a sixteen bit field at a different offset, so each
// is filled to a value that would be visible if it landed on its neighbour.
func TestEnvelopesDoNotOverlap(t *testing.T) {
	patch := fullPatch(Version6F)
	patch.Filter.Attack, patch.Amp.Attack = 0x1111, 0x2222
	patch.Filter.Decay, patch.Amp.Decay = 0x3333, 0x4444
	patch.Filter.Sustain, patch.Amp.Sustain = 0x5555, 0x6666
	patch.Filter.Release, patch.Amp.Release = 0x7777, 0x8888
	encoded, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var got PatchData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if got.Patch != patch {
		t.Errorf("round trip changed the patch:\n got %+v\nwant %+v", got.Patch, patch)
	}
}

// The flag that says whether a dump carries the aftertouch amount only means
// anything in the older layout. The newer one always does, so decoding a 6F
// dump always reports it present, and a hand-built patch that left it unset
// comes back with it set. The message is unaffected; only the struct is.
func TestAftertouchFlagNormalisesInTheNewerLayout(t *testing.T) {
	patch := fullPatch(Version6F)
	patch.LfoAftertouch = 0
	patch.LfoAftertouchPresent = false
	encoded, err := (&PatchData{Address: 0, Patch: patch}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var got PatchData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if !got.Patch.LfoAftertouchPresent {
		t.Error("a 6F dump decoded as not carrying the aftertouch amount")
	}
	// The bytes must not depend on the flag: two patches that differ only in
	// it are the same patch, so the 6F layout writes the amount either way.
	again, err := got.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	if !bytes.Equal(again, encoded) {
		t.Error("re-encoding changed the message")
	}
}
