package main

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

type AlsaSeq struct {
	seq *C.snd_seq_t
}

func (a *AlsaSeq) Close() {
	C.snd_seq_close(a.seq)
}

func snderr2error(err C.int) error {
	if err >= 0 {
		return nil
	}
	return fmt.Errorf("%s", C.snd_strerror(err))
}

func OpenAlsaSeq(clientName string) (a *AlsaSeq, err error) {
	a = &AlsaSeq{}

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
		C.SND_SEQ_PORT_CAP_WRITE|
			C.SND_SEQ_PORT_CAP_SUBS_WRITE,
		C.SND_SEQ_PORT_TYPE_MIDI_GENERIC|
			C.SND_SEQ_PORT_TYPE_APPLICATION); err < 0 {
		return nil, snderr2error(err)
	}
	return a, nil
}

func (a *AlsaSeq) OpenPort(client, port int) error {
	return snderr2error(C.snd_seq_connect_from(a.seq, 0, C.int(client), C.int(port)))
}

func (a *AlsaSeq) OpenPortName(portName string) error {
	devs, err := a.Devices()
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if dev.PortName == portName {
			return a.OpenPort(dev.Client, dev.Port)
		}
	}
	return io.EOF
}

func (a *AlsaSeq) Read() ([]byte, error) {
	var event *C.snd_seq_event_t
	for {
		if err := C.snd_seq_event_input(a.seq, &event); err < 0 {
			return nil, snderr2error(err)
		}
		switch event._type {
		case C.SND_SEQ_EVENT_SYSEX:
			ext := (*C.snd_seq_ev_ext_t)(unsafe.Pointer(&event.data))
			data := C.snd_seq_ev_ext_data(ext)
			return C.GoBytes(unsafe.Pointer(data), C.int(ext.len)), nil
		}
	}
	return nil, nil
}

type Device struct {
	Client     int
	Port       int
	ClientName string
	PortName   string
}

func (a *AlsaSeq) Devices() (ret []Device, err error) {
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
			dev := Device{
				Client:     int(C.snd_seq_port_info_get_client(pinfo)),
				Port:       int(C.snd_seq_port_info_get_port(pinfo)),
				ClientName: C.GoString(C.snd_seq_client_info_get_name(cinfo)),
				PortName:   C.GoString(C.snd_seq_port_info_get_name(pinfo)),
			}
			ret = append(ret, dev)
		}
	}
	return ret, nil
}

func (d *Device) PortString() string {
	return fmt.Sprintf("%3d:%3d", d.Port, d.Client)
}
