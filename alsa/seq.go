package alsa

// This file holds the sequencer's policy: which ports it owns, which direction a
// subscription runs in, how a name becomes an address, and which ports a caller may see.
// None of that needs a sequencer to be open, so all of it can be exercised without one.
// The libasound calls the policy makes live in seq_cgo.go.

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/chzchzchz/midispa/midi"
)

var errExpectedSysEx = errors.New("expected sysex")

type PortDir int

const (
	PortSource PortDir = iota
	PortDest
	PortDuplex
	PortAny
)

const maxSeqAddress = 255

var errNotFound = errors.New("port not found")

type AmbiguousPortError struct {
	Name    string
	Matches []SeqDevice
}

func (e *AmbiguousPortError) Error() string {
	addrs := make([]string, 0, len(e.Matches))
	for _, m := range e.Matches {
		addrs = append(addrs, fmt.Sprintf("%d:%d (%s)", m.Client, m.Port, m.ClientName))
	}
	return fmt.Sprintf("port %q matches %d ports: %s",
		e.Name, len(e.Matches), strings.Join(addrs, ", "))
}

// The sequencer reports its own subscription changes as events with these leading bytes,
// so a consumer can tell an ALSA notification from a MIDI message.
const (
	EvPortSubscribed   = 0
	EvPortUnsubscribed = 1
)

var (
	ErrUnsupportedEvent = errors.New("unsupported ALSA event")
	ErrInvalidMessage   = errors.New("invalid midi message")
)

type SeqAddr struct {
	Client int
	Port   int
}

type SeqEvent struct {
	SeqAddr
	Data []byte
}

func MakeEvent(data []byte) SeqEvent {
	return SeqEvent{SeqAddr: SubsSeqAddr, Data: data}
}

func (ev *SeqEvent) IsControl() bool {
	return ev.Data[0]&0x80 == 0
}

type seqWriter struct {
	seq *Seq
	dst SeqAddr
}

func (a *seqWriter) Write(data []byte) (int, error) {
	if err := a.seq.Write(SeqEvent{a.dst, data}); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (a *Seq) NewWriter(sa SeqAddr) io.Writer { return &seqWriter{a, sa} }

func (a *Seq) CreatePort(name string) error {
	_, err := a.CreatePortAddr(name)
	return err
}

func (a *Seq) CreatePortAddr(name string) (SeqAddr, error) {
	return a.createPortAddrCaps(name, PortCapDuplex|PortCapRead|PortCapSubsRead|PortCapWrite|PortCapSubsWrite)
}

// localPort reports whether addr names a port this sequencer created. Ownership is what
// keeps one client's DeletePort or ClosePort from reaching another's port, and it is the
// only record of ownership Seq keeps: ALSA is not asked, because it does not track who
// created what.
func (a *Seq) localPort(addr SeqAddr) error {
	if err := addr.validate(); err != nil {
		return err
	}
	if a.ports == nil {
		return errors.New("sequencer is closed")
	}
	if _, ok := a.ports[addr.Port]; !ok || addr.Client != a.Client {
		return fmt.Errorf("port %v is not owned by this sequencer", addr)
	}
	return nil
}

func (a *Seq) OpenPort(client, port int) error {
	return a.OpenPortRead(SeqAddr{client, port})
}

func (a *Seq) OpenPortRead(sa SeqAddr) error {
	return a.OpenPortReadAt(a.SeqAddr, sa)
}

func (a *Seq) OpenPortWrite(sa SeqAddr) error {
	return a.OpenPortWriteAt(a.SeqAddr, sa)
}

func (a *Seq) ClosePortWrite(sa SeqAddr) error {
	return a.ClosePortWriteAt(a.SeqAddr, sa)
}

func (a *Seq) ClosePortRead(sa SeqAddr) error {
	return a.ClosePortReadAt(a.SeqAddr, sa)
}

func (a *Seq) OpenPortName(portName string) error {
	return a.OpenPortNameRead(portName)
}

// OpenPortNameRead resolves against the ports that can be read from, so a name that only
// matches a destination fails here rather than silently subscribing to nothing.
func (a *Seq) OpenPortNameRead(portName string) error {
	dev, err := a.resolvePortFiltered(portName, PortSource)
	if err != nil {
		return err
	}
	return a.OpenPortRead(dev.SeqAddr)
}

func (a *Seq) OpenPortNameWrite(portName string) error {
	dev, err := a.resolvePortFiltered(portName, PortDest)
	if err != nil {
		return err
	}
	return a.OpenPortWrite(dev.SeqAddr)
}

// OpenPortDuplex undoes only the half it just made. An already-subscribed input is left
// alone, because adopting or removing a subscription the caller did not ask for here
// would be a change nobody could see.
func (a *Seq) OpenPortDuplex(sa SeqAddr) error {
	if err := a.OpenPortRead(sa); err != nil {
		return err
	}
	if err := a.OpenPortWrite(sa); err != nil {
		if cerr := a.ClosePortRead(sa); cerr != nil {
			return errors.Join(err, cerr)
		}
		return err
	}
	return nil
}

func parseSeqAddr(s string) (SeqAddr, error) {
	client, port, found := strings.Cut(s, ":")
	if !found {
		return SeqAddr{}, fmt.Errorf("invalid address %q, want client:port", s)
	}
	c, err := strconv.Atoi(client)
	if err != nil {
		return SeqAddr{}, fmt.Errorf("invalid client in %q", s)
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return SeqAddr{}, fmt.Errorf("invalid port in %q", s)
	}
	addr := SeqAddr{c, p}
	return addr, addr.validate()
}

func (a *Seq) PortAddress(portName string) (sa SeqAddr, err error) {
	if numericSeqName(portName) {
		return parseSeqAddr(portName)
	}
	dev, err := a.resolvePort(portName)
	if err != nil {
		return SeqAddr{-1, -1}, err
	}
	return dev.SeqAddr, nil
}

// numericSeqName reports whether the name should be read as a client:port pair rather
// than matched against device names. A client prefix of digits followed by a colon is
// unambiguous, and treating it as a name would silently find nothing.
func numericSeqName(name string) bool {
	client, _, found := strings.Cut(name, ":")
	if !found {
		return false
	}
	for index, char := range client {
		if index == 0 && (char == '-' || char == '+') {
			continue
		}
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (a *Seq) resolvePort(name string) (SeqDevice, error) {
	return a.resolvePortFiltered(name, PortAny)
}

// resolvePortFiltered matches a numeric address exactly, or a name against the client
// name, the port name, or the two joined. Several matches are reported rather than
// picked, because choosing one would connect to a port the caller did not name.
func (a *Seq) resolvePortFiltered(name string, dir PortDir) (SeqDevice, error) {
	var addr SeqAddr
	numeric := numericSeqName(name)
	if numeric {
		var err error
		addr, err = parseSeqAddr(name)
		if err != nil {
			return SeqDevice{}, err
		}
	}
	devs, err := a.DevicesFiltered(dir)
	if err != nil {
		return SeqDevice{}, err
	}
	var matches []SeqDevice
	for _, dev := range devs {
		if numeric {
			if dev.SeqAddr == addr {
				return dev, nil
			}
			continue
		}
		if dev.ClientName == name || dev.PortName == name || dev.ClientName+":"+dev.PortName == name {
			matches = append(matches, dev)
		}
	}
	if len(matches) == 0 {
		return SeqDevice{}, fmt.Errorf("%w: %q (direction %d)", errNotFound, name, dir)
	}
	if len(matches) > 1 {
		return SeqDevice{}, &AmbiguousPortError{name, matches}
	}
	return matches[0], nil
}

// ReadSysEx assembles a system exclusive message from the pieces the sequencer delivers.
// The pieces arrive as separate events, and only the first has the F0 status, so the
// check on the following events' first byte is what distinguishes a continuation from
// the start of an unrelated message.
func (a *Seq) ReadSysEx() (ret SeqEvent, err error) {
	for {
		ev, err := a.Read()
		if err != nil {
			return ret, err
		}
		if len(ret.Data) == 0 && ev.Data[0] != midi.SysEx {
			return ret, errExpectedSysEx
		}
		ret.SeqAddr, ret.Data = ev.SeqAddr, append(ret.Data, ev.Data...)
		if ret.Data[len(ret.Data)-1] == midi.EndSysEx {
			break
		}
	}
	return ret, nil
}

// Write sends on the default port, which SeqAddr holds. Assigning another owned address
// to SeqAddr therefore redirects every later write without changing this method.
func (a *Seq) Write(ev SeqEvent) error {
	return a.WritePort(ev, a.Port)
}

type SeqDevice struct {
	SeqAddr
	ClientName string
	PortName   string
	Caps       PortCaps
}

func (a *Seq) Devices() (ret []SeqDevice, err error) {
	return a.DevicesFiltered(PortSource)
}

// DevicesFiltered narrows the port list to the directions a caller asked about. The
// filter is on the remote port, so PortSource means something this client can read from.
func (a *Seq) DevicesFiltered(dir PortDir) ([]SeqDevice, error) {
	if dir < PortSource || dir > PortAny {
		return nil, fmt.Errorf("invalid port direction %d", dir)
	}
	devs, err := a.devices()
	if err != nil {
		return nil, err
	}
	var ret []SeqDevice
	for _, dev := range devs {
		switch dir {
		case PortAny:
			ret = append(ret, dev)
		case PortSource:
			if dev.Caps.Has(PortCapRead | PortCapSubsRead) {
				ret = append(ret, dev)
			}
		case PortDest:
			if dev.Caps.Has(PortCapWrite | PortCapSubsWrite) {
				ret = append(ret, dev)
			}
		case PortDuplex:
			if dev.Caps.Has(PortCapRead | PortCapSubsRead | PortCapWrite | PortCapSubsWrite) {
				ret = append(ret, dev)
			}
		}
	}
	return ret, nil
}

func (d *SeqAddr) String() string {
	return fmt.Sprintf("%d:%d", d.Client, d.Port)
}

// validate keeps an address inside the single byte ALSA carries it in, which is what
// makes narrowing it later safe.
func (d SeqAddr) validate() error {
	if d.Client < 0 || d.Client > maxSeqAddress || d.Port < 0 || d.Port > maxSeqAddress {
		return fmt.Errorf("address %d:%d out of range (0:%d)", d.Client, d.Port, maxSeqAddress)
	}
	return nil
}
