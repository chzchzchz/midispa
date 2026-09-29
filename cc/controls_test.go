package cc

import (
	"reflect"
	"sort"
	"testing"
)

func TestControlFieldsPreservesOrderAndDuplicateControllers(t *testing.T) {
	model := &WorldeEasyControl9{}
	fields, err := ControlFields(model)
	if err != nil {
		t.Fatalf("ControlFields: %v", err)
	}
	byName := make(map[string]ControlField)
	for _, field := range fields {
		if field.Value == nil {
			t.Fatalf("field %s has nil value", field.Name)
		}
		byName[field.Name] = field
	}
	if byName["SliderAB"].Controller != 9 || byName["Slider7"].Controller != 9 {
		t.Fatalf("duplicate CC fields were not preserved: %+v", fields)
	}
	*byName["SliderAB"].Value = 11
	*byName["Slider7"].Value = 22
	if model.SliderAB == nil || *model.SliderAB != 11 {
		t.Fatalf("SliderAB is %v, want 11", model.SliderAB)
	}
	if model.Slider7 == nil || *model.Slider7 != 22 {
		t.Fatalf("Slider7 is %v, want 22", model.Slider7)
	}
}

func TestControlFieldsInitializesEveryModel(t *testing.T) {
	for _, name := range ModelNames() {
		t.Run(name, func(t *testing.T) {
			params, err := NewModelParams(name)
			if err != nil {
				t.Fatalf("NewModelParams: %v", err)
			}
			fields, err := ControlFields(params)
			if err != nil {
				t.Fatalf("ControlFields: %v", err)
			}
			if len(fields) == 0 {
				t.Fatal("model has no CC fields")
			}
			for _, field := range fields {
				if field.Value == nil || field.Controller < 0 || field.Controller > 127 {
					t.Fatalf("invalid field metadata: %+v", field)
				}
			}
		})
	}
}

func TestMidiControlsKeepsSparsePatchValues(t *testing.T) {
	value := 42
	model := &SoundController{SoundController1: &value}
	controls := NewMidiControlsCC(model)
	if controls == nil {
		t.Fatal("NewMidiControlsCC returned nil")
	}
	groups := controls.ControlCodes()
	want := []ControlGroup{{
		Name:     "SoundController1",
		Messages: [][]byte{{0xb0, 70, 42}},
	}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups are %v, want %v", groups, want)
	}
}

func TestMidiControlsSetInitializesMissingField(t *testing.T) {
	model := &SoundController{}
	controls := NewMidiControlsCC(model)
	if controls == nil {
		t.Fatal("NewMidiControlsCC returned nil")
	}
	if !controls.Set(70, 55) {
		t.Fatal("Set rejected a valid controller")
	}
	if model.SoundController1 == nil || *model.SoundController1 != 55 {
		t.Fatalf("SoundController1 is %v, want 55", model.SoundController1)
	}
}

func TestControlFieldsRejectsInvalidModels(t *testing.T) {
	if _, err := ControlFields(nil); err == nil {
		t.Fatal("accepted a nil model")
	}
	if _, err := ControlFields(SoundController{}); err == nil {
		t.Fatal("accepted a non-pointer model")
	}

	type invalidTag struct {
		Value *int `cc:"invalid"`
	}
	if _, err := ControlFields(&invalidTag{}); err == nil {
		t.Fatal("accepted an invalid controller tag")
	}

	type invalidField struct {
		Value int `cc:"1"`
	}
	if _, err := ControlFields(&invalidField{}); err == nil {
		t.Fatal("accepted a non-pointer control field")
	}
}

func TestNRPNFieldsDecodeMicrokorgXLParameters(t *testing.T) {
	model := &MicrokorgXL{}
	fields, err := NRPNFields(model)
	if err != nil {
		t.Fatalf("NRPNFields: %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("model has no NRPN fields")
	}
	want := map[string][2]int{
		"ArpOnOff":           {0, 2},
		"ArpGate":            {0, 10},
		"VoiceMode":          {5, 0},
		"Patch6Dest":         {4, 13},
		"VocoderBandLevel16": {4, 79},
	}
	for _, field := range fields {
		if field.Value == nil {
			t.Fatalf("field %s has nil value", field.Name)
		}
		if field.MSB < 0 || field.MSB > 127 || field.LSB < 0 || field.LSB > 127 {
			t.Fatalf("field %s is not a 7-bit parameter: %+v", field.Name, field)
		}
	}
	for name, address := range want {
		field := findNRPNField(t, fields, name)
		if field.MSB != address[0] || field.LSB != address[1] {
			t.Fatalf("%s addresses %d.%d, want %d.%d", name, field.MSB, field.LSB, address[0], address[1])
		}
	}
	*findNRPNField(t, fields, "ArpType").Value = 100
	if model.ArpType == nil || *model.ArpType != 100 {
		t.Fatalf("ArpType is %v, want 100", model.ArpType)
	}
}

// NRPN 4.0 is Patch1Source in a synth program and the vocoder's FC.MOD.SRC in
// a vocoder program. The device has one active meaning per address, so the
// model must not declare it twice: a second field would overwrite the first in
// the address maps while both still transmitted.
func TestNRPNAddressesAreUnique(t *testing.T) {
	fields, err := NRPNFields(&MicrokorgXL{})
	if err != nil {
		t.Fatalf("NRPNFields: %v", err)
	}
	seen := make(map[[2]int]string, len(fields))
	for _, field := range fields {
		address := [2]int{field.MSB, field.LSB}
		if other, dup := seen[address]; dup {
			t.Fatalf("%s and %s both address NRPN %d.%d", other, field.Name, field.MSB, field.LSB)
		}
		seen[address] = field.Name
	}
	if _, ok := seen[[2]int{4, 0}]; !ok {
		t.Fatal("NRPN 4.0 is not addressed by any field")
	}
}

func TestMidiControlsNRPNEncodesSelectAndSet(t *testing.T) {
	model := &MicrokorgXL{ArpType: new(int)}
	controls := NewMidiControlsNRPN(model)
	if controls == nil {
		t.Fatal("NewMidiControlsNRPN returned nil")
	}
	if !controls.Set(controls.CC("ArpType"), 100) {
		t.Fatal("Set rejected a valid NRPN")
	}
	groups := controls.ControlCodes()
	want := []ControlGroup{{
		Name: "ArpType",
		Messages: [][]byte{
			{0xb0, 99, 0},  // NRPN MSB selects the arpeggiator block
			{0xb0, 98, 7},  // NRPN LSB selects the arpeggio type
			{0xb0, 6, 100}, // data entry MSB carries the value
		},
	}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups are %v, want %v", groups, want)
	}
	if got := groups[0].Value(); got != 100 {
		t.Fatalf("Value() is %d, want 100", got)
	}
}

// A cc-tagged field must keep its plain controller number as its address, so
// the packed NRPN keying cannot disturb the existing models.
func TestMidiControlsCCAddressesAreUnpacked(t *testing.T) {
	controls := NewMidiControlsCC(&MicrokorgXL{})
	if controls == nil {
		t.Fatal("NewMidiControlsCC returned nil")
	}
	if got := controls.CC("Filter1Cutoff"); got != 74 {
		t.Fatalf("Filter1Cutoff address is %d, want 74", got)
	}
	if got := controls.Name(74); got != "Filter1Cutoff" {
		t.Fatalf("Name(74) is %q, want Filter1Cutoff", got)
	}
}

// A device can hold a CC set and an NRPN set at once, and a single controller
// number must only ever resolve against the CC set. NRPN 0.2, 0.4 and 0.11
// would otherwise pack onto 2, 4 and 11, so an incoming expression pedal
// (CC#11) would be misreported as ArpTimbreSelect and routed.
func TestNRPNAddressesNeverResolveAsControllers(t *testing.T) {
	m := &Model{Model: "microKORG XL", MicrokorgXL: &MicrokorgXL{}}
	slice := MidiControlsSlice{
		NewMidiControlsCC(m.MidiParams()),
		NewMidiControlsNRPN(m.MidiParams()),
	}
	// 11 is the one that matters in practice: the microKORG XL declares no
	// CC#11, so an expression pedal used to fall through to the NRPN set.
	if got := slice.Name(0xb0, 11); got != "" {
		t.Fatalf("incoming CC#11 resolved to %q, want no match", got)
	}
	for cc := range 128 {
		got := slice.Name(0xb0, cc)
		controls := NewMidiControlsCC(m.MidiParams())
		if want := controls.Name(cc); got != want {
			t.Fatalf("incoming CC#%d resolved to %q, want %q", cc, got, want)
		}
	}
}

// ControlCodes builds its sequence from each field's own name rather than an
// address lookup, so two fields sharing a controller number stay separately
// identified even though the address maps can only hold one of them.
func TestControlCodesNamesDuplicateControllers(t *testing.T) {
	a, b := 11, 22
	model := &WorldeEasyControl9{SliderAB: &a, Slider7: &b}
	groups := NewMidiControlsCC(model).ControlCodes()
	want := []ControlGroup{
		{Name: "SliderAB", Messages: [][]byte{{0xb0, 9, 11}}},
		{Name: "Slider7", Messages: [][]byte{{0xb0, 9, 22}}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups are %v, want %v", groups, want)
	}
}

// ToMessages hands back fresh slices so a caller can OR its channel into the
// status byte without corrupting later sends.
func TestToMessagesCopiesMessages(t *testing.T) {
	controls := NewMidiControlsNRPN(&MicrokorgXL{ArpOnOff: new(int)})
	address := controls.CC("ArpOnOff")
	controls.Set(address, 127)
	first := controls.ToMessages(address)
	first[0][0] |= 0x05
	second := controls.ToMessages(address)
	if second[0][0] != 0xb0 {
		t.Fatalf("status byte is %#x after a previous send was modified", second[0][0])
	}
	if got := controls.ToMessages(address + 1); got != nil {
		t.Fatalf("unknown address returned %v, want nil", got)
	}
}

func TestNRPNFieldsIgnoreControlChangeFields(t *testing.T) {
	fields, err := NRPNFields(&MicrokorgXL{})
	if err != nil {
		t.Fatalf("NRPNFields: %v", err)
	}
	for _, field := range fields {
		if field.Name == "Filter1Cutoff" {
			t.Fatal("a cc-tagged field was reported as an NRPN field")
		}
	}
	ccFields, err := ControlFields(&MicrokorgXL{})
	if err != nil {
		t.Fatalf("ControlFields: %v", err)
	}
	if _, ok := findControlField(ccFields, "ArpType"); ok {
		t.Fatal("an nrpn-tagged field was reported as a control change")
	}
}

func TestNRPNFieldsRejectsMalformedTags(t *testing.T) {
	type missingLSB struct {
		Value *int `nrpn:"0"`
	}
	if _, err := NRPNFields(&missingLSB{}); err == nil {
		t.Fatal("accepted an NRPN tag without an LSB")
	}

	type nonNumeric struct {
		Value *int `nrpn:"0.x"`
	}
	if _, err := NRPNFields(&nonNumeric{}); err == nil {
		t.Fatal("accepted a non-numeric NRPN LSB")
	}

	type tooWide struct {
		Value *int `nrpn:"0.128"`
	}
	if _, err := NRPNFields(&tooWide{}); err == nil {
		t.Fatal("accepted an NRPN LSB outside 0-127")
	}
}

// The Perform-VE implements every controller in one place, its "MIDI CC
// List" of Appendix B, and the manual describes none of them as user
// remappable. Pinning the set catches a mistyped controller number and
// catches a field added that the unit does not actually answer.
func TestPerformVEAddressesTheWholeManualCCList(t *testing.T) {
	fields, err := ControlFields(&PerformVE{})
	if err != nil {
		t.Fatalf("ControlFields: %v", err)
	}
	want := []int{
		1,
		16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28,
		41, 42, 43, 44, 45, 46, 47, 48,
		51, 52, 53, 54, 55, 56, 58, 59,
		64,
		72, 73,
		80, 81, 82, 83,
	}
	got := make([]int, 0, len(fields))
	seen := make(map[int]string, len(fields))
	for _, field := range fields {
		if other, dup := seen[field.Controller]; dup {
			t.Errorf("%s and %s both address CC %d", other, field.Name, field.Controller)
		}
		seen[field.Controller] = field.Name
		got = append(got, field.Controller)
	}
	sort.Ints(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addressed controllers are %v, want %v", got, want)
	}
}

// The Perform-VE implements no non-registered parameters and no note
// triggers. Appendix B lists only control changes, and the unit's own MIDI
// settings, the channel and the split point, are reached by holding SET and
// playing a note rather than through an addressable parameter (p. 7, p. 24).
// An nrpn- or note-tagged field here would make the router emit a
// three-message NRPN sequence, or a note, for something the unit does not
// listen for.
func TestPerformVEHasNoNRPNOrNoteFields(t *testing.T) {
	nrpn, err := NRPNFields(&PerformVE{})
	if err != nil {
		t.Fatalf("NRPNFields: %v", err)
	}
	if len(nrpn) != 0 {
		t.Fatalf("model has %d NRPN fields, want none: %+v", len(nrpn), nrpn)
	}
	note, err := taggedControlFields(&PerformVE{}, noteTag, true)
	if err != nil {
		t.Fatalf("taggedControlFields: %v", err)
	}
	if len(note) != 0 {
		t.Fatalf("model has %d note fields, want none: %+v", len(note), note)
	}
}

func findNRPNField(t *testing.T, fields []NRPNField, name string) NRPNField {
	t.Helper()
	for _, field := range fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("no NRPN field named %s", name)
	return NRPNField{}
}

func findControlField(fields []ControlField, name string) (ControlField, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return ControlField{}, false
}
