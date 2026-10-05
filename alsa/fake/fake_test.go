package fake

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// The failures a test asks the stand-in to produce, kept in one place so each test
// asserts on the one it provoked.
var (
	errReadFailed  = errors.New("read failed")
	errWriteFailed = errors.New("write failed")
	errCloseFailed = errors.New("close failed")

	errStartFailed    = errors.New("start failed")
	errScheduleFailed = errors.New("schedule failed")
	errResetFailed    = errors.New("reset failed")
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

// A stand-in for a queue is only useful if it records what the caller asked for, in the
// form the caller asked for it: the delay it named, and the port it named it from.
func TestQueueRecordsWhatWasScheduled(t *testing.T) {
	queue := NewQueue()
	queue.Port = 3
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}

	if err := queue.ScheduleIn(20*time.Millisecond, noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}
	if err := queue.SchedulePort(0, noteOn(0, 64), 7); err != nil {
		t.Fatal(err)
	}

	scheduled := queue.Scheduled()
	if len(scheduled) != 2 {
		t.Fatalf("recorded %d submissions, want 2", len(scheduled))
	}
	if scheduled[0].Port != 3 || scheduled[0].Delay != 20*time.Millisecond {
		t.Errorf("first submission was port %d after %v, want port 3 after 20ms",
			scheduled[0].Port, scheduled[0].Delay)
	}
	if scheduled[1].Port != 7 || scheduled[1].Delay != 0 {
		t.Errorf("second submission was port %d after %v, want port 7 after 0",
			scheduled[1].Port, scheduled[1].Delay)
	}
	assertMessage(t, scheduled[1].Event.Data, []byte{midi.MakeNoteOn(0), 64, 100})
}

// The real queue accepts an event submitted to one that is not running and never plays
// it, which is why the stand-in refuses rather than recording, so that a consumer's
// missing Start shows up in its own tests.
func TestQueueRefusesUntilStarted(t *testing.T) {
	queue := NewQueue()
	if queue.Running() {
		t.Error("a new queue reports itself running")
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); !errors.Is(err, alsa.ErrQueueStopped) {
		t.Fatalf("scheduling before Start gave %v, want %v", err, alsa.ErrQueueStopped)
	}

	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if !queue.Running() {
		t.Error("Start left the queue not running")
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
		t.Fatalf("scheduling after Start gave %v, want success", err)
	}
	if err := queue.Start(); err != nil {
		t.Fatalf("starting again gave %v, want success", err)
	}
	if queue.Starts() != 1 {
		t.Errorf("Start was called twice and began a queue %d times, want 1", queue.Starts())
	}

	if err := queue.Reset(); err != nil {
		t.Fatal(err)
	}
	if queue.Running() {
		t.Error("Reset left the queue running")
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); !errors.Is(err, alsa.ErrQueueStopped) {
		t.Fatalf("scheduling after Reset gave %v, want %v", err, alsa.ErrQueueStopped)
	}
}

// A negative delay is the caller's own mistake and the real queue names it, so a consumer
// with a branch for that branch has to be able to reach it without hardware.
func TestQueueRefusesNegativeDelay(t *testing.T) {
	queue := NewQueue()
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if err := queue.ScheduleIn(-time.Second, noteOn(0, 60)); !errors.Is(err, alsa.ErrScheduleInPast) {
		t.Fatalf("scheduling a negative delay gave %v, want %v", err, alsa.ErrScheduleInPast)
	}
	if len(queue.Scheduled()) != 0 {
		t.Error("a refused submission was recorded")
	}
}

// Reset frees the queue, so what was on it goes with it. A stand-in that kept the
// recording would let a consumer's stop path look correct without discarding anything.
func TestQueueResetDiscardsWhatWasOnIt(t *testing.T) {
	queue := NewQueue()
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}

	if err := queue.Reset(); err != nil {
		t.Fatal(err)
	}
	if got := queue.Scheduled(); len(got) != 0 {
		t.Errorf("%d submissions survived Reset, want none", len(got))
	}
	if queue.Resets() != 1 {
		t.Errorf("Resets counted %d, want 1", queue.Resets())
	}
}

func TestQueueInjectedFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		arrange func(*Queue)
		act     func(*Queue) error
		want    error
	}{
		{
			name:    "start",
			arrange: func(q *Queue) { q.StartErr = errStartFailed },
			act:     func(q *Queue) error { return q.Start() },
			want:    errStartFailed,
		},
		{
			name:    "schedule",
			arrange: func(q *Queue) { q.ScheduleErr = errScheduleFailed },
			act:     func(q *Queue) error { return q.ScheduleIn(time.Millisecond, noteOn(0, 60)) },
			want:    errScheduleFailed,
		},
		{
			name:    "schedule port",
			arrange: func(q *Queue) { q.ScheduleErr = errScheduleFailed },
			act:     func(q *Queue) error { return q.SchedulePort(time.Millisecond, noteOn(0, 60), 7) },
			want:    errScheduleFailed,
		},
		{
			name:    "reset",
			arrange: func(q *Queue) { q.ResetErr = errResetFailed },
			act:     func(q *Queue) error { return q.Reset() },
			want:    errResetFailed,
		},
		{
			name:    "close",
			arrange: func(q *Queue) { q.CloseErr = errCloseFailed },
			act:     func(q *Queue) error { return q.Close() },
			want:    errCloseFailed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			queue := NewQueue()
			if err := queue.Start(); err != nil {
				t.Fatal(err)
			}
			test.arrange(queue)
			if err := test.act(queue); !errors.Is(err, test.want) {
				t.Errorf("got %v, want %v", err, test.want)
			}
		})
	}
}

// The same two behaviours as the sequencer's own Close: repeatable, and everything after
// it refused rather than accepted by a stand-in that had tidied the state up.
func TestQueueCloseIsRepeatableAndStopsScheduling(t *testing.T) {
	queue := NewQueue()
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
		t.Fatal(err)
	}

	if err := queue.Close(); err != nil {
		t.Fatal(err)
	}
	if err := queue.Close(); err != nil {
		t.Fatalf("closing again: %v", err)
	}
	if queue.Closes() != 2 {
		t.Errorf("Close was called twice but recorded %d", queue.Closes())
	}
	if queue.Port != -1 {
		t.Errorf("default port is %d after Close, want -1", queue.Port)
	}
	for name, call := range map[string]func() error{
		"ScheduleIn":   func() error { return queue.ScheduleIn(time.Millisecond, noteOn(0, 64)) },
		"SchedulePort": func() error { return queue.SchedulePort(time.Millisecond, noteOn(0, 64), 7) },
		"Start":        queue.Start,
		"Reset":        queue.Reset,
	} {
		if err := call(); !errors.Is(err, ErrClosed) {
			t.Errorf("%s after Close gave %v, want %v", name, err, ErrClosed)
		}
	}
	if len(queue.Scheduled()) != 1 {
		t.Errorf("recorded %d submissions, want only the one from before Close", len(queue.Scheduled()))
	}
}

// A caller that reuses one buffer must not be able to change what the recording says,
// which is the same requirement the sequencer's recording has.
func TestTheScheduledRecordingIsIndependentOfTheCallersBytes(t *testing.T) {
	buffer := []byte{midi.MakeNoteOn(0), 60, 100}
	queue := NewQueue()
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if err := queue.ScheduleIn(time.Millisecond, alsa.SeqEvent{Data: buffer}); err != nil {
		t.Fatal(err)
	}

	copy(buffer, []byte{midi.MakeNoteOn(0), 64, 0})

	assertMessage(t, queue.Scheduled()[0].Event.Data, []byte{midi.MakeNoteOn(0), 60, 100})
}

// A consumer that recovers from a full queue has a branch for alsa.ErrQueueFull, and the
// real ceiling is a number this package cannot see, so the stand-in takes one as a field
// and counts submissions rather than pretending to know it.
func TestQueueCeilingIsOptIn(t *testing.T) {
	unlimited := NewQueue()
	if err := unlimited.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := unlimited.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
			t.Fatalf("submission %d with no ceiling set gave %v", i, err)
		}
	}

	queue := NewQueue()
	queue.Ceiling = 2
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
			t.Fatalf("submission %d below the ceiling gave %v", i, err)
		}
	}
	err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60))
	if !errors.Is(err, alsa.ErrQueueFull) {
		t.Fatalf("submission past the ceiling gave %v, want %v", err, alsa.ErrQueueFull)
	}
	if len(queue.Scheduled()) != 2 {
		t.Errorf("recorded %d submissions, want the 2 that were accepted", len(queue.Scheduled()))
	}

	// Reset is the recovery, and it is the only thing that makes room.
	if err := queue.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(); err != nil {
		t.Fatal(err)
	}
	if err := queue.ScheduleIn(time.Millisecond, noteOn(0, 60)); err != nil {
		t.Errorf("submission after Reset gave %v, want success", err)
	}
}

func assertMessage(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("MIDI data = %v, want %v", got, want)
	}
}
