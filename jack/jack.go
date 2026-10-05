package jack

import (
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// PortID identifies a JACK port. The registration and connect callbacks speak in
// these rather than in port handles, so a test can name a port without owning one.
type PortID uint32

// ProcessCallback runs once per cycle and returns nonzero to tell JACK to drop the
// rest of the graph for that cycle.
type ProcessCallback func(nframes uint32) int

// PortRegistrationCallback reports a port appearing or disappearing. made is false
// when the port has just gone away.
type PortRegistrationCallback func(id PortID, made bool)

// PortConnectCallback reports two ports being wired or unwired. Nothing here acts on
// it; JACK wants a client's callback set filled in before it will activate.
type PortConnectCallback func(a, b PortID, isConnect bool)

// MidiBuffer is the outgoing MIDI of a single cycle. It is an interface rather than a
// byte slice because JACK hands out a pointer to its own scratch space, and a write
// needs that pointer rather than a copy of the bytes.
type MidiBuffer interface {
	Bytes() []byte
}

// MidiEvent is one MIDI message stamped with the frame inside the cycle at which
// JACK should emit it.
type MidiEvent struct {
	Frame uint32
	Msg   []byte
}

// JackPort is the part of a JACK port this package drives. go-jack's *jack.Port keeps
// its state behind cgo, so a test cannot build one; wrapping it lets a test supply a
// port of its own instead.
type JackPort interface {
	// Name is the fully qualified JACK name, "client:port".
	Name() string
	// AudioBuffer is the nframes-long scratch buffer for the cycle in progress.
	AudioBuffer(nframes uint32) []float32
	// MidiClearBuffer empties the outgoing MIDI buffer and hands it back for MidiWrite.
	MidiClearBuffer(nframes uint32) MidiBuffer
	// MidiWrite queues one event into a buffer previously returned by MidiClearBuffer.
	MidiWrite(ev MidiEvent, buf MidiBuffer) error
}

// JackClient is the part of a JACK client this package drives. Failures arrive as
// errors because JACK reports them as bare ints, which a caller has no way to read.
type JackClient interface {
	SetPortRegistrationCallback(cb PortRegistrationCallback) error
	SetPortConnectCallback(cb PortConnectCallback) error
	SetProcessCallback(cb ProcessCallback) error
	Activate() error
	Close() error
	// PortRegister creates one of this client's own ports, named "client:name". The
	// buffer size is a hint JACK uses when sizing the port's buffers.
	PortRegister(name, portType string, flags, bufferSize uint64) JackPort
	PortByID(id PortID) JackPort
	PortByName(name string) JackPort
	// PortNames lists the names matching a pattern and port type; an empty pattern
	// matches every port.
	PortNames(name, portType string, flags uint64) []string
	// ConnectPorts wires src to dst.
	ConnectPorts(src, dst JackPort) error
	SampleRate() uint32
	BufferSize() uint32
}

// OpenClientFunc opens the JACK client a port lives on.
type OpenClientFunc func(name string) (JackClient, error)

// portBufferHint is the buffer size asked for when a port is registered. JACK treats
// it as a hint and reallocates if the server asks for more.
const portBufferHint uint64 = 8192

// connectQueueDepth is how many ports may be waiting to be connected at once. A device
// publishing its channels together should fit without anything being dropped, and the
// queue is only a hand-off buffer rather than a place to accumulate work.
const connectQueueDepth = 16

// The suffixes a stereo output's two ports are named with. They are named rather than
// built inline because they are visible in jack_lsp and end up in the sampler's
// documentation.
const (
	leftSuffix  = "_l"
	rightSuffix = "_r"
)

// Port is one JACK port owned by this package, wired to whichever external ports match
// PortConfig.MatchName. It holds one internal port for a mono or MIDI port and two for
// a stereo audio port, so that both channels are filled by the one process callback
// rather than by two clients half a period apart.
type Port struct {
	PortConfig

	fl uint64

	client JackClient
	// ports are this package's own ports, filled before Activate so the realtime
	// callbacks can index them without a lock and without finding the slice short.
	// Index 0 is the one the MIDI path uses, a MIDI port having one channel by
	// definition.
	ports []JackPort

	// mu guards the external port bookkeeping, which the registration callback and the
	// connect goroutine both reach. The realtime callbacks cannot afford a lock, so
	// they read isReady instead.
	mu           sync.Mutex
	portExternal map[string]wiring
	// nWired counts, per one of this package's own ports, the external ports reaching
	// it. It is per port rather than one total because a stereo port with a single
	// channel wired is not half a note, it is a note heard hard against one speaker.
	nWired []int
	// isReadyFlag publishes whether every one of those counts is above zero. It is a
	// single word rather than a slice so the realtime callback reads it with one atomic
	// load and no lock.
	isReadyFlag atomic.Int32

	// portc hands matches to the goroutine that connects them, so that a connection is
	// never attempted on the thread JACK calls back into.
	portc chan match

	// closed stops a port registering at from being handed to the channel after Close
	// has shut that channel, which would panic the process on a send. A registration
	// can still arrive while the client is going down, so the check has to be there
	// rather than assumed away.
	closed bool

	closeOnce sync.Once

	wg sync.WaitGroup

	mw *midiWriter
}

// match is one queued connection: which entry of MatchName the port belongs to, and
// the port itself. A stereo port needs the index as well as the port, since the entry
// is what says which of its own ports the destination is wired to.
type match struct {
	index int
	port  JackPort
}

// wiring is what an external port was connected to: the port itself, and every one of
// this package's own ports it reached. One external can be reached by both channels at
// once, and taking it away has to undo exactly those, or a channel would go on counting
// a connection the server no longer has.
type wiring struct {
	port JackPort
	own  []int
}

type PortConfig struct {
	ClientName string
	PortName   string

	MatchName []string

	AudioCallback  JackAudioCallback
	StereoCallback StereoAudioCallback
	MidiCallback   JackMidiCallback

	// OpenClient overrides how the JACK client is opened, which is the one step that
	// needs a running JACK server. Tests set it to a stand-in so the routing below
	// can run without one; nil means the real thing.
	OpenClient OpenClientFunc
}

// matchIndex reports which entry of MatchName a port name belongs to, or -1 if it
// matches none. The index is what decides which of a stereo port's two ports a
// destination is wired to, so a bool is not enough: left and right have to be told
// apart rather than merely recognised. A name matching several entries belongs to the
// first, which is the order the mono path already resolved them in. An empty entry
// matches nothing, rather than everything the server happens to publish.
func (pc *PortConfig) matchIndex(name string) int {
	for i, mn := range pc.MatchName {
		if mn != "" && nameMatchesPort(name, mn) {
			return i
		}
	}
	return -1
}

// nameMatchesPort reports whether a port name belongs to a match entry. Containment is
// what lets one entry name a whole family of ports, so "system:playback" still reaches
// every channel of a device. Bare containment also lets a numbered channel swallow the
// next one: "system:playback_1" is contained in "system:playback_10", which on a stereo
// port quietly puts two sinks on one channel and none on the other. An entry is
// therefore only read as a prefix when what follows it is not a digit, since that is
// where one channel number stops and a longer one begins.
func nameMatchesPort(name, match string) bool {
	for at := 0; at <= len(name)-len(match); {
		found := strings.Index(name[at:], match)
		if found < 0 {
			return false
		}
		at += found
		after := at + len(match)
		if after == len(name) || name[after] < '0' || name[after] > '9' {
			return true
		}
		at = after
	}
	return false
}

// open resolves the client opener for this configuration.
func (pc *PortConfig) open() OpenClientFunc {
	if pc.OpenClient != nil {
		return pc.OpenClient
	}
	return openClient
}

type JackAudioCallback func([]float32) int
type JackMidiCallback func(io.Writer)

// StereoAudioCallback fills one cycle of both output channels. The two slices are the
// two ports' own buffers, are unrelated to each other, and are the same length, which is
// what lets a caller clear and scale them in one loop.
type StereoAudioCallback func(left, right []float32) int

// portKind is the kind of port a configuration asks for. It is resolved once, before
// anything is opened, because the same three callback fields decide both what gets
// validated and what gets registered, and answering that twice is how the two answers
// drift apart.
type portKind int

const (
	kindNone portKind = iota
	kindAudio
	kindStereo
	kindMidi
)

// kind resolves a configuration to the one kind of port it describes. More than one
// callback set is an error rather than a preference, and none set is an error rather
// than a port that connects and outputs silence, since a misconfiguration of that kind
// would otherwise only be found by listening rather than by starting.
func (pc *PortConfig) kind() (portKind, error) {
	set := 0
	for _, filled := range []bool{pc.AudioCallback != nil, pc.StereoCallback != nil, pc.MidiCallback != nil} {
		if filled {
			set++
		}
	}
	switch {
	case set > 1:
		return kindNone, fmt.Errorf("set exactly one of AudioCallback, StereoCallback or MidiCallback")
	case pc.StereoCallback != nil:
		return kindStereo, nil
	case pc.AudioCallback != nil:
		return kindAudio, nil
	case pc.MidiCallback != nil:
		return kindMidi, nil
	}
	return kindNone, fmt.Errorf("no callback set, so there is nothing to fill the port with")
}

func NewReadPort(pc PortConfig) (*Port, error) {
	return NewJackPort(pc, portIsInput|portIsTerminal)
}

func NewWritePort(pc PortConfig) (*Port, error) {
	return NewJackPort(pc, portIsOutput|portIsTerminal)
}

// NewStereoWritePort opens a two channel output: two of this package's own ports on
// one client, filled by one process callback over one cycle. Two clients would get a
// callback each and render the two halves of a note half a period apart, which is a comb
// filter on everything the caller plays.
func NewStereoWritePort(pc PortConfig) (*Port, error) {
	return newJackPort(pc, portIsOutput|portIsTerminal, []string{
		pc.PortName + leftSuffix,
		pc.PortName + rightSuffix,
	})
}

// GetBuffer is one port's cycle buffer, which on a stereo port is the left channel. A
// stereo caller wants both, which is what StereoCallback is for.
func (j *Port) GetBuffer(nf int) []float32 {
	return j.ports[0].AudioBuffer(uint32(nf))
}

// SampleRate is the rate the JACK server runs this port at.
func (j *Port) SampleRate() uint32 {
	return j.client.SampleRate()
}

// BufferSize is the largest frame count the JACK server will ask for in one cycle.
func (j *Port) BufferSize() uint32 {
	return j.client.BufferSize()
}

// portKindFor resolves what a configuration asks for and checks it against the number
// of ports being registered. Both answers have to come from one place: the realtime
// callback picks what fills it from the port count, so a configuration whose callback
// and port count disagree would be accepted here and then call a function nobody set,
// on JACK's own thread, where a nil call takes the process down rather than returning
// an error.
func portKindFor(pc *PortConfig, names []string) (portKind, error) {
	kind, err := pc.kind()
	if err != nil {
		return kindNone, err
	}
	switch {
	case kind == kindStereo && len(names) < 2:
		return kindNone, fmt.Errorf("StereoCallback needs a stereo port, which registers two ports")
	case kind != kindStereo && len(names) > 1:
		return kindNone, fmt.Errorf("a port with %d outputs needs StereoCallback, not the callback that was set", len(names))
	case kind == kindStereo && len(pc.MatchName) > len(names):
		// Only a stereo port routes by position, so only a stereo port can run out of
		// channels. A match entry past the last one has nothing to wire to, and taking
		// it would index off the end of the ports slice, which took the process down
		// during the sweep. Refusing here keeps that a message the caller can read.
		return kindNone, fmt.Errorf("a stereo port has %d channels but %d match names, so the last %d name no channel",
			len(names), len(pc.MatchName), len(pc.MatchName)-len(names))
	}
	return kind, nil
}

// NewJackPort opens one of this package's own ports. It is the whole of the shared
// constructor; names is the ports to register, and a stereo port passes two of them.
func NewJackPort(pc PortConfig, fl uint64) (*Port, error) {
	return newJackPort(pc, fl, []string{pc.PortName})
}

func newJackPort(pc PortConfig, fl uint64, names []string) (*Port, error) {
	// Resolved before the client is opened, so that a configuration this package
	// refuses never reaches the server and never leaves one open behind it.
	kind, err := portKindFor(&pc, names)
	if err != nil {
		return nil, err
	}
	client, err := pc.open()(pc.ClientName)
	if err != nil {
		return nil, err
	}
	j := &Port{
		PortConfig:   pc,
		client:       client,
		portExternal: make(map[string]wiring),
		nWired:       make([]int, len(names)),
		fl:           fl,
		portc:        make(chan match, connectQueueDepth),
	}
	if err := j.client.SetPortRegistrationCallback(j.portRegistration); err != nil {
		j.client.Close()
		return nil, err
	}
	cb := ProcessCallback(j.processAudio)
	if kind == kindMidi {
		cb = j.processMidi
		j.mw = &midiWriter{port: j}
	}
	if err := client.SetProcessCallback(cb); err != nil {
		j.client.Close()
		return nil, err
	}
	if err := client.SetPortConnectCallback(j.portConnect); err != nil {
		j.client.Close()
		return nil, err
	}
	// Registered before Activate so that the ports slice is complete before the
	// process callback can run. That thread reads it without a lock, and a slice read
	// while it is still being filled is an index panic rather than a nil dereference.
	// JACK refuses a process callback set after activation and a connection made before
	// it, but says nothing against registering first, which is the order a JACK client
	// normally uses.
	if err := j.registerPorts(kind, fl, names); err != nil {
		j.client.Close()
		return nil, err
	}
	if err := client.Activate(); err != nil {
		j.client.Close()
		return nil, err
	}
	log.Println("jack activated")
	j.startConnector()
	if err := j.sweepExisting(); err != nil {
		j.Close()
		return nil, err
	}
	return j, nil
}

// registerPorts claims the named ports on the server. It runs before activation, and
// the caller closes the client on failure like every other step here.
func (j *Port) registerPorts(kind portKind, fl uint64, names []string) error {
	portType := audioPortType
	if kind == kindMidi {
		portType = midiPortType
	}
	for _, name := range names {
		p := j.client.PortRegister(name, portType, fl, portBufferHint)
		if p == nil {
			return fmt.Errorf("jack refused to register %q", name)
		}
		j.ports = append(j.ports, p)
	}
	return nil
}

// startConnector runs the goroutine that makes connections. It cannot start before
// activation, since JACK refuses a connection made before it, so anything that
// registers in the meantime waits in the queue rather than being wired.
func (j *Port) startConnector() {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		for m := range j.portc {
			if err := j.connectMatch(m.index, m.port); err != nil {
				log.Println("failed to connect a matched port:", err)
			}
		}
	}()
}

// sweepExisting wires the ports that were already there when the client opened, which
// never announced themselves. Each port is asked which entry it belongs to rather than
// each entry being asked what it matches: a name matching two entries would then be
// wired once per entry, and the bookkeeping keyed by name would collapse those into
// one, leaving the connected count short of what is actually wired.
func (j *Port) sweepExisting() error {
	found := make([]bool, len(j.MatchName))
	for _, ext := range j.externalPorts() {
		i := j.matchIndex(ext.Name())
		if i < 0 {
			continue
		}
		if err := j.connectMatch(i, ext); err != nil {
			return err
		}
		found[i] = true
	}
	for i, mn := range j.MatchName {
		if !found[i] {
			log.Printf("matching port not found on %s; will wait to register", mn)
		}
	}
	return nil
}

// connectMatch wires one external port to every one of this package's own ports that
// the match entry it belongs to feeds, which is one port for a mono or MIDI port and
// both for a stereo port reached by a single ambiguous name.
func (j *Port) connectMatch(index int, ext JackPort) error {
	for _, own := range j.matchPorts(index) {
		if err := j.connectExternal(own, ext); err != nil {
			return err
		}
	}
	return nil
}

// matchPorts reports which of this package's own ports match entry i connects to. A
// mono or MIDI port is always 0, which is what leaves the existing routing unchanged
// however many match names there are. A stereo port with two or more entries is entry
// i, so the list order is the channel order; a stereo port with a single entry is every
// port, because one ambiguous request should reach every matched sink with the down mix
// rather than pick an end of it.
func (j *Port) matchPorts(i int) []int {
	if len(j.ports) < 2 {
		return []int{0}
	}
	if len(j.MatchName) < 2 {
		return []int{0, 1}
	}
	return []int{i}
}

type midiWriter struct {
	port *Port
	ts   uint32
	buf  MidiBuffer
}

func (mw *midiWriter) Write(msg []byte) (int, error) {
	event := MidiEvent{mw.ts, msg}
	mw.ts += 1
	if err := mw.port.ports[0].MidiWrite(event, mw.buf); err != nil {
		return 0, err
	}
	return len(msg), nil
}

// isReady reports whether every one of this package's own ports has somewhere to send
// samples. A stereo port waits rather than running on whichever channel happens to be
// wired, since a half image is worse than either channel alone and nothing about it
// looks like a fault from where the player is sitting.
func (j *Port) isReady() bool {
	return j.isReadyFlag.Load() > 0
}

// publish records whether every one of this package's own ports has somewhere to send
// samples. It runs under the lock so the answer cannot be worked out from a snapshot a
// concurrent change has already overtaken.
func (j *Port) publish() {
	ready := int32(1)
	for _, n := range j.nWired {
		if n == 0 {
			ready = 0
			break
		}
	}
	j.isReadyFlag.Store(ready)
}

func (j *Port) processMidi(nFrames uint32) int {
	if !j.isReady() {
		return 0
	}
	j.mw.buf = j.ports[0].MidiClearBuffer(nFrames)
	j.MidiCallback(j.mw)
	return 0
}

func (j *Port) processAudio(nFrames uint32) int {
	if !j.isReady() {
		return 0
	}
	// Both of a stereo port's buffers are taken whether or not anything is wired to
	// them, so an unwired side is filled and discarded rather than left holding stale
	// samples. They come from two ports rather than from one interleaved buffer, so
	// there is no channel mapping to get wrong here.
	left := j.ports[0].AudioBuffer(nFrames)
	if len(j.ports) > 1 {
		return j.StereoCallback(left, j.ports[1].AudioBuffer(nFrames))
	}
	return j.AudioCallback(left)
}

func (j *Port) portConnect(a, b PortID, is_connect bool) {}

// portRegistration follows ports as they come and go. JACK names a port only for as
// long as it exists, so nothing is resolved once a port has gone: without that care a
// device unplugged mid-session leaves this port feeding a callback JACK has nothing
// left to deliver to.
func (j *Port) portRegistration(id PortID, made bool) {
	if !made {
		j.unregisterExternal()
		return
	}
	p := j.client.PortByID(id)
	if p == nil {
		return
	}
	name := p.Name()
	index := j.matchIndex(name)
	if strings.HasPrefix(name, j.ClientName) || index < 0 {
		log.Println("ignoring non-match:", name)
		return
	}
	// The check and the handoff share one lock, so that Close cannot shut the channel
	// between the two. The handoff cannot block: the queue is drained continuously and
	// only ever holds a bounded number of ports, so a registration that finds it full
	// has nowhere to wait and must not hold up the server's own thread.
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		log.Println("ignoring match, port is closing:", name)
		return
	}
	if _, tracked := j.portExternal[name]; tracked {
		log.Println("ignoring match, already connected:", name)
		return
	}
	select {
	case j.portc <- match{index, p}:
		log.Println("matched:", name)
	default:
		// A device publishing more ports at once than the queue holds is the only way
		// to get here. Dropping one is better than blocking JACK's thread, but it is
		// still a lost port, so it is worth seeing in the log.
		log.Printf("connect queue full, ignoring match: %s", name)
	}
}

// unregisterExternal forgets any tracked port the server no longer knows. An
// unregistering port is announced by ID, and a port that has gone cannot be resolved
// from one, so the tracked names are re-checked against the server instead. That also
// covers ports which were already connected when this client started, since those
// never announced themselves.
func (j *Port) unregisterExternal() {
	j.mu.Lock()
	dropped := false
	for name, w := range j.portExternal {
		if j.client.PortByName(name) != nil {
			continue
		}
		log.Println("unregistered:", name)
		for _, own := range w.own {
			j.nWired[own]--
		}
		delete(j.portExternal, name)
		dropped = true
	}
	if dropped {
		j.publish()
	}
	j.mu.Unlock()
}

// externalPorts lists every port the server knows except this package's own, which is
// what the startup sweep wants: it asks each port which entry it belongs to, so there
// is nothing to narrow the search by.
func (j *Port) externalPorts() (ret []JackPort) {
	for _, pname := range j.client.PortNames("", "", 0) {
		if !strings.HasPrefix(pname, j.ClientName) {
			p := j.client.PortByName(pname)
			ret = append(ret, p)
		}
	}
	return ret
}

// connectExternal wires one of this package's own ports to one it matched. index is
// which of them, and is 0 for a mono or MIDI port.
func (j *Port) connectExternal(index int, ext JackPort) error {
	src, dst := j.ports[index], ext
	if j.fl&portIsInput == portIsInput {
		src, dst = dst, src
	}
	log.Printf("connecting src=%q to dst=%q", src.Name(), dst.Name())
	if err := j.client.ConnectPorts(src, dst); err != nil {
		log.Println("failed to connect ports")
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	w := j.portExternal[ext.Name()]
	if !slices.Contains(w.own, index) {
		w.own = append(w.own, index)
		j.nWired[index]++
	}
	w.port = ext
	j.portExternal[ext.Name()] = w
	j.publish()
	return nil
}

func Ports() (ret []string, err error) {
	client, err := openClient("listports")
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.PortNames("", "", 0), nil
}

// Close shuts the client and stops the goroutine that makes connections, returning
// only once that goroutine has finished. It is safe to call more than once: a port
// opened successfully after an error path already closed it, or a program that closes
// explicitly and again through a defer, would otherwise panic on the closed channel.
func (j *Port) Close() {
	j.closeOnce.Do(func() {
		// closed is published under the lock that portRegistration holds, so that a
		// registration already in flight finishes deciding before the channel goes.
		j.mu.Lock()
		j.closed = true
		j.mu.Unlock()
		// The connect goroutine is stopped before the client, not after: a connection
		// already under way calls into the client, and closing the client out from
		// under it would be using a handle JACK has already torn down.
		close(j.portc)
		j.wg.Wait()
		j.client.Close()
	})
}
