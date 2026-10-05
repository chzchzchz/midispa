package fake

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// The failures a test asks the stand-in to produce, kept in one place so each test
// asserts on the one it provoked.
var (
	errReadFailed  = errors.New("read failed")
	errWriteFailed = errors.New("write failed")
	errCloseFailed = errors.New("close failed")
)

func noteOn(channel, note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(channel), byte(note), 100}}
}

// The routing is the reason a fake is worth having at all: a consumer with several of
// its own ports must be able to tell which one an event left by.
func TestWritesRecordThePortTheyWentOutOn(t *testing.T) {
	seq := New()
	seq.Port = 3

	if err := seq.Write(noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}
	if err := seq.WritePort(noteOn(0, 64), 7); err != nil {
		t.Fatal(err)
	}

	writes := seq.Writes()
	if len(writes) != 2 {
		t.Fatalf("recorded %d writes, want 2", len(writes))
	}
	if writes[0].Port != 3 || writes[1].Port != 7 {
		t.Errorf("writes went out on ports %d and %d, want 3 and 7", writes[0].Port, writes[1].Port)
	}
	assertMessage(t, writes[1].Event.Data, []byte{midi.MakeNoteOn(0), 64, 100})
}

// Writes has to hand back a copy of the recording, or a test holding on to what an
// earlier assertion returned could change what a later one sees.
func TestWritesHandsBackACopy(t *testing.T) {
	seq := New()
	if err := seq.Write(noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}

	seq.Writes()[0].Port = 99

	if got := seq.Writes()[0].Port; got != 0 {
		t.Errorf("port %d after editing an earlier result, want 0", got)
	}
}

// A caller that reuses one buffer for every message must not be able to change what the
// recording says afterwards, and neither must one that queues events. The real sequencer
// keeps neither: the bytes go to libasound on write and come back fresh from a read.
func TestTheRecordingIsIndependentOfTheCallersBytes(t *testing.T) {
	// Each case gets its own buffer, since the point is what survives a caller
	// overwriting the one it wrote from.
	reused := func() []byte { return []byte{midi.MakeNoteOn(0), 60, 100} }

	t.Run("written", func(t *testing.T) {
		buffer := reused()
		seq := New()
		if err := seq.Write(alsa.SeqEvent{Data: buffer}); err != nil {
			t.Fatal(err)
		}
		copy(buffer, []byte{midi.MakeNoteOn(0), 64, 0})
		assertMessage(t, seq.Writes()[0].Event.Data, reused())
	})

	t.Run("queued", func(t *testing.T) {
		buffer := reused()
		seq := New()
		seq.Queue(alsa.SeqEvent{Data: buffer})
		copy(buffer, []byte{midi.MakeNoteOn(0), 64, 0})
		ev, err := seq.Read()
		if err != nil {
			t.Fatal(err)
		}
		assertMessage(t, ev.Data, reused())
	})
}

func TestReadHandsBackQueuedEventsInOrder(t *testing.T) {
	seq := New()
	seq.Queue(noteOn(0, 60), noteOn(0, 64))

	for _, want := range []byte{60, 64} {
		ev, err := seq.Read()
		if err != nil {
			t.Fatal(err)
		}
		assertMessage(t, ev.Data, []byte{midi.MakeNoteOn(0), want, 100})
	}
	if _, err := seq.Read(); !errors.Is(err, ErrNoEvents) {
		t.Errorf("reading a drained queue gave %v, want %v", err, ErrNoEvents)
	}
}

// A client that answered for a while and then went away has to deliver what it already
// sent before the failure surfaces. Failing first would swallow those events, and a
// consumer's read loop would never see what was on its way.
func TestAQueuedEventArrivesBeforeTheReadFailure(t *testing.T) {
	seq := New()
	seq.ReadErr = errReadFailed
	seq.Queue(noteOn(0, 60))

	ev, err := seq.Read()
	if err != nil {
		t.Fatalf("reading a queued event gave %v, want the event", err)
	}
	assertMessage(t, ev.Data, []byte{midi.MakeNoteOn(0), 60, 100})

	if _, err := seq.Read(); !errors.Is(err, errReadFailed) {
		t.Errorf("reading a drained queue gave %v, want %v", err, errReadFailed)
	}
}

// MayRead is how a consumer decides whether to block on Read, so it has to agree with
// whether Read would actually produce an event.
func TestMayReadTracksTheQueue(t *testing.T) {
	seq := New()
	if seq.MayRead() {
		t.Error("MayRead on an empty queue, want false")
	}

	seq.Queue(noteOn(0, 60))
	if !seq.MayRead() {
		t.Fatal("MayRead with an event queued, want true")
	}

	if _, err := seq.Read(); err != nil {
		t.Fatal(err)
	}
	if seq.MayRead() {
		t.Error("MayRead after the last event was read, want false")
	}
}

func TestInjectedFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		arrange func(*Seq)
		act     func(*Seq) error
		want    error
	}{
		{
			name:    "read",
			arrange: func(s *Seq) { s.ReadErr = errReadFailed },
			act:     func(s *Seq) error { _, err := s.Read(); return err },
			want:    errReadFailed,
		},
		{
			name:    "write",
			arrange: func(s *Seq) { s.WriteErr = errWriteFailed },
			act:     func(s *Seq) error { return s.Write(noteOn(0, 60)) },
			want:    errWriteFailed,
		},
		{
			name:    "write port",
			arrange: func(s *Seq) { s.WriteErr = errWriteFailed },
			act:     func(s *Seq) error { return s.WritePort(noteOn(0, 60), 7) },
			want:    errWriteFailed,
		},
		{
			name:    "close",
			arrange: func(s *Seq) { s.CloseErr = errCloseFailed },
			act:     func(s *Seq) error { return s.Close() },
			want:    errCloseFailed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			seq := New()
			test.arrange(seq)
			if err := test.act(seq); !errors.Is(err, test.want) {
				t.Errorf("got %v, want %v", err, test.want)
			}
		})
	}
}

// A failed write must not be recorded, or a test asserting on the recording would see an
// event that never reached the device.
func TestAFailedWriteIsNotRecorded(t *testing.T) {
	seq := New()
	seq.WriteErr = errWriteFailed

	if err := seq.Write(noteOn(0, 60)); err == nil {
		t.Fatal("expected the injected write failure")
	}
	if writes := seq.Writes(); len(writes) != 0 {
		t.Errorf("recorded %d writes after a failure, want none", len(writes))
	}
}

// Repeatable shutdown and an invalidated default port are the two behaviours a consumer
// relies on when it closes a sequencer twice, so the stand-in has to match them rather
// than invent tidier ones.
func TestCloseIsRepeatableAndStopsWrites(t *testing.T) {
	seq := New()
	if err := seq.Write(noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}

	if err := seq.Close(); err != nil {
		t.Fatal(err)
	}
	if err := seq.Close(); err != nil {
		t.Fatalf("closing again: %v", err)
	}
	if seq.Closes() != 2 {
		t.Errorf("Close was called %d times but recorded %d", 2, seq.Closes())
	}
	if seq.Port != -1 {
		t.Errorf("default port is %d after Close, want -1", seq.Port)
	}
	if err := seq.Write(noteOn(0, 64)); !errors.Is(err, ErrClosed) {
		t.Errorf("writing after Close gave %v, want %v", err, ErrClosed)
	}
	if _, err := seq.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("reading after Close gave %v, want %v", err, ErrClosed)
	}
	if len(seq.Writes()) != 1 {
		t.Errorf("recorded %d writes, want only the one from before Close", len(seq.Writes()))
	}
}

func assertMessage(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("MIDI data = %v, want %v", got, want)
	}
}
