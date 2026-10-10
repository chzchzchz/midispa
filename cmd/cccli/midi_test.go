package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/chzchzchz/midispa/midi"
)

func TestSendCC(t *testing.T) {
	buffer := &bytes.Buffer{}
	if err := sendCC(buffer, 0, 10, 42); err != nil {
		t.Fatalf("sendCC: %v", err)
	}
	want := []byte{midi.MakeCC(0), 10, 42}
	if !bytes.Equal(buffer.Bytes(), want) {
		t.Errorf("sent %v, want %v", buffer.Bytes(), want)
	}
}

func TestSendCCOnEveryChannel(t *testing.T) {
	for channel := 0; channel < 16; channel++ {
		buffer := &bytes.Buffer{}
		if err := sendCC(buffer, channel, 127, 1); err != nil {
			t.Fatalf("channel %d: %v", channel, err)
		}
		want := []byte{midi.MakeCC(channel), 127, 1}
		if !bytes.Equal(buffer.Bytes(), want) {
			t.Errorf("channel %d sent %v, want %v", channel, buffer.Bytes(), want)
		}
	}
}

// shortWriter writes one byte short, the way a full buffer
// or a closed port fails a write.
type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) {
	return len(data) - 1, nil
}

func TestSendCCReportsShortWrite(t *testing.T) {
	if err := sendCC(shortWriter{}, 0, 10, 42); !errors.Is(err, io.ErrShortWrite) {
		t.Errorf("err = %v, want io.ErrShortWrite", err)
	}
}
