package jack

import (
	"io"
	"log"
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

// Port is one JACK port owned by this package: a single internal port that is wired
// to whichever external ports match PortConfig.MatchName.
type Port struct {
	PortConfig

	fl uint64

	client       JackClient
	portInternal JackPort

	// mu guards the external port bookkeeping, which the registration callback and the
	// connect goroutine both reach. The realtime callbacks cannot afford a lock, so
	// they read nConnected instead.
	mu           sync.Mutex
	portExternal map[string]JackPort
	nConnected   atomic.Int32

	// portc hands ports to the goroutine that connects them, so that a connection is
	// never attempted on the thread JACK calls back into.
	portc chan JackPort

	// closed stops a port registering at from being handed to the channel after Close
	// has shut that channel, which would panic the process on a send. A registration
	// can still arrive while the client is going down, so the check has to be there
	// rather than assumed away.
	closed bool

	closeOnce sync.Once

	wg sync.WaitGroup

	mw *midiWriter
}

type PortConfig struct {
	ClientName string
	PortName   string

	MatchName []string

	AudioCallback JackAudioCallback
	MidiCallback  JackMidiCallback

	// OpenClient overrides how the JACK client is opened, which is the one step that
	// needs a running JACK server. Tests set it to a stand-in so the routing below
	// can run without one; nil means the real thing.
	OpenClient OpenClientFunc
}

func (pc *PortConfig) isNameMatch(s string) bool {
	ret := false
	for _, mn := range pc.MatchName {
		ret = ret || strings.Contains(s, mn)
	}
	return ret
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

func NewReadPort(pc PortConfig) (*Port, error) {
	return NewJackPort(pc, portIsInput|portIsTerminal)
}

func NewWritePort(pc PortConfig) (*Port, error) {
	return NewJackPort(pc, portIsOutput|portIsTerminal)
}

func (j *Port) GetBuffer(nf int) []float32 {
	return j.portInternal.AudioBuffer(uint32(nf))
}

// SampleRate is the rate the JACK server runs this port at.
func (j *Port) SampleRate() uint32 {
	return j.client.SampleRate()
}

// BufferSize is the largest frame count the JACK server will ask for in one cycle.
func (j *Port) BufferSize() uint32 {
	return j.client.BufferSize()
}

func NewJackPort(pc PortConfig, fl uint64) (*Port, error) {
	client, err := pc.open()(pc.ClientName)
	if err != nil {
		return nil, err
	}
	j := &Port{
		PortConfig:   pc,
		client:       client,
		portExternal: make(map[string]JackPort),
		fl:           fl,
		portc:        make(chan JackPort, connectQueueDepth),
	}
	if err := j.client.SetPortRegistrationCallback(j.portRegistration); err != nil {
		j.client.Close()
		return nil, err
	}
	var cb ProcessCallback
	if pc.AudioCallback != nil {
		cb = j.processAudio
	} else {
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
	if err := client.Activate(); err != nil {
		j.client.Close()
		return nil, err
	}
	log.Println("jack activated")
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		for p := range j.portc {
			j.connectExternal(p)
		}
	}()

	portType := audioPortType
	if pc.MidiCallback != nil {
		portType = midiPortType
	}

	j.portInternal = j.client.PortRegister(pc.PortName, portType, fl, portBufferHint)

	for _, mn := range j.MatchName {
		srcs := j.ports(mn)
		for _, src := range srcs {
			if err := j.connectExternal(src); err != nil {
				j.Close()
				return nil, err
			}
		}
		if len(srcs) == 0 {
			log.Printf("matching port not found on %s; will wait to register", mn)
		}
	}
	return j, nil
}

type midiWriter struct {
	port *Port
	ts   uint32
	buf  MidiBuffer
}

func (mw *midiWriter) Write(msg []byte) (int, error) {
	event := MidiEvent{mw.ts, msg}
	mw.ts += 1
	if err := mw.port.portInternal.MidiWrite(event, mw.buf); err != nil {
		return 0, err
	}
	return len(msg), nil
}

// isConnected reports whether anything is wired to this port. The realtime callbacks
// use it to sit out cycles while JACK has nowhere to deliver samples.
func (j *Port) isConnected() bool {
	return j.nConnected.Load() > 0
}

// setConnected publishes the external port count to the realtime callbacks. It runs
// under the lock so the count cannot be published from a snapshot that a concurrent
// change has already overtaken.
func (j *Port) setConnected() {
	j.nConnected.Store(int32(len(j.portExternal)))
}

func (j *Port) processMidi(nFrames uint32) int {
	if !j.isConnected() {
		return 0
	}
	j.mw.buf = j.portInternal.MidiClearBuffer(nFrames)
	j.MidiCallback(j.mw)
	return 0
}

func (j *Port) processAudio(nFrames uint32) int {
	if !j.isConnected() {
		return 0
	}
	return j.AudioCallback(j.portInternal.AudioBuffer(nFrames))
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
	if strings.HasPrefix(name, j.ClientName) || !j.isNameMatch(name) {
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
	case j.portc <- p:
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
	dropped := 0
	for name := range j.portExternal {
		if j.client.PortByName(name) == nil {
			log.Println("unregistered:", name)
			delete(j.portExternal, name)
			dropped++
		}
	}
	if dropped > 0 {
		j.setConnected()
	}
	j.mu.Unlock()
}

func (j *Port) ports(name string) (ret []JackPort) {
	for _, pname := range j.client.PortNames(name, "", 0) {
		if !strings.HasPrefix(pname, j.ClientName) {
			p := j.client.PortByName(pname)
			ret = append(ret, p)
		}
	}
	return ret
}

func (j *Port) connectExternal(ext JackPort) error {
	src, dst := j.portInternal, ext
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
	j.portExternal[ext.Name()] = ext
	j.setConnected()
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
