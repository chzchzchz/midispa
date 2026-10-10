package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/alsa/fake"
	"github.com/chzchzchz/midispa/midi"
)

// runReader runs readInput on its own goroutine and reports
// when the loop has returned, so a test can tell the loop
// stopped rather than guess how long to wait for it.
func runReader(seq alsa.EventReader, send func(alsa.SeqEvent)) <-chan struct{} {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		readInput(seq, send)
	}()
	return stopped
}

// failingSeq reads like a sequencer that fails its first
// reads with the errors it was given and then runs dry, so
// a loop that survives the failures ends on the empty queue
// the way it would on a sequencer that stopped answering.
type failingSeq struct {
	errs []error
}

func (s *failingSeq) Read() (alsa.SeqEvent, error) {
	if len(s.errs) == 0 {
		return alsa.SeqEvent{}, fake.ErrNoEvents
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return alsa.SeqEvent{}, err
}

func TestReadInputForwardsControlChangesOnly(t *testing.T) {
	seq := fake.New()
	seq.Queue(
		alsa.SeqEvent{Data: []byte{midi.MakeCC(0), 10, 42}},
		alsa.SeqEvent{Data: []byte{midi.Clock}},
		alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(0), 60, 100}},
		alsa.SeqEvent{Data: []byte{alsa.EvPortSubscribed, 0, 0, 0, 0}},
		alsa.SeqEvent{Data: []byte{midi.MakeCC(9), 13, 55}},
	)
	sent := make(chan alsa.SeqEvent, 2)
	// The fake does not block, so the loop ends when the
	// queue drains, the way a quiet sequencer never does.
	stopped := runReader(seq, func(ev alsa.SeqEvent) { sent <- ev })
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("the loop did not stop after the queue drained")
	}
	want := [][]byte{
		{midi.MakeCC(0), 10, 42},
		{midi.MakeCC(9), 13, 55},
	}
	for _, expected := range want {
		select {
		case ev := <-sent:
			if !bytes.Equal(ev.Data, expected) {
				t.Errorf("forwarded %v, want %v", ev.Data, expected)
			}
		default:
			t.Fatalf("nothing forwarded, want %v", expected)
		}
	}
}

func TestReadInputStopsOnFatalReadError(t *testing.T) {
	stopped := runReader(&failingSeq{errs: []error{fake.ErrClosed}}, func(alsa.SeqEvent) {})
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("a fatal read error did not end the loop")
	}
}

// TestReadInputSurvivesDecodeErrors proves a per-event decode
// failure leaves the loop running: a device that breathes
// active sensing at the editor must not cost it its input.
// The loop ends on the empty queue after the failures, so a
// loop that ended on a failure would leave errors unconsumed.
func TestReadInputSurvivesDecodeErrors(t *testing.T) {
	for _, err := range []error{
		fmt.Errorf("sensing: %w", alsa.ErrUnsupportedEvent),
		fmt.Errorf("quarter frame: %w", alsa.ErrInvalidMessage),
	} {
		seq := &failingSeq{errs: []error{err, err}}
		stopped := runReader(seq, func(alsa.SeqEvent) {})
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatalf("the loop did not stop, error %v", err)
		}
		if len(seq.errs) != 0 {
			t.Fatalf("a survivable read error %v ended the loop", err)
		}
	}
}

func TestReadErrFatal(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"unsupported event", alsa.ErrUnsupportedEvent, false},
		{"wrapped unsupported event", fmt.Errorf("type %d: %w", 0xfe, alsa.ErrUnsupportedEvent), false},
		{"invalid message", alsa.ErrInvalidMessage, false},
		{"wrapped invalid message", fmt.Errorf("quarter frame: %w", alsa.ErrInvalidMessage), false},
		{"released client", fake.ErrClosed, true},
		{"any other failure", errors.New("boom"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := readErrFatal(test.err); got != test.want {
				t.Errorf("readErrFatal(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
