package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/alsa/fake"
	"github.com/chzchzchz/midispa/midi"
)

// Two route destinations, so a test can tell a re-armed route from the one it replaced.
var (
	routeA = alsa.SeqAddr{Client: 16, Port: 0}
	routeB = alsa.SeqAddr{Client: 20, Port: 1}
)

// routeChannelMax is the highest MIDI channel, so a route naming one above it is one the
// decoder has to reject rather than act on.
const routeChannelMax = 15

var errReadFailed = errors.New("read failed")

// policy passes everything through except one status byte, standing in for the clock
// filter both real policies are built around. Those are build-tagged programs, so a test
// that used either would be testing the build tag rather than the routing.
//
// The zero value drops nothing, and the guard is what makes that true: a zero byte is a
// real status, so a pass-through policy that merely compared against zero would drop
// exactly the internal notices these tests check are not forwarded.
type policy struct {
	drop byte
}

func (p policy) handle(msg []byte) bool { return p.drop != 0 && msg[0] == p.drop }

// filterHarness is a filter wired to a stand-in sequencer, holding the two things a test
// needs from that pairing: feeding events in, and stopping it so the recording can be read.
type filterHarness struct {
	*FilterSeq
	seq     *fake.Seq
	stopped bool
}

// newFilterHarness takes the concrete policy type rather than the Policy interface: an
// omitted table field would then be the zero policy that drops nothing, where an omitted
// interface field would be nil and panic on the first message.
func newFilterHarness(t *testing.T, p policy) *filterHarness {
	t.Helper()
	harness := &filterHarness{seq: fake.New()}
	harness.FilterSeq = newFilterSeq(harness.seq, harness.seq, p)
	t.Cleanup(harness.stop)
	return harness
}

// feed queues the events and runs the filter's own loop once per event, which is what main
// does with what the sequencer hands it.
func (h *filterHarness) feed(t *testing.T, events ...alsa.SeqEvent) {
	t.Helper()
	h.seq.Queue(events...)
	for range events {
		require.NoError(t, h.handleEvent())
	}
}

// stop closes the filter's writers. Each one drains its channel and waits for its own
// goroutine, so once this returns nothing is still being written and the recording can be
// read with no lock at all, which is the same bargain the real sequencer makes. Closing
// twice would panic on the channel, so the cleanup can call it again safely.
func (h *filterHarness) stop() {
	if h.stopped {
		return
	}
	h.stopped = true
	h.FilterSeq.Close()
}

// armRoute is the SysEx a client sends to route a channel: F0 00 30 33 00 CH CC CC PP F7.
func armRoute(channel int, dst alsa.SeqAddr) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{
		midi.SysEx, 0x00, 0x30, 0x33, 0x00,
		byte(channel), byte(dst.Client >> 7), byte(dst.Client & 0x7f), byte(dst.Port),
		midi.EndSysEx,
	}}
}

func noteOn(channel, note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(channel), byte(note), 100}}
}

// routed is the same event on a destination.
func routed(ev alsa.SeqEvent, dst alsa.SeqAddr) alsa.SeqEvent {
	ev.SeqAddr = dst
	return ev
}

// broadcast is the same event on its way to subscribers, which is where an unrouted event
// goes.
func broadcast(ev alsa.SeqEvent) alsa.SeqEvent {
	return routed(ev, alsa.SubsSeqAddr)
}

// written returns what reached the sequencer, oldest first. It is nil rather than empty
// when nothing was written, so a case expecting no events can say nil and compare equal.
func written(seq *fake.Seq) []alsa.SeqEvent {
	var events []alsa.SeqEvent
	for _, write := range seq.Writes() {
		events = append(events, write.Event)
	}
	return events
}

// Routing is one decision over several inputs: which routes are armed, what the policy
// claims, and whether the event is MIDI at all. Each case below is a combination of those,
// and each asks the same two things -- how many routes the filter believes it holds, and
// where every event ended up.
//
// The cases add up to a rule that is easy to get half right: arming any route turns
// broadcast off, and the routed channel's neighbours are dropped rather than leaked to
// subscribers that never asked for them. That is why a route the decoder rejects has to
// leave routing off, since arming one wrongly would silently drop every later message.
func TestRouting(t *testing.T) {
	for _, test := range []struct {
		name    string
		policy  policy
		events  []alsa.SeqEvent
		want    []alsa.SeqEvent
		routing int
	}{
		{
			name:   "nothing armed broadcasts every channel",
			events: []alsa.SeqEvent{noteOn(0, 60), noteOn(5, 72)},
			want:   []alsa.SeqEvent{broadcast(noteOn(0, 60)), broadcast(noteOn(5, 72))},
		},
		{
			name:    "an armed route takes its channel and drops the rest",
			events:  []alsa.SeqEvent{armRoute(5, routeA), noteOn(5, 72), noteOn(0, 60)},
			want:    []alsa.SeqEvent{routed(noteOn(5, 72), routeA)},
			routing: 1,
		},
		{
			name:   "a rejected route leaves routing off",
			events: []alsa.SeqEvent{armRoute(routeChannelMax+1, routeA), noteOn(0, 60)},
			want:   []alsa.SeqEvent{broadcast(noteOn(0, 60))},
		},
		{
			name:    "re-arming a channel replaces its route",
			events:  []alsa.SeqEvent{armRoute(5, routeA), armRoute(5, routeB), noteOn(5, 72)},
			want:    []alsa.SeqEvent{routed(noteOn(5, 72), routeB)},
			routing: 1,
		},
		{
			name:   "the policy drops before routing is consulted",
			policy: policy{drop: midi.Clock},
			events: []alsa.SeqEvent{{Data: []byte{midi.Clock}}, noteOn(0, 60)},
			want:   []alsa.SeqEvent{broadcast(noteOn(0, 60))},
		},
		{
			name:   "a subscription notice is not forwarded",
			events: []alsa.SeqEvent{{Data: []byte{alsa.EvPortSubscribed, 16, 0, 14, 0}}},
			want:   nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := newFilterHarness(t, test.policy)
			defer harness.stop()
			harness.feed(t, test.events...)
			harness.stop()

			assert.Equal(t, test.routing, harness.routing, "routes the filter holds")
			assert.Equal(t, test.want, written(harness.seq), "events that reached the sequencer")
		})
	}
}

// A read that fails has to reach main's loop, or a sequencer that went away would leave the
// filter spinning on nothing. An empty queue is the same case against a stand-in, where
// the real sequencer would have blocked instead.
func TestAFailedReadStopsTheLoop(t *testing.T) {
	for _, test := range []struct {
		name    string
		arrange func(*fake.Seq)
		want    error
	}{
		{
			name:    "empty queue",
			arrange: func(*fake.Seq) {},
			want:    fake.ErrNoEvents,
		},
		{
			name:    "read failure",
			arrange: func(seq *fake.Seq) { seq.ReadErr = errReadFailed },
			want:    errReadFailed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			harness := newFilterHarness(t, policy{})
			defer harness.stop()
			test.arrange(harness.seq)

			require.ErrorIs(t, harness.handleEvent(), test.want)
		})
	}
}
