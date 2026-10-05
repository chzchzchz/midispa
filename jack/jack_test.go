package jack

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClientName = "testclient"
	testPortName   = "out"
	testFrames     = 64

	// playbackMatch and the ports under it stand in for a device publishing its
	// channels, which is what most of these tests are about.
	playbackMatch = "system:playback"
	playbackPort  = "system:playback_1"
)

// The failures a fake can be asked to produce. They are shared so a test can assert on
// the one it provoked without inventing a second error to compare against.
var (
	errNoServer = errors.New("no server")
	errActivate = errors.New("activate failed")
	errConnect  = errors.New("connect failed")
	errWrite    = errors.New("write failed")
)

// fakePort stands in for a JACK port. It records what was written to it so a test can
// assert on what the routing handed over.
type fakePort struct {
	name string

	mu       sync.Mutex
	audio    []float32
	events   []MidiEvent
	cleared  uint32
	writeErr error
}

func newFakePort(name string) *fakePort {
	return &fakePort{name: name}
}

func (p *fakePort) Name() string { return p.name }

func (p *fakePort) AudioBuffer(nframes uint32) []float32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if uint32(len(p.audio)) < nframes {
		p.audio = make([]float32, nframes)
	}
	return p.audio[:nframes]
}

func (p *fakePort) MidiClearBuffer(nframes uint32) MidiBuffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleared = nframes
	return &fakeBuffer{p: p}
}

func (p *fakePort) MidiWrite(ev MidiEvent, buf MidiBuffer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeErr != nil {
		return p.writeErr
	}
	p.events = append(p.events, ev)
	return nil
}

func (p *fakePort) written() []MidiEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]MidiEvent{}, p.events...)
}

// samples is what the last cycle left in this port's audio buffer.
func (p *fakePort) samples() []float32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]float32{}, p.audio...)
}

// fakeBuffer is the cycle buffer a fake port hands back. Nothing in this package reads
// its bytes, so it only has to exist.
type fakeBuffer struct {
	p *fakePort
}

func (b *fakeBuffer) Bytes() []byte { return nil }

// connection records one ConnectPorts call.
type connection struct {
	src, dst string
}

// registration records one PortRegister call, so a test can assert on what the package
// asked the server for rather than only on what came back.
type registration struct {
	name, portType string
	flags          uint64
	bufferSize     uint64
	// active records whether the client had been activated when the call was made.
	// The realtime callbacks read the ports a registration produced, so registering
	// after activation leaves that slice being filled while it is being read.
	active bool
}

// fakeClient stands in for a JACK server: it keeps the callbacks it was given and
// reports port registrations on demand.
type fakeClient struct {
	mu sync.Mutex

	// clientName is the prefix the server puts on every port the package registers.
	// The package relies on it to keep its own ports out of its own matches, so a fake
	// that left it off would quietly stop testing that.
	clientName string

	activated bool
	closed    bool
	opened    bool
	// opens counts the clients handed out, so a test can tell one client carrying two
	// ports from two clients carrying one each.
	opens int

	sampleRate, bufferSize uint32

	registration PortRegistrationCallback
	process      ProcessCallback

	// internal is the first port the package registered, which is what the MIDI tests
	// read what was written to. registered is every one of them, since a stereo port
	// registers two and has to be told apart from a mono one.
	internal      *fakePort
	registered    []*fakePort
	registrations []registration
	existing      []*fakePort

	// registerFailAt makes the nth PortRegister call hand back nothing, the way the
	// server refuses a port it will not have. Zero means every call succeeds.
	registerFailAt int

	// portWriteErr is the midi write failure every port this client registers starts
	// with, so a test can provoke one on a port the package creates itself.
	portWriteErr error

	connections []connection
	connectErr  error
	activateErr error

	// openErr makes opening a client fail, standing in for no server running.
	openErr error
}

func newFakeClient(name string) *fakeClient {
	return &fakeClient{
		clientName: name,
		sampleRate: 48000,
		bufferSize: 1024,
	}
}

func (c *fakeClient) SetPortRegistrationCallback(cb PortRegistrationCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registration = cb
	return nil
}

func (c *fakeClient) SetPortConnectCallback(cb PortConnectCallback) error { return nil }

func (c *fakeClient) SetProcessCallback(cb ProcessCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.process = cb
	return nil
}

func (c *fakeClient) Activate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activateErr != nil {
		return c.activateErr
	}
	c.activated = true
	return nil
}

func (c *fakeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeClient) PortRegister(name, portType string, flags, bufferSize uint64) JackPort {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registrations = append(c.registrations, registration{name, portType, flags, bufferSize, c.activated})
	if c.registerFailAt == len(c.registrations) {
		return nil
	}
	p := newFakePort(c.clientName + ":" + name)
	p.writeErr = c.portWriteErr
	c.registered = append(c.registered, p)
	if c.internal == nil {
		c.internal = p
	}
	return p
}

func (c *fakeClient) PortByID(id PortID) JackPort {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.all() {
		if PortID(id) == portID(p) {
			return p
		}
	}
	return nil
}

func (c *fakeClient) PortByName(name string) JackPort {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.all() {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func (c *fakeClient) PortNames(name, portType string, flags uint64) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var ret []string
	for _, p := range c.all() {
		if name == "" || strings.Contains(p.Name(), name) {
			ret = append(ret, p.Name())
		}
	}
	return ret
}

func (c *fakeClient) ConnectPorts(src, dst JackPort) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connectErr != nil {
		return c.connectErr
	}
	c.connections = append(c.connections, connection{src.Name(), dst.Name()})
	return nil
}

func (c *fakeClient) SampleRate() uint32 { return c.sampleRate }
func (c *fakeClient) BufferSize() uint32 { return c.bufferSize }

// all lists every port the fake knows about: the ones the package registered first,
// then the ones other clients published. The package filters its own out by name.
func (c *fakeClient) all() []*fakePort {
	return append(append([]*fakePort{}, c.registered...), c.existing...)
}

// made is a snapshot of the ports the package has registered.
func (c *fakeClient) made() []registration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]registration{}, c.registrations...)
}

// own is the nth port the package registered, which is how a test sees what its
// callback was handed rather than only what the callback did with it.
func (c *fakeClient) own(n int) *fakePort {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.registered[n]
}

// wired is the list of connections made so far.
func (c *fakeClient) wired() []connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]connection{}, c.connections...)
}

// add makes a port appear, reporting it to the registration callback the way JACK
// would when another client publishes it. The name is already qualified: the prefix is
// the publishing client's, not this one.
func (c *fakeClient) add(name string) *fakePort {
	p := newFakePort(name)
	c.mu.Lock()
	c.existing = append(c.existing, p)
	cb := c.registration
	c.mu.Unlock()
	if cb != nil {
		cb(portID(p), true)
	}
	return p
}

// remove makes a port disappear, reporting it to the registration callback.
func (c *fakeClient) remove(p *fakePort) {
	c.mu.Lock()
	kept := c.existing[:0]
	for _, e := range c.existing {
		if e != p {
			kept = append(kept, e)
		}
	}
	c.existing = kept
	cb := c.registration
	c.mu.Unlock()
	if cb != nil {
		cb(portID(p), false)
	}
}

// cycle runs one process callback with the given frame count. It takes testing.TB so a
// benchmark can drive the same path a test does.
func (c *fakeClient) cycle(t testing.TB, nframes uint32) {
	t.Helper()
	c.mu.Lock()
	cb := c.process
	c.mu.Unlock()
	if cb == nil {
		require.FailNow(t, "no process callback registered")
	}
	cb(nframes)
}

// wasOpened reports whether a client was ever handed out, which decides whether there
// was anything that could have been closed.
func (c *fakeClient) wasOpened() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opened
}

func (c *fakeClient) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// portID numbers fake ports so the registration callback can name one.
func portID(p *fakePort) PortID {
	return PortID(p.Name()[0])<<16 | PortID(len(p.Name()))
}

// opener returns an OpenClient that hands out the fake.
func (c *fakeClient) opener(string) (JackClient, error) {
	if c.openErr != nil {
		return nil, c.openErr
	}
	c.mu.Lock()
	c.opened = true
	c.opens++
	c.mu.Unlock()
	return c, nil
}

// openCount is how many clients this fake has handed out.
func (c *fakeClient) openCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opens
}

// tracked is a snapshot of the external ports the package is holding on to. The
// package reaches them from its own goroutine, so a test has to read them the same
// guarded way rather than looking at the map itself.
func tracked(p *Port) map[string]wiring {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.portExternal)
}

// waitFor blocks until cond holds, so a test can follow a connection the package makes
// on its own goroutine.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	require.FailNow(t, "timed out waiting for "+what)
}

// waitWired blocks until the client has made n connections. The package connects from
// its own goroutine, so a test that adds a port has to follow it rather than assert
// straight away.
func waitWired(t *testing.T, c *fakeClient, n int) {
	t.Helper()
	waitFor(t, fmt.Sprintf("%d ports to be connected", n), func() bool { return len(c.wired()) == n })
}

// waitTracked blocks until the package is holding n external ports.
func waitTracked(t *testing.T, p *Port, n int) {
	t.Helper()
	waitFor(t, fmt.Sprintf("%d ports to be tracked", n), func() bool { return len(tracked(p)) == n })
}

func testConfig(c *fakeClient, pc PortConfig) PortConfig {
	pc.ClientName = testClientName
	pc.PortName = testPortName
	pc.OpenClient = c.opener
	return pc
}

// newTestPort opens a mono write port against the fake client.
func newTestPort(t *testing.T, c *fakeClient, pc PortConfig) *Port {
	t.Helper()
	return openTestPort(t, NewWritePort, c, pc)
}

// openTestPort opens a port with the given constructor and closes it when the test
// ends. The constructor is a parameter because the mono, midi and stereo ports are the
// same shape and differ only in which one is called.
func openTestPort(t *testing.T, open func(PortConfig) (*Port, error), c *fakeClient, pc PortConfig) *Port {
	t.Helper()
	p, err := open(testConfig(c, pc))
	require.NoError(t, err, "opening port")
	t.Cleanup(p.Close)
	return p
}

// requireRejected opens a port the configuration says this package should refuse, and
// checks both halves of that: the error comes back, and the configuration never reached
// the server. The second half is the one a caller cannot see, and a refusal that had
// already opened a client would leave one running with nothing to close it.
func requireRejected(t *testing.T, open func(PortConfig) (*Port, error), pc PortConfig) {
	t.Helper()
	c := newFakeClient(testClientName)
	pc.MatchName = []string{playbackMatch}
	_, err := open(testConfig(c, pc))
	require.Error(t, err, "a configuration this package refuses was accepted")
	assert.False(t, c.wasOpened(), "a configuration this package refuses reached the server")
}

// requireNoClientLeft asserts the other half of a failed open. A caller that gets an
// error has no port to close, so a client that was handed out must already be shut. One
// that never opened has nothing to close and is not a failure.
func requireNoClientLeft(t *testing.T, c *fakeClient) {
	t.Helper()
	if c.wasOpened() {
		assert.True(t, c.isClosed(), "the client was left open after the failure")
	}
}

// silentAudio is the callback for a test that is about routing or lifetime rather than
// about what the callback does. A port with no callback at all is an error, so a test
// that does not care still has to ask for one.
func silentAudio([]float32) int { return 0 }

// TestConnectionDirection covers both port directions. Which end of the connection
// this package's own port sits on is the only thing that differs between them.
func TestConnectionDirection(t *testing.T) {
	internal := testClientName + ":" + testPortName
	tests := []struct {
		name string
		// open is the constructor, which is what decides the direction.
		open     func(PortConfig) (*Port, error)
		match    string
		existing string
		wantSrc  string
		wantDst  string
	}{
		{
			name: "write port feeds the sink",
			// A write port pushes its own output at what it matched, so its internal
			// port is the source of the connection.
			open:     NewWritePort,
			match:    playbackMatch,
			existing: playbackPort,
			wantSrc:  internal,
			wantDst:  playbackPort,
		},
		{
			name: "read port takes from the source",
			// A read port takes its audio from what it matched, so the direction is
			// the other way around.
			open:     NewReadPort,
			match:    "system:capture",
			existing: "system:capture_1",
			wantSrc:  "system:capture_1",
			wantDst:  internal,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClient(testClientName)
			c.existing = []*fakePort{newFakePort(tc.existing)}

			p, err := tc.open(testConfig(c, PortConfig{MatchName: []string{tc.match}, AudioCallback: silentAudio}))
			require.NoError(t, err, "opening port")
			defer p.Close()

			assert.Equal(t, []connection{{tc.wantSrc, tc.wantDst}}, c.wired())
			assert.Len(t, tracked(p), 1, "external ports held")
		})
	}
}

func TestSkipsPortsOfOurOwnClient(t *testing.T) {
	c := newFakeClient(testClientName)
	// The internal port is registered under the client name, so a match that would
	// otherwise catch it must not produce a connection.
	c.existing = []*fakePort{newFakePort(testClientName + ":out"), newFakePort("system:playback_1")}

	newTestPort(t, c, PortConfig{MatchName: []string{"out", "system:playback"}, AudioCallback: silentAudio})

	assert.Equal(t, []connection{{testClientName + ":" + testPortName, playbackPort}}, c.wired())
}

func TestConnectsPortsThatAppearLater(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	assert.Empty(t, c.wired(), "connections made before anything registered")

	c.add(playbackPort)

	// The package connects from its own goroutine, so the assertion has to follow it.
	waitWired(t, c, 1)
	waitTracked(t, p, 1)
	assert.Contains(t, tracked(p), playbackPort, "tracked external ports")
}

func TestIgnoresPortsThatDoNotMatch(t *testing.T) {
	c := newFakeClient(testClientName)
	newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	c.add("system:capture_1")
	c.add(playbackPort)

	waitWired(t, c, 1)
	assert.Equal(t, []connection{{testClientName + ":" + testPortName, playbackPort}}, c.wired(),
		"a port that does not match must not be connected to")
}

func TestUnregisteringStopsThePortBeingUsed(t *testing.T) {
	c := newFakeClient(testClientName)
	ext := c.add(playbackPort)
	var calls int
	p := newTestPort(t, c, PortConfig{
		MatchName:     []string{"system:playback"},
		AudioCallback: func([]float32) int { calls++; return 0 },
	})
	waitTracked(t, p, 1)

	c.cycle(t, testFrames)
	require.Equal(t, 1, calls, "callback runs while connected")

	c.remove(ext)

	require.Empty(t, tracked(p), "external ports held after removal")

	// JACK has dropped the other end, so the callback must stop rather than keep the
	// program working against a device that is no longer there.
	c.cycle(t, testFrames)
	assert.Equal(t, 1, calls, "callback runs after removal")
}

func TestAudioCallbackRunsOnceConnected(t *testing.T) {
	c := newFakeClient(testClientName)
	var (
		gotFrames int
		gotBuffer []float32
	)
	pc := PortConfig{
		MatchName: []string{playbackMatch},
		AudioCallback: func(s []float32) int {
			gotFrames, gotBuffer = len(s), s
			return 0
		},
	}
	newTestPort(t, c, pc)

	// Unconnected: the callback stays out of the way so a client does no work while
	// JACK is throwing its samples away.
	c.cycle(t, testFrames)
	require.Zero(t, gotFrames, "callback ran while nothing was connected")

	c.add(playbackPort)
	waitFor(t, "the new port to be connected", func() bool { return len(c.wired()) == 1 })

	c.cycle(t, testFrames)
	assert.Len(t, gotBuffer, testFrames, "samples handed to the callback")
}

func TestMidiCallbackWritesToPort(t *testing.T) {
	c := newFakeClient(testClientName)
	msgs := [][]byte{{0x90, 60, 127}, {0x80, 60, 0}}
	pc := PortConfig{
		MatchName:    []string{playbackMatch},
		MidiCallback: func(w io.Writer) { writeAll(w, msgs) },
	}
	newTestPort(t, c, pc)

	c.add(playbackPort)
	waitWired(t, c, 1)
	c.cycle(t, testFrames)

	port := c.internal
	written := port.written()
	require.Len(t, written, len(msgs), "events written")
	for i, msg := range msgs {
		assert.Equalf(t, msg, []byte(written[i].Msg), "event %d", i)
	}
	assert.Equal(t, uint32(testFrames), port.cleared, "frames the buffer was cleared for")
}

func TestMidiCallbackStopsOnWriteFailure(t *testing.T) {
	c := newFakeClient(testClientName)
	c.portWriteErr = errWrite

	var writes int
	pc := PortConfig{
		MatchName:    []string{playbackMatch},
		MidiCallback: func(w io.Writer) { writes += countWrites(w, 3) },
	}
	newTestPort(t, c, pc)

	c.add(playbackPort)
	waitWired(t, c, 1)
	c.cycle(t, testFrames)

	// The callback is handed an io.Writer, so the only way it learns a port is gone is
	// the error; that error has to reach it rather than being swallowed.
	assert.Zero(t, writes, "successful writes")
}

func TestGetBufferAndServerParameters(t *testing.T) {
	c := newFakeClient(testClientName)
	c.sampleRate, c.bufferSize = 44100, 512

	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	assert.Equal(t, uint32(44100), p.SampleRate())
	assert.Equal(t, uint32(512), p.BufferSize())
	assert.Len(t, p.GetBuffer(8), 8)
}

// TestOpenFailures covers every step that can fail while opening a port. Each has to
// hand the failure back and leave no client running, since a caller that gets an error
// has no port to close.
func TestOpenFailures(t *testing.T) {
	tests := []struct {
		name string
		// break_ arranges the failure on the fake.
		break_ func(*fakeClient) error
		// existing names a port already present, for the step that needs one to fail on.
		existing string
	}{
		{
			name:   "no server to connect to",
			break_: func(c *fakeClient) error { c.openErr = errNoServer; return c.openErr },
		},
		{
			name:   "client will not activate",
			break_: func(c *fakeClient) error { c.activateErr = errActivate; return c.activateErr },
		},
		{
			name:     "ports will not connect",
			break_:   func(c *fakeClient) error { c.connectErr = errConnect; return c.connectErr },
			existing: playbackPort,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClient(testClientName)
			if tc.existing != "" {
				c.existing = []*fakePort{newFakePort(tc.existing)}
			}
			wantErr := tc.break_(c)

			_, err := NewWritePort(testConfig(c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio}))
			require.ErrorIs(t, err, wantErr)
			requireNoClientLeft(t, c)
		})
	}
}

func TestCloseReleasesEverything(t *testing.T) {
	c := newFakeClient(testClientName)
	c.add(playbackPort)

	p, err := NewWritePort(testConfig(c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio}))
	require.NoError(t, err, "opening port")
	waitFor(t, "the new port to be connected", func() bool { return len(c.wired()) == 1 })

	p.Close()

	assert.True(t, c.isClosed(), "the client was closed")
	// Close returns only once the goroutine feeding connections has stopped, which is
	// what makes it safe to tear the rest of the program down behind it.
	waitFor(t, "the connection channel to drain", func() bool {
		select {
		case _, ok := <-p.portc:
			return !ok
		default:
			return false
		}
	})
}

func TestConnectsPortsAppearingAtTheSameTime(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	// A device commonly publishes several channels together. Refusing the ones that
	// arrive while another is still being wired would strand them for good, since
	// nothing asks for them again.
	c.add(playbackPort)
	c.add("system:playback_2222")
	c.add("system:playback_33333")

	waitWired(t, c, 3)
	for _, name := range []string{"system:playback_1", "system:playback_2222", "system:playback_33333"} {
		assert.Containsf(t, tracked(p), name, "%s was connected", name)
	}
}

// lateConnectClient counts any connection attempt that arrives after Close, which is
// what using a JACK handle the server has already torn down would look like.
type lateConnectClient struct {
	*fakeClient

	mu       sync.Mutex
	closed   bool
	lateCall int
}

func (c *lateConnectClient) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return c.fakeClient.Close()
}

func (c *lateConnectClient) ConnectPorts(src, dst JackPort) error {
	c.mu.Lock()
	if c.closed {
		c.lateCall++
	}
	c.mu.Unlock()
	return c.fakeClient.ConnectPorts(src, dst)
}

func TestNoConnectAfterClientClosed(t *testing.T) {
	// Repeated because the port registering and the port closing can interleave either
	// way round, and only one of those ordersings is a problem.
	for range 200 {
		lc := &lateConnectClient{fakeClient: newFakeClient(testClientName)}
		p, err := NewWritePort(PortConfig{
			ClientName:    testClientName,
			PortName:      testPortName,
			MatchName:     []string{"system:playback"},
			AudioCallback: silentAudio,
			OpenClient:    func(string) (JackClient, error) { return lc, nil },
		})
		require.NoError(t, err, "opening port")

		lc.add(playbackPort)
		p.Close()

		lc.mu.Lock()
		late := lc.lateCall
		lc.mu.Unlock()
		assert.Zero(t, late, "connections attempted after the client was closed")
	}
}

func TestCloseTwiceIsHarmless(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	p.Close()
	// A program that closes a port explicitly and again through a defer must not die
	// on the second close.
	p.Close()

	assert.Empty(t, c.connections)
}

func TestPortAppearingAfterCloseIsIgnored(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}, AudioCallback: silentAudio})

	p.Close()

	// A device can be plugged in while the client is shutting down. Queuing it then
	// would be a send on the closed channel, which takes the process down.
	c.add(playbackPort)

	assert.Empty(t, c.wired(), "connections after close")
	assert.Empty(t, tracked(p), "tracked ports after close")
}

// writeAll writes every message, stopping at the first refusal.
func writeAll(w io.Writer, msgs [][]byte) {
	for _, msg := range msgs {
		if _, err := w.Write(msg); err != nil {
			return
		}
	}
}

// countWrites writes up to n messages and reports how many the writer accepted.
func countWrites(w io.Writer, n int) (accepted int) {
	for i := 0; i < n; i++ {
		if _, err := w.Write([]byte{0x90, 60, 127}); err != nil {
			return accepted
		}
		accepted++
	}
	return accepted
}
