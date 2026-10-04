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

// fakeClient stands in for a JACK server: it keeps the callbacks it was given and
// reports port registrations on demand.
type fakeClient struct {
	mu sync.Mutex

	activated bool
	closed    bool
	opened    bool

	sampleRate, bufferSize uint32

	registration PortRegistrationCallback
	process      ProcessCallback

	internal *fakePort
	existing []*fakePort

	connections []connection
	connectErr  error
	activateErr error

	// openErr makes opening a client fail, standing in for no server running.
	openErr error
}

func newFakeClient(name string) *fakeClient {
	return &fakeClient{
		sampleRate: 48000,
		bufferSize: 1024,
		internal:   newFakePort(name + ":" + testPortName),
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
	return c.internal
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

// all lists every port the fake knows about, internal first.
func (c *fakeClient) all() []*fakePort {
	return append([]*fakePort{c.internal}, c.existing...)
}

// wired is the list of connections made so far.
func (c *fakeClient) wired() []connection {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]connection{}, c.connections...)
}

// add makes a port appear, reporting it to the registration callback the way JACK
// would when another client publishes it.
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

// cycle runs one process callback with the given frame count.
func (c *fakeClient) cycle(t *testing.T, nframes uint32) {
	t.Helper()
	c.mu.Lock()
	cb := c.process
	c.mu.Unlock()
	if cb == nil {
		t.Fatal("no process callback registered")
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
func (c *fakeClient) opener(name string) (JackClient, error) {
	if c.openErr != nil {
		return nil, c.openErr
	}
	c.mu.Lock()
	c.opened = true
	c.mu.Unlock()
	if c.internal.name == "" {
		c.internal = newFakePort(name + ":" + testPortName)
	}
	return c, nil
}

// tracked is a snapshot of the external ports the package is holding on to. The
// package reaches them from its own goroutine, so a test has to read them the same
// guarded way rather than looking at the map itself.
func tracked(p *Port) map[string]JackPort {
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
	t.Fatalf("timed out waiting for %s", what)
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

// newTestPort opens a write port against the fake client.
func newTestPort(t *testing.T, c *fakeClient, pc PortConfig) *Port {
	t.Helper()
	p, err := NewWritePort(testConfig(c, pc))
	if err != nil {
		t.Fatalf("opening port: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

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

			p, err := tc.open(testConfig(c, PortConfig{MatchName: []string{tc.match}}))
			if err != nil {
				t.Fatalf("opening port: %v", err)
			}
			defer p.Close()

			wired := c.wired()
			if len(wired) != 1 {
				t.Fatalf("got %d connections, want 1", len(wired))
			}
			if want := (connection{tc.wantSrc, tc.wantDst}); wired[0] != want {
				t.Errorf("connected %+v, want %+v", wired[0], want)
			}
			if got := len(tracked(p)); got != 1 {
				t.Errorf("got %d external ports, want 1", got)
			}
		})
	}
}

func TestSkipsPortsOfOurOwnClient(t *testing.T) {
	c := newFakeClient(testClientName)
	// The internal port is registered under the client name, so a match that would
	// otherwise catch it must not produce a connection.
	c.existing = []*fakePort{newFakePort(testClientName + ":out"), newFakePort("system:playback_1")}

	newTestPort(t, c, PortConfig{MatchName: []string{"out", "system:playback"}})

	wired := c.wired()
	if len(wired) != 1 {
		t.Fatalf("got %d connections, want 1: %+v", len(wired), wired)
	}
	if wired[0].dst != "system:playback_1" {
		t.Errorf("connected to %q, want system:playback_1", wired[0].dst)
	}
}

func TestConnectsPortsThatAppearLater(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	if got := len(c.wired()); got != 0 {
		t.Fatalf("got %d connections before anything registered, want 0", got)
	}

	c.add(playbackPort)

	// The package connects from its own goroutine, so the assertion has to follow it.
	waitWired(t, c, 1)
	waitTracked(t, p, 1)
	if _, ok := tracked(p)["system:playback_1"]; !ok {
		t.Errorf("system:playback_1 is not tracked as external")
	}
}

func TestIgnoresPortsThatDoNotMatch(t *testing.T) {
	c := newFakeClient(testClientName)
	newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	c.add("system:capture_1")
	c.add(playbackPort)

	waitWired(t, c, 1)
	wired := c.wired()
	if wired[0].dst != "system:playback_1" {
		t.Errorf("connected to %q, want system:playback_1", wired[0].dst)
	}
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
	if calls != 1 {
		t.Fatalf("callback ran %d times while connected, want 1", calls)
	}

	c.remove(ext)

	if got := len(tracked(p)); got != 0 {
		t.Fatalf("got %d external ports after removal, want 0", got)
	}

	// JACK has dropped the other end, so the callback must stop rather than keep the
	// program working against a device that is no longer there.
	c.cycle(t, testFrames)
	if calls != 1 {
		t.Errorf("callback ran %d times after removal, want 1", calls)
	}
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
	if gotFrames != 0 {
		t.Fatalf("callback ran with %d frames while nothing was connected", gotFrames)
	}

	c.add(playbackPort)
	waitFor(t, "the new port to be connected", func() bool { return len(c.wired()) == 1 })

	c.cycle(t, testFrames)
	if gotFrames != testFrames {
		t.Errorf("callback got %d frames, want %d", gotFrames, testFrames)
	}
	if len(gotBuffer) != testFrames {
		t.Errorf("callback got a %d sample buffer, want %d", len(gotBuffer), testFrames)
	}
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
	if len(written) != len(msgs) {
		t.Fatalf("got %d events written, want %d", len(written), len(msgs))
	}
	for i, msg := range msgs {
		if string(written[i].Msg) != string(msg) {
			t.Errorf("event %d is % x, want % x", i, written[i].Msg, msg)
		}
	}
	if port.cleared != testFrames {
		t.Errorf("buffer cleared for %d frames, want %d", port.cleared, testFrames)
	}
}

func TestMidiCallbackStopsOnWriteFailure(t *testing.T) {
	c := newFakeClient(testClientName)
	c.internal.writeErr = errWrite

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
	if writes != 0 {
		t.Errorf("got %d successful writes, want 0", writes)
	}
}

func TestGetBufferAndServerParameters(t *testing.T) {
	c := newFakeClient(testClientName)
	c.sampleRate, c.bufferSize = 44100, 512

	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	if got := p.SampleRate(); got != 44100 {
		t.Errorf("sample rate %d, want 44100", got)
	}
	if got := p.BufferSize(); got != 512 {
		t.Errorf("buffer size %d, want 512", got)
	}
	if got := len(p.GetBuffer(8)); got != 8 {
		t.Errorf("got a %d sample buffer, want 8", got)
	}
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

			_, err := NewWritePort(testConfig(c, PortConfig{MatchName: []string{playbackMatch}}))
			if !errors.Is(err, wantErr) {
				t.Errorf("got error %v, want %v", err, wantErr)
			}
			// A client that never opened has nothing to close, so only one that was
			// handed out can have been left running.
			if c.wasOpened() && !c.isClosed() {
				t.Error("the client was left open after the failure")
			}
		})
	}
}

func TestCloseReleasesEverything(t *testing.T) {
	c := newFakeClient(testClientName)
	c.add(playbackPort)

	p, err := NewWritePort(testConfig(c, PortConfig{MatchName: []string{playbackMatch}}))
	if err != nil {
		t.Fatalf("opening port: %v", err)
	}
	waitFor(t, "the new port to be connected", func() bool { return len(c.wired()) == 1 })

	p.Close()

	if !c.isClosed() {
		t.Error("the client was not closed")
	}
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
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	// A device commonly publishes several channels together. Refusing the ones that
	// arrive while another is still being wired would strand them for good, since
	// nothing asks for them again.
	c.add(playbackPort)
	c.add("system:playback_2222")
	c.add("system:playback_33333")

	waitWired(t, c, 3)
	for _, name := range []string{"system:playback_1", "system:playback_2222", "system:playback_33333"} {
		if _, ok := tracked(p)[name]; !ok {
			t.Errorf("%s was not connected", name)
		}
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
			ClientName: testClientName,
			PortName:   testPortName,
			MatchName:  []string{"system:playback"},
			OpenClient: func(string) (JackClient, error) { return lc, nil },
		})
		if err != nil {
			t.Fatalf("opening port: %v", err)
		}

		lc.add(playbackPort)
		p.Close()

		lc.mu.Lock()
		late := lc.lateCall
		lc.mu.Unlock()
		if late != 0 {
			t.Fatalf("attempted %d connections after the client was closed", late)
		}
	}
}

func TestCloseTwiceIsHarmless(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	p.Close()
	// A program that closes a port explicitly and again through a defer must not die
	// on the second close.
	p.Close()

	if got := len(c.connections); got != 0 {
		t.Errorf("got %d connections, want 0", got)
	}
}

func TestPortAppearingAfterCloseIsIgnored(t *testing.T) {
	c := newFakeClient(testClientName)
	p := newTestPort(t, c, PortConfig{MatchName: []string{playbackMatch}})

	p.Close()

	// A device can be plugged in while the client is shutting down. Queuing it then
	// would be a send on the closed channel, which takes the process down.
	c.add(playbackPort)

	if got := len(c.wired()); got != 0 {
		t.Errorf("got %d connections after close, want 0: %+v", got, c.wired())
	}
	if got := len(tracked(p)); got != 0 {
		t.Errorf("got %d tracked ports after close, want 0", got)
	}
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
