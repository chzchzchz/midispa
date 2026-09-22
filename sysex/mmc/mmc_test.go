package mmc

import (
	"testing"
)

// TestMarshalMMCCommand verifies the basic command byte format.
// Format: F0 7F <device_ID> 06 <command_code> F7
func TestMarshalMMCCommand(t *testing.T) {
	// STOP command on device 0x10
	bytes, err := MarshalMMCCommand(0x10, MMCStop)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x01, 0xf7}
	if len(bytes) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(bytes))
	}
	for i := range expected {
		if bytes[i] != expected[i] {
			t.Fatalf("STOP: byte %d: expected 0x%02x, got 0x%02x", i, expected[i], bytes[i])
		}
	}

	// PLAY command on device 0x7e (max valid device ID)
	bytes, err = MarshalMMCCommand(0x7e, MMCPlay)
	if err != nil {
		t.Fatal(err)
	}
	expected = []byte{0xf0, 0x7f, 0x7e, 0x06, 0x02, 0xf7}
	for i := range expected {
		if bytes[i] != expected[i] {
			t.Fatalf("PLAY: byte %d: expected 0x%02x, got 0x%02x", i, expected[i], bytes[i])
		}
	}

	// Invalid device ID should return error
	_, err = MarshalMMCCommand(0x7f, MMCStop)
	if err == nil {
		t.Fatal("expected error for device ID 0x7f")
	}
	_, err = MarshalMMCCommand(-1, MMCStop)
	if err == nil {
		t.Fatal("expected error for negative device ID")
	}
}

// TestMarshalMMCCommandWithData verifies the command-with-data byte format.
// Format: F0 7F <device_ID> 06 <command_code> <count> <data...> F7
func TestMarshalMMCCommandWithData(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	bytes, err := MarshalMMCCommandWithData(0x10, MMCPlay, data)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x02, 0x03, 0x01, 0x02, 0x03, 0xf7}
	if len(bytes) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(bytes))
	}
	for i := range expected {
		if bytes[i] != expected[i] {
			t.Fatalf("byte %d: expected 0x%02x, got 0x%02x", i, expected[i], bytes[i])
		}
	}
}

// TestMarshalBinary verifies that each command type's MarshalBinary
// returns the correct byte sequence per the spec (Section 2).
func TestMarshalBinary(t *testing.T) {
	tests := []struct {
		name     string
		cmd      CommandMarshaler
		deviceID int
		code     byte
	}{
		{"Stop", &Stop{DeviceId: 0x10}, 0x10, MMCStop},
		{"Play", &Play{DeviceId: 0x10}, 0x10, MMCPlay},
		{"Rewind", &Rewind{DeviceId: 0x10}, 0x10, MMCRewind},
		{"FastForward", &FastForward{DeviceId: 0x10}, 0x10, MMCFastForward},
		{"RecordStrobe", &RecordStrobe{DeviceId: 0x10}, 0x10, MMCRecordStrobe},
		{"RecordExit", &RecordExit{DeviceId: 0x10}, 0x10, MMCRecordExit},
		{"Eject", &Eject{DeviceId: 0x10}, 0x10, MMCEject},
		{"DeferredPlay", &DeferredPlay{DeviceId: 0x10}, 0x10, MMCDeferredPlay},
		{"RecordPause", &RecordPause{DeviceId: 0x10}, 0x10, MMCRecordPause},
		{"Pause", &Pause{DeviceId: 0x10}, 0x10, MMCPause},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify MarshalBinary exists and returns correct bytes
			b, err := tt.cmd.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			if len(b) != 6 {
				t.Fatalf("expected 6 bytes, got %d", len(b))
			}
			if b[0] != 0xf0 || b[1] != 0x7f || b[3] != 0x06 || b[5] != 0xf7 {
				t.Fatalf("bad framing: %v", b)
			}
			if b[2] != byte(tt.deviceID) {
				t.Fatalf("expected device 0x%02x, got 0x%02x", tt.deviceID, b[2])
			}
			if b[4] != tt.code {
				t.Fatalf("expected code 0x%02x, got 0x%02x", tt.code, b[4])
			}
		})
	}
}

// TestWaitMarshalBinary verifies the WAIT handshake command encoding.
// Per the spec, WAIT is command code 0x7C.
func TestWaitMarshalBinary(t *testing.T) {
	w := &Wait{DeviceId: 0x10}
	b, err := w.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x7c, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("WAIT byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestResumeMarshalBinary verifies the RESUME handshake command encoding.
// Per the spec, RESUME is command code 0x7D.
func TestResumeMarshalBinary(t *testing.T) {
	r := &Resume{DeviceId: 0x10}
	b, err := r.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x7d, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("RESUME byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestCommandConstants verifies that the command code constants match
// the values defined in the MMC spec (RP-013 v1.0).
func TestCommandConstants(t *testing.T) {
	if MMCStop != 1 {
		t.Errorf("MMCStop = %d, want 1", MMCStop)
	}
	if MMCPlay != 2 {
		t.Errorf("MMCPlay = %d, want 2", MMCPlay)
	}
	if MMCDeferredPlay != 3 {
		t.Errorf("MMCDeferredPlay = %d, want 3", MMCDeferredPlay)
	}
	if MMCFastForward != 4 {
		t.Errorf("MMCFastForward = %d, want 4", MMCFastForward)
	}
	if MMCRewind != 5 {
		t.Errorf("MMCRewind = %d, want 5", MMCRewind)
	}
	if MMCRecordStrobe != 6 {
		t.Errorf("MMCRecordStrobe = %d, want 6", MMCRecordStrobe)
	}
	if MMCRecordExit != 7 {
		t.Errorf("MMCRecordExit = %d, want 7", MMCRecordExit)
	}
	if MMCRecordPause != 8 {
		t.Errorf("MMCRecordPause = %d, want 8", MMCRecordPause)
	}
	if MMCPause != 9 {
		t.Errorf("MMCPause = %d, want 9", MMCPause)
	}
	if MMCEject != 0x0a {
		t.Errorf("MMCEject = 0x%02x, want 0x0a", MMCEject)
	}
	if MMCWait != 0x7c {
		t.Errorf("MMCWait = 0x%02x, want 0x7c", MMCWait)
	}
	if MMCResume != 0x7d {
		t.Errorf("MMCResume = 0x%02x, want 0x7d", MMCResume)
	}
}

// TestPhase2CommandConstants verifies Phase 2 command code constants.
func TestPhase2CommandConstants(t *testing.T) {
	if MMCChase != 0x0b {
		t.Errorf("MMCChase = 0x%02x, want 0x0b", MMCChase)
	}
	if MMCCommandErrorReset != 0x0c {
		t.Errorf("MMCCommandErrorReset = 0x%02x, want 0x0c", MMCCommandErrorReset)
	}
	if MMCReset != 0x0d {
		t.Errorf("MMCReset = 0x%02x, want 0x0d", MMCReset)
	}
	if MMCWrite != 0x0e {
		t.Errorf("MMCWrite = 0x%02x, want 0x0e", MMCWrite)
	}
	if MMCRead != 0x10 {
		t.Errorf("MMCRead = 0x%02x, want 0x10", MMCRead)
	}
	if MMCUpdate != 0x11 {
		t.Errorf("MMCUpdate = 0x%02x, want 0x11", MMCUpdate)
	}
	if MMCLocate != 0x12 {
		t.Errorf("MMCLocate = 0x%02x, want 0x12", MMCLocate)
	}
	if MMCSearch != 0x13 {
		t.Errorf("MMCSearch = 0x%02x, want 0x13", MMCSearch)
	}
	if MMCShuttle != 0x14 {
		t.Errorf("MMCShuttle = 0x%02x, want 0x14", MMCShuttle)
	}
	if MMCVariablePlay != 0x15 {
		t.Errorf("MMCVariablePlay = 0x%02x, want 0x15", MMCVariablePlay)
	}
	if MMCStep != 0x16 {
		t.Errorf("MMCStep = 0x%02x, want 0x16", MMCStep)
	}
	if MMCDeferredVariablePlay != 0x22 {
		t.Errorf("MMCDeferredVariablePlay = 0x%02x, want 0x22", MMCDeferredVariablePlay)
	}
	if MMCRecordStrobeVariable != 0x23 {
		t.Errorf("MMCRecordStrobeVariable = 0x%02x, want 0x23", MMCRecordStrobeVariable)
	}
}

// TestChaseMarshalBinary verifies CHASE encodes correctly.
func TestChaseMarshalBinary(t *testing.T) {
	c := &Chase{DeviceId: 0x10}
	b, err := c.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x0b, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("CHASE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestResetMarshalBinary verifies MMC RESET encodes correctly.
func TestResetMarshalBinary(t *testing.T) {
	r := &Reset{DeviceId: 0x10}
	b, err := r.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x0d, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("RESET byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestLocateIFMarshalBinary verifies LOCATE [I/F] encodes correctly.
func TestLocateIFMarshalBinary(t *testing.T) {
	l := &LocateIF{DeviceId: 0x10, Name: 0x08}
	b, err := l.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x12, 0x02, 0x00, 0x08, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("LOCATE[I/F] byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestLocateTargetMarshalBinary verifies LOCATE [TARGET] encodes correctly.
func TestLocateTargetMarshalBinary(t *testing.T) {
	lt := &LocateTarget{DeviceId: 0x10, TimeCode: [5]byte{0x01, 0x02, 0x03, 0x04, 0x05}}
	b, err := lt.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x12, 0x06, 0x01, 0x01, 0x02, 0x03, 0x04, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("LOCATE[TARGET] byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestSearchMarshalBinary verifies SEARCH encodes correctly.
func TestSearchMarshalBinary(t *testing.T) {
	s := &Search{DeviceId: 0x10, Speed: StandardSpeed{0x01, 0x02, 0x03}}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x13, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("SEARCH byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestVariablePlayMarshalBinary verifies VARIABLE PLAY encodes correctly.
func TestVariablePlayMarshalBinary(t *testing.T) {
	v := &VariablePlay{DeviceId: 0x10, Speed: StandardSpeed{0x01, 0x02, 0x03}}
	b, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x15, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("VARIABLE PLAY byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestStepMarshalBinary verifies STEP encodes correctly.
func TestStepMarshalBinary(t *testing.T) {
	s := &Step{DeviceId: 0x10, Steps: 0x05}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x16, 0x01, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("STEP byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestWriteMarshalBinary verifies WRITE encodes correctly.
func TestWriteMarshalBinary(t *testing.T) {
	w := &Write{DeviceId: 0x10, Pairs: []WritePair{
		{Name: 0x01, Data: []byte{0x02, 0x03}},
		{Name: 0x04, Data: []byte{0x05}},
	}}
	b, err := w.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x0e, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("WRITE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestReadMarshalBinary verifies READ encodes correctly.
func TestReadMarshalBinary(t *testing.T) {
	r := &Read{DeviceId: 0x10, Names: []byte{0x01, 0x02}}
	b, err := r.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x10, 0x02, 0x01, 0x02, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("READ byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestUpdateMarshalBinary verifies UPDATE [BEGIN] encodes correctly.
func TestUpdateMarshalBinary(t *testing.T) {
	u := &Update{DeviceId: 0x10, SubCmd: 0x00, Names: []byte{0x01, 0x02}}
	b, err := u.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x11, 0x03, 0x00, 0x01, 0x02, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("UPDATE[BEGIN] byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestShuttleMarshalBinary verifies SHUTTLE encodes correctly.
func TestShuttleMarshalBinary(t *testing.T) {
	s := &Shuttle{DeviceId: 0x10, Speed: StandardSpeed{0x01, 0x02, 0x03}}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x14, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("SHUTTLE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestMaskedWriteMarshalBinary verifies MASKED WRITE encodes correctly.
func TestMaskedWriteMarshalBinary(t *testing.T) {
	m := &MaskedWrite{DeviceId: 0x10, Name: 0x01, ByteNum: 0x02, Mask: 0x03, Data: 0x04}
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x0f, 0x04, 0x01, 0x02, 0x03, 0x04, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("MASKED WRITE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestDeferredVariablePlayMarshalBinary verifies DEFERRED VARIABLE PLAY encodes correctly.
func TestDeferredVariablePlayMarshalBinary(t *testing.T) {
	d := &DeferredVariablePlay{DeviceId: 0x10, Speed: StandardSpeed{0x01, 0x02, 0x03}}
	b, err := d.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x22, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("DEFERRED VARIABLE PLAY byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestRecordStrobeVariableMarshalBinary verifies RECORD STROBE VARIABLE encodes correctly.
func TestRecordStrobeVariableMarshalBinary(t *testing.T) {
	r := &RecordStrobeVariable{DeviceId: 0x10, Speed: StandardSpeed{0x01, 0x02, 0x03}}
	b, err := r.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x23, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("RECORD STROBE VARIABLE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestPhase2CommandMarshalerInterface verifies all new types implement CommandMarshaler.
func TestPhase2CommandMarshalerInterface(t *testing.T) {
	var _ CommandMarshaler = &Chase{}
	var _ CommandMarshaler = &CommandErrorReset{}
	var _ CommandMarshaler = &Reset{}
	var _ CommandMarshaler = &LocateIF{}
	var _ CommandMarshaler = &LocateTarget{}
	var _ CommandMarshaler = &Search{}
	var _ CommandMarshaler = &Shuttle{}
	var _ CommandMarshaler = &VariablePlay{}
	var _ CommandMarshaler = &Step{}
	var _ CommandMarshaler = &Write{}
	var _ CommandMarshaler = &MaskedWrite{}
	var _ CommandMarshaler = &Read{}
	var _ CommandMarshaler = &Update{}
	var _ CommandMarshaler = &DeferredVariablePlay{}
	var _ CommandMarshaler = &RecordStrobeVariable{}
}

// TestTimeCodeMethods verifies TimeCode bit-level parsing.
func TestTimeCodeMethods(t *testing.T) {
	// Time code: 01:02:03:04.05, 30fps, status byte
	tc := TimeCode{0xC1, 0x02, 0x03, 0x24, 0x05}
	if tc.Hours() != 1 {
		t.Errorf("Hours = %d, want 1", tc.Hours())
	}
	if tc.TimeType() != TimeCodeType30 {
		t.Errorf("TimeType = %d, want %d", tc.TimeType(), TimeCodeType30)
	}
	if tc.Minutes() != 2 {
		t.Errorf("Minutes = %d, want 2", tc.Minutes())
	}
	if tc.Seconds() != 3 {
		t.Errorf("Seconds = %d, want 3", tc.Seconds())
	}
	if tc.Frames() != 4 {
		t.Errorf("Frames = %d, want 4", tc.Frames())
	}
	if tc.FinalByteID() != 1 {
		t.Errorf("FinalByteID = %d, want 1", tc.FinalByteID())
	}
	if tc.StatusByte() != 0x05 {
		t.Errorf("StatusByte = 0x%02x, want 0x05", tc.StatusByte())
	}
}

// TestTimeCodeType30Drop verifies 30 drop-frame time type parsing.
func TestTimeCodeType30Drop(t *testing.T) {
	// Time type 10 = 30 drop frame, hours = 10
	tc := TimeCode{0x8A, 0x00, 0x00, 0x00, 0x00}
	if tc.TimeType() != TimeCodeType30Drop {
		t.Errorf("TimeType = %d, want %d", tc.TimeType(), TimeCodeType30Drop)
	}
	if tc.Hours() != 10 {
		t.Errorf("Hours = %d, want 10", tc.Hours())
	}
}

// TestShortTimeCodeMethods verifies ShortTimeCode parsing.
func TestShortTimeCodeMethods(t *testing.T) {
	st := ShortTimeCode{0x05, 0x06}
	if st.Frames() != 5 {
		t.Errorf("Frames = %d, want 5", st.Frames())
	}
	if st.StatusByte() != 0x06 {
		t.Errorf("StatusByte = 0x%02x, want 0x06", st.StatusByte())
	}
}

// TestSelectedTimeCodeMarshalBinary verifies SELECTED TIME CODE encoding.
func TestSelectedTimeCodeMarshalBinary(t *testing.T) {
	s := &SelectedTimeCode{DeviceId: 0x10, Time: TimeCode{0x01, 0x02, 0x03, 0x64, 0x05}}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x07, 0x01, 0x05, 0x01, 0x02, 0x03, 0x64, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("SelectedTimeCode byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestMotionControlTallyMarshalBinary verifies MOTION CONTROL TALLY encoding.
func TestMotionControlTallyMarshalBinary(t *testing.T) {
	m := &MotionControlTally{DeviceId: 0x10, MSC: 0x02, MCP: 0x01, Status: 0x34}
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x07, 0x12, 0x03, 0x02, 0x01, 0x34, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("MotionControlTally byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestVelocityTallyMarshalBinary verifies VELOCITY TALLY encoding.
func TestVelocityTallyMarshalBinary(t *testing.T) {
	v := &VelocityTally{DeviceId: 0x10, Speed: 0x05}
	b, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x07, 0x13, 0x01, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("VelocityTally byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestUnmarshalSelectedTimeCode verifies SELECTED TIME CODE decoding.
func TestUnmarshalSelectedTimeCode(t *testing.T) {
	data := []byte{0xf0, 0x7f, 0x10, 0x07, 0x01, 0x05, 0x01, 0x02, 0x03, 0x64, 0x05, 0xf7}
	s, err := UnmarshalSelectedTimeCode(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.DeviceId != 0x10 {
		t.Errorf("DeviceId = 0x%02x, want 0x10", s.DeviceId)
	}
	if s.Time.Hours() != 1 {
		t.Errorf("Hours = %d, want 1", s.Time.Hours())
	}
}

// TestUnmarshalMotionControlTally verifies MOTION CONTROL TALLY decoding.
func TestUnmarshalMotionControlTally(t *testing.T) {
	data := []byte{0xf0, 0x7f, 0x10, 0x07, 0x12, 0x03, 0x02, 0x01, 0x34, 0xf7}
	m, err := UnmarshalMotionControlTally(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.DeviceId != 0x10 {
		t.Errorf("DeviceId = 0x%02x, want 0x10", m.DeviceId)
	}
	if m.MSC != 0x02 {
		t.Errorf("MSC = 0x%02x, want 0x02", m.MSC)
	}
	if m.MCP != 0x01 {
		t.Errorf("MCP = 0x%02x, want 0x01", m.MCP)
	}
	if m.Status != 0x34 {
		t.Errorf("Status = 0x%02x, want 0x34", m.Status)
	}
}

// TestSignatureMarshalBinary verifies SIGNATURE encoding.
func TestSignatureMarshalBinary(t *testing.T) {
	s := &Signature{DeviceId: 0x10, VersionMajor: 0x01, VersionMinor: 0x00}
	s.CommandBitmaps = [][]byte{{0x01, 0x02}, {0x03, 0x04}}
	s.ResponseBitmaps = [][]byte{{0x05, 0x06}}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	// F0 7F 10 07 40 <count=13> <data...> F7
	if b[0] != 0xf0 || b[1] != 0x7f || b[3] != 0x07 || b[4] != 0x40 || b[5] != 0x0d {
		t.Fatalf("bad framing: %v", b)
	}
	// Check version bytes (after count byte)
	if b[6] != 0x01 || b[7] != 0x00 || b[8] != 0x00 || b[9] != 0x00 {
		t.Fatalf("bad version bytes: %v", b)
	}
}

// TestProcedureMarshalBinary verifies PROCEDURE encodes correctly.
func TestProcedureMarshalBinary(t *testing.T) {
	p := &Procedure{DeviceId: 0x10, Action: 0x01}
	b, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1e, 0x01, 0x01, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("PROCEDURE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestEventMarshalBinary verifies EVENT encodes correctly.
func TestEventMarshalBinary(t *testing.T) {
	e := &Event{DeviceId: 0x10, Action: 0x01}
	b, err := e.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1f, 0x01, 0x01, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("EVENT byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestGroupMarshalBinary verifies GROUP encodes correctly.
func TestGroupMarshalBinary(t *testing.T) {
	g := &Group{DeviceId: 0x10, GroupID: 0x05}
	b, err := g.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x20, 0x01, 0x05, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("GROUP byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestCommandSegmentMarshalBinary verifies COMMAND SEGMENT encodes correctly.
func TestCommandSegmentMarshalBinary(t *testing.T) {
	c := &CommandSegment{DeviceId: 0x10, SegmentNum: 0x01, Data: []byte{0x02, 0x03}}
	b, err := c.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x21, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("COMMAND SEGMENT byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestMoveMarshalBinary verifies MOVE encodes correctly.
func TestMoveMarshalBinary(t *testing.T) {
	m := &Move{DeviceId: 0x10, Dest: 0x01, Source: 0x02}
	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1a, 0x02, 0x01, 0x02, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("MOVE byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestAddMarshalBinary verifies ADD encodes correctly.
func TestAddMarshalBinary(t *testing.T) {
	a := &Add{DeviceId: 0x10, Dest: 0x01, Source1: 0x02, Source2: 0x03}
	b, err := a.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1b, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("ADD byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestSubtractMarshalBinary verifies SUBTRACT encodes correctly.
func TestSubtractMarshalBinary(t *testing.T) {
	s := &Subtract{DeviceId: 0x10, Dest: 0x01, Source1: 0x02, Source2: 0x03}
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1c, 0x03, 0x01, 0x02, 0x03, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("SUBTRACT byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestDropFrameAdjustMarshalBinary verifies DROP FRAME ADJUST encodes correctly.
func TestDropFrameAdjustMarshalBinary(t *testing.T) {
	d := &DropFrameAdjust{DeviceId: 0x10, Name: 0x01}
	b, err := d.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x1d, 0x01, 0x01, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("DROP FRAME ADJUST byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestGeneratorCommandMarshalBinary verifies GENERATOR COMMAND encodes correctly.
func TestGeneratorCommandMarshalBinary(t *testing.T) {
	g := &GeneratorCommand{DeviceId: 0x10, Action: 0x01}
	b, err := g.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x18, 0x01, 0x01, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("GENERATOR COMMAND byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestTimeCodeCommandMarshalBinary verifies MIDI TIME CODE COMMAND encodes correctly.
func TestTimeCodeCommandMarshalBinary(t *testing.T) {
	t2 := &TimeCodeCommand{DeviceId: 0x10, Action: 0x01}
	b, err := t2.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0xf0, 0x7f, 0x10, 0x06, 0x19, 0x01, 0x01, 0xf7}
	for i := range expected {
		if b[i] != expected[i] {
			t.Fatalf("MIDI TIME CODE COMMAND byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b[i])
		}
	}
}

// TestUnmarshalWait verifies WAIT decoding with EndSysEx validation.
func TestUnmarshalWait(t *testing.T) {
	data := []byte{0xf0, 0x7f, 0x10, 0x06, 0x7c, 0xf7}
	w, err := UnmarshalWait(data)
	if err != nil {
		t.Fatal(err)
	}
	if w.DeviceId != 0x10 {
		t.Errorf("DeviceId = 0x%02x, want 0x10", w.DeviceId)
	}
	// Missing EndSysEx should fail
	_, err = UnmarshalWait([]byte{0xf0, 0x7f, 0x10, 0x06, 0x7c})
	if err == nil {
		t.Error("expected error for missing EndSysEx")
	}
}

// TestUnmarshalResume verifies RESUME decoding with EndSysEx validation.
func TestUnmarshalResume(t *testing.T) {
	data := []byte{0xf0, 0x7f, 0x10, 0x06, 0x7d, 0xf7}
	r, err := UnmarshalResume(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.DeviceId != 0x10 {
		t.Errorf("DeviceId = 0x%02x, want 0x10", r.DeviceId)
	}
}

// TestUnmarshalSignature verifies SIGNATURE decoding.
func TestUnmarshalSignature(t *testing.T) {
	// Build a valid signature response
	// F0 7F 10 07 40 <count=0x0C> <data...> F7
	// count=0x0C (12), vi=0x01, vf=0x00, va=0x00, vb=0x00
	// command bitmap count=0x02, bytes=[0x01, 0x02]
	// response bitmap count=0x03, bytes=[0x03, 0x04, 0x05, 0x06]
	// Total payload = 4(version) + 1(cmd_count) + 2(cmd_data) + 1(resp_count) + 4(resp_data) = 12
	data := []byte{0xf0, 0x7f, 0x10, 0x07, 0x40, 0x0c, 0x01, 0x00, 0x00, 0x00,
		0x02, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0xf7}
	s, err := UnmarshalSignature(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.DeviceId != 0x10 {
		t.Errorf("DeviceId = 0x%02x, want 0x10", s.DeviceId)
	}
	if s.VersionMajor != 0x01 || s.VersionMinor != 0x00 {
		t.Errorf("Version = 0x%02x/0x%02x, want 0x01/0x00", s.VersionMajor, s.VersionMinor)
	}
	if len(s.CommandBitmaps) != 1 || len(s.CommandBitmaps[0]) != 2 {
		t.Errorf("CommandBitmaps = %v, want 1 bitmap of len 2", s.CommandBitmaps)
	}
	if len(s.ResponseBitmaps) != 1 || len(s.ResponseBitmaps[0]) != 4 {
		t.Errorf("ResponseBitmaps = %v, want 1 bitmap of len 4", s.ResponseBitmaps)
	}
	// Short data should fail
	_, err = UnmarshalSignature([]byte{0xf0, 0x7f, 0x10, 0x07, 0x40, 0x04, 0x01, 0x00, 0x00, 0x00, 0xf7})
	if err == nil {
		t.Error("expected error for short data")
	}
}
