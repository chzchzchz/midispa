package cc

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

	tag string
	Cmd byte
}

type Control *int

type MidiControlsMap map[string]MidiControlsSlice

type MidiControlsSlice []*MidiControls

func (m MidiControlsSlice) Name(cmd byte, cc int) string {
	for _, mcs := range m {
		if mcs.Cmd != cmd {
			continue
		} else if s := mcs.Name(cc); s != "" {
			return s
		}
	}
	return ""
}

func (m MidiControlsSlice) Get(name string) (*MidiControls, int) {
	for _, mcs := range m {
		if cc := mcs.CC(name); cc >= 0 {
			return mcs, cc
		}
	}
	return nil, -1
}

func (m MidiControlsSlice) Set(name string, val int) (*MidiControls, int) {
	for _, mcs := range m {
		if cc := mcs.CC(name); cc >= 0 && mcs.Set(cc, val) {
			return mcs, cc
		}
	}
	return nil, -1
}

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
	val *int
}

func NewMidiControlsCC(model interface{}) *MidiControls {
	return newMidiControls("cc", 0xb0, model)
}

func NewMidiControlsNote(model interface{}) *MidiControls {
	return newMidiControls("note", 0x90, model)
}

func newMidiControls(tag string, cmd byte, model interface{}) *MidiControls {
	tt := reflect.TypeOf(model).Elem()
	n := tt.NumField()
	ret := &MidiControls{
		name2cc: make(map[string]*ccInfo),
		cc2cc:   make(map[int]*ccInfo),
		cc2name: make(map[int]string),
		model:   model,
		Cmd:     cmd,
	}
	for i := 0; i < n; i++ {
		field := tt.Field(i)
		// TODO: nrpns, msb/lsbs etc
		tagValue := field.Tag.Get(tag)
		if tagValue == "" {
			continue
		}
		fPtr := reflect.ValueOf(model).Elem().FieldByName(field.Name)
		cc := &ccInfo{min: 0, max: 127}
		if _, err := fmt.Sscanf(field.Tag.Get(tag), "%d", &cc.msb); err != nil {
			panic("field " + field.Name +
				" failed to parse tag " + tag +
				": " + err.Error())
		}
		if !fPtr.IsZero() {
			v := int(reflect.Indirect(fPtr).Int())
			cc.val = &v
		}
		ret.name2cc[field.Name], ret.cc2cc[cc.msb] = cc, cc
		ret.cc2name[cc.msb] = field.Name
		ret.ccInfos = append(ret.ccInfos, cc)
	}
	if len(ret.ccInfos) == 0 {
		return nil
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
	ccInfo.val = &v
	rv := reflect.ValueOf(m.model).Elem().FieldByName(m.cc2name[cc])
	rv.Set(reflect.ValueOf(ccInfo.val))
	return true
}

func (m *MidiControls) Get(cc int) *int {
	if cc == -1 {
		panic("bad cc")
	}
	return m.cc2cc[cc].val
}

// Convert a midi-tagged struct to control codes.
func (m *MidiControls) ToControlCodes() (ret [][]byte) {
	for _, cc := range m.ccInfos {
		if cc.val != nil {
			msg := []byte{m.Cmd, byte(cc.msb), byte(*cc.val)}
			ret = append(ret, msg)
		}
	}
	return ret
}
