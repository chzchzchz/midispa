package yamaha

import (
	"bytes"
	"testing"
)

func TestBulkDataEncodeChecksum(t *testing.T) {
	encoded, err := (&BulkData{}).encode()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(encoded), 6+32*128+2; got != want {
		t.Fatalf("encoded length = %d, want %d", got, want)
	}

	sum := 0
	for _, value := range encoded[sysexHeaderSize:len(encoded)-2] {
		sum += int(value)
	}
	want := byte((-sum) & 0x7f)
	if got := encoded[len(encoded)-2]; got != want {
		t.Errorf("checksum = 0x%02x, want 0x%02x", got, want)
	}
}

func TestBulkDataEncodeRejectsInvalidChannel(t *testing.T) {
	tests := []struct {
		name    string
		channel int
	}{
		{name: "negative", channel: -1},
		{name: "above Yamaha channel range", channel: 16},
		{name: "large value", channel: 255},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&BulkData{Channel: test.channel}).encode(); err == nil {
				t.Fatalf("encode accepted channel %d", test.channel)
			}
		})
	}
}

func TestParameterChangeEncodeGroup(t *testing.T) {
	tests := []struct {
		name           string
		parameterGroup int
		parameter      int
		wantGroup      byte
	}{
		{name: "voice parameter", parameterGroup: parameterGroupVoice, parameter: 0, wantGroup: 0x00},
		{name: "extended voice parameter", parameterGroup: parameterGroupVoice, parameter: 128, wantGroup: 0x01},
		{name: "function parameter", parameterGroup: parameterGroupFunction, parameter: 64, wantGroup: 0x08},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := (&ParameterChange{
				Group:     test.parameterGroup,
				Parameter: test.parameter,
			}).encode()
			if err != nil {
				t.Fatal(err)
			}
			want := []byte{0xf0, 0x43, 0x10, test.wantGroup, byte(test.parameter & 0x7f), 0x00, 0xf7}
			if !bytes.Equal(encoded, want) {
				t.Errorf("encoded = %v, want %v", encoded, want)
			}
		})
	}
}

func TestParameterChangeRejectsUnsupportedParameter(t *testing.T) {
	tests := []struct {
		name      string
		parameter int
	}{
		{name: "after voice range", parameter: 156},
		{name: "unused high parameter", parameter: 256},
		{name: "maximum currently accepted", parameter: 511},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&ParameterChange{Parameter: test.parameter}).encode(); err == nil {
				t.Fatalf("encode accepted parameter %d", test.parameter)
			}
		})
	}
}

func TestParameterChangeRejectsOutOfRangeData(t *testing.T) {
	tests := []struct {
		name           string
		parameterGroup int
		parameter      int
		data           int
	}{
		{name: "operator EG rate", parameterGroup: parameterGroupVoice, parameter: 0, data: 100},
		{name: "algorithm", parameterGroup: parameterGroupVoice, parameter: 134, data: 32},
		{name: "operator on mask", parameterGroup: parameterGroupVoice, parameter: 155, data: 64},
		{name: "voice name", parameterGroup: parameterGroupVoice, parameter: 145, data: 128},
		{name: "function mode", parameterGroup: parameterGroupFunction, parameter: 64, data: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&ParameterChange{
				Group:     test.parameterGroup,
				Parameter: test.parameter,
				Data:      test.data,
			}).encode(); err == nil {
				t.Fatalf("encode accepted parameter %d with data %d", test.parameter, test.data)
			}
		})
	}
}

func TestVoiceEncodeRejectsNonASCIIVoiceName(t *testing.T) {
	voice := Voice{}
	voice.VoiceName[0] = 0xff
	if _, err := voice.encode(); err == nil {
		t.Fatal("encode accepted a non-ASCII voice name")
	}
}

func TestOscEncodeRejectsOutOfRangeField(t *testing.T) {
	osc := Osc{Detune: 15}
	if _, err := osc.encode(); err == nil {
		t.Fatal("encode accepted an out-of-range oscillator field")
	}
}

func TestVoiceEncodeRejectsOutOfRangeField(t *testing.T) {
	voice := Voice{PitchEgRate: [4]int{0, 0, 0, 100}}
	if _, err := voice.encode(); err == nil {
		t.Fatal("encode accepted an out-of-range voice field")
	}
}

func TestVoiceEncodeRejectsInvalidOperatorOn(t *testing.T) {
	voice := Voice{OperatorOn: 64}
	if _, err := voice.encode(); err == nil {
		t.Fatal("encode accepted an invalid operator mask")
	}
}

func TestParameterChangeRejectsInvalidGroup(t *testing.T) {
	if _, err := (&ParameterChange{Group: 1}).encode(); err == nil {
		t.Fatal("encode accepted an invalid parameter group")
	}
}

// The fixture exercises every logical field while preserving the DX7 wire order of OP6 through OP1.
func makeTestOsc(index int) Osc {
	base := index * 7
	return Osc{
		EgRate:     [4]int{base, base + 1, base + 2, base + 3},
		EgLevel:    [4]int{base + 10, base + 11, base + 12, base + 13},
		BrkPt:      base + 20,
		LftDepth:   base + 30,
		RhtDepth:   base + 40,
		LftCurve:   index % 4,
		RhtCurve:   (index + 1) % 4,
		RateScale:  index % 8,
		ModSens:    index % 4,
		VelSens:    (index + 1) % 8,
		OutLevel:   base + 50,
		Mode:       index % 2,
		FreqCoarse: (index * 5) % 32,
		FreqFine:   base + 60,
		Detune:     index % 15,
	}
}

func makeTestVoice() Voice {
	voice := Voice{
		PitchEgRate:      [4]int{1, 2, 3, 4},
		PitchEgLevel:     [4]int{5, 6, 7, 8},
		Algorithm:        17,
		Feedback:         5,
		OscSync:          1,
		LfoSpeed:         20,
		LfoDelay:         30,
		LfoPitchModDepth: 40,
		LfoAmpModDepth:   50,
		LfoSync:          1,
		LfoWaveform:      4,
		PitchModSens:     6,
		Transpose:        24,
		VoiceName:        [10]byte{'D', 'X', '7', ' ', 'T', 'E', 'S', 'T', '!', '?'},
	}
	for index := range voice.Osc {
		voice.Osc[index] = makeTestOsc(index)
	}
	return voice
}

func TestOscRoundTrip(t *testing.T) {
	want := makeTestOsc(3)
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got Osc
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("decoded oscillator = %+v, want %+v", got, want)
	}
}

func TestVoicePackedRoundTrip(t *testing.T) {
	want := makeTestVoice()
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got Voice
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("decoded voice = %+v, want %+v", got, want)
	}
}

func TestBulkDataRoundTrip(t *testing.T) {
	want := BulkData{Channel: 7}
	for index := range want.Voices {
		want.Voices[index] = makeTestVoice()
	}
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got BulkData
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("decoded bulk data differs from source")
	}
}

func TestParameterChangeRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value ParameterChange
	}{
		{name: "voice", value: ParameterChange{Channel: 3, Group: parameterGroupVoice, Parameter: 128, Data: 99}},
		{name: "function", value: ParameterChange{Channel: 15, Group: parameterGroupFunction, Parameter: 77, Data: 7}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := test.value.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			var got ParameterChange
			if err := got.UnmarshalBinary(encoded); err != nil {
				t.Fatal(err)
			}
			if got != test.value {
				t.Errorf("decoded parameter change = %+v, want %+v", got, test.value)
			}
		})
	}
}

func TestSingleVoiceRoundTrip(t *testing.T) {
	want := SingleVoice{Channel: 9, Voice: makeTestVoice()}
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var got SingleVoice
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("decoded single voice differs from source")
	}
}

func TestBulkDataUnmarshalRejectsCorruptMessage(t *testing.T) {
	valid, err := (&BulkData{}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "checksum", mutate: func(data []byte) { data[len(data)-2] ^= 0x01 }},
		{name: "format", mutate: func(data []byte) { data[3] = 0x08 }},
		{name: "non MIDI data", mutate: func(data []byte) { data[6] = 0x80 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			test.mutate(data)
			var got BulkData
			if err := got.UnmarshalBinary(data); err == nil {
				t.Fatal("UnmarshalBinary accepted a corrupt bulk message")
			}
		})
	}
}

func TestParameterChangeUnmarshalRejectsMalformedMessage(t *testing.T) {
	valid, err := (&ParameterChange{Channel: 3, Parameter: 128, Data: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "channel", mutate: func(data []byte) { data[2] = 0x00 }},
		{name: "group", mutate: func(data []byte) { data[3] = 0x04 }},
		{name: "parameter", mutate: func(data []byte) { data[4] = 0x80 }},
		{name: "data", mutate: func(data []byte) { data[5] = 0x80 }},
		{name: "end", mutate: func(data []byte) { data[6] = 0x00 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			test.mutate(data)
			var got ParameterChange
			if err := got.UnmarshalBinary(data); err == nil {
				t.Fatal("UnmarshalBinary accepted a malformed parameter message")
			}
		})
	}
}

func TestVoiceUnmarshalRejectsMalformedPayload(t *testing.T) {
	voice := makeTestVoice()
	valid, err := voice.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "non MIDI data", mutate: func(data []byte) { data[0] = 0x80 }},
		{name: "range", mutate: func(data []byte) { data[8] = 100 }},
		{name: "packed value", mutate: func(data []byte) { data[12] = 0xff }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			test.mutate(data)
			var got Voice
			if err := got.UnmarshalBinary(data); err == nil {
				t.Fatal("UnmarshalBinary accepted a malformed voice payload")
			}
		})
	}
}

func TestSingleVoiceUnmarshalRejectsCorruptMessage(t *testing.T) {
	valid, err := (&SingleVoice{Voice: makeTestVoice()}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "checksum", mutate: func(data []byte) { data[len(data)-2] ^= 0x01 }},
		{name: "header", mutate: func(data []byte) { data[4] = 0x00 }},
		{name: "payload", mutate: func(data []byte) { data[6] = 0x80 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			test.mutate(data)
			var got SingleVoice
			if err := got.UnmarshalBinary(data); err == nil {
				t.Fatal("UnmarshalBinary accepted a corrupt single-voice message")
			}
		})
	}
}

// Dexed treats reserved high bits in packed records as don't-care bits.
func TestVoiceUnmarshalIgnoresReservedPackedBits(t *testing.T) {
	want := makeTestVoice()
	encoded, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	encoded[11] |= 0x10
	encoded[13] |= 0x20
	encoded[15] |= 0x40
	encoded[110] |= 0x20
	encoded[111] |= 0x10
	var got Voice
	if err := got.UnmarshalBinary(encoded); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("decoded voice = %+v, want %+v", got, want)
	}
}
