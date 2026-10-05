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

// The stand-in is only worth having where the real sequencer would go, which is the
// whole reason for the narrow interfaces in alsa.
var (
	_ alsa.EventReader = (*Seq)(nil)
	_ alsa.EventWriter = (*Seq)(nil)
	_ alsa.PortWriter  = (*Seq)(nil)
	_ alsa.Closer      = (*Seq)(nil)
)
