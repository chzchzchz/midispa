package jack

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// A stereo sink as this machine publishes one: two mono ports whose names carry
	// the channel, since the driver has no stereo port type to register.
	sinkLeft  = "system:playback:front-left"
	sinkRight = "system:playback:front-right"

	// leftVal and rightVal are what the callback writes. They differ so a swapped
	// channel or a shared buffer shows up rather than looking plausible.
	leftVal  float32 = 0.25
	rightVal float32 = 0.75
)

// newTestStereoPort opens a stereo write port against the fake client.
func newTestStereoPort(t *testing.T, c *fakeClient, pc PortConfig) *Port {
	t.Helper()
	return openTestPort(t, NewStereoWritePort, c, pc)
}

// fillStereo is the callback for a test that is about routing or registration rather
// than about what a caller does with the samples.
func fillStereo(left, right []float32) int {
	for i := range left {
		left[i] = leftVal
	}
	for i := range right {
		right[i] = rightVal
	}
	return 0
}

// wantSamples asserts that the port's whole cycle buffer holds v.
func wantSamples(t *testing.T, p *fakePort, v float32) {
	t.Helper()
	want := make([]float32, testFrames)
	for i := range want {
		want[i] = v
	}
	assert.Equalf(t, want, p.samples(), "%s samples", p.Name())
}

// ownPorts is the qualified names of the two ports a stereo port registers.
var ownPorts = []string{
	testClientName + ":" + testPortName + leftSuffix,
	testClientName + ":" + testPortName + rightSuffix,
}

func TestStereoPortFillsBothChannelsInOneCallback(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(sinkLeft), newFakePort(sinkRight)}
	var (
		calls      int
		gotL, gotR []float32
	)
	newTestStereoPort(t, c, PortConfig{
		MatchName: []string{"front-left", "front-right"},
		StereoCallback: func(left, right []float32) int {
			calls++
			gotL, gotR = left, right
			return fillStereo(left, right)
		},
	})
	waitWired(t, c, 2)

	c.cycle(t, testFrames)

	// Two clients would each get their own process callback, so the count is what
	// separates the one client this package needs from the two it must not open.
	require.Equal(t, 1, calls, "callbacks run in one cycle")
	assert.Len(t, gotL, testFrames, "left samples handed to the callback")
	assert.Len(t, gotR, testFrames, "right samples handed to the callback")
	wantSamples(t, c.own(0), leftVal)
	wantSamples(t, c.own(1), rightVal)
}

func TestStereoPortRegistersTwoPortsOnOneClient(t *testing.T) {
	c := newFakeClient(testClientName)
	newTestStereoPort(t, c, PortConfig{MatchName: []string{playbackMatch}, StereoCallback: fillStereo})

	assert.Equal(t, 1, c.openCount(), "clients opened")
	made := c.made()
	require.Len(t, made, 2, "ports registered")
	for i, r := range made {
		assert.Equalf(t, audioPortType, r.portType, "port %d type", i)
		// The terminal flag cannot be recovered from a sibling call, and a port
		// without it connects differently.
		assert.Equalf(t, portIsOutput|portIsTerminal, r.flags, "port %d flags", i)
		assert.Equalf(t, portBufferHint, r.bufferSize, "port %d buffer hint", i)
	}
}

func TestStereoPortNamesBothChannelsFromOnePortName(t *testing.T) {
	c := newFakeClient(testClientName)
	newTestStereoPort(t, c, PortConfig{MatchName: []string{playbackMatch}, StereoCallback: fillStereo})

	registered := []string{testPortName + leftSuffix, testPortName + rightSuffix}
	made := c.made()
	require.Len(t, made, len(registered), "ports registered")
	for i, name := range registered {
		assert.Equalf(t, name, made[i].name, "name registered")
		// The client prefix is what keeps a match on the bare port name from making
		// the package wire itself to itself.
		assert.Equalf(t, testClientName+":"+name, ownPorts[i], "qualified name of port %d", i)
	}
}

func TestPortsAreRegisteredBeforeActivate(t *testing.T) {
	c := newFakeClient(testClientName)
	newTestStereoPort(t, c, PortConfig{MatchName: []string{playbackMatch}, StereoCallback: fillStereo})

	// The process callback reads the ports slice without a lock, so registering after
	// the client is live leaves it reading a slice that is still being filled.
	for i, r := range c.made() {
		assert.Falsef(t, r.active, "port %d was registered after the client was activated", i)
	}
}

func TestPortRegisterFailureIsAnError(t *testing.T) {
	c := newFakeClient(testClientName)
	c.registerFailAt = 2

	_, err := NewStereoWritePort(testConfig(c, PortConfig{
		MatchName:      []string{playbackMatch},
		StereoCallback: fillStereo,
	}))
	require.Error(t, err, "a refused second port was accepted")
	assert.Contains(t, err.Error(), testPortName+rightSuffix, "error does not say which port was refused")
	requireNoClientLeft(t, c)
}

// TestPortWithNoCallbackIsRejected covers a port with nothing to fill it, which used to
// be a MIDI port rather than an error.
func TestPortWithNoCallbackIsRejected(t *testing.T) {
	requireRejected(t, NewWritePort, PortConfig{})
}

// TestStereoPortWithoutACallbackIsRejected covers two ports and nothing to fill them
// with, which has no reading other than broken.
func TestStereoPortWithoutACallbackIsRejected(t *testing.T) {
	requireRejected(t, NewStereoWritePort, PortConfig{})
}

// TestAmbiguousCallbacksAreRejected covers every pair. One of the two would be
// silently ignored, and a port registered but never filled connects, counts as
// connected and outputs silence, so the mistake would be heard rather than reported.
func TestAmbiguousCallbacksAreRejected(t *testing.T) {
	tests := []struct {
		name string
		pc   PortConfig
	}{
		{"audio and stereo", PortConfig{AudioCallback: silentAudio, StereoCallback: fillStereo}},
		{"audio and midi", PortConfig{AudioCallback: silentAudio, MidiCallback: func(io.Writer) {}}},
		{"stereo and midi", PortConfig{StereoCallback: fillStereo, MidiCallback: func(io.Writer) {}}},
	}
	open := func(pc PortConfig) (*Port, error) { return NewJackPort(pc, portIsOutput|portIsTerminal) }
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { requireRejected(t, open, tc.pc) })
	}
}

func TestStereoPortRoutesMatchNamesByPosition(t *testing.T) {
	// The pairings are what matter, not the order they were made in: the startup sweep
	// makes them in one pass while a late port is wired from the connect goroutine.
	want := []connection{{ownPorts[0], sinkLeft}, {ownPorts[1], sinkRight}}
	match := []string{"front-left", "front-right"}

	t.Run("already there when the client opened", func(t *testing.T) {
		c := newFakeClient(testClientName)
		c.existing = []*fakePort{newFakePort(sinkLeft), newFakePort(sinkRight)}
		newTestStereoPort(t, c, PortConfig{MatchName: match, StereoCallback: fillStereo})

		assert.ElementsMatch(t, want, c.wired())
	})

	t.Run("appearing later", func(t *testing.T) {
		c := newFakeClient(testClientName)
		p := newTestStereoPort(t, c, PortConfig{MatchName: match, StereoCallback: fillStereo})

		c.add(sinkRight)
		c.add(sinkLeft)

		waitWired(t, c, len(want))
		assert.ElementsMatch(t, want, c.wired(), "a late port was not routed by position")
		assert.Len(t, tracked(p), len(want), "external ports held")
	})
}

func TestStereoPortWithOneMatchNameFeedsBothPorts(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(playbackPort)}
	newTestStereoPort(t, c, PortConfig{MatchName: []string{playbackMatch}, StereoCallback: fillStereo})

	// JACK sums whatever lands on one sink, so reaching it from both of our ports is
	// what makes the mono default a down mix rather than one channel of it.
	assert.ElementsMatch(t, []connection{{ownPorts[0], playbackPort}, {ownPorts[1], playbackPort}}, c.wired())
}

func TestMatchIndexPrefersTheFirstEntry(t *testing.T) {
	pc := PortConfig{MatchName: []string{"front", "front-left"}}
	tests := []struct {
		name string
		want int
	}{
		{"system:playback:front-left", 0},
		{"system:playback:front-right", 0},
		{"system:capture_1", -1},
	}
	for _, tc := range tests {
		assert.Equalf(t, tc.want, pc.matchIndex(tc.name), "entry %q matched", tc.name)
	}
}

func TestOverlappingMatchNamesConnectOnce(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(sinkLeft), newFakePort(sinkRight)}
	p := newTestPort(t, c, PortConfig{
		MatchName:     []string{"front", "front-left", "front-right"},
		AudioCallback: silentAudio,
	})

	// A name matching two entries used to be wired once per entry, and the bookkeeping
	// keyed by name then collapsed those into one, leaving the count that gates the
	// callback short of what was actually wired.
	assert.Len(t, c.wired(), 2, "connections made")
	assert.Len(t, tracked(p), 2, "external ports held")
}

func TestMatchNameDoesNotSwallowALongerNumber(t *testing.T) {
	// The device this runs against publishes playback_1 through playback_10, so a
	// containment match on playback_1 used to reach playback_10 as well.
	pc := PortConfig{MatchName: []string{"system:playback_1"}}
	tests := []struct {
		name string
		want int
	}{
		{"system:playback_1", 0},
		{"system:playback_10", -1},
		{"system:playback_11", -1},
		{"system:playback_2", -1},
	}
	for _, tc := range tests {
		assert.Equalf(t, tc.want, pc.matchIndex(tc.name), "entry matched by %q", tc.name)
	}
}

func TestMatchNameStillReachesAWholeFamilyOfChannels(t *testing.T) {
	// The one-name default depends on this: naming the device rather than a channel
	// has to keep reaching every channel it publishes, numbers included.
	pc := PortConfig{MatchName: []string{"system:playback"}}
	for _, name := range []string{"system:playback", "system:playback_1", "system:playback_10", "system:playback_2222"} {
		assert.Equalf(t, 0, pc.matchIndex(name), "%q is a channel of the device", name)
	}
}

func TestEmptyMatchNameMatchesNothing(t *testing.T) {
	// An empty entry used to contain every port the server published, which on a
	// stereo port wired both channels to whatever happened to be there.
	pc := PortConfig{MatchName: []string{""}}
	assert.Equal(t, -1, pc.matchIndex("system:playback_1"))
}

func TestStereoPortWithNumberedMatchNamesIgnoresLongerNumbers(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{
		newFakePort("system:playback_1"),
		newFakePort("system:playback_2"),
		newFakePort("system:playback_10"),
		newFakePort("system:playback_11"),
	}
	p := newTestStereoPort(t, c, PortConfig{
		MatchName:      []string{"system:playback_1", "system:playback_2"},
		StereoCallback: fillStereo,
	})

	// Two channels asked for, two channels wired. The longer numbers must not turn up
	// as extra sinks on the left, which is what makes a stereo image lopsided rather
	// than merely duplicated.
	assert.ElementsMatch(t, []connection{
		{ownPorts[0], "system:playback_1"},
		{ownPorts[1], "system:playback_2"},
	}, c.wired())
	assert.Len(t, tracked(p), 2, "external ports held")
}

// TestCallbackMustFitThePortCount covers the mismatch that would otherwise reach the
// realtime callback: processAudio picks what fills it from the port count, so a
// constructor and a callback that disagree leave it calling a function nobody set, on
// JACK's own thread, where a nil call takes the whole process down.
func TestCallbackMustFitThePortCount(t *testing.T) {
	tests := []struct {
		name string
		open func(PortConfig) (*Port, error)
		pc   PortConfig
	}{
		{
			name: "stereo port with a mono callback",
			open: NewStereoWritePort,
			pc:   PortConfig{AudioCallback: silentAudio},
		},
		{
			name: "mono port with a stereo callback",
			open: NewWritePort,
			pc:   PortConfig{StereoCallback: fillStereo},
		},
		{
			name: "midi port with a stereo callback",
			open: NewWritePort,
			pc:   PortConfig{StereoCallback: fillStereo, MidiCallback: func(io.Writer) {}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { requireRejected(t, tc.open, tc.pc) })
	}
}

// TestMoreMatchNamesThanChannelsIsRejected covers the one way a match entry can run out
// of ports. A stereo port wires entry i to channel i, so a third entry names a channel
// that was never registered, and the startup sweep reached it and indexed past the end
// of the ports slice, taking the process down before the caller read anything.
func TestMoreMatchNamesThanChannelsIsRejected(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort("front-left"), newFakePort("front-right"), newFakePort("front-centre")}
	p, err := NewStereoWritePort(PortConfig{
		MatchName:      []string{"front-left", "front-right", "front-centre"},
		StereoCallback: fillStereo,
		OpenClient:     func(string) (JackClient, error) { return c, nil },
	})
	require.Error(t, err)
	assert.Nil(t, p)
	assert.False(t, c.wasOpened(), "a configuration this package refuses reached the server")
}

// TestMonoPortStillTakesSeveralMatchNames is the other side of that refusal: a second
// name on a mono port is not a channel with nowhere to go, it is another sink for the
// one port it has, which is how this package has always routed.
func TestMonoPortStillTakesSeveralMatchNames(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(playbackPort), newFakePort("front-right")}
	p := newTestPort(t, c, PortConfig{
		MatchName:     []string{playbackMatch, "front-right"},
		AudioCallback: silentAudio,
	})
	waitWired(t, c, 2)
	assert.Len(t, tracked(p), 2, "both sinks tracked against the one port")
}

// TestStereoPortWithOneMatchNameWiresALatePortToBothChannels covers the async half of
// the one-name rule. The startup sweep is exercised by the test above; this is the
// path a device takes when it is plugged in after the client is already open.
func TestStereoPortWithOneMatchNameWiresALatePortToBothChannels(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestStereoPort(t, c, PortConfig{MatchName: []string{playbackMatch}, StereoCallback: fillStereo})

	c.add(playbackPort)

	// One registration produces one queued match, and the goroutine expands that into
	// a connection per channel. Were it queued once per channel instead, the second
	// would find the port already tracked and be dropped, leaving one connection.
	waitWired(t, c, 2)
	assert.ElementsMatch(t, []connection{{ownPorts[0], playbackPort}, {ownPorts[1], playbackPort}}, c.wired())
	assert.Len(t, tracked(p), 1, "external ports held")
}

func TestGetBufferOnAStereoPortIsTheLeftChannel(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(sinkRight)}
	p := newTestStereoPort(t, c, PortConfig{
		MatchName:      []string{"front-left", "front-right"},
		StereoCallback: fillStereo,
	})
	waitWired(t, c, 1)

	// A method returning one slice has only one reading on a two port object, and the
	// doc says which. Writing through it has to land in the left port: a copy would
	// leave the callback filling a buffer the caller cannot see.
	p.GetBuffer(testFrames)[0] = leftVal
	assert.Equal(t, leftVal, c.own(0).samples()[0], "GetBuffer writes into the left port")
}

// TestStereoPortStaysQuietWhenOnlyOneChannelIsWired covers the half-connect. One
// channel of a stereo port is not half a note, it is a note heard hard against a
// single speaker, so a port missing either end waits rather than running on the half
// that happens to be wired.
func TestStereoPortStaysQuietWhenOnlyOneChannelIsWired(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(sinkRight)}
	var calls int
	p := newTestStereoPort(t, c, PortConfig{
		MatchName: []string{"front-left", "front-right"},
		StereoCallback: func(left, right []float32) int {
			calls++
			return fillStereo(left, right)
		},
	})
	waitWired(t, c, 1)

	c.cycle(t, testFrames)

	assert.Zero(t, calls, "callback ran with one channel unwired")
	assert.False(t, p.isReady(), "a half-wired stereo port is not ready")
}

// TestStereoPortResumesWhenTheMissingChannelArrives is the other half of that rule: the
// port starts itself as soon as the second channel has somewhere to go, rather than
// needing to be reopened.
func TestStereoPortResumesWhenTheMissingChannelArrives(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(sinkRight)}
	var calls int
	p := newTestStereoPort(t, c, PortConfig{
		MatchName: []string{"front-left", "front-right"},
		StereoCallback: func(left, right []float32) int {
			calls++
			return fillStereo(left, right)
		},
	})
	waitWired(t, c, 1)
	c.cycle(t, testFrames)
	require.Zero(t, calls, "callback ran with one channel unwired")

	c.add(sinkLeft)
	waitWired(t, c, 2)

	c.cycle(t, testFrames)

	assert.Equal(t, 1, calls, "callback once both channels are wired")
	wantSamples(t, c.own(0), leftVal)
	wantSamples(t, c.own(1), rightVal)
	assert.True(t, p.isReady(), "a fully wired stereo port is ready")
}

// TestUnregisteringOneChannelStopsTheStereoCallback covers a channel going away while
// the other stays. The count that gates the callback has to fall with it, or the port
// goes on believing it can still play both ends.
func TestUnregisteringOneChannelStopsTheStereoCallback(t *testing.T) {
	c := newFakeClient(testClientName)
	c.add(sinkLeft)
	right := c.add(sinkRight)
	var calls int
	p := newTestStereoPort(t, c, PortConfig{
		MatchName: []string{"front-left", "front-right"},
		StereoCallback: func(left, right []float32) int {
			calls++
			return 0
		},
	})
	waitWired(t, c, 2)

	c.cycle(t, testFrames)
	require.Equal(t, 1, calls, "callback runs while both channels are wired")

	c.remove(right)
	assert.False(t, p.isReady(), "a stereo port with one channel left is not ready")

	c.cycle(t, testFrames)
	assert.Equal(t, 1, calls, "callback ran again after one channel was unplugged")
}

// TestMonoPortKeepsPlayingOnOneChannel is the other side of the rule. A mono port has
// one output, so anything wired to it is everything it needs.
func TestMonoPortKeepsPlayingOnOneChannel(t *testing.T) {
	c := newFakeClient(testClientName)
	c.existing = []*fakePort{newFakePort(playbackPort)}
	var calls int
	p := newTestPort(t, c, PortConfig{
		MatchName:     []string{playbackMatch},
		AudioCallback: func([]float32) int { calls++; return 0 },
	})
	waitWired(t, c, 1)

	c.cycle(t, testFrames)

	assert.Equal(t, 1, calls, "callback ran")
	assert.True(t, p.isReady())
}
