package alsa

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/midi"
)

const (
	testChannels     = 16
	testMaxByte      = 255
	testMaxData      = 127
	testSourcePort   = 3
	testSourceClient = 128
)

var supportedMessages = []struct {
	name      string
	data      []byte
	eventType byte
}{
	{"note_off", []byte{midi.NoteOff, 1, testMaxData}, 7},
	{"note_on", []byte{midi.NoteOn, 1, testMaxData}, 6},
	{"key_aftertouch", []byte{midi.KeyAftertouch, 1, testMaxData}, 8},
	{"control_change", []byte{midi.CC, 1, testMaxData}, 10},
	{"program_change", []byte{midi.Pgm, testMaxData}, 11},
	{"song_position", []byte{midi.SongPosition, 1, testMaxData}, 20},
	{"song_select", []byte{midi.SongSelect, testMaxData}, 21},
	{"quarter_frame", []byte{midi.QuarterFrame, testMaxData}, 22},
	{"clock", []byte{midi.Clock}, 36},
	{"start", []byte{midi.Start}, 30},
	{"continue", []byte{midi.Continue}, 31},
	{"stop", []byte{midi.Stop}, 32},
	{"channel_aftertouch", []byte{midi.ChannelAftertouch, testMaxData}, 12},
	{"pitch_bend", []byte{midi.Pitch, 0, testMaxData}, 13},
	{"sysex", []byte{midi.SysEx, 0, 1, testMaxData, midi.EndSysEx}, 130},
}

func newTestSeq() *Seq {
	return &Seq{
		SeqAddr: SeqAddr{Client: testSourceClient, Port: testSourcePort},
		ports:   map[int]struct{}{testSourcePort: {}},
		output:  func(*outputEvent) error { return nil },
	}
}

func checkMessage(t *testing.T, data []byte, kind string) {
	t.Helper()
	original := bytes.Clone(data)
	calls := 0
	var gotType byte
	seq := newTestSeq()
	seq.output = func(event *outputEvent) error {
		calls++
		gotType = byte(event._type)
		return nil
	}
	checkError := func(err error) {
		t.Helper()
		switch kind {
		case "valid":
			if err != nil {
				t.Fatalf("message %x: %v", data, err)
			}
		case "invalid":
			var invalid *InvalidMessageError
			if !errors.As(err, &invalid) || invalid.Status != data[0] || invalid.Reason == "" {
				t.Fatalf("message %x: expected invalid error, got %v", data, err)
			}
		case "unsupported":
			var unsupported *UnsupportedMessageError
			if !errors.As(err, &unsupported) || unsupported.Status != data[0] {
				t.Fatalf("message %x: expected unsupported error, got %v", data, err)
			}
		default:
			t.Fatalf("unknown test error kind %q", kind)
		}
		if err != nil && err.Error() == "" {
			t.Fatal("empty error description")
		}
	}
	checkError(validateMessage(data))
	_, err := encodeEvent(MakeEvent(data), SeqAddr{})
	checkError(err)
	checkError(seq.Write(MakeEvent(data)))
	checkError(seq.WritePort(MakeEvent(data), testSourcePort))
	count, err := seq.NewWriter(SubsSeqAddr).Write(data)
	checkError(err)
	wantCount, wantCalls := 0, 0
	if kind == "valid" && len(data) != 0 {
		wantCount, wantCalls = len(data), 3
		if gotType != 0 && kind == "valid" {
			if wantType, ok := eventTypeFor(data[0]); !ok || gotType != wantType {
				t.Fatalf("message %x: event type %d, want %d", data, gotType, wantType)
			}
		}
	}
	if count != wantCount || calls != wantCalls {
		t.Fatalf("message %x: count=%d calls=%d, want %d and %d", data, count, calls, wantCount, wantCalls)
	}
	if !bytes.Equal(data, original) {
		t.Fatalf("input mutated: %x -> %x", original, data)
	}
}

func eventTypeFor(status byte) (byte, bool) {
	for _, message := range supportedMessages {
		if midi.Message(status) == message.data[0] {
			return message.eventType, true
		}
	}
	return 0, false
}

func TestSupportedMessages(t *testing.T) {
	for _, message := range supportedMessages {
		channels := 1
		if midi.IsChannelMessage(message.data[0]) {
			channels = testChannels
		}
		for channel := 0; channel < channels; channel++ {
			t.Run(fmt.Sprintf("%s/%d", message.name, channel), func(t *testing.T) {
				data := bytes.Clone(message.data)
				data[0] |= byte(channel)
				checkMessage(t, data, "valid")
				for length := 1; length < len(data); length++ {
					checkMessage(t, data[:length], "invalid")
				}
				checkMessage(t, append(bytes.Clone(data), 0), "invalid")
				checkMessage(t, append(bytes.Clone(data), midi.Clock), "invalid")
				payloadEnd := len(data)
				if data[0] == midi.SysEx {
					payloadEnd--
				}
				for index := 1; index < payloadEnd; index++ {
					for value := 0; value <= testMaxByte; value++ {
						changed := bytes.Clone(data)
						changed[index] = byte(value)
						kind := "valid"
						if value > testMaxData {
							kind = "invalid"
						}
						checkMessage(t, changed, kind)
					}
				}
			})
		}
	}
}

func TestStatusClassification(t *testing.T) {
	for status := 0; status <= testMaxByte; status++ {
		if status <= testMaxData {
			checkMessage(t, []byte{byte(status), 0}, "invalid")
			continue
		}
		supported := false
		for _, message := range supportedMessages {
			if midi.Message(byte(status)) == message.data[0] {
				supported = true
				break
			}
		}
		if !supported {
			for length := 1; length <= 4; length++ {
				data := make([]byte, length)
				data[0] = byte(status)
				checkMessage(t, data, "unsupported")
			}
		}
	}
}

func TestSysExFraming(t *testing.T) {
	for _, data := range [][]byte{
		{midi.SysEx, midi.EndSysEx},
		{midi.SysEx, 0, midi.EndSysEx},
		append(append([]byte{midi.SysEx}, make([]byte, 4096)...), midi.EndSysEx),
	} {
		checkMessage(t, data, "valid")
	}
	for _, data := range [][]byte{
		{midi.SysEx},
		{midi.SysEx, 0},
		{midi.SysEx, midi.SysEx, midi.EndSysEx},
		{midi.SysEx, midi.EndSysEx, midi.EndSysEx},
		{midi.SysEx, midi.Clock, midi.EndSysEx},
		{midi.SysEx, midi.EndSysEx, 0},
		{midi.SysEx, midi.EndSysEx, midi.SysEx, midi.EndSysEx},
	} {
		checkMessage(t, data, "invalid")
	}
}

func TestEmptyWrites(t *testing.T) {
	checkMessage(t, nil, "valid")
	checkMessage(t, []byte{}, "valid")
	seq := newTestSeq()
	if err := seq.Write(SeqEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := seq.WritePort(SeqEvent{}, seq.Port); err != nil {
		t.Fatal(err)
	}
	if count, err := seq.NewWriter(SubsSeqAddr).Write(nil); count != 0 || err != nil {
		t.Fatalf("empty writer: %d, %v", count, err)
	}
}

func TestOutputFailures(t *testing.T) {
	outputError := errors.New("injected output failure")
	for _, message := range supportedMessages {
		t.Run(message.name, func(t *testing.T) {
			calls := 0
			seq := newTestSeq()
			seq.output = func(event *outputEvent) error {
				calls++
				return outputError
			}
			if err := seq.Write(MakeEvent(message.data)); err != outputError {
				t.Fatalf("Write error = %v", err)
			}
			if err := seq.WritePort(MakeEvent(message.data), testSourcePort); err != outputError {
				t.Fatalf("WritePort error = %v", err)
			}
			if count, err := seq.NewWriter(SubsSeqAddr).Write(message.data); count != 0 || err != outputError {
				t.Fatalf("writer result = %d, %v", count, err)
			}
			if calls != 3 {
				t.Fatalf("output calls = %d, want 3", calls)
			}
		})
	}
}

func FuzzMessageOutput(f *testing.F) {
	f.Add([]byte{})
	for _, message := range supportedMessages {
		f.Add(message.data)
		f.Add(message.data[:len(message.data)-1])
		f.Add(append(bytes.Clone(message.data), 0))
	}
	for status := 0; status <= testMaxByte; status++ {
		f.Add([]byte{byte(status), testMaxByte, 0})
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		err := validateMessage(data)
		kind := "valid"
		if err != nil {
			var invalid *InvalidMessageError
			var unsupported *UnsupportedMessageError
			switch {
			case errors.As(err, &invalid):
				kind = "invalid"
			case errors.As(err, &unsupported):
				kind = "unsupported"
			default:
				t.Fatalf("unexpected validation error: %v", err)
			}
		}
		checkMessage(t, data, kind)
	})
}
