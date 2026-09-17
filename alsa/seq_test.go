package alsa

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
)

const (
	testTimeout    = 2 * time.Second
	testSourcePort = 1
)

func openTestSeq(t *testing.T, name string) *Seq {
	t.Helper()
	seq, err := OpenSeq(name)
	if err != nil {
		t.Skipf("ALSA sequencer unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := seq.Close(); err != nil {
			t.Errorf("close sequencer: %v", err)
		}
	})
	return seq
}

func readTestEvent(t *testing.T, seq *Seq) SeqEvent {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for !seq.MayRead() {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for ALSA event")
		}
		time.Sleep(time.Millisecond)
	}
	event, err := seq.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return event
}

func TestExpressionRoundTrip(t *testing.T) {
	sender := openTestSeq(t, "midispa-test-tx")
	receiver := openTestSeq(t, "midispa-test-rx")
	if err := sender.CreatePort("expression"); err != nil {
		t.Fatal(err)
	}
	for _, channel := range []byte{0, 7, 15} {
		var messages [][]byte
		for _, note := range []byte{0, 127} {
			for _, pressure := range []byte{0, 1, 127} {
				messages = append(messages, []byte{midi.KeyAftertouch | channel, note, pressure})
			}
		}
		for _, pressure := range []byte{0, 127} {
			messages = append(messages, []byte{midi.ChannelAftertouch | channel, pressure})
		}
		for _, pitch := range [][2]byte{{0, 0}, {0, 64}, {127, 127}, {1, 0}, {0, 1}, {1, 64}} {
			messages = append(messages, []byte{midi.Pitch | channel, pitch[0], pitch[1]})
		}
		for _, data := range messages {
			t.Run(fmt.Sprintf("%x", data), func(t *testing.T) {
				if err := sender.WritePort(SeqEvent{receiver.SeqAddr, data}, testSourcePort); err != nil {
					t.Fatalf("write: %v", err)
				}
				marker := []byte{midi.Clock}
				if err := sender.WritePort(SeqEvent{receiver.SeqAddr, marker}, testSourcePort); err != nil {
					t.Fatalf("write marker: %v", err)
				}
				event := readTestEvent(t, receiver)
				if !bytes.Equal(event.Data, data) {
					t.Fatalf("MIDI bytes = % x, want % x", event.Data, data)
				}
				wantSource := SeqAddr{sender.Client, testSourcePort}
				if event.SeqAddr != wantSource {
					t.Errorf("source = %v, want %v", event.SeqAddr, wantSource)
				}
				if event := readTestEvent(t, receiver); !bytes.Equal(event.Data, marker) {
					t.Fatalf("marker = % x, want % x", event.Data, marker)
				}
			})
			if t.Failed() {
				return
			}
		}
	}
}

func TestWritePortRejectsInvalidExpression(t *testing.T) {
	seq := &Seq{}
	for _, status := range []byte{midi.KeyAftertouch, midi.ChannelAftertouch, midi.Pitch} {
		length := 3
		if status == midi.ChannelAftertouch {
			length = 2
		}
		for size := 1; size <= length+1; size++ {
			if size == length {
				continue
			}
			data := make([]byte, size)
			data[0] = status
			t.Run(fmt.Sprintf("length/%x", data), func(t *testing.T) {
				if err := seq.WritePort(MakeEvent(data), 0); err == nil {
					t.Fatal("expected invalid length error")
				}
			})
		}
		for index := 1; index < length; index++ {
			for _, value := range []byte{128, 255} {
				data := make([]byte, length)
				data[0], data[index] = status, value
				t.Run(fmt.Sprintf("data/%x", data), func(t *testing.T) {
					if err := seq.WritePort(MakeEvent(data), 0); err == nil {
						t.Fatal("expected non-seven-bit data error")
					}
				})
			}
		}
	}
}
