package alsa

// This file holds scheduled output: a queue the kernel delivers from, and the events a
// caller stamps with how long from now each should sound. The calls those methods make
// live in seq_cgo.go, and what is written here is what a caller may rely on rather than
// how the behaviour behind it was established.

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/chzchzchz/midispa/midi"
)

var (
	// ErrScheduleInPast is a ScheduleIn whose delay was negative.
	ErrScheduleInPast = errors.New("scheduled delay is not in the future")

	// ErrQueueStopped is a ScheduleIn against a queue that is not running.
	ErrQueueStopped = errors.New("queue is not running")

	// ErrQueueFull is a ScheduleIn against a queue already holding queueEventCeiling
	// events. Reset is the recovery, and is the only thing that clears it.
	ErrQueueFull = errors.New("queue is full")

	// ErrSeqClosed is a sequencer operation reached after the client was closed.
	ErrSeqClosed = errors.New("sequencer is closed")
)

// queueEventCeiling is how many events this package will leave pending on one queue
// before it refuses another.
//
// It is a policy of this package rather than a limit reported by the kernel, because the
// kernel reports nothing: measured here, the 501st submission is accepted by
// snd_seq_event_output and then blocks forever in the flush that follows it, because the
// queue will not take it and will not deliver anything else either. A ceiling the kernel
// enforces by blocking is worse than no ceiling, and worse than one of ours, because it
// takes the writer down mid-performance with nothing to recover with.
//
// The number is well under the lowest ceiling measured on this host (500 accepted, 583 on
// an earlier measurement of the same machine) so that the refusal arrives before the block
// rather than racing it. It is far more lookahead than this repository asks for: a flam is
// three events.
const queueEventCeiling = 256

// errQueueClosed is what the methods of a released SeqQueue return. It stays unexported
// because it reports the caller's own lifetime mistake rather than a condition to branch
// on, and the three above are the conditions a caller may be asked to handle.
var errQueueClosed = errors.New("queue is closed")

// SeqQueue is a scheduled-output queue. A caller hands it events stamped with how long
// from now each should sound and the kernel delivers them, so the timing of a gap between
// two notes is the kernel's rather than Go's.
//
// Each stamp is relative to the call that carries it rather than to a shared origin, so
// two calls made a millisecond apart sit a millisecond further apart than the caller may
// have intended. That is the price of not publishing a clock the caller could compare
// wrongly, and it is a price worth paying only for the gaps a flam, a ratchet or a drag
// needs: the kernel's timer rounds each target up to its next millisecond, so a gap shorter
// than that is not merely imprecise, it is a tick long whatever was asked for.
//
// A queue is not safe for concurrent use. *Seq is not thread-safe for a shared client and
// this adds no lock to it, so a caller with one writer needs nothing here and a caller
// with two has a bug no type in this package can see.
type SeqQueue struct {
	seq *Seq
	q   int
	// started is what this type believes about the kernel, and the only thing
	// ErrQueueStopped can be decided from. A start reports success whether or not it has
	// been flushed, so there is nowhere to read the real state back from.
	started bool
	// closed marks the queue released, and is separate from the handle because ALSA
	// reuses a queue identifier for the next allocation: a second Close has to be
	// answered from here rather than by asking the kernel about an id that may since
	// have become somebody else's.
	closed bool
	// name is kept because Reset reallocates through snd_seq_alloc_named_queue, which
	// takes a name. Dropping it would force Reset onto the unnamed allocation and rename
	// the queue in aconnect and aplay without saying so.
	name string
}

// AllocQueue creates a queue and attaches it to the client. One per Seq is enough for
// every use in this repository; a second is allowed and independent.
//
// A client may hold 32 queues and the next allocation fails with an out-of-memory error
// rather than a distinguishable one, so a caller keeping a queue per song has to Close
// the ones it is finished with.
func (a *Seq) AllocQueue(name string) (*SeqQueue, error) {
	if !a.isOpen() {
		return nil, ErrSeqClosed
	}
	return a.newQueue(name)
}

// newQueue is the allocation behind AllocQueue and Reset, kept in one place because a
// queue that came back from Reset has to be indistinguishable from one that was just
// created, and two constructors would be two chances to make them differ.
func (a *Seq) newQueue(name string) (*SeqQueue, error) {
	queue, err := a.allocNamedQueue(name)
	if err != nil {
		return nil, err
	}
	return &SeqQueue{seq: a, q: queue, name: name}, nil
}

// Start begins delivery, and is safe to repeat: starting a running queue returns nil and
// changes nothing. A queue has to be started before anything can be scheduled into it,
// and the scheduling methods say so rather than accepting an event that will never sound.
//
// This is also the only point at which the type learns whether the queue is running, which
// is why the flush the kernel requires happens inside it. An unstarted queue that has been
// told it started is indistinguishable from a working one from here, and every event after
// it disappears without an error.
func (q *SeqQueue) Start() error {
	if err := q.usable(); err != nil {
		return err
	}
	if q.started {
		return nil
	}
	if err := q.seq.startQueue(q.q); err != nil {
		return err
	}
	q.started = true
	return nil
}

// Reset stops the queue, discards everything still scheduled on it and allocates a fresh
// one. It is the only way to stop, because a stop that keeps what is queued does not exist
// from this kernel: its own stop takes the pending events with it, so there is no
// pause-and-keep behaviour to offer under any name.
//
// The new queue comes up stopped, so scheduling into it is refused until Start is called
// again. It also has a new identifier and a new clock origin, which costs a caller nothing
// here because no absolute time crosses this API: what was pending is discarded, and what
// comes after is measured from the call that submits it.
func (q *SeqQueue) Reset() error {
	if err := q.usable(); err != nil {
		return err
	}
	if err := q.seq.stopQueue(q.q); err != nil {
		return err
	}
	if err := q.seq.freeQueue(q.q); err != nil {
		return err
	}
	fresh, err := q.seq.newQueue(q.name)
	if err != nil {
		return err
	}
	// After a Reset the queue is indistinguishable from a fresh allocation, so the old
	// one's state is replaced wholesale rather than field by field.
	*q = *fresh
	return nil
}

// Close releases the queue. Whatever is pending is discarded rather than delivered, which
// is what the sequencer does with a queue that no longer exists. Closing is safe to repeat
// and safe after the owning Seq has been closed, because the client took its queues with
// it: a caller that closes both, in either order, has nothing left to release.
//
// The other methods report a released queue with an error rather than acting on an
// identifier the sequencer has already handed to somebody else.
func (q *SeqQueue) Close() error {
	if q.closed {
		return nil
	}
	q.closed = true
	q.started = false
	if q.seq == nil || !q.seq.isOpen() {
		return nil
	}
	return q.seq.freeQueue(q.q)
}

// ScheduleIn queues ev for delivery d from now, out of the sequencer's default port. It
// is the delayed twin of Seq.Write and takes the same event, and which port that is
// follows SeqAddr exactly as Write follows it, so redirecting the default redirects this
// too.
//
// A delay of zero means as soon as the queue reaches it, which is what lets a caller put a
// whole flam on one queue and have the notes arrive in the order they asked for. Only a
// negative delay is refused, with ErrScheduleInPast, and only a queue that is not running
// is refused, with ErrQueueStopped. Neither of those is a restriction the kernel shares:
// it accepts either event and then never delivers it, which is a note that does not sound
// rather than an error the caller can see.
//
// The kernel's queue timer ticks at a millisecond and rounds every target up to the next
// tick from the moment of the call. Zero is already due and goes out at once, which is
// what lets a flam's first note sit on the queue without costing a millisecond. Anything
// from a microsecond up to a tick lands on that same next tick, so a gap shorter than the
// timer's resolution cannot be had: two notes come out a tick apart rather than half a
// millisecond apart, and not together either, because each call is rounded up from its own
// moment. A gap of a millisecond or more is kept as asked for, to within the timer and
// whatever the scheduler is doing at the time — measured here, a 20ms flam arrives at
// 20.1ms.
func (q *SeqQueue) ScheduleIn(d time.Duration, ev SeqEvent) error {
	if err := q.usable(); err != nil {
		return err
	}
	return q.schedule(d, ev, q.seq.Port)
}

// SchedulePort is ScheduleIn out of a named port. The port must still be owned by the
// sequencer, and one that has been deleted is refused rather than quietly written from the
// default, which is the rule WritePort already follows.
func (q *SeqQueue) SchedulePort(d time.Duration, ev SeqEvent, port int) error {
	if err := q.usable(); err != nil {
		return err
	}
	return q.schedule(d, ev, port)
}

// schedule is the body both scheduling methods share, differing only in the port the
// event is written from. Every refusal happens before the sequencer is touched, because
// the sequencer's own answer to each of them is silence.
func (q *SeqQueue) schedule(delay time.Duration, ev SeqEvent, port int) error {
	if delay < 0 {
		return ErrScheduleInPast
	}
	if !q.started {
		return ErrQueueStopped
	}
	if err := checkSchedulable(ev.Data); err != nil {
		return err
	}
	// Read after everything refusable without a syscall and before the submission,
	// because past the kernel's own limit the flush that carries a submission through
	// blocks rather than failing.
	pending, err := q.seq.queuePending(q.q)
	if err != nil {
		return err
	}
	if pending >= queueEventCeiling {
		return fmt.Errorf("%w: %d of %d events pending", ErrQueueFull, pending, queueEventCeiling)
	}
	event, err := q.seq.encodeForPort(ev, port)
	if err != nil || event == nil {
		return err
	}
	if err := q.seq.outputStamped(event, q.q, delay); err != nil {
		return fmt.Errorf("queue %q: %s after %v: %w", q.name, midi.MessageName(ev.Data[0]), delay, err)
	}
	// A variable-length event points into this slice. The sequencer copies the payload
	// while the submission is being made rather than when the queued event is finally
	// played, so the slice has to outlive the call and nothing beyond it: measured here,
	// clobbering the buffer between the submission and the flush leaves the event intact.
	//
	// The KeepAlive is belt and braces rather than the only thing holding it. The encoded
	// event is an argument to outputStamped, and the pointer it carries is a Go pointer,
	// so the collector already keeps the slice alive for the length of that call. It
	// stays for the day the event stops being Go memory, which is the day this quietly
	// stops being true, and it costs nothing at runtime.
	runtime.KeepAlive(ev.Data)
	return nil
}

// checkSchedulable refuses the transport messages. Each names the queue it acts on inside
// the event's data, and that field is not the one the scheduling macro writes, so a
// stamped Start would carry a timestamp and still start queue 253 rather than the queue it
// was submitted to. Rewriting that field works today and is the obvious fix; refusing
// keeps the correctness of the queue in one place instead of spread across every message
// type someone remembers to enumerate.
//
// An event carrying no bytes is nothing to refuse, the same way it is nothing to send.
func checkSchedulable(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	switch midi.Message(data[0]) {
	case midi.Start, midi.Continue, midi.Stop, midi.Clock:
		return fmt.Errorf("%w: %s names a queue in its data and cannot be scheduled",
			ErrUnsupportedEvent, midi.MessageName(data[0]))
	}
	return nil
}

// usable reports whether this queue may still act. A caller holding both a Seq and its
// queues closes them in whatever order its shutdown reaches them, so the check belongs
// here rather than being repeated at each call site.
func (q *SeqQueue) usable() error {
	if q.closed {
		return errQueueClosed
	}
	if q.seq == nil || !q.seq.isOpen() {
		return ErrSeqClosed
	}
	return nil
}
