package alsa

// The sequencer is used through four narrow views rather than through *Seq, so that the
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

// The seams are only correct if the real sequencer fits them. A signature that does not
// match here means the interface is wrong, not the caller, so these assertions catch a
// divergence in *Seq at build time rather than leaving it to a test to find.
var (
	_ EventReader = (*Seq)(nil)
	_ EventWriter = (*Seq)(nil)
	_ PortWriter  = (*Seq)(nil)
	_ Closer      = (*Seq)(nil)
)
