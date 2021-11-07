package alsa

/*
#cgo linux LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stddef.h>
#include <stdlib.h>

uint8_t* snd_seq_ev_ext_data(const snd_seq_ev_ext_t* ext) { return ext->ptr; }
*/
import "C"

import (
	"fmt"
	"io"
	"unsafe"
)

type Seq struct {
	seq *C.snd_seq_t
	SeqAddr
}

type SeqAddr struct {
	Client int
	Port   int
}

type SeqEvent struct {
	SeqAddr
	Data []byte
}

func (a *Seq) Close() {
	C.snd_seq_close(a.seq)
}

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
	if err := C.snd_seq_create_simple_port(a.seq, cname,
		C.SND_SEQ_PORT_CAP_READ|
			C.SND_SEQ_PORT_CAP_SUBS_READ|
			C.SND_SEQ_PORT_CAP_WRITE|
			C.SND_SEQ_PORT_CAP_SUBS_WRITE,
		C.SND_SEQ_PORT_TYPE_MIDI_GENERIC|
			C.SND_SEQ_PORT_TYPE_APPLICATION); err < 0 {
		return nil, snderr2error(err)
	}
	c, err := C.snd_seq_client_id(a.seq)
	if err != nil {
		fmt.Println("oops no seq addr " + clientName)
		return nil, err
	}
	a.Client = int(c)
	return a, nil
}

func (a *Seq) OpenPort(client, port int) error {
	return snderr2error(C.snd_seq_connect_from(a.seq, 0, C.int(client), C.int(port)))
}

func (a *Seq) OpenPortWrite(sa SeqAddr) error {
	return snderr2error(C.snd_seq_connect_to(a.seq, 0, C.int(sa.Client), C.int(sa.Port)))
}

func (a *Seq) OpenPortName(portName string) error {
	sa, err := a.PortAddress(portName)
	if err != nil {
		return err
	}
	return a.OpenPort(sa.Client, sa.Port)
}

func (a *Seq) PortAddress(portName string) (SeqAddr, error) {
	devs, err := a.Devices()
	if err != nil {
		return SeqAddr{-1, -1}, err
	}
	for _, dev := range devs {
		if dev.PortName == portName {
			return dev.SeqAddr, nil
		}
	}
	return SeqAddr{-1, -1}, io.EOF
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
				0xB0 | byte(ctrl.channel),
				byte(ctrl.param),
				byte(ctrl.value),
			}
		case C.SND_SEQ_EVENT_PGMCHANGE:
			ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
			ret.Data = []byte{
				0xC0 | byte(ctrl.channel),
				byte(ctrl.value)}
		default:
			continue
		}
		return ret, nil
	}
	return ret, nil
}

func (a *Seq) Write(ev SeqEvent) error {
	if len(ev.Data) != 3 {
		panic("bad length")
	}
	if ev.Data[0]&0xf0 != 0xb0 {
		panic("not ctrl code")
	}
	var event C.snd_seq_event_t
	event.source.client, event.source.port = a.CAddrValues()
	event.dest.client, event.dest.port = ev.CAddrValues()
	//	event.dest.client, event.dest.port = C.SND_SEQ_ADDRESS_SUBSCRIBERS, C.SND_SEQ_ADDRESS_UNKNOWN
	event.queue = C.SND_SEQ_QUEUE_DIRECT
	ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
	ctrl.channel = C.uchar(ev.Data[0] & 0xf)
	ctrl.param = C.uint(ev.Data[1])
	ctrl.value = C.int(ev.Data[2])
	event._type = C.SND_SEQ_EVENT_CONTROLLER
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

func (d *SeqAddr) PortString() string {
	return fmt.Sprintf("%3d:%3d", d.Client, d.Port)
}

func (d *SeqAddr) CAddrValues() (C.uchar, C.uchar) {
	return C.uchar(d.Client), C.uchar(d.Port)
}
