package main

import (
	"fmt"
	"reflect"
)

type MidiControls struct {
	name2cc map[string]*ccInfo
	cc2cc   map[int]*ccInfo
	cc2name map[int]string
	model   interface{}
	// consistent ordering
	ccInfos []*ccInfo
}

type MidiControlsMap map[string]MidiControlsSlice

type MidiControlsSlice []*MidiControls

func (m MidiControlsSlice) Name(cc int) string {
	for _, mcs := range m {
		if s := mcs.Name(cc); s != "" {
			return s
		}
	}
	return ""
}

func (m MidiControlsSlice) Set(name string, val int) (int, bool) {
	for _, mcs := range m {
		if cc := mcs.CC(name); cc >= 0 {
			return cc, mcs.Set(cc, val)
		}
	}
	return -1, false
}

// Convert a midi-tagged struct to control codes.
func (m MidiControlsSlice) ToControlCodes() (ret [][]byte) {
	for _, mcs := range m {
		ret = append(ret, mcs.ToControlCodes()...)
	}
	return ret
}

type ccInfo struct {
	msb int
	min int
	max int
	val int
}

func NewMidiControls(model interface{}) *MidiControls {
	tt := reflect.TypeOf(model).Elem()
	n := tt.NumField()
	ret := &MidiControls{
		name2cc: make(map[string]*ccInfo),
		cc2cc:   make(map[int]*ccInfo),
		cc2name: make(map[int]string),
		model:   model,
	}
	for i := 0; i < n; i++ {
		field := tt.Field(i)
		cc := &ccInfo{msb: 0, min: 0, max: 127, val: 0}
		cc.val = int(reflect.ValueOf(model).Elem().FieldByName(field.Name).Int())
		// TODO: nrpns, msb/lsbs etc
		_, err := fmt.Sscanf(field.Tag.Get("midicc"), "%d", &cc.msb)
		if err != nil {
			panic(err)
		}
		ret.name2cc[field.Name], ret.cc2cc[cc.msb] = cc, cc
		ret.cc2name[cc.msb] = field.Name
		ret.ccInfos = append(ret.ccInfos, cc)
	}
	return ret
}

func (m *MidiControls) Name(cc int) string {
	if s, ok := m.cc2name[cc]; ok {
		return s
	}
	return ""
}

func (m *MidiControls) CC(name string) int {
	if v, ok := m.name2cc[name]; ok {
		return v.msb
	}
	return -1
}

func (m *MidiControls) Set(cc, v int) bool {
	ccInfo, ok := m.cc2cc[cc]
	if !ok {
		return false
	}
	ccInfo.val = v
	reflect.ValueOf(m.model).Elem().FieldByName(m.cc2name[cc]).SetInt(int64(v))
	return true
}

func (m *MidiControls) Get(cc int) int {
	if cc == -1 {
		panic("bad cc")
	}
	return m.cc2cc[cc].val
}

// Convert a midi-tagged struct to control codes.
func (m *MidiControls) ToControlCodes() (ret [][]byte) {
	for _, cc := range m.ccInfos {
		ret = append(ret, []byte{0xb0, byte(cc.msb), byte(cc.val)})
	}
	return ret
}
