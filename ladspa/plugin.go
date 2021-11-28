package ladspa

/*
#include <ladspa.h>
LADSPA_Handle instantiate(LADSPA_Descriptor* d, unsigned long sr) { return d->instantiate(d, sr); }
void activate(LADSPA_Descriptor* d, LADSPA_Handle h) { d->activate(h); }
void deactivate(LADSPA_Descriptor* d, LADSPA_Handle h) { if (d->deactivate) d->deactivate(h); }
void cleanup(LADSPA_Descriptor* d, LADSPA_Handle h) { return d->cleanup(h); }
const LADSPA_Descriptor* call_ladspa_descriptor(LADSPA_Descriptor_Function f, unsigned long idx) {
	return f(idx);
}
void run(LADSPA_Descriptor* d, LADSPA_Handle h, unsigned long sc) { d->run(h, sc); }
const LADSPA_PortDescriptor port_desc(LADSPA_Descriptor* d, int i) { return d->PortDescriptors[i]; }
const char* port_name(LADSPA_Descriptor* d, int i) { return d->PortNames[i]; }
void connect_port(LADSPA_Descriptor* d, LADSPA_Handle h, unsigned long port, float* v) {
	d->connect_port(h, port, v);
}
*/
import "C"

import (
	"fmt"
)

type Port struct {
	Name    string
	Input   bool
	Output  bool
	Control bool
	Audio   bool
	// todo: port hints
}

type Plugin struct {
	Label string
	Name  string
	Maker string
	Ports []Port

	handle C.LADSPA_Handle
	desc   *C.LADSPA_Descriptor
	so     *SoLib

	name2port map[string]int
}

func NewPlugin(lib string, sampleRate int) (*Plugin, error) {
	so, err := NewSoLib(lib)
	if err != nil {
		return nil, err
	}
	descf, err := so.Symbol("ladspa_descriptor")
	if err != nil {
		so.Close()
		return nil, err
	}
	desc := C.call_ladspa_descriptor(C.LADSPA_Descriptor_Function(descf), 0)
	h := C.instantiate(desc, C.ulong(sampleRate))
	if h == nil {
		so.Close()
		return nil, fmt.Errorf("cant instantiate")
	}
	C.activate(desc, h)
	p := &Plugin{
		Label:     C.GoString(desc.Label),
		Name:      C.GoString(desc.Name),
		Maker:     C.GoString(desc.Maker),
		so:        so,
		desc:      desc,
		handle:    h,
		name2port: make(map[string]int),
	}
	pc := int(p.desc.PortCount)
	for i := 0; i < pc; i++ {
		pd := C.port_desc(desc, C.int(i))
		port := Port{
			Name:    C.GoString(C.port_name(desc, C.int(i))),
			Input:   pd&C.LADSPA_PORT_INPUT != 0,
			Output:  pd&C.LADSPA_PORT_OUTPUT != 0,
			Control: pd&C.LADSPA_PORT_CONTROL != 0,
			Audio:   pd&C.LADSPA_PORT_AUDIO != 0,
		}
		p.Ports = append(p.Ports, port)
		p.name2port[port.Name] = i
	}

	return p, nil
}

func (p *Plugin) Connect(name string, data *float32) {
	idx, ok := p.name2port[name]
	if !ok {
		panic("no port by name " + name)
	}
	C.connect_port(p.desc, p.handle, C.ulong(idx), (*C.float)(data))
}

func (p *Plugin) Run(samps int) {
	C.run(p.desc, p.handle, C.ulong(samps))
}

func (p *Plugin) Close() error {
	C.deactivate(p.desc, p.handle)
	C.cleanup(p.desc, p.handle)
	return p.so.Close()
}
