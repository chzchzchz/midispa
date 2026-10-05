package alsa

// This file is the whole of the sequencer's contact with libasound: every call into C
// and every type that carries one. seq.go holds the policy those calls serve, so the two
// can be read separately.
//
// The split does not make the package build without cgo. Cgo is a property of the
// package rather than of a file, and Seq holds an *snd_seq_t either way, so moving this
// out of seq.go buys legibility and nothing else. A cgo-free alsa would need Seq behind
// an opaque handle, which is a different change.

/*
#cgo linux LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stddef.h>
#include <stdlib.h>

uint8_t* snd_seq_ev_ext_data(const snd_seq_ev_ext_t* ext) { return ext->ptr; }
void snd_seq_ev_ext_data_set(snd_seq_ev_ext_t* ext, uint8_t* v) { ext->ptr = v; }

// cgo cannot call a C macro, and the queue's entry points are macros. The wrappers below
// keep the flag arithmetic and the flush ordering in the header that owns them, so an
// upstream change arrives as a change to one line here rather than as a copy of a macro
// that quietly drifts from the original.

// The queue status struct is large and its allocation helpers are macros, so reading it is
// one wrapper rather than a hand-rolled block of cgo. Pending comes back in the same struct
// as the return code so that a caller crossing into cgo once does not pay for a pointer to a
// four-byte out-parameter on every submission.
typedef struct seq_queue_state {
	int rc;
	int pending;
} seq_queue_state_t;

seq_queue_state_t seq_queue_pending(snd_seq_t* seq, int q) {
	seq_queue_state_t state;
	snd_seq_queue_status_t* status;
	snd_seq_queue_status_alloca(&status);
	state.rc = snd_seq_get_queue_status(seq, q, status);
	state.pending = state.rc < 0 ? 0 : snd_seq_queue_status_get_events(status);
	return state;
}

// Stamp, submit and flush, in one crossing. Each cgo call costs more than the drain it
// would be hiding, and there is a correctness reason to keep them adjacent: a submission
// the kernel refused must not be followed by a flush, because the flush blocks until the
// queue takes what is already buffered and a full queue never will.
int seq_submit_stamped(snd_seq_t* seq, snd_seq_event_t* ev, unsigned char queue,
                       unsigned int sec, unsigned int nsec) {
	snd_seq_real_time_t rtime = { sec, nsec };
	snd_seq_ev_schedule_real(ev, queue, 1, &rtime);
	int rc = snd_seq_event_output(seq, ev);
	if (rc < 0)
		return rc;
	return snd_seq_drain_output(seq);
}

// Start or stop a queue, and flush it, for the same reason as seq_submit_stamped: the
// control event lands in the same buffer a scheduled one does, and a start that has not
// been flushed is a queue that is stopped.
int seq_control_queue(snd_seq_t* seq, int q, int type) {
	int rc = snd_seq_control_queue(seq, q, type, 0, NULL);
	if (rc < 0)
		return rc;
	return snd_seq_drain_output(seq);
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/chzchzchz/midispa/midi"
)

// The capability bits are ALSA's, so they are defined here rather than as bare numbers.
type PortCaps int

const (
	PortCapRead      PortCaps = C.SND_SEQ_PORT_CAP_READ
	PortCapWrite     PortCaps = C.SND_SEQ_PORT_CAP_WRITE
	PortCapSubsRead  PortCaps = C.SND_SEQ_PORT_CAP_SUBS_READ
	PortCapSubsWrite PortCaps = C.SND_SEQ_PORT_CAP_SUBS_WRITE
	PortCapDuplex    PortCaps = C.SND_SEQ_PORT_CAP_DUPLEX
)

func (c PortCaps) Has(want PortCaps) bool { return c&want == want }

// outputEvent is the libasound event struct. It stays unexported: it appears only in
// Seq.output, encodeEvent and decodeSeqEvent, and nothing outside this file needs to
// name it.
type outputEvent = C.snd_seq_event_t

// Seq holds the libasound client. The Go fields track which ports this sequencer owns
// and where its default one lives; ALSA owns the handle itself.
type Seq struct {
	seq   *C.snd_seq_t
	ports map[int]struct{}
	SeqAddr
	// output is indirected so a test can observe or redirect what the encoder produced
	// without a sequencer behind it.
	output func(*outputEvent) error
}

// SubsSeqAddr is ALSA's subscribers address: writing there fans an event out to every
// client that subscribed to the local port.
var SubsSeqAddr = SeqAddr{C.SND_SEQ_ADDRESS_SUBSCRIBERS, 0}

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

func (a *Seq) createPortAddrCaps(name string, caps PortCaps) (SeqAddr, error) {
	if a.seq == nil {
		return SeqAddr{}, ErrSeqClosed
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

func (a *Seq) OpenPortReadAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := a.checkPortCaps(remote, PortCapRead|PortCapSubsRead); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_connect_from(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
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

func (a *Seq) ClosePortWriteAt(local, remote SeqAddr) error {
	if err := a.localPort(local); err != nil {
		return err
	}
	if err := remote.validate(); err != nil {
		return err
	}
	return snderr2error(C.snd_seq_disconnect_to(a.seq, C.int(local.Port), C.int(remote.Client), C.int(remote.Port)))
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

func (a *Seq) MayRead() bool {
	return C.snd_seq_event_input_pending(a.seq, 1) > 0
}

func (a *Seq) Read() (ret SeqEvent, err error) {
	var event *C.snd_seq_event_t
	for {
		if err := C.snd_seq_event_input(a.seq, &event); err < 0 {
			return ret, snderr2error(err)
		}
		ret, err = decodeSeqEvent(event)
		if len(ret.Data) != 0 || err != nil {
			return ret, err
		}
	}
}

func (a *Seq) outputDirect(event *outputEvent) error {
	return snderr2error(C.snd_seq_event_output_direct(a.seq, event))
}

// Everything below is SeqQueue's contact with libasound. It lives here rather than in
// queue.go so that this file keeps the promise its own comment makes: every call into C
// is here, and the file beside it holds the policy those calls serve.

// isOpen reports whether the client is still usable. SeqQueue holds its Seq rather than
// the handle, so it cannot ask about a nil pointer of a type it does not name.
func (a *Seq) isOpen() bool { return a.seq != nil }

// allocNamedQueue creates a queue attached to the client. The name is what the queue is
// called in aconnect and aplay, so Reset allocates through here rather than through the
// unnamed form and keeps the queue recognisable after it is reallocated.
func (a *Seq) allocNamedQueue(name string) (int, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	queue := C.snd_seq_alloc_named_queue(a.seq, cname)
	if queue < 0 {
		return 0, snderr2error(queue)
	}
	return int(queue), nil
}

// freeQueue releases a queue this client owns. ALSA hands the identifier to the next
// allocation, which is why a caller that stops a queue has already flushed the stop by
// the time it gets here.
func (a *Seq) freeQueue(queue int) error {
	return snderr2error(C.snd_seq_free_queue(a.seq, C.int(queue)))
}

// startQueue and stopQueue are the two things snd_seq_control_queue does, named so that a
// call site says which it means. The flush is inside them rather than left to the caller
// because snd_seq_control_queue writes a control event into the client's output buffer,
// exactly as snd_seq_event_output does. Without the flush the queue does not start, the
// call still reports success, and every event scheduled afterwards is accepted by the
// kernel and never delivered. Nothing about that failure is visible from Go, which is why
// it cannot be left as a rule the caller has to remember.
func (a *Seq) startQueue(queue int) error {
	return a.controlQueue(queue, C.int(C.SND_SEQ_EVENT_START))
}

func (a *Seq) stopQueue(queue int) error {
	return a.controlQueue(queue, C.int(C.SND_SEQ_EVENT_STOP))
}

func (a *Seq) controlQueue(queue int, control C.int) error {
	return snderr2error(C.seq_control_queue(a.seq, C.int(queue), control))
}

// queuePending reports how many events the queue is holding, which is what the ceiling in
// queue.go is measured against. It is the most expensive thing a submission does and the
// submission itself is the second: both are one call into the kernel, and this one only
// asks a question. Measured on an FX-8350, roughly 600ns against the submission's 600ns
// and the encoder's 80ns, which is why nothing else in the path is worth trimming for a
// caller that sends tens of notes a second.
func (a *Seq) queuePending(queue int) (int, error) {
	state := C.seq_queue_pending(a.seq, C.int(queue))
	if state.rc < 0 {
		return 0, snderr2error(state.rc)
	}
	return int(state.pending), nil
}

// outputStamped submits an event for delivery after delay and then flushes it.
//
// The event is not routed through Seq.output: that field exists so a test can observe
// what the encoder produced, and it is bound to the immediate path, so an event reaching
// it would go out at once with its timestamp ignored.
//
// The delay is split here rather than converted by cgo because snd_seq_real_time_t holds
// unsigned fields, and the caller has already refused a negative one: an unchecked
// conversion would turn that refusal into a stamp decades away rather than into an error.
func (a *Seq) outputStamped(event *outputEvent, queue int, delay time.Duration) error {
	return snderr2error(C.seq_submit_stamped(a.seq, event, C.uchar(queue),
		C.uint(delay/time.Second), C.uint(delay%time.Second)))
}

// encodeForPort is what WritePort and SeqQueue share: it refuses an event or a port the
// sequencer will not take, and hands back the encoded event for that port. A nil event
// with no error is an event carrying no bytes, which both callers read as nothing to send
// rather than as a failure.
//
// It lives beside the encoder rather than in seq.go because the thing it returns is the
// libasound struct, and queue.go is not a file that may name one.
func (a *Seq) encodeForPort(ev SeqEvent, port int) (*outputEvent, error) {
	if err := ev.SeqAddr.validate(); err != nil {
		return nil, err
	}
	if err := a.localPort(SeqAddr{a.Client, port}); err != nil {
		return nil, err
	}
	event, err := encodeEvent(ev, SeqAddr{a.Client, port})
	if err != nil || len(ev.Data) == 0 {
		return nil, err
	}
	return event, nil
}

// WritePort validates and owns the output event. It lives beside the encoder because
// KeepAlive below is a cgo lifetime concern: encodeEvent stores a pointer into the Go
// byte slice in the event, and the slice has to outlive the call into libasound.
func (a *Seq) WritePort(ev SeqEvent, port int) error {
	event, err := a.encodeForPort(ev, port)
	if err != nil || event == nil {
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
		ctrl.value = C.int(ev.Data[1]) | C.int(ev.Data[2])<<midi.DataBits
	case midi.QuarterFrame:
		event._type = C.SND_SEQ_EVENT_QFRAME
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		ctrl.value = C.int(ev.Data[1])
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
		ctrl.value = C.int(int(ev.Data[1])|int(ev.Data[2])<<midi.DataBits) - midi.PitchCenter
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

func decodeSeqEvent(event *C.snd_seq_event_t) (ret SeqEvent, err error) {
	ret.Client, ret.Port = int(event.source.client), int(event.source.port)
	switch event._type {
	case C.SND_SEQ_EVENT_SYSEX:
		ext := (*C.snd_seq_ev_ext_t)(unsafe.Pointer(&event.data))
		data := C.snd_seq_ev_ext_data(ext)
		ret.Data = C.GoBytes(unsafe.Pointer(data), C.int(ext.len))
	case C.SND_SEQ_EVENT_SONGSEL:
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		if ctrl.value < 0 || ctrl.value > midi.DataMax {
			return ret, fmt.Errorf("%w: song select %d", ErrInvalidMessage, ctrl.value)
		}
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
			midi.MakeKeyAftertouch(int(note.channel)),
			byte(note.note),
			byte(note.velocity)}
	case C.SND_SEQ_EVENT_CHANPRESS:
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		if ctrl.value < 0 || ctrl.value > midi.DataMax {
			return ret, fmt.Errorf("invalid channel pressure: %d", ctrl.value)
		}
		ret.Data = []byte{midi.MakeChannelAftertouch(int(ctrl.channel)), byte(ctrl.value)}
	case C.SND_SEQ_EVENT_PITCHBEND:
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		if ctrl.value < -midi.PitchCenter || ctrl.value > midi.PitchMax-midi.PitchCenter {
			return ret, fmt.Errorf("invalid pitch bend: %d", ctrl.value)
		}
		low, high := midi.MakePitchBend(uint16(int(ctrl.value) + midi.PitchCenter))
		ret.Data = []byte{midi.MakePitch(int(ctrl.channel)), low, high}
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
	case C.SND_SEQ_EVENT_TICK, C.SND_SEQ_EVENT_TUNE_REQUEST,
		C.SND_SEQ_EVENT_SENSING, C.SND_SEQ_EVENT_RESET:
		return ret, fmt.Errorf("%w: type %d", ErrUnsupportedEvent, event._type)
	case C.SND_SEQ_EVENT_START:
		ret.Data = []byte{midi.Start}
	case C.SND_SEQ_EVENT_CONTINUE:
		ret.Data = []byte{midi.Continue}
	case C.SND_SEQ_EVENT_STOP:
		ret.Data = []byte{midi.Stop}
	case C.SND_SEQ_EVENT_SONGPOS:
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		if ctrl.value < 0 || ctrl.value > midi.SongPositionMax {
			return ret, fmt.Errorf("%w: song position %d", ErrInvalidMessage, ctrl.value)
		}
		ret.Data = []byte{midi.SongPosition, byte(ctrl.value & midi.DataMax), byte(ctrl.value >> midi.DataBits)}
	case C.SND_SEQ_EVENT_QFRAME:
		ctrl := (*C.snd_seq_ev_ctrl_t)(unsafe.Pointer(&event.data))
		if ctrl.value < 0 || ctrl.value > midi.DataMax {
			return ret, fmt.Errorf("%w: quarter frame %d", ErrInvalidMessage, ctrl.value)
		}
		ret.Data = []byte{midi.QuarterFrame, byte(ctrl.value)}
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
	}
	return ret, nil
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

// CAddrValues narrows a validated address to the byte fields libasound carries. It is
// an exported shim over the old signature, so invalid input still panics instead of
// silently wrapping.
func (d *SeqAddr) CAddrValues() (C.uchar, C.uchar) {
	if err := d.validate(); err != nil {
		panic(err)
	}
	return C.uchar(d.Client), C.uchar(d.Port)
}
