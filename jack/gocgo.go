package jack

import (
	"unsafe"

	gojack "github.com/xthexder/go-jack"
)

// The JACK constants used to create ports live here so the routing in jack.go stays
// free of cgo and can be tested against stand-in ports.
const (
	portIsInput    uint64 = gojack.PortIsInput
	portIsOutput   uint64 = gojack.PortIsOutput
	portIsTerminal uint64 = gojack.PortIsTerminal

	audioPortType = gojack.DEFAULT_AUDIO_TYPE
	midiPortType  = gojack.DEFAULT_MIDI_TYPE
)

// jackClient adapts *gojack.Client to JackClient.
type jackClient struct {
	c *gojack.Client
}

// openClient is the real JACK server. Tests replace it through PortConfig.OpenClient
// so that opening a port does not need a server running.
func openClient(name string) (JackClient, error) {
	c, status := gojack.ClientOpen(name, gojack.NoStartServer)
	if status != 0 {
		return nil, gojack.StrError(status)
	}
	return &jackClient{c}, nil
}

func (c *jackClient) SetPortRegistrationCallback(cb PortRegistrationCallback) error {
	return status(c.c.SetPortRegistrationCallback(func(id gojack.PortId, made bool) {
		cb(PortID(id), made)
	}))
}

func (c *jackClient) SetPortConnectCallback(cb PortConnectCallback) error {
	return status(c.c.SetPortConnectCallback(func(a, b gojack.PortId, isConnect bool) {
		cb(PortID(a), PortID(b), isConnect)
	}))
}

func (c *jackClient) SetProcessCallback(cb ProcessCallback) error {
	return status(c.c.SetProcessCallback(func(nframes uint32) int { return cb(nframes) }))
}

func (c *jackClient) Activate() error    { return status(c.c.Activate()) }
func (c *jackClient) Close() error       { return status(c.c.Close()) }
func (c *jackClient) SampleRate() uint32 { return c.c.GetSampleRate() }
func (c *jackClient) BufferSize() uint32 { return c.c.GetBufferSize() }
func (c *jackClient) PortNames(name, portType string, flags uint64) []string {
	return c.c.GetPorts(name, portType, flags)
}

func (c *jackClient) PortRegister(name, portType string, flags, bufferSize uint64) JackPort {
	return wrapPort(c.c.PortRegister(name, portType, flags, bufferSize))
}

func (c *jackClient) PortByID(id PortID) JackPort {
	return wrapPort(c.c.GetPortById(gojack.PortId(id)))
}

func (c *jackClient) PortByName(name string) JackPort {
	return wrapPort(c.c.GetPortByName(name))
}

func (c *jackClient) ConnectPorts(src, dst JackPort) error {
	return status(c.c.ConnectPorts(src.(*jackPort).p, dst.(*jackPort).p))
}

// jackPort adapts *gojack.Port to JackPort.
type jackPort struct {
	p *gojack.Port
}

// wrapPort keeps a missing port missing, so callers see the absence the cgo call
// reported rather than a wrapper that only fails once it is used.
func wrapPort(p *gojack.Port) JackPort {
	if p == nil {
		return nil
	}
	return &jackPort{p}
}

func (p *jackPort) Name() string { return p.p.GetName() }

// AudioBuffer hands back the port's own cycle buffer as plain float32. The cast is
// safe because go-jack defines AudioSample as float32, so both slices have the same
// layout, and it saves a per-cycle copy on the realtime thread.
func (p *jackPort) AudioBuffer(nframes uint32) []float32 {
	buf := p.p.GetBuffer(nframes)
	return *(*[]float32)(unsafe.Pointer(&buf))
}

func (p *jackPort) MidiClearBuffer(nframes uint32) MidiBuffer {
	return &midiBuffer{p.p.MidiClearBuffer(nframes)}
}

func (p *jackPort) MidiWrite(ev MidiEvent, buf MidiBuffer) error {
	data := gojack.MidiData{Time: ev.Frame, Buffer: ev.Msg}
	return status(p.p.MidiEventWrite(&data, buf.(*midiBuffer).b))
}

// midiBuffer wraps go-jack's *[]byte, which cannot be carried as a plain []byte
// without losing the pointer the write needs.
type midiBuffer struct {
	b gojack.MidiBuffer
}

func (m *midiBuffer) Bytes() []byte { return *m.b }

// status turns a JACK return code into an error, since JACK reports every failure as
// a bare int.
func status(code int) error {
	if code != 0 {
		return gojack.StrError(code)
	}
	return nil
}
