package cc

import (
	"fmt"
	"reflect"
	"strconv"
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

// ControlField exposes one ordered CC-tagged field. Separate fields remain
// separate even when they share a controller number.
type ControlField struct {
	Name       string
	Controller int
	Value      *int
}

// ControlFields initializes and returns all cc-tagged fields in declaration
// order, preserving duplicate controller numbers.
func ControlFields(model any) ([]ControlField, error) {
	fields, err := taggedControlFields(model, "cc", true)
	if err != nil {
		return nil, err
	}
	ret := make([]ControlField, 0, len(fields))
	for _, field := range fields {
		ret = append(ret, ControlField{
			Name:       field.name,
			Controller: field.controller,
			Value:      field.value,
		})
	}
	return ret, nil
}

type taggedControlField struct {
	name       string
	controller int
	value      *int
}

func taggedControlFields(model any, tag string, initializeMissing bool) ([]taggedControlField, error) {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return nil, fmt.Errorf("model is not a non-nil struct pointer")
	}
	modelValue = modelValue.Elem()
	if modelValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("model is not a struct pointer")
	}

	modelType := modelValue.Type()
	fields := make([]taggedControlField, 0, modelType.NumField())
	for index := 0; index < modelType.NumField(); index++ {
		fieldType := modelType.Field(index)
		controllerText := fieldType.Tag.Get(tag)
		if controllerText == "" {
			continue
		}
		controller, err := strconv.Atoi(controllerText)
		if err != nil || controller < 0 || controller > 127 {
			return nil, fmt.Errorf("field %s has invalid %s tag %q", fieldType.Name, tag, controllerText)
		}

		fieldValue := modelValue.Field(index)
		if fieldValue.Kind() != reflect.Pointer || fieldValue.Type().Elem().Kind() != reflect.Int {
			return nil, fmt.Errorf("field %s is not a pointer to an integer", fieldType.Name)
		}
		if fieldValue.IsNil() {
			if !initializeMissing {
				fields = append(fields, taggedControlField{name: fieldType.Name, controller: controller})
				continue
			}
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()).Convert(fieldValue.Type()))
		}

		var value *int
		switch typedValue := fieldValue.Interface().(type) {
		case Control:
			value = (*int)(typedValue)
		case *int:
			value = typedValue
		default:
			return nil, fmt.Errorf("field %s has unsupported pointer type %s", fieldType.Name, fieldValue.Type())
		}
		fields = append(fields, taggedControlField{
			name:       fieldType.Name,
			controller: controller,
			value:      value,
		})
	}
	return fields, nil
}

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

func newMidiControls(tag string, cmd byte, model any) *MidiControls {
	fields, err := taggedControlFields(model, tag, false)
	if err != nil {
		panic(err.Error())
	}
	ret := &MidiControls{
		name2cc: make(map[string]*ccInfo),
		cc2cc:   make(map[int]*ccInfo),
		cc2name: make(map[int]string),
		model:   model,
		tag:     tag,
		Cmd:     cmd,
	}
	for _, field := range fields {
		info := &ccInfo{msb: field.controller, min: 0, max: 127, val: field.value}
		ret.name2cc[field.name], ret.cc2cc[info.msb] = info, info
		ret.cc2name[info.msb] = field.name
		ret.ccInfos = append(ret.ccInfos, info)
	}
	if len(ret.ccInfos) == 0 {
		return nil
	}
	return ret
}

func (m *MidiControls) Names() (ret []string) {
	for _, n := range m.cc2name {
		ret = append(ret, n)
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
