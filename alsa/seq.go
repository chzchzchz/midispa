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
	"unsafe"

	"github.com/chzchzchz/midispa/midi"
)

var errExpectedSysEx = errors.New("expected sysex")

const (
	EvPortSubscribed   = 0
	EvPortUnsubscribed = 1
)

type Seq struct {
	seq *C.snd_seq_t
	SeqAddr
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

func (a *Seq) Close() error {
	if err := C.snd_seq_close(a.seq); err != 0 {
		return snderr2error(err)
	}
	return nil

}

func (ev *SeqEvent) IsControl() bool {
	return ev.Data[0]&0x80 == 0
}

type seqWriter struct {
	seq *Seq
	dst SeqAddr
}

func (a *seqWriter) Write(data []byte) (int, error) {
	return len(data), a.seq.Write(SeqEvent{a.dst, data})
}

func (a *Seq) NewWriter(sa SeqAddr) io.Writer { return &seqWriter{a, sa} }

func snderr2error(err C.int) error {
	if err >= 0 {
		return nil
	}
	return fmt.Errorf("%s", C.GoString(C.snd_strerror(err)))
}

func OpenSeq(clientName string) (a *Seq, err error) {
	a = &Seq{}

	seqname := C.CString("default")
	defer C.free(unsafe.Pointer(seqname))
	if err := C.snd_seq_open(&a.seq, seqname, C.SND_SEQ_OPEN_DUPLEX, 0); err < 0 {
		return nil, snderr2error(err)
	}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()
	cname := C.CString(clientName)
	defer C.free(unsafe.Pointer(cname))
	if err := C.snd_seq_set_client_name(a.seq, cname); err < 0 {
		return nil, snderr2error(err)
	}
	c, err := C.snd_seq_client_id(a.seq)
	if err != nil {
		return nil, err
	}
	a.Client = int(c)
	if err = a.CreatePort(clientName); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Seq) CreatePort(name string) error {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	if err := C.snd_seq_create_simple_port(a.seq, cname,
		C.SND_SEQ_PORT_CAP_DUPLEX|
			C.SND_SEQ_PORT_CAP_READ|
			C.SND_SEQ_PORT_CAP_SUBS_READ|
			C.SND_SEQ_PORT_CAP_WRITE|
			C.SND_SEQ_PORT_CAP_SUBS_WRITE,
		C.SND_SEQ_PORT_TYPE_MIDI_GENERIC|
			C.SND_SEQ_PORT_TYPE_PORT|
			C.SND_SEQ_PORT_TYPE_APPLICATION); err < 0 {
		return snderr2error(err)
	}
	return nil
}

func (a *Seq) OpenPort(client, port int) error {
	return snderr2error(C.snd_seq_connect_from(a.seq, 0, C.int(client), C.int(port)))
}

func (a *Seq) OpenPortRead(sa SeqAddr) error {
	return snderr2error(C.snd_seq_connect_from(a.seq, 0, C.int(sa.Client), C.int(sa.Port)))
}

func (a *Seq) OpenPortWrite(sa SeqAddr) error {
	return snderr2error(C.snd_seq_connect_to(a.seq, 0, C.int(sa.Client), C.int(sa.Port)))
}

func (a *Seq) ClosePortWrite(sa SeqAddr) error {
	return snderr2error(C.snd_seq_disconnect_to(a.seq, 0, C.int(sa.Client), C.int(sa.Port)))
}

func (a *Seq) ClosePortRead(sa SeqAddr) error {
	return snderr2error(C.snd_seq_disconnect_from(a.seq, 0, C.int(sa.Client), C.int(sa.Port)))
}

func (a *Seq) OpenPortName(portName string) error {
	sa, err := a.PortAddress(portName)
	if err != nil {
		return err
	}
	return a.OpenPort(sa.Client, sa.Port)
}

func (a *Seq) PortAddress(portName string) (sa SeqAddr, err error) {
	if n, _ := fmt.Sscanf(portName, "%d:%d", &sa.Client, &sa.Port); n == 2 {
		return sa, nil
	}
	devs, err := a.Devices()
	if err != nil {
		return SeqAddr{-1, -1}, err
	}
	for _, dev := range devs {
		if dev.PortName == portName {
			return dev.SeqAddr, nil
		}
	}
	return SeqAddr{-1, -1}, fmt.Errorf("port %q not found", portName)
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
	return a.WritePort(ev, 0)
}

func (a *Seq) WritePort(ev SeqEvent, port int) error {
	if len(ev.Data) == 0 {
		return nil
	}
	var event C.snd_seq_event_t
	src := SeqAddr{a.SeqAddr.Client, port}
	event.source.client, event.source.port = src.CAddrValues()
	event.dest.client, event.dest.port = ev.CAddrValues()
	event.queue = C.SND_SEQ_QUEUE_DIRECT
	// event.dest.client, event.dest.port = C.SND_SEQ_ADDRESS_SUBSCRIBERS, C.SND_SEQ_ADDRESS_UNKNOWN
	switch midi.Message(ev.Data[0]) {
	case midi.SysEx:
		event._type = C.SND_SEQ_EVENT_SYSEX
		event.flags = C.SND_SEQ_EVENT_LENGTH_VARIABLE
		ext := (*C.snd_seq_ev_ext_t)(unsafe.Pointer(&event.data))
		ext.len = C.uint(len(ev.Data))
		C.snd_seq_ev_ext_data_set(ext, (*C.uchar)(&ev.Data[0]))
	case midi.CC:
		if len(ev.Data) != 3 {
			panic("bad length")
		}
		event._type = C.SND_SEQ_EVENT_CONTROLLER
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.param = C.uint(ev.Data[1])
		ctrl.value = C.int(ev.Data[2])
	case midi.NoteOff:
		if len(ev.Data) != 3 {
			panic("bad length")
		}
		event._type = C.SND_SEQ_EVENT_NOTEOFF
		ctrl := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.note = C.uchar(ev.Data[1])
		ctrl.velocity = C.uchar(ev.Data[2])
	case midi.NoteOn:
		if len(ev.Data) != 3 {
			panic("bad length")
		}
		event._type = C.SND_SEQ_EVENT_NOTEON
		ctrl := (*C.snd_seq_ev_note_t)(unsafe.Pointer(&event.data))
		ctrl.channel = C.uchar(midi.Channel(ev.Data[0]))
		ctrl.note = C.uchar(ev.Data[1])
		ctrl.velocity = C.uchar(ev.Data[2])
	case midi.Start:
		if len(ev.Data) != 1 {
			panic("bad size for START")
		}
		event._type = C.SND_SEQ_EVENT_START
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Continue:
		if len(ev.Data) != 1 {
			panic("bad size for CONTINUE")
		}
		event._type = C.SND_SEQ_EVENT_CONTINUE
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Stop:
		if len(ev.Data) != 1 {
			panic("bad size for STOP")
		}
		event._type = C.SND_SEQ_EVENT_STOP
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Clock:
		if len(ev.Data) != 1 {
			panic("bad size for CLOCK")
		}
		event._type = C.SND_SEQ_EVENT_CLOCK
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.Pgm:
		if len(ev.Data) != 2 {
			panic("bad size for PGM")
		}
		event._type = C.SND_SEQ_EVENT_PGMCHANGE
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.SongPosition:
		if len(ev.Data) != 3 {
			panic("bad size for SONGPOS")
		}
		event._type = C.SND_SEQ_EVENT_SONGPOS
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	case midi.SongSelect:
		if len(ev.Data) != 2 {
			panic("bad size for SONGSEL")
		}
		event._type = C.SND_SEQ_EVENT_SONGSEL
		qc := (*C.snd_seq_ev_queue_control_t)(unsafe.Pointer(&event.data))
		qc.queue = C.SND_SEQ_QUEUE_DIRECT
	default:
		panic("unknown midi data: " + fmt.Sprintf("%+v", event))
	}
	return snderr2error(C.snd_seq_event_output_direct(a.seq, &event))
}

type SeqDevice struct {
	SeqAddr
	ClientName string
	PortName   string
}

func (a *Seq) Devices() (ret []SeqDevice, err error) {
	var cinfo *C.snd_seq_client_info_t
	var pinfo *C.snd_seq_port_info_t

	C.snd_seq_client_info_malloc(&cinfo)
	defer C.snd_seq_client_info_free(cinfo)

	C.snd_seq_port_info_malloc(&pinfo)
	defer C.snd_seq_port_info_free(pinfo)

	C.snd_seq_client_info_set_client(cinfo, -1)
	for C.snd_seq_query_next_client(a.seq, cinfo) >= 0 {
		client := C.snd_seq_client_info_get_client(cinfo)
		C.snd_seq_port_info_set_client(pinfo, client)
		C.snd_seq_port_info_set_port(pinfo, -1)
		for C.snd_seq_query_next_port(a.seq, pinfo) >= 0 {
			mask := C.SND_SEQ_PORT_CAP_READ | C.SND_SEQ_PORT_CAP_SUBS_READ
			if int(C.snd_seq_port_info_get_capability(pinfo))&mask != mask {
				continue
			}
			dev := SeqDevice{
				SeqAddr: SeqAddr{
					Client: int(C.snd_seq_port_info_get_client(pinfo)),
					Port:   int(C.snd_seq_port_info_get_port(pinfo)),
				},
				ClientName: C.GoString(C.snd_seq_client_info_get_name(cinfo)),
				PortName:   C.GoString(C.snd_seq_port_info_get_name(pinfo)),
			}
			ret = append(ret, dev)
		}
	}
	return ret, nil
}

func (d *SeqAddr) String() string {
	return fmt.Sprintf("%d:%d", d.Client, d.Port)
}

func (d *SeqAddr) CAddrValues() (C.uchar, C.uchar) {
	return C.uchar(d.Client), C.uchar(d.Port)
}
