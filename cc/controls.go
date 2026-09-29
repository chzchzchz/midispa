package cc

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Struct tags that address a control. cc and note carry a single 7-bit
// controller number; nrpn carries a non-registered parameter number as
// "<msb>.<lsb>".
const (
	ccTag   = "cc"
	noteTag = "note"
	nrpnTag = "nrpn"
)

// Controllers that carry a non-registered parameter selection. An NRPN is
// selected by sending its MSB then its LSB, then the value on data entry MSB.
// The microKORG XL implements data entry MSB only, never data entry LSB.
const (
	nrpnMSBController       = 99
	nrpnLSBController       = 98
	nrpnDataEntryMSBControl = 6
)

// packAddress folds an NRPN's MSB and LSB into one map key. It is only used
// for the nrpn tag; cc and note address a single controller and keep it as-is.
//
// The result is biased by nrpnAddressBase so it can never be read as a plain
// controller number. Without that, an NRPN with MSB 0 would pack to 2, 4 or
// 11 and an incoming CC#11 (expression) would be looked up in the NRPN map and
// reported as ArpTimbreSelect.
const nrpnAddressBase = 1 << 14

func packAddress(msb, lsb int) int {
	return nrpnAddressBase | msb<<7 | lsb
}

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

// ControlGroup is one addressed field's wire messages, kept together because a
// non-registered parameter needs several messages to take effect. A cc or note
// group holds exactly one.
type ControlGroup struct {
	Name     string
	Messages [][]byte
}

// Value returns the data byte the group sets, which is the last message's
// value for a cc, note or NRPN.
func (g ControlGroup) Value() byte {
	return g.Messages[len(g.Messages)-1][2]
}

// ControlFields initializes and returns all cc-tagged fields in declaration
// order, preserving duplicate controller numbers.
func ControlFields(model any) ([]ControlField, error) {
	fields, err := taggedControlFields(model, ccTag, true)
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

// NRPNField exposes one ordered nrpn-tagged field. MSB and LSB together select
// the parameter; the value is written with data entry MSB.
type NRPNField struct {
	Name  string
	MSB   int
	LSB   int
	Value *int
}

// NRPNFields initializes and returns all nrpn-tagged fields in declaration
// order, preserving duplicate parameter numbers.
func NRPNFields(model any) ([]NRPNField, error) {
	fields, err := taggedControlFields(model, nrpnTag, true)
	if err != nil {
		return nil, err
	}
	ret := make([]NRPNField, 0, len(fields))
	for _, field := range fields {
		ret = append(ret, NRPNField{
			Name:  field.name,
			MSB:   field.controller,
			LSB:   field.lsb,
			Value: field.value,
		})
	}
	return ret, nil
}

type taggedControlField struct {
	name string
	// controller holds the primary address: the controller number for cc and
	// note, the NRPN MSB for nrpn.
	controller int
	// lsb holds the NRPN LSB; it is always zero for the single-number tags.
	lsb   int
	value *int
}

// decodeTag turns a field's tag value into the data bytes it addresses. cc
// and note address one 7-bit controller and leave secondary zero; nrpn
// addresses an MSB/LSB parameter number.
func decodeTag(field, tag, text string) (controller, secondary int, err error) {
	if tag != nrpnTag {
		controller, err = strconv.Atoi(text)
		if err != nil || controller < 0 || controller > 127 {
			return 0, 0, fmt.Errorf("field %s has invalid %s tag %q", field, tag, text)
		}
		return controller, 0, nil
	}
	msbText, lsbText, found := strings.Cut(text, ".")
	if !found {
		return 0, 0, fmt.Errorf("field %s has invalid %s tag %q: want <msb>.<lsb>", field, tag, text)
	}
	msb, err := parseDataByte(msbText)
	if err != nil {
		return 0, 0, fmt.Errorf("field %s has invalid %s tag %q: %w", field, tag, text, err)
	}
	lsb, err := parseDataByte(lsbText)
	if err != nil {
		return 0, 0, fmt.Errorf("field %s has invalid %s tag %q: %w", field, tag, text, err)
	}
	return msb, lsb, nil
}

func parseDataByte(text string) (int, error) {
	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", text)
	}
	if value < 0 || value > 127 {
		return 0, fmt.Errorf("%q is not a 7-bit data byte", text)
	}
	return value, nil
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
		controller, lsb, err := decodeTag(fieldType.Name, tag, controllerText)
		if err != nil {
			return nil, err
		}

		fieldValue := modelValue.Field(index)
		if fieldValue.Kind() != reflect.Pointer || fieldValue.Type().Elem().Kind() != reflect.Int {
			return nil, fmt.Errorf("field %s is not a pointer to an integer", fieldType.Name)
		}
		if fieldValue.IsNil() {
			if !initializeMissing {
				fields = append(fields, taggedControlField{
					name:       fieldType.Name,
					controller: controller,
					lsb:        lsb,
				})
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
			lsb:        lsb,
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

func (m MidiControlsSlice) ControlCodes() (ret []ControlGroup) {
	for _, mcs := range m {
		ret = append(ret, mcs.ControlCodes()...)
	}
	return ret
}

type ccInfo struct {
	// name is the field's own name. Two fields may share an address, in which
	// case the address maps hold only the last of them, so the sequence is
	// built from this instead of a map lookup.
	name string
	// address is the map key: the controller number for cc and note, or the
	// packed MSB/LSB pair for nrpn. It is stored rather than recomputed so
	// every lookup agrees on how a field is addressed.
	address int
	msb     int
	// lsb is the NRPN's lower byte; it stays zero for the single-number tags.
	lsb int
	min int
	max int
	val *int
}

func NewMidiControlsCC(model interface{}) *MidiControls {
	return newMidiControls(ccTag, 0xb0, model)
}

func NewMidiControlsNote(model interface{}) *MidiControls {
	return newMidiControls(noteTag, 0x90, model)
}

// NewMidiControlsNRPN returns the model's nrpn-tagged fields. Each field
// transmits as three messages, so a caller that only handles one control
// change per parameter cannot drive these.
func NewMidiControlsNRPN(model any) *MidiControls {
	return newMidiControls(nrpnTag, 0xb0, model)
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
		address := field.controller
		if tag == nrpnTag {
			address = packAddress(field.controller, field.lsb)
		}
		info := &ccInfo{
			name:    field.name,
			address: address,
			msb:     field.controller,
			lsb:     field.lsb,
			min:     0,
			max:     127,
			val:     field.value,
		}
		ret.name2cc[field.name], ret.cc2cc[address] = info, info
		ret.cc2name[address] = field.name
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
		return v.address
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

// Convert a midi-tagged struct to control codes. Each field becomes one group
// of messages: a single control change, or the three-message NRPN sequence.
func (m *MidiControls) ControlCodes() (ret []ControlGroup) {
	for _, cc := range m.ccInfos {
		msgs := m.messages(cc)
		if msgs == nil {
			continue
		}
		ret = append(ret, ControlGroup{
			Name:     cc.name,
			Messages: msgs,
		})
	}
	return ret
}

// ToMessages returns the wire messages for the field at address, or nil if the
// address is unknown or the field has no value. The returned messages are fresh
// slices, so a caller may rewrite the status byte to add its channel.
func (m *MidiControls) ToMessages(address int) [][]byte {
	ccInfo, ok := m.cc2cc[address]
	if !ok {
		return nil
	}
	return m.messages(ccInfo)
}

func (m *MidiControls) messages(ccInfo *ccInfo) [][]byte {
	if ccInfo.val == nil {
		return nil
	}
	if m.tag != nrpnTag {
		return [][]byte{{m.Cmd, byte(ccInfo.msb), byte(*ccInfo.val)}}
	}
	// Every NRPN is sent as a full select-and-set. Tracking the selection
	// across fields would save messages, but a sparse patch can skip a field
	// whose selection was never sent, which would silently retarget the next
	// one.
	return [][]byte{
		{m.Cmd, nrpnMSBController, byte(ccInfo.msb)},
		{m.Cmd, nrpnLSBController, byte(ccInfo.lsb)},
		{m.Cmd, nrpnDataEntryMSBControl, byte(*ccInfo.val)},
	}
}
