package alsa

// The scheduling tests need a sequencer and inherit openTestSeq's hardware skip, so on a
// machine without /dev/snd/seq this whole file passes by not running. The queue's
// refusals are worth checking without one, and the ones that can be are: checkSchedulable
// is a plain function over a message and runs anywhere.

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
)

// The kernel's queue timer ticks at a millisecond and delivers on the first tick after
// the stamp, so an event asked for at d arrives at d or a little after it. A test
// comparing exactly fails on every run, and these are the bounds the timing claim is
// actually made of.
const (
	queueDelay  = 60 * time.Millisecond
	queueMargin = 10 * time.Millisecond
	// queueEarly is how far before its stamp a delivery may measure. The kernel's timer
	// ticks at a millisecond, so the delivery itself is on or after the stamp; what
	// this covers is the comparison between the kernel's clock and Go's, which are read
	// at different instants and turned a 300ms event into 299.98ms.
	queueEarly = time.Millisecond
)

func TestQueueDefersDelivery(t *testing.T) {
	seq := openTestSeq(t, "queue defer")
	queue := startEchoQueue(t, seq, "defer")

	note := noteOn(0, 60, 100)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(queueDelay, MakeEvent(note)))
	if _, arrived := queueWait(seq, note, 40*time.Millisecond); arrived {
		t.Fatal("event arrived before the stamp it was given")
	}
	// Nothing arriving at all is the other failure this test has to catch: a queue that
	// was never really started accepts everything and delivers none of it.
	if elapsed := mustQueueEvent(t, seq, since, note); !withinDelay(elapsed, queueDelay) {
		t.Fatalf("delivered after %v, want %v to %v", elapsed, queueDelay, queueDelay+queueMargin)
	}
}

func TestQueueDelayIsDelay(t *testing.T) {
	seq := openTestSeq(t, "queue delay")
	queue := startEchoQueue(t, seq, "delay")
	for _, delay := range []time.Duration{30 * time.Millisecond, 300 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			note := noteOn(0, 60, 100)
			since := time.Now()
			seqOK(t, queue.ScheduleIn(delay, MakeEvent(note)))
			elapsed := mustQueueEvent(t, seq, since, note)
			if !withinDelay(elapsed, delay) {
				t.Fatalf("delivered after %v, want %v to %v", elapsed, delay, delay+queueMargin)
			}
		})
	}
}

func TestQueueSubMillisecondGapIsOneTick(t *testing.T) {
	seq := openTestSeq(t, "queue collapse")
	queue := startEchoQueue(t, seq, "collapse")

	// A stamp below the timer resolution cannot land on a fraction of a tick, so the
	// pair separates by one tick rather than by the half millisecond asked for. It is
	// not zero either, and that is worth pinning down: the timer rounds each target up
	// from the moment of the call, so two calls a short gap apart straddle a tick
	// boundary about half the time and land on different ones. Measured over twenty
	// trials here, always about 1.07ms apart. A test expecting the two to arrive
	// together fails roughly half the time, and a caller relying on it would be relying
	// on the phase of the clock rather than on the queue.
	first := noteOn(0, 60, 100)
	second := noteOn(0, 67, 100)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(0, MakeEvent(first)))
	seqOK(t, queue.ScheduleIn(500*time.Microsecond, MakeEvent(second)))

	earlier := mustQueueEvent(t, seq, since, first)
	later := mustQueueEvent(t, seq, since, second)
	if gap := later - earlier; gap > 3*time.Millisecond {
		t.Fatalf("500us request arrived %v after its neighbour, want at most one tick", gap)
	}
}

func TestQueueZeroDelayIsImmediate(t *testing.T) {
	seq := openTestSeq(t, "queue zero")
	queue := startEchoQueue(t, seq, "zero")

	// A delay of zero means now, not the next tick, and the distinction is the whole
	// reason a flam's first note can go out on the queue without costing a millisecond.
	note := noteOn(0, 60, 100)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(0, MakeEvent(note)))
	if elapsed := mustQueueEvent(t, seq, since, note); elapsed >= time.Millisecond {
		t.Fatalf("a zero delay was delivered after %v, want immediately", elapsed)
	}
}

func TestQueueRefusesNegativeDelay(t *testing.T) {
	seq := openTestSeq(t, "queue negative")
	queue := startTestQueue(t, seq, "negative")
	note := MakeEvent(noteOn(0, 60, 100))

	if err := queue.ScheduleIn(-time.Second, note); !errors.Is(err, ErrScheduleInPast) {
		t.Fatalf("ScheduleIn(-1s) = %v, want %v", err, ErrScheduleInPast)
	}
	if err := queue.SchedulePort(-time.Second, note, seq.Port); !errors.Is(err, ErrScheduleInPast) {
		t.Fatalf("SchedulePort(-1s) = %v, want %v", err, ErrScheduleInPast)
	}
	// Zero is not negative. It is the first note of a flam, and refusing it would push
	// the flam onto the immediate path and lose the order between its two notes.
	if err := queue.ScheduleIn(0, note); err != nil {
		t.Fatalf("ScheduleIn(0) = %v, want success", err)
	}
}

func TestQueueRefusesStoppedQueue(t *testing.T) {
	seq := openTestSeq(t, "queue stopped")
	fresh, err := seq.AllocQueue("stopped")
	seqOK(t, err)
	t.Cleanup(func() {
		if err := fresh.Close(); err != nil {
			t.Errorf("closing queue: %v", err)
		}
	})
	note := MakeEvent(noteOn(0, 60, 100))

	// The kernel accepts this and never delivers it, so the refusal is the whole
	// difference between a loud failure and silence.
	if err := fresh.ScheduleIn(time.Millisecond, note); !errors.Is(err, ErrQueueStopped) {
		t.Fatalf("schedule into a queue that was never started = %v, want %v", err, ErrQueueStopped)
	}
	seqOK(t, fresh.Start())
	seqOK(t, fresh.Reset())
	if err := fresh.ScheduleIn(time.Millisecond, note); !errors.Is(err, ErrQueueStopped) {
		t.Fatalf("schedule after Reset = %v, want %v", err, ErrQueueStopped)
	}
	seqOK(t, fresh.Start())
	if err := fresh.ScheduleIn(time.Millisecond, note); err != nil {
		t.Fatalf("schedule after starting again = %v, want success", err)
	}
}

// transportStatuses is the set a scheduling call refuses, in one place so that the check
// that refuses them and the tests that assert it cannot drift apart without it being
// visible in both.
var transportStatuses = []byte{midi.Start, midi.Continue, midi.Stop, midi.Clock}

func TestCheckSchedulable(t *testing.T) {
	// Each transport message names the queue it acts on inside its data, which the
	// scheduling macro does not write, so a stamped one would carry a timestamp and
	// still start the wrong queue.
	for _, status := range transportStatuses {
		t.Run(midi.MessageName(status), func(t *testing.T) {
			if err := checkSchedulable([]byte{status}); !errors.Is(err, ErrUnsupportedEvent) {
				t.Errorf("= %v, want %v", err, ErrUnsupportedEvent)
			}
		})
	}
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"note on", noteOn(0, 60, 100)},
		{"note off", noteOff(0, 60)},
		{"control change", []byte{midi.MakeCC(0), 1, 64}},
		{"program change", []byte{midi.MakePgm(0), 1}},
		{"pitch bend", []byte{midi.MakePitch(0), 0x00, 0x40}},
		{"no data", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := checkSchedulable(test.data); err != nil {
				t.Errorf("refused with %v", err)
			}
		})
	}
}

func TestQueueRefusesTransportMessages(t *testing.T) {
	seq := openTestSeq(t, "queue transport")
	queue := startTestQueue(t, seq, "transport")
	for _, status := range transportStatuses {
		t.Run(midi.MessageName(status), func(t *testing.T) {
			err := queue.ScheduleIn(queueDelay, MakeEvent([]byte{status}))
			if !errors.Is(err, ErrUnsupportedEvent) {
				t.Errorf("= %v, want %v", err, ErrUnsupportedEvent)
			}
		})
	}
}

func TestQueuePorts(t *testing.T) {
	seq := openTestSeq(t, "queue ports")
	queue := startEchoQueue(t, seq, "ports")
	other := createTestPort(t, seq, t.Name()+" other")
	seqOK(t, seq.OpenPortReadAt(other, other))
	note := noteOn(0, 60, 100)

	// A named port is honoured all the way through, not merely accepted.
	since := time.Now()
	seqOK(t, queue.SchedulePort(queueDelay, MakeEvent(note), other.Port))
	event := mustQueueData(t, seq, since, note)
	if event.SeqAddr.Port != other.Port {
		t.Errorf("event came from port %d, want %d", event.SeqAddr.Port, other.Port)
	}

	// A port number this sequencer never created is refused, whoever has it.
	if err := queue.SchedulePort(queueDelay, MakeEvent(note), maxSeqAddress); err == nil {
		t.Error("SchedulePort accepted a port this sequencer never created")
	}
	if err := queue.SchedulePort(queueDelay, MakeEvent(note), -1); err == nil {
		t.Error("SchedulePort accepted a negative port")
	}
	seqOK(t, seq.DeletePort(other))
	if err := queue.SchedulePort(queueDelay, MakeEvent(note), other.Port); err == nil {
		t.Error("SchedulePort accepted a deleted port")
	}
}

func TestQueueResetDiscardsPending(t *testing.T) {
	seq := openTestSeq(t, "queue reset")
	queue := startEchoQueue(t, seq, "reset")

	first := noteOn(0, 60, 100)
	grace := noteOn(0, 67, 100)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(0, MakeEvent(first)))
	seqOK(t, queue.ScheduleIn(200*time.Millisecond, MakeEvent(grace)))
	mustQueueEvent(t, seq, since, first)

	seqOK(t, queue.Reset())
	if _, arrived := queueWait(seq, grace, 250*time.Millisecond); arrived {
		t.Fatal("event pending at Reset was delivered")
	}
	// The queue is usable again, and comes up stopped, which is the other half of what
	// a caller has to remember after a stop.
	if err := queue.ScheduleIn(0, MakeEvent(grace)); !errors.Is(err, ErrQueueStopped) {
		t.Fatalf("schedule after Reset = %v, want %v", err, ErrQueueStopped)
	}
	seqOK(t, queue.Start())
	restarted := time.Now()
	seqOK(t, queue.ScheduleIn(0, MakeEvent(grace)))
	mustQueueEvent(t, seq, restarted, grace)
}

func TestQueueImmediateWritePassesPending(t *testing.T) {
	seq := openTestSeq(t, "queue ordering")
	queue := startEchoQueue(t, seq, "ordering")

	queued := noteOn(0, 60, 100)
	direct := noteOn(0, 67, 100)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(200*time.Millisecond, MakeEvent(queued)))
	seqOK(t, seq.Write(MakeEvent(direct)))

	mustQueueEvent(t, seq, since, direct)
	// The direct write did not take the queued event's place: an event submitted first
	// and stamped later is still waiting, which is why a caller stopping mid-flam has
	// to reset before it writes its note-offs.
	if _, arrived := queueWait(seq, queued, 100*time.Millisecond); arrived {
		t.Fatal("queued event was delivered before its stamp")
	}
	mustQueueEvent(t, seq, since, queued)
}

func TestQueueClosedParent(t *testing.T) {
	seq := openTestSeq(t, "queue closed parent")
	queue, err := seq.AllocQueue("closed-parent")
	seqOK(t, err)
	seqOK(t, queue.Start())
	seqOK(t, seq.Close())

	note := MakeEvent(noteOn(0, 60, 100))
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"ScheduleIn", func() error { return queue.ScheduleIn(time.Millisecond, note) }},
		{"SchedulePort", func() error { return queue.SchedulePort(time.Millisecond, note, seq.Port) }},
		{"Start", queue.Start},
		{"Reset", queue.Reset},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, ErrSeqClosed) {
				t.Errorf("on a closed sequencer = %v, want %v", err, ErrSeqClosed)
			}
		})
	}
	if _, err := seq.AllocQueue("closed"); !errors.Is(err, ErrSeqClosed) {
		t.Errorf("AllocQueue on a closed sequencer = %v, want %v", err, ErrSeqClosed)
	}
	// The client took its queues with it, so there is nothing left to release, and a
	// caller that closes both in either order has not made a mistake.
	if err := queue.Close(); err != nil {
		t.Errorf("Close after the sequencer closed = %v, want nil", err)
	}
	if err := queue.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

func TestQueueClosedQueue(t *testing.T) {
	seq := openTestSeq(t, "queue released")
	queue := startTestQueue(t, seq, "released")
	seqOK(t, queue.Close())

	note := MakeEvent(noteOn(0, 60, 100))
	for _, test := range []struct {
		name string
		call func() error
	}{
		{"ScheduleIn", func() error { return queue.ScheduleIn(time.Millisecond, note) }},
		{"Start", queue.Start},
		{"Reset", queue.Reset},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Error("on a released queue succeeded")
			}
		})
	}
	// Repeating Close is not a caller's mistake, and the queue identifier has since
	// belonged to somebody else, so it must be answered from here.
	if err := queue.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

func TestQueueCapacity(t *testing.T) {
	seq := openTestSeq(t, "queue capacity")
	queue := startTestQueue(t, seq, "capacity")
	note := MakeEvent(noteOn(0, 60, 100))

	// The kernel does not report its own ceiling: past it the submission is accepted and
	// the flush that carries it through blocks until the queue delivers something. So
	// what this asserts is the package's ceiling, the error it returns, and that the
	// queue is still usable afterwards rather than a number the kernel happened to have
	// on this host.
	for accepted := 0; accepted < queueEventCeiling+16; accepted++ {
		err := queue.ScheduleIn(time.Hour, note)
		if errors.Is(err, ErrQueueFull) {
			if accepted != queueEventCeiling {
				t.Fatalf("queue filled after %d events, want %d", accepted, queueEventCeiling)
			}
			seqOK(t, queue.Reset())
			seqOK(t, queue.Start())
			seqOK(t, queue.ScheduleIn(time.Hour, note))
			return
		}
		if err != nil {
			t.Fatalf("event %d: %v", accepted, err)
		}
	}
	t.Fatalf("queue accepted more than %d events", queueEventCeiling)
}

func TestQueueSysExSurvives(t *testing.T) {
	seq := openTestSeq(t, "queue sysex")
	queue := startEchoQueue(t, seq, "sysex")

	// The sequencer copies a variable-length event while it is being submitted, so the
	// caller's buffer is free the moment ScheduleIn returns. Overwriting it is what
	// makes this a test of that copy rather than of the delivery: without it the event
	// would arrive as filler, or the process would not survive the test at all.
	original := []byte{midi.SysEx, 0x7d, 0x01, 0x02, 0x03, midi.EndSysEx}
	payload := bytes.Clone(original)
	since := time.Now()
	seqOK(t, queue.ScheduleIn(queueDelay, MakeEvent(payload)))
	for index := range payload {
		payload[index] = 0xEE
	}

	event := mustQueueData(t, seq, since, original)
	if !bytes.Equal(event.Data, original) {
		t.Fatalf("SysEx arrived as % x, want % x", event.Data, original)
	}
}

func BenchmarkQueueSchedule(b *testing.B) {
	seq, err := OpenSeq("queue-benchmark")
	if err != nil {
		b.Skipf("ALSA sequencer unavailable: %v", err)
	}
	defer seq.Close()
	queue, err := seq.AllocQueue("benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer queue.Close()
	if err := queue.Start(); err != nil {
		b.Fatal(err)
	}
	event := MakeEvent(noteOn(0, 60, 100))
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		// Far enough ahead that nothing delivered during the run can refill the queue,
		// which matters because the submission checks how much is pending. That check
		// then means the run has to clear the queue itself, and a Reset is not part of
		// what is being measured.
		err := queue.ScheduleIn(time.Hour, event)
		if errors.Is(err, ErrQueueFull) {
			b.StopTimer()
			seqOK(b, queue.Reset())
			seqOK(b, queue.Start())
			b.StartTimer()
			continue
		}
		seqOK(b, err)
	}
}

// noteOn and noteOff build whole messages. The midi package makes status bytes and
// leaves the payload to the caller, which is right at the wire and means every message
// in this file would otherwise repeat its own three bytes.
func noteOn(channel, note, velocity byte) []byte {
	return []byte{midi.MakeNoteOn(int(channel)), note, velocity}
}

func noteOff(channel, note byte) []byte {
	return []byte{midi.MakeNoteOff(int(channel)), note, 0}
}

// withinDelay is the timing assertion the queue's contract is stated in: at or after the
// stamp, and inside a margin that absorbs the kernel's tick without accepting a delivery
// that is late enough to hear.
func withinDelay(elapsed, want time.Duration) bool {
	return elapsed >= want-queueEarly && elapsed <= want+queueMargin
}

// startTestQueue allocates a queue on seq, starts it and releases it when the test ends.
func startTestQueue(t *testing.T, seq *Seq, name string) *SeqQueue {
	t.Helper()
	queue, err := seq.AllocQueue(name)
	seqOK(t, err)
	t.Cleanup(func() {
		if err := queue.Close(); err != nil {
			t.Errorf("closing queue: %v", err)
		}
	})
	seqOK(t, queue.Start())
	return queue
}

// startEchoQueue is the arrangement every test that reads back what it scheduled needs: a
// running queue, and a port that reads this client's own output. The subscription is what
// turns "the call returned" into "the note sounded", which is the only thing most of these
// tests are for.
func startEchoQueue(t *testing.T, seq *Seq, name string) *SeqQueue {
	t.Helper()
	// The default port is duplex, so it can be the source of an event and its own
	// subscriber. It also delivers ALSA's subscription notifications on that same read,
	// which is why the waiters skip anything they are not looking for.
	seqOK(t, seq.OpenPortReadAt(seq.SeqAddr, seq.SeqAddr))
	return startTestQueue(t, seq, name)
}

// queueWait reads until it sees want or the timeout passes, and reports whether it did.
// Not arriving is not a failure here: the deferral test asks the question and needs an
// answer either way, and failing inside the wait would end the test before it could ask
// the second half.
//
// It polls rather than calling Read, which blocks inside libasound and would hang the
// test goroutine instead of failing it. Events that are not the one being waited for are
// dropped, which is what a self-subscribed port needs: ALSA's own subscription
// notifications arrive on the same read.
func queueWait(seq *Seq, want []byte, timeout time.Duration) (SeqEvent, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if seq.MayRead() {
			if event, err := seq.Read(); err == nil && slices.Equal(event.Data, want) {
				return event, true
			}
		}
		if time.Now().After(deadline) {
			return SeqEvent{}, false
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// mustQueueData waits for want and hands back the event, failing the test if it never
// arrives.
func mustQueueData(t *testing.T, seq *Seq, since time.Time, want []byte) SeqEvent {
	t.Helper()
	event, arrived := queueWait(seq, want, seqTestTimeout)
	if !arrived {
		t.Fatalf("timed out waiting for %v", want)
	}
	return event
}

// mustQueueEvent is mustQueueData for a test that only cares when the event arrived.
func mustQueueEvent(t *testing.T, seq *Seq, since time.Time, want []byte) time.Duration {
	t.Helper()
	mustQueueData(t, seq, since, want)
	return time.Since(since)
}
