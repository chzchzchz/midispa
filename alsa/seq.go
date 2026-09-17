package alsa

/*
#cgo linux LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stddef.h>
#include <stdlib.h>

uint8_t* snd_seq_ev_ext_data(const snd_seq_ev_ext_t* ext) { return ext->ptr; }
void snd_seq_ev_ext_data_set(snd_seq_ev_ext_t* ext, uint8_t* v) { ext->ptr = v; }
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/chzchzchz/midispa/midi"
)

var errExpectedSysEx = errors.New("expected sysex")

type PortCaps int

const (
	PortCapRead      PortCaps = C.SND_SEQ_PORT_CAP_READ
	PortCapWrite     PortCaps = C.SND_SEQ_PORT_CAP_WRITE
	PortCapSubsRead  PortCaps = C.SND_SEQ_PORT_CAP_SUBS_READ
	PortCapSubsWrite PortCaps = C.SND_SEQ_PORT_CAP_SUBS_WRITE
	PortCapDuplex    PortCaps = C.SND_SEQ_PORT_CAP_DUPLEX
)

func (c PortCaps) Has(want PortCaps) bool { return c&want == want }

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

const (
	midiDataMax  = 127
	midiDataBits = 7
	pitchCenter  = 8192
	pitchMax     = 16383

	EvPortSubscribed   = 0
	EvPortUnsubscribed = 1
)

type outputEvent = C.snd_seq_event_t

type Seq struct {
	seq   *C.snd_seq_t
	ports map[int]struct{}
	SeqAddr
	output func(*outputEvent) error
}

type SeqAddr struct {
	Client int
	Port   int
}

var SubsSeqAddr = SeqAddr{C.SND_SEQ_ADDRESS_SUBSCRIBERS, 0}

type SeqEvent struct {
	SeqAddr
	Data []byte
}

func MakeEvent(data []byte) SeqEvent {
	return SeqEvent{SeqAddr: SubsSeqAddr, Data: data}
}

func (a *Seq) Close() error {
	if a.seq == nil {
		return nil
	}
	err := snderr2error(C.snd_seq_close(a.seq))
	a.seq = nil
	a.output = nil
	a.ports = nil
	a.Port = -1
	return err
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

func snderr2error(err C.int) error {
	if err >= 0 {
		return nil
	}
	return fmt.Errorf("%s", C.GoString(C.snd_strerror(err)))
}

func OpenSeq(clientName string) (a *Seq, err error) {
	a = &Seq{SeqAddr: SeqAddr{Port: -1}, ports: make(map[int]struct{})}

	seqname := C.CString("default")
	defer C.free(unsafe.Pointer(seqname))
	if err := C.snd_seq_open(&a.seq, seqname, C.SND_SEQ_OPEN_DUPLEX, 0); err < 0 {
		return nil, snderr2error(err)
	}
	opened := a
	defer func() {
		if err != nil {
			opened.Close()
		}
	}()
	cname := C.CString(clientName)
	defer C.free(unsafe.Pointer(cname))
	if err := C.snd_seq_set_client_name(a.seq, cname); err < 0 {
		return nil, snderr2error(err)
	}
	c := C.snd_seq_client_id(a.seq)
	if c < 0 {
		return nil, snderr2error(c)
	}
	a.Client = int(c)
	a.output = a.outputDirect
	if err = a.CreatePort(clientName); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Seq) CreatePort(name string) error {
	_, err := a.CreatePortAddr(name)
	return err
}

func (a *Seq) CreatePortAddr(name string) (SeqAddr, error) {
	return a.createPortAddrCaps(name, PortCapDuplex|PortCapRead|PortCapSubsRead|PortCapWrite|PortCapSubsWrite)
}

func (a *Seq) createPortAddrCaps(name string, caps PortCaps) (SeqAddr, error) {
	if a.seq == nil {
		return SeqAddr{}, errors.New("sequencer is closed")
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	port := C.snd_seq_create_simple_port(a.seq, cname,
		C.uint(caps),
		C.SND_SEQ_PORT_TYPE_MIDI_GENERIC|
			C.SND_SEQ_PORT_TYPE_PORT|
			C.SND_SEQ_PORT_TYPE_APPLICATION)
	if port < 0 {
		return SeqAddr{}, snderr2error(port)
	}
	addr := SeqAddr{a.Client, int(port)}
	a.ports[addr.Port] = struct{}{}
	if a.Port < 0 {
		a.SeqAddr = addr
	}
	return addr, nil
}

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

func (a *Seq) DeletePort(addr SeqAddr) error {
	if err := a.localPort(addr); err != nil {
		return err
	}
	if err := snderr2error(C.snd_seq_delete_simple_port(a.seq, C.int(addr.Port))); err != nil {
		return err
	}
	delete(a.ports, addr.Port)
	if a.Port == addr.Port {
		a.Port = -1
	}
	return nil
}

func (a *Seq) OpenPort(client, port int) error {
	return a.OpenPortRead(SeqAddr{client, port})
}

func (a *Seq) OpenPortRead(sa SeqAddr) error {
	return a.OpenPortReadAt(a.SeqAddr, sa)
}

func (a *Seq) OpenPortReadAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := a.checkPortCaps(remote, PortCapRead|PortCapSubsRead); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_connect_from(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
}

func (a *Seq) OpenPortWrite(sa SeqAddr) error {
	return a.OpenPortWriteAt(a.SeqAddr, sa)
}

func (a *Seq) OpenPortWriteAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := a.checkPortCaps(remote, PortCapWrite|PortCapSubsWrite); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_connect_to(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
}

func (a *Seq) checkPortCaps(sa SeqAddr, required PortCaps) error {
	if err := sa.validate(); err != nil {
		return err
	}
	var info *C.snd_seq_port_info_t
	if err := C.snd_seq_port_info_malloc(&info); err < 0 {
		return snderr2error(err)
	}
	defer C.snd_seq_port_info_free(info)
	if err := C.snd_seq_get_any_port_info(a.seq, C.int(sa.Client), C.int(sa.Port), info); err < 0 {
		return snderr2error(err)
	}
	caps := PortCaps(C.snd_seq_port_info_get_capability(info))
	if !caps.Has(required) {
		return fmt.Errorf("port %d:%d capabilities %#x do not support required %#x", sa.Client, sa.Port, caps, required)
	}
	return nil
}

func (a *Seq) ClosePortWrite(sa SeqAddr) error {
	return a.ClosePortWriteAt(a.SeqAddr, sa)
}

func (a *Seq) ClosePortWriteAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := remote.validate(); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_disconnect_to(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
}

func (a *Seq) ClosePortRead(sa SeqAddr) error {
	return a.ClosePortReadAt(a.SeqAddr, sa)
}

func (a *Seq) ClosePortReadAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := remote.validate(); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_disconnect_from(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
}

func (a *Seq) OpenPortName(portName string) error {
	return a.OpenPortNameRead(portName)
}

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

func (a *Seq) MayRead() bool {
	return C.snd_seq_event_input_pending(a.seq, 1) > 0
}

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

func (a *Seq) Read() (ret SeqEvent, err error) {
	var event *C.snd_seq_event_t
	for {
		if err := C.snd_seq_event_input(a.seq, &event); err < 0 {
			return ret, snderr2error(err)
		}
		ret.Client, ret.Port = int(event.source.client), int(event.source.port)
		switch event._type {
		case C.SND_SEQ_EVENT_SYSEX:
			ext := (*C.snd_seq_ev_ext_t)(unsafe.Pointer(&event.data))
			data := C.snd_seq_ev_ext_data(ext)
			ret.Data = C.GoBytes(unsafe.Pointer(data), C.int(ext.len))
		case C.SND_SEQ_EVENT_SONGSEL:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{midi.SongSelect, byte(ctrl.value)}
		case C.SND_SEQ_EVENT_CONTROLLER:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				midi.MakeCC(int(ctrl.channel)),
				byte(ctrl.param),
				byte(ctrl.value),
			}
		case C.SND_SEQ_EVENT_PGMCHANGE:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				midi.MakePgm(int(ctrl.channel)),
				byte(ctrl.value)}
		case C.SND_SEQ_EVENT_KEYPRESS:
			note := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				midi.KeyAftertouch | byte(note.channel),
				byte(note.note),
				byte(note.velocity)}
		case C.SND_SEQ_EVENT_CHANPRESS:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			if ctrl.value < 0 || ctrl.value > midiDataMax {
				return ret, fmt.Errorf("invalid channel pressure: %d", ctrl.value)
			}
			ret.Data = []byte{midi.ChannelAftertouch | byte(ctrl.channel), byte(ctrl.value)}
		case C.SND_SEQ_EVENT_PITCHBEND:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			if ctrl.value < -pitchCenter || ctrl.value > pitchMax-pitchCenter {
				return ret, fmt.Errorf("invalid pitch bend: %d", ctrl.value)
			}
			value := int(ctrl.value) + pitchCenter
			ret.Data = []byte{
				midi.Pitch | byte(ctrl.channel),
				byte(value & midiDataMax),
				byte(value >> midiDataBits)}
		case C.SND_SEQ_EVENT_NOTEON:
			note := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				midi.MakeNoteOn(int(note.channel)),
				byte(note.note),
				byte(note.velocity)}
			if note.velocity == 0 {
				ret.Data[0] = midi.MakeNoteOff(int(note.channel))
			}
		case C.SND_SEQ_EVENT_NOTEOFF:
			note := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				midi.MakeNoteOff(int(note.channel)),
				byte(note.note),
				byte(note.velocity)}
		case C.SND_SEQ_EVENT_CLOCK:
			ret.Data = []byte{midi.Clock}
		case C.SND_SEQ_EVENT_TICK:
			ret.Data = []byte{midi.Tick}
		case C.SND_SEQ_EVENT_START:
			ret.Data = []byte{midi.Start}
		case C.SND_SEQ_EVENT_CONTINUE:
			ret.Data = []byte{midi.Continue}
		case C.SND_SEQ_EVENT_STOP:
			ret.Data = []byte{midi.Stop}
		case C.SND_SEQ_EVENT_PORT_SUBSCRIBED:
			c := (*C.snd_seq_connect_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				EvPortSubscribed,
				byte(c.sender.client), byte(c.sender.port),
				byte(c.dest.client), byte(c.dest.port),
			}
		case C.SND_SEQ_EVENT_PORT_UNSUBSCRIBED:
			c := (*C.snd_seq_connect_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				EvPortUnsubscribed,
				byte(c.sender.client), byte(c.sender.port),
				byte(c.dest.client), byte(c.dest.port),
			}
		default:
			continue
		}
		return ret, nil
	}
}

func (a *Seq) Write(ev SeqEvent) error {
	return a.WritePort(ev, a.Port)
}

func (a *Seq) outputDirect(event *outputEvent) error {
	return snderr2error(C.snd_seq_event_output_direct(a.seq, event))
}

func (a *Seq) WritePort(ev SeqEvent, port int) error {
	if err := ev.SeqAddr.validate(); err != nil {
		return err
	}
	if err := a.localPort(SeqAddr{a.Client, port}); err != nil {
		return err
	}
	event, err := encodeEvent(ev, SeqAddr{a.Client, port})
	if err != nil || len(ev.Data) == 0 {
		return err
	}
	err = a.output(event)
	runtime.KeepAlive(ev.Data)
	return err
}

func encodeEvent(ev SeqEvent, src SeqAddr) (event *outputEvent, err error) {
	if err = validateMessage(ev.Data); err != nil || len(ev.Data) == 0 {
		return nil, err
	}
	event = new(outputEvent)
	event.source.client, event.source.port = src.CAddrValues()
	event.dest.client, event.dest.port = ev.CAddrValues()
	event.queue = C.SND_SEQ_QUEUE_DIRECT
	switch midi.Message(ev.Data[0]) {
	case midi.SysEx:
		event._type = C.SND_SEQ_EVENT_SYSEX
		event.flags = C.SND_SEQ_EVENT_LENGTH_VARIABLE
		ext := (*C.snd_seq_ev_ext_t)(unsafe.Pointer(&event.data))
		ext.len = C.uint(len(ev.Data))
		C.snd_seq_ev_ext_data_set(ext, (*C.uchar)(&ev.Data[0]))
	case midi.CC:
		event._type = C.SND_SEQ_EVENT_CONTROLLER
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.param = C.uint(ev.Data[1])
		ctrl.value = C.int(ev.Data[2])
	case midi.NoteOff:
		event._type = C.SND_SEQ_EVENT_NOTEOFF
		ctrl := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.note = C.uchar(ev.Data[1])
		ctrl.velocity = C.uchar(ev.Data[2])
	case midi.NoteOn:
		event._type = C.SND_SEQ_EVENT_NOTEON
		ctrl := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.note = C.uchar(ev.Data[1])
		ctrl.velocity = C.uchar(ev.Data[2])
	case midi.Start:
		event._type = C.SND_SEQ_EVENT_START
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Continue:
		event._type = C.SND_SEQ_EVENT_CONTINUE
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Stop:
		event._type = C.SND_SEQ_EVENT_STOP
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Clock:
		event._type = C.SND_SEQ_EVENT_CLOCK
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Pgm:
		event._type = C.SND_SEQ_EVENT_PGMCHANGE
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.value = C.int(ev.Data[1])
	case midi.SongPosition:
		event._type = C.SND_SEQ_EVENT_SONGPOS
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.value = C.int(ev.Data[1]) | C.int(ev.Data[2])<<midiDataBits
	case midi.SongSelect:
		event._type = C.SND_SEQ_EVENT_SONGSEL
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.value = C.int(ev.Data[1])
	case midi.ChannelAftertouch:
		event._type = C.SND_SEQ_EVENT_CHANPRESS
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.value = C.int(ev.Data[1])
	case midi.Pitch:
		event._type = C.SND_SEQ_EVENT_PITCHBEND
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.value = C.int(int(ev.Data[1])|int(ev.Data[2])<<midiDataBits) - pitchCenter
	case midi.KeyAftertouch:
		event._type = C.SND_SEQ_EVENT_KEYPRESS
		ctrl := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.note = C.uchar(ev.Data[1])
		ctrl.velocity = C.uchar(ev.Data[2])
	default:
		return event, &UnsupportedMessageError{ev.Data[0]}
	}
	return event, nil
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

func (a *Seq) devices() (ret []SeqDevice, err error) {
	var cinfo *C.snd_seq_client_info_t
	var pinfo *C.snd_seq_port_info_t

	if err := C.snd_seq_client_info_malloc(&cinfo); err < 0 {
		return nil, snderr2error(err)
	}
	defer C.snd_seq_client_info_free(cinfo)

	if err := C.snd_seq_port_info_malloc(&pinfo); err < 0 {
		return nil, snderr2error(err)
	}
	defer C.snd_seq_port_info_free(pinfo)

	C.snd_seq_client_info_set_client(cinfo, -1)
	for C.snd_seq_query_next_client(a.seq, cinfo) >= 0 {
		client := C.snd_seq_client_info_get_client(cinfo)
		C.snd_seq_port_info_set_client(pinfo, client)
		C.snd_seq_port_info_set_port(pinfo, -1)
		for C.snd_seq_query_next_port(a.seq, pinfo) >= 0 {
			caps := PortCaps(C.snd_seq_port_info_get_capability(pinfo))
			dev := SeqDevice{
				SeqAddr: SeqAddr{
					Client: int(C.snd_seq_port_info_get_client(pinfo)),
					Port:   int(C.snd_seq_port_info_get_port(pinfo)),
				},
				ClientName: C.GoString(C.snd_seq_client_info_get_name(cinfo)),
				PortName:   C.GoString(C.snd_seq_port_info_get_name(pinfo)),
				Caps:       caps,
			}
			ret = append(ret, dev)
		}
	}
	return ret, nil
}

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

func (d SeqAddr) validate() error {
	if d.Client < 0 || d.Client > maxSeqAddress || d.Port < 0 || d.Port > maxSeqAddress {
		return fmt.Errorf("address %d:%d out of range (0:%d)", d.Client, d.Port, maxSeqAddress)
	}
	return nil
}

func (d *SeqAddr) CAddrValues() (C.uchar, C.uchar) {
	if err := d.validate(); err != nil {
		panic(err)
	}
	return C.uchar(d.Client), C.uchar(d.Port)
}
