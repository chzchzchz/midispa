package alsa

/*
#cgo linux LDFLAGS: -lasound
#include <assert.h>
#include <stddef.h>
#include <stdlib.h>
#include <alsa/asoundlib.h>

struct Midi_open_result {
	struct Midi* midi;
	int  error;
};

struct Midi {
	snd_rawmidi_t *in;
	snd_rawmidi_t *out;
};

typedef struct Midi* MidiPtr;

// Midi_open opens a MIDI connection to the specified device.
struct Midi_open_result Midi_open(const char *name) {
	struct Midi* midi = malloc(sizeof(struct Midi));
	int rc = snd_rawmidi_open(&midi->in, &midi->out, name, SND_RAWMIDI_SYNC);
	if (rc != 0) {
		return (struct Midi_open_result){ .midi = NULL, .error = rc };
	}
	return (struct Midi_open_result){.midi = midi, .error = 0 };
}

// Midi_close closes a MIDI connection.
int Midi_close(struct Midi* midi) {
	int inrc = snd_rawmidi_close(midi->in);
	int outrc = snd_rawmidi_close(midi->out);
	free(midi);
	if (inrc != 0) {
		return inrc;
	}
	return outrc;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func openBy(f func(d *Device) bool) (*Device, error) {
	devs, err := Devices()
	if err != nil {
		return nil, err
	}
	for _, dev := range devs {
		if f(dev) {
			if err = dev.Open(); err != nil {
				return nil, err
			}
			return dev, nil
		}
	}
	return nil, nil
}

func OpenDeviceById(id string) (*Device, error) {
	return openBy(func(d *Device) bool { return d.ID == id })
}

func OpenDeviceByName(name string) (d *Device, err error) {
	return openBy(func(d *Device) bool { return d.Name == name })
}

type Device struct {
	ID        string
	Name      string
	QueueSize int
	Type      DeviceType

	conn C.MidiPtr
}

// Open opens a MIDI device.
func (d *Device) Open() error {
	id := C.CString(d.ID)
	result := C.Midi_open(id)
	defer C.free(unsafe.Pointer(id))
	if result.error != 0 {
		return fmt.Errorf("error opening device %d", result.error)
	}
	d.conn = result.midi
	return nil
}

// Close closes the MIDI connection.
func (d *Device) Close() error {
	_, err := C.Midi_close(d.conn)
	return err
}

// Read reads data from a MIDI device.
func (d *Device) Read(buf []byte) (int, error) {
	cbuf := make([]C.char, len(buf))
	n, err := C.snd_rawmidi_read(d.conn.in, unsafe.Pointer(&cbuf[0]), C.size_t(len(buf)))
	for i := C.ssize_t(0); i < n; i++ {
		buf[i] = byte(cbuf[i])
	}
	return int(n), err
}

// Write writes data to a MIDI device.
func (d *Device) Write(buf []byte) (int, error) {
	cs := C.CString(string(buf))
	n, err := C.snd_rawmidi_write(d.conn.out, unsafe.Pointer(cs), C.size_t(len(buf)))
	C.free(unsafe.Pointer(cs))
	return int(n), err
}

// Devices returns a list of devices.
func Devices() ([]*Device, error) {
	card := C.int(-1)
	if rc := C.snd_card_next(&card); rc != 0 {
		return nil, alsaMidiError(rc)
	}
	devices := []*Device{}
	for card >= 0 {
		cardDevices, err := getCardDevices(card)
		if err != nil {
			return nil, err
		}
		devices = append(devices, cardDevices...)
		if rc := C.snd_card_next(&card); rc != 0 {
			return nil, alsaMidiError(rc)
		}
	}
	return devices, nil
}

func getCardDevices(card C.int) ([]*Device, error) {
	var ctl *C.snd_ctl_t
	name := C.CString(fmt.Sprintf("hw:%d", card))
	defer C.free(unsafe.Pointer(name))

	if rc := C.snd_ctl_open(&ctl, name, 0); rc != 0 {
		return nil, alsaMidiError(rc)
	}
	defer C.snd_ctl_close(ctl)

	cardDevices := []*Device{}
	device := C.int(-1)
	if rc := C.snd_ctl_rawmidi_next_device(ctl, &device); rc != 0 {
		return nil, alsaMidiError(rc)
	}
	for device >= 0 {
		deviceDevices, err := getDeviceDevices(ctl, card, C.uint(device))
		if err != nil {
			return nil, err
		}
		cardDevices = append(cardDevices, deviceDevices...)
		if rc := C.snd_ctl_rawmidi_next_device(ctl, &device); rc != 0 {
			return nil, alsaMidiError(rc)
		}
	}
	return cardDevices, nil
}

func getDeviceDevices(ctl *C.snd_ctl_t, card C.int, device C.uint) ([]*Device, error) {
	var info *C.snd_rawmidi_info_t
	C.snd_rawmidi_info_malloc(&info)
	defer C.snd_rawmidi_info_free(info)
	C.snd_rawmidi_info_set_device(info, device)

	// Get inputs.
	C.snd_rawmidi_info_set_stream(info, C.SND_RAWMIDI_STREAM_INPUT)
	if rc := C.snd_ctl_rawmidi_info(ctl, info); rc != 0 {
		return nil, alsaMidiError(rc)
	}
	subsIn := C.snd_rawmidi_info_get_subdevices_count(info)

	// Get outputs.
	C.snd_rawmidi_info_set_stream(info, C.SND_RAWMIDI_STREAM_OUTPUT)
	if rc := C.snd_ctl_rawmidi_info(ctl, info); rc != 0 {
		return nil, alsaMidiError(rc)
	}
	subsOut := C.snd_rawmidi_info_get_subdevices_count(info)

	// List subdevices.
	var subs C.uint
	if subsIn > subsOut {
		subs = subsIn
	} else {
		subs = subsOut
	}
	devices := []*Device{}
	for sub := C.uint(0); sub < subs; sub++ {
		subDevice, err := getSubdevice(ctl, info, card, device, sub, subsIn, subsOut)
		if err != nil {
			return nil, err
		}
		devices = append(devices, subDevice)
	}
	return devices, nil
}

type DeviceType int

const (
	DeviceInput DeviceType = iota
	DeviceOutput
	DeviceDuplex
)

func getSubdevice(ctl *C.snd_ctl_t, info *C.snd_rawmidi_info_t, card C.int, device, sub, subsIn, subsOut C.uint) (*Device, error) {
	if sub < subsIn {
		C.snd_rawmidi_info_set_stream(info, C.SND_RAWMIDI_STREAM_INPUT)
	} else {
		C.snd_rawmidi_info_set_stream(info, C.SND_RAWMIDI_STREAM_OUTPUT)
	}
	C.snd_rawmidi_info_set_subdevice(info, sub)
	if rc := C.snd_ctl_rawmidi_info(ctl, info); rc != 0 {
		return nil, alsaMidiError(rc)
	}

	var dt DeviceType
	if sub < subsIn && sub >= subsOut {
		dt = DeviceInput
	} else if sub >= subsIn && sub < subsOut {
		dt = DeviceOutput
	} else {
		dt = DeviceDuplex
	}

	subName := C.GoString(C.snd_rawmidi_info_get_subdevice_name(info))
	if sub == 0 && len(subName) > 0 && subName[0] == 0 {
		return &Device{
			ID:   fmt.Sprintf("hw:%d,%d", card, device),
			Name: C.GoString(C.snd_rawmidi_info_get_name(info)),
			Type: dt,
		}, nil
	}
	return &Device{
		ID:   fmt.Sprintf("hw:%d,%d,%d", card, device, sub),
		Name: subName,
		Type: dt,
	}, nil
}

func alsaMidiError(code C.int) error {
	if code == C.int(0) {
		return nil
	}
	return fmt.Errorf("%s", C.GoString(C.snd_strerror(code)))
}
