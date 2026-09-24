package cc

import (
	"reflect"
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
	messages := controls.ToControlCodes()
	want := [][]byte{{0xb0, 70, 42}}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("messages are %v, want %v", messages, want)
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
