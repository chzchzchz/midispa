package alsa

import "time"

// The sequencer is used through five narrow views rather than through *Seq, so that the
// logic above it can be driven by a fake. ALSA is the only part of this package that
// needs hardware: everything else is address arithmetic, name resolution and event
// routing. Consumers that hold one of these instead of the concrete type can be tested
// with a stand-in sequencer and no /dev/snd/seq.
//
// Each interface has one job, and a consumer takes the one it uses rather than a
// combination. A single combined interface would force every stand-in to implement methods
// its caller never reaches; a caller that only writes events should not have to supply a
// reader, even if the real sequencer has one. Where something needs two directions at once,
// a field per direction says which is which, and sending an event out of the wrong one
// stops compiling.
//
// Scheduler is the one view that carries a lifecycle as well as a submission, and that is
// not a combination of jobs: none of its scheduling methods mean anything without a queue
// that is running, so splitting them would let a stand-in record something the real queue
// could never do.

// EventReader is the blocking read half of a sequencer. Read returns only once an event
// arrives, so a caller polling with MayRead first should reach for the concrete type:
// MayRead is deliberately not on this interface because it is not reachable through it.
type EventReader interface {
	Read() (SeqEvent, error)
}

// EventWriter is the write half addressed at the sequencer's default port.
type EventWriter interface {
	Write(SeqEvent) error
}

// PortWriter writes from an explicitly chosen local port, which is how a client with
// more than one of its own ports directs output.
type PortWriter interface {
	WritePort(SeqEvent, int) error
}

// Closer releases the underlying client and every port it owns. It is separate from the
// readers and writers because shutdown needs it regardless of which half was in use.
type Closer interface {
	Close() error
}

// Scheduler is a queue that delivers events at a moment the caller names rather than at
// once, which is what a gap between two notes of the same hit needs. Start, Reset and
// Close are part of it because a queue that cannot be started cannot deliver anything, so
// a consumer holding this view owns a running object rather than a function to call.
type Scheduler interface {
	Start() error
	ScheduleIn(time.Duration, SeqEvent) error
	SchedulePort(time.Duration, SeqEvent, int) error
	Reset() error
	Close() error
}

// The seams are only correct if the real types fit them. A signature that does not match
// here means the interface is wrong, not the caller, so these assertions catch a
// divergence at build time rather than leaving it to a test to find.
var (
	_ EventReader = (*Seq)(nil)
	_ EventWriter = (*Seq)(nil)
	_ PortWriter  = (*Seq)(nil)
	_ Closer      = (*Seq)(nil)
	_ Scheduler   = (*SeqQueue)(nil)
)
