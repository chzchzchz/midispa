// Package fake provides a stand-in for the ALSA sequencer.
//
// The sequencer is the only part of this repository's MIDI plumbing that needs hardware,
// so a consumer holding one of the narrow interfaces alsa publishes can be driven by a
// Seq from here instead. That is what lets the event loop, the port routing and the
// shutdown path above the sequencer be exercised with no /dev/snd/seq and no MIDI
// hardware.
//
// It is a separate package on purpose. A fake in alsa's own test files is invisible to
// every consumer package, which is where the untested code is; a fake in the alsa
// package itself would be linked into every production binary. Nothing imports this
// package unless a test does.
package fake

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

var (
	// ErrNoEvents is what Read returns once the queue is empty. The real sequencer
	// blocks in libasound instead, so a read loop driven by a fake has to treat this as
	// "nothing right now". Every consumer already handles a read error, so this needs
	// no code path of its own, but it does mean a consumer's loop must not be written so
	// that an error ends the program.
	ErrNoEvents = errors.New("alsa/fake: no events queued")
	// ErrClosed is what a read or a write returns after Close, matching *alsa.Seq
	// rejecting operations on a released client.
	ErrClosed = errors.New("alsa/fake: sequencer is closed")
)

// Write records one event that reached the sequencer.
type Write struct {
	// Port is the local port the event went out on: the default for Write, and whatever
	// WritePort was handed for WritePort. A test asserts on it to tell two ports of one
	// sequencer apart.
	Port int
	// Event is the event as it was written, destination included.
	Event alsa.SeqEvent
}

// Seq stands in for *alsa.Seq and satisfies every interface alsa publishes, so a
// consumer can be handed one wherever it would otherwise take a sequencer.
//
// It has no lock, for the same reason *alsa.Seq has none: the real type makes callers
// serialize lifecycle changes against other operations by contract, and a lock here
// would let a test pass against a guarantee the real type does not make. A test reading
// recorded writes while a worker goroutine is still writing has a sequencing bug; fix the
// test's sequencing rather than reaching for a mutex.
//
// It is more permissive than the real type in two places, both stated at the method:
//
//   - A write on any local port is accepted. Ownership is Seq's own bookkeeping rather
//     than anything the transport does, and there is no ALSA here to have assigned a
//     port number, so enforcing it would only mean every test had to invent one.
//   - Addresses and messages are not validated. That validation is Seq's policy, it is
//     covered by alsa's own tests, and repeating it here would prove nothing beyond
//     that this file agrees with itself.
type Seq struct {
	// SeqAddr mirrors the address embedded in *alsa.Seq, so a consumer that reads
	// SeqAddr.Port to decide where to write sees the same thing here.
	alsa.SeqAddr

	// ReadErr, WriteErr and CloseErr are the failures a test asks for. They are fields
	// rather than constructor arguments because a test usually sets one halfway
	// through, after it has seen the working case.
	//
	// ReadErr waits its turn: a queued event is still handed over before the failure
	// surfaces, so a test can describe a client that answered for a while and then went
	// away. Which is the only way that happens to a real one, and it is what a consumer's
	// read loop has to survive.
	ReadErr  error
	WriteErr error
	CloseErr error

	writes []Write
	reads  []alsa.SeqEvent
	closes int
	closed bool
}

// New returns a sequencer whose default port is port 0. The real OpenSeq stores
// whatever port number ALSA assigned, which is not knowable in advance; zero is as
// serviceable a stand-in and is the value a consumer that never called CreatePortAddr
// will see.
func New() *Seq {
	return &Seq{SeqAddr: alsa.SeqAddr{Port: 0}}
}

// Write sends on the default port, as *alsa.Seq does.
func (s *Seq) Write(ev alsa.SeqEvent) error {
	return s.WritePort(ev, s.Port)
}

// WritePort sends from an explicitly chosen local port.
//
// The event data is copied. The real sequencer hands the bytes to libasound and
// retains nothing, so a caller that reuses one buffer for every message must not be able
// to change what the recording says afterwards.
func (s *Seq) WritePort(ev alsa.SeqEvent, port int) error {
	if s.closed {
		return ErrClosed
	}
	if s.WriteErr != nil {
		return s.WriteErr
	}
	s.writes = append(s.writes, Write{Port: port, Event: alsa.SeqEvent{
		SeqAddr: ev.SeqAddr,
		Data:    bytes.Clone(ev.Data),
	}})
	return nil
}

// Read hands back the next queued event. It does not block, where the real Read waits
// inside libasound: a fake that waited would need something to wake it, and that
// something would be a lock the real type does not have.
//
// A drained queue gives ErrNoEvents, or ReadErr when one is set. Closing outranks both:
// a released client answers nothing, not even the events already queued.
func (s *Seq) Read() (alsa.SeqEvent, error) {
	if s.closed {
		return alsa.SeqEvent{}, ErrClosed
	}
	if len(s.reads) == 0 {
		if s.ReadErr != nil {
			return alsa.SeqEvent{}, s.ReadErr
		}
		return alsa.SeqEvent{}, ErrNoEvents
	}
	ev := s.reads[0]
	s.reads = s.reads[1:]
	return ev, nil
}

// MayRead reports whether an event is waiting, which is the question the real MayRead
// asks of libasound without consuming anything. It says nothing about ReadErr: a consumer
// looping on MayRead until it is false will never see a read failure, and has to call Read
// to find out whether the queue is empty or the client has gone away.
func (s *Seq) MayRead() bool {
	return len(s.reads) > 0
}

// Queue adds events for Read to hand back, oldest first. They are copied for the same
// reason writes are: the real Read decodes into fresh bytes for every event.
func (s *Seq) Queue(events ...alsa.SeqEvent) {
	for _, ev := range events {
		s.reads = append(s.reads, alsa.SeqEvent{
			SeqAddr: ev.SeqAddr,
			Data:    bytes.Clone(ev.Data),
		})
	}
}

// Writes returns the events written so far, oldest first. It copies, so a test cannot
// reach into the recording and change what a later assertion sees.
func (s *Seq) Writes() []Write {
	return append([]Write(nil), s.writes...)
}

// Closes returns how many times Close was called, which is what tells a caller that
// Close really is repeatable rather than merely returning nil.
func (s *Seq) Closes() int { return s.closes }

// Close releases the sequencer. It is safe to repeat, as *alsa.Seq.Close is, and it
// invalidates the default port the same way so that a later write fails loudly rather
// than going to a port nobody owns.
func (s *Seq) Close() error {
	s.closes++
	if s.closed {
		return nil
	}
	s.closed = true
	s.Port = -1
	return s.CloseErr
}

// Scheduled records one event a caller asked to have delivered later.
type Scheduled struct {
	// Port is the local port the event was submitted from: the default for ScheduleIn,
	// and whatever was named for SchedulePort.
	Port int
	// Delay is how long after the call the caller asked for the event to sound. Nothing
	// here waits that long. What is recorded is the request, which is what a test of
	// timing logic can assert on without a clock underneath it.
	Delay time.Duration
	// Event is the event as it was submitted, destination included.
	Event alsa.SeqEvent
}

// Queue stands in for *alsa.SeqQueue and satisfies alsa.Scheduler.
//
// It mirrors the guards a consumer's own error handling depends on, and nothing
// more. Scheduling into a queue that is not running is refused with alsa.ErrQueueStopped,
// a negative delay with alsa.ErrScheduleInPast, a released queue with ErrClosed, and a
// full one with alsa.ErrQueueFull once Ceiling is set, because a consumer holding a branch
// for any of those has to be able to reach it with no hardware. Messages, addresses and the
// real queue's ceiling are not mirrored, for the same reason WritePort does not validate
// them here: that is the real package's policy, it is covered by that package's tests, and
// repeating it would only prove that this file agrees with itself.
//
// Like Seq it has no lock, and for the same reason. It also copies the event data, and
// for the same reason: the real queue hands the bytes to the sequencer and keeps nothing,
// so a caller reusing one buffer must not be able to change the recording afterwards.
type Queue struct {
	// SeqAddr mirrors the address on the sequencer that owns the queue, so a consumer
	// that reads SeqAddr.Port to decide where to submit sees the same thing here.
	alsa.SeqAddr

	// StartErr, ScheduleErr, ResetErr and CloseErr are the failures a test asks for.
	// They are fields rather than constructor arguments on the same terms as Seq's: a
	// test usually sets one halfway through, after it has seen the working case.
	StartErr    error
	ScheduleErr error
	ResetErr    error
	CloseErr    error

	// Ceiling is how many submissions this queue accepts before refusing with
	// alsa.ErrQueueFull, and zero for no limit. It is opt-in rather than mirroring the
	// real queue's because the real number is a policy of that package and a guess about
	// one kernel, so a stand-in that invented it would be testing itself. A test that
	// cares about a full queue sets it to a small number and gets there in three lines.
	Ceiling int

	scheduled []Scheduled
	starts    int
	resets    int
	closes    int
	running   bool
	closed    bool
}

// NewQueue returns a stopped queue whose default port is port 0. It comes up stopped
// because the real one does: allocating a queue does not start it, and a stand-in that
// started itself would let a consumer's missing Start go untested.
func NewQueue() *Queue { return &Queue{SeqAddr: alsa.SeqAddr{Port: 0}} }

// Start makes the queue accept events. Repeating it changes nothing, as it does on the
// real queue.
func (q *Queue) Start() error {
	if q.closed {
		return ErrClosed
	}
	if q.StartErr != nil {
		return q.StartErr
	}
	if q.running {
		return nil
	}
	q.running = true
	q.starts++
	return nil
}

// ScheduleIn submits ev from the default port, as *alsa.SeqQueue does.
func (q *Queue) ScheduleIn(d time.Duration, ev alsa.SeqEvent) error {
	return q.schedule(d, ev, q.Port)
}

// SchedulePort submits ev from an explicitly chosen local port.
func (q *Queue) SchedulePort(d time.Duration, ev alsa.SeqEvent, port int) error {
	return q.schedule(d, ev, port)
}

// Reset stops the queue and discards what was on it, which is what the real one does
// when it frees a queue. The recording goes with the events rather than outliving them,
// so a test that wants to know what was pending reads Scheduled before it resets.
func (q *Queue) Reset() error {
	if q.closed {
		return ErrClosed
	}
	if q.ResetErr != nil {
		return q.ResetErr
	}
	q.resets++
	q.running = false
	q.scheduled = nil
	return nil
}

// Close releases the queue. It is safe to repeat, as *alsa.SeqQueue.Close is, and
// invalidates the default port so that a later submission fails loudly.
func (q *Queue) Close() error {
	q.closes++
	if q.closed {
		return nil
	}
	q.closed = true
	q.running = false
	q.Port = -1
	return q.CloseErr
}

// Running reports whether the queue is started, which is the question a consumer asks
// when it wants to know whether a refusal was a bug in its own sequencing.
func (q *Queue) Running() bool { return q.running }

// Scheduled returns what has been submitted and not yet discarded, oldest first. It
// copies, so a test cannot reach into the recording and change what a later assertion
// sees.
func (q *Queue) Scheduled() []Scheduled { return append([]Scheduled(nil), q.scheduled...) }

// Starts returns how many times Start actually began a queue, so a test can tell a
// repeat that was refused from one that was taken.
func (q *Queue) Starts() int { return q.starts }

// Resets returns how many times Reset ran.
func (q *Queue) Resets() int { return q.resets }

// Closes returns how many times Close was called.
func (q *Queue) Closes() int { return q.closes }

func (q *Queue) schedule(d time.Duration, ev alsa.SeqEvent, port int) error {
	if q.closed {
		return ErrClosed
	}
	if d < 0 {
		return alsa.ErrScheduleInPast
	}
	if !q.running {
		return alsa.ErrQueueStopped
	}
	if q.ScheduleErr != nil {
		return q.ScheduleErr
	}
	// The ceiling is checked before the empty event, in the same order as the real queue:
	// a stand-in that refused in a different order would let a consumer's ordering
	// assumptions go untested, which is the one thing it is here to prevent.
	if q.Ceiling > 0 && len(q.scheduled) >= q.Ceiling {
		return fmt.Errorf("%w: %d of %d submissions", alsa.ErrQueueFull, len(q.scheduled), q.Ceiling)
	}
	// An event with no bytes is not recorded, because the real queue does not send one:
	// a stand-in that invented a submission would let a consumer's conditional send go
	// untested.
	if len(ev.Data) == 0 {
		return nil
	}
	q.scheduled = append(q.scheduled, Scheduled{
		Port:  port,
		Delay: d,
		Event: alsa.SeqEvent{SeqAddr: ev.SeqAddr, Data: bytes.Clone(ev.Data)},
	})
	return nil
}

// The stand-in is only worth having where the real sequencer would go, which is the
// whole reason for the narrow interfaces in alsa.
var (
	_ alsa.EventReader = (*Seq)(nil)
	_ alsa.EventWriter = (*Seq)(nil)
	_ alsa.PortWriter  = (*Seq)(nil)
	_ alsa.Closer      = (*Seq)(nil)
	_ alsa.Scheduler   = (*Queue)(nil)
)
