package main

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/internal/fieldrules"
	"github.com/chzchzchz/midispa/midi"
)

// newTestModel builds an editor around a buffer, so a test
// sees the control changes the editor sends without an
// instrument on the other end of the port.
func newTestModel(t *testing.T, output string) (*cliModel, []cc.ControlField, *bytes.Buffer) {
	t.Helper()
	fields := testFields()
	buffer := &bytes.Buffer{}
	cfg := configuration{
		modelName:         "Meeblip SE",
		portName:          "test port",
		outputMIDIChannel: 1,
		output:            output,
	}
	model := newCLI(cfg, fields, buffer)
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return model, fields, buffer
}

func TestStepValue(t *testing.T) {
	tests := []struct {
		value, delta, want int
	}{
		{0, -1, 0},
		{127, 1, 127},
		{50, 10, 60},
		{50, -60, 0},
		{50, 5, 55},
		{126, 10, 127},
	}
	for _, test := range tests {
		if got := stepValue(test.value, test.delta); got != test.want {
			t.Errorf("stepValue(%d, %d) = %d, want %d", test.value, test.delta, got, test.want)
		}
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		text  string
		value int
		ok    bool
	}{
		{"", 0, false},
		{"0", 0, true},
		{"42", 42, true},
		{"127", 127, true},
		{"128", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"12a", 0, false},
	}
	for _, test := range tests {
		value, ok := parseValue(test.text)
		if value != test.value || ok != test.ok {
			t.Errorf("parseValue(%q) = (%d, %v), want (%d, %v)", test.text, value, ok, test.value, test.ok)
		}
	}
}

func TestValidateValue(t *testing.T) {
	if err := validateValue(""); err != nil {
		t.Errorf("empty buffer: %v", err)
	}
	if err := validateValue("127"); err != nil {
		t.Errorf("in-range buffer: %v", err)
	}
	if err := validateValue("999"); err == nil {
		t.Error("out-of-range buffer accepted")
	}
}

// TestValueKeys drives the value keys from a table:
// each key moves the focused field by its step, clamped
// at the MIDI range, and every press sends the control
// change at once.
func TestValueKeys(t *testing.T) {
	tests := []struct {
		name       string
		start      int
		key        tea.KeyType
		presses    int
		wantValue  int
		wantBuffer []byte
		wantStatus string
		wantKind   statusKind
	}{
		{"up steps by one", 0, tea.KeyUp, 1, 1,
			[]byte{midi.MakeCC(0), 10, 1}, "Alpha = 1", statusGood},
		{"down steps by one", 1, tea.KeyDown, 1, 0,
			[]byte{midi.MakeCC(0), 10, 0}, "Alpha = 0", statusGood},
		{"pgup steps by ten", 0, tea.KeyPgUp, 1, 10,
			[]byte{midi.MakeCC(0), 10, 10}, "Alpha = 10", statusGood},
		{"pgdn steps by ten", 10, tea.KeyPgDown, 1, 0,
			[]byte{midi.MakeCC(0), 10, 0}, "Alpha = 0", statusGood},
		{"repeats stack", 0, tea.KeyUp, 3, 3,
			[]byte{midi.MakeCC(0), 10, 1, midi.MakeCC(0), 10, 2, midi.MakeCC(0), 10, 3},
			"Alpha = 3", statusGood},
		{"up clamps at the top", 127, tea.KeyUp, 1, 127, nil, "", statusPlain},
		{"down clamps at the bottom", 0, tea.KeyDown, 1, 0, nil, "", statusPlain},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, fields, buffer := newTestModel(t, "")
			*fields[0].Value = test.start
			model.inputs[0].SetValue(strconv.Itoa(test.start))
			buffer.Reset()
			for range test.presses {
				model.Update(tea.KeyMsg{Type: test.key})
			}
			if *fields[0].Value != test.wantValue {
				t.Errorf("value = %d, want %d", *fields[0].Value, test.wantValue)
			}
			if got := model.inputs[0].Value(); got != strconv.Itoa(test.wantValue) {
				t.Errorf("cell shows %q, want %d", got, test.wantValue)
			}
			if !bytes.Equal(buffer.Bytes(), test.wantBuffer) {
				t.Errorf("sent %v, want %v", buffer.Bytes(), test.wantBuffer)
			}
			if model.status != test.wantStatus || model.statusKind != test.wantKind {
				t.Errorf("status = %q (kind %d), want %q (kind %d)",
					model.status, model.statusKind, test.wantStatus, test.wantKind)
			}
		})
	}
}

// clearCell empties the focused cell's buffer, so a
// test types a value from scratch rather than appending
// to the value the model already holds.
func clearCell(model *cliModel) {
	for range len(model.inputs[model.focusIndex].Value()) {
		model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
}

func TestTypingSendsEachIntermediateValue(t *testing.T) {
	model, _, buffer := newTestModel(t, "")
	clearCell(model)
	for _, digit := range "127" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{digit}})
	}
	want := []byte{
		midi.MakeCC(0), 10, 1,
		midi.MakeCC(0), 10, 12,
		midi.MakeCC(0), 10, 127,
	}
	if !bytes.Equal(buffer.Bytes(), want) {
		t.Errorf("sent %v, want %v", buffer.Bytes(), want)
	}
	if got := model.inputs[0].Value(); got != "127" {
		t.Errorf("cell shows %q, want 127", got)
	}
}

func TestTypingAcrossFieldsSendsOnThatChannel(t *testing.T) {
	model, fields, buffer := newTestModel(t, "")
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	want := []byte{midi.MakeCC(0), 11, 9}
	if !bytes.Equal(buffer.Bytes(), want) {
		t.Errorf("sent %v, want %v", buffer.Bytes(), want)
	}
	if *fields[1].Value != 9 {
		t.Errorf("beta = %d, want 9", *fields[1].Value)
	}
}

func TestInvalidBufferLeavesModelUntouched(t *testing.T) {
	model, fields, buffer := newTestModel(t, "")
	// Type "50", then one digit too many: the buffer
	// holds a value the MIDI range cannot, which is not
	// a value at all.
	clearCell(model)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	buffer.Reset()
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})

	if *fields[0].Value != 50 {
		t.Errorf("model value = %d, want the last valid 50", *fields[0].Value)
	}
	if got := model.inputs[0].Value(); got != "500" {
		t.Errorf("cell shows %q, want the typed 500", got)
	}
	if model.status != "invalid" || model.statusKind != statusBad {
		t.Errorf("status = %q (kind %d), want invalid bad", model.status, model.statusKind)
	}
	if buffer.Len() != 0 {
		t.Errorf("an invalid buffer sent %d bytes, want none", buffer.Len())
	}

	// Leaving the cell restores what the model holds.
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := model.inputs[0].Value(); got != "50" {
		t.Errorf("after blur cell shows %q, want 50", got)
	}
}

func TestEmptyBufferIsInvalidAndKeepsTheValue(t *testing.T) {
	model, fields, buffer := newTestModel(t, "")
	clearCell(model)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	buffer.Reset()
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	if got := model.inputs[0].Value(); got != "" {
		t.Errorf("cell shows %q, want empty", got)
	}
	if *fields[0].Value != 5 {
		t.Errorf("model value = %d, want the kept 5", *fields[0].Value)
	}
	if model.status != "invalid" {
		t.Errorf("status = %q, want invalid", model.status)
	}
	if buffer.Len() != 0 {
		t.Errorf("an empty buffer sent %d bytes, want none", buffer.Len())
	}
}

func TestTabCyclesFocus(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if model.focusIndex != 1 || !model.inputs[1].Focused() {
		t.Fatalf("tab: focus = %d, want 1", model.focusIndex)
	}
	if model.inputs[0].Focused() {
		t.Error("the cell left behind is still focused")
	}
	// Tab wraps from the last field back to the first.
	for range len(model.inputs) - 2 {
		model.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if model.focusIndex != 0 || !model.inputs[0].Focused() {
		t.Errorf("tab from the last field: focus = %d, want 0", model.focusIndex)
	}
}

func TestShiftTabMovesFocusBack(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if model.focusIndex != len(model.inputs)-1 {
		t.Errorf("shift-tab from the first field: focus = %d, want %d", model.focusIndex, len(model.inputs)-1)
	}
}

func TestLeftRightMoveByRow(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	// A width of 24 fits two cells of width 11
	// inside the grid's box, so the five fields
	// pack into two columns of three rows:
	// Alpha Delta, Beta Echo, Gamma.
	model.Update(tea.WindowSizeMsg{Width: 24, Height: 30})
	if model.columns != 2 {
		t.Fatalf("columns = %d, want 2", model.columns)
	}

	// Each key moves focus by one visual row, clamped
	// at the first and last field.
	tests := []struct {
		key  tea.KeyType
		name string
		want int
	}{
		{tea.KeyRight, "Delta", 3},
		{tea.KeyRight, "Echo, clamped", 4},
		{tea.KeyLeft, "Beta", 1},
		{tea.KeyLeft, "Alpha, clamped", 0},
	}
	for _, test := range tests {
		model.Update(tea.KeyMsg{Type: test.key})
		if model.focusIndex != test.want {
			t.Errorf("%v: focus = %d, want %d (%s)",
				test.key, model.focusIndex, test.want, test.name)
		}
	}
}

func TestEscapeAndCtrlCQuit(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		model, _, _ := newTestModel(t, "")
		_, cmd := model.Update(tea.KeyMsg{Type: key})
		if cmd == nil {
			t.Fatalf("%v did not quit", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%v returned %T, want tea.QuitMsg", key, cmd())
		}
	}
}

func TestSaveWritesOutputFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "saved.mid")
	model, fields, _ := newTestModel(t, output)
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if model.status != "saved 5 controls" || model.statusKind != statusGood {
		t.Errorf("status = %q (kind %d), want %q good", model.status, model.statusKind, "saved 5 controls")
	}
	messages, err := readCCSMF(output)
	if err != nil {
		t.Fatalf("read back the save: %v", err)
	}
	if len(messages) != len(fields) {
		t.Fatalf("saved %d messages, want %d", len(messages), len(fields))
	}
	want := []byte{midi.MakeCC(0), 10, 1}
	if !bytes.Equal(messages[0], want) {
		t.Errorf("first message = %v, want %v", messages[0], want)
	}
}

func TestSaveWithoutOutputFlag(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if model.status != "--output is required to save" {
		t.Errorf("status = %q, want the missing-flag notice", model.status)
	}
}

func TestSaveWritesOnlyNonExcludedFields(t *testing.T) {
	output := filepath.Join(t.TempDir(), "saved.mid")
	rules := map[string]fieldrules.Rule{
		"Gamma": {Policy: fieldrules.PolicyExclude},
	}
	fields, _, err := applyExcludes(testFields(), rules)
	if err != nil {
		t.Fatalf("applyExcludes: %v", err)
	}
	cfg := configuration{
		modelName:         "Meeblip SE",
		portName:          "test port",
		outputMIDIChannel: 1,
		output:            output,
	}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	messages, err := readCCSMF(output)
	if err != nil {
		t.Fatalf("read back the save: %v", err)
	}
	for _, message := range messages {
		if len(message) < 2 {
			t.Fatalf("short message %v", message)
		}
		if int(message[1]) == 12 {
			t.Errorf("an excluded field was saved: %v", message)
		}
	}
	if len(messages) != 4 {
		t.Errorf("saved %d messages, want the 4 kept fields", len(messages))
	}
}

func TestHeaderCountsSentMessages(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	if view := model.View(); !strings.Contains(view, "(0 sent)") {
		t.Errorf("header before any send:\n%s", view)
	}
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if view := model.View(); !strings.Contains(view, "(2 sent)") {
		t.Errorf("header after two sends:\n%s", view)
	}
}

func TestViewContainsFieldNamesAndValues(t *testing.T) {
	model, fields, _ := newTestModel(t, "")
	*fields[0].Value = 42
	model.inputs[0].SetValue("42")
	view := model.View()
	for _, name := range []string{"Alpha", "Beta", "Gamma", "Delta", "Echo"} {
		if !strings.Contains(view, name) {
			t.Errorf("view is missing the field %q:\n%s", name, view)
		}
	}
	if !strings.Contains(view, "42") {
		t.Errorf("view is missing the value 42:\n%s", view)
	}
	if !strings.Contains(view, "Meeblip SE") || !strings.Contains(view, "test port") {
		t.Errorf("view header is missing the model or the port:\n%s", view)
	}
}

func TestViewHidesExcludedFields(t *testing.T) {
	rules := map[string]fieldrules.Rule{
		"Gamma": {Policy: fieldrules.PolicyExclude},
	}
	fields, _, err := applyExcludes(testFields(), rules)
	if err != nil {
		t.Fatalf("applyExcludes: %v", err)
	}
	cfg := configuration{modelName: "Meeblip SE", portName: "test port", outputMIDIChannel: 1}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if view := model.View(); strings.Contains(view, "Gamma") {
		t.Errorf("view shows the excluded field Gamma:\n%s", view)
	}
}

func TestMouseWheelScrolls(t *testing.T) {
	values := make([]int, 40)
	fields := make([]cc.ControlField, 40)
	for index := range fields {
		fields[index] = cc.ControlField{
			Name:       "Field" + string(rune('a'+index/26)) + string(rune('a'+index%26)),
			Controller: index,
			Value:      &values[index],
		}
	}
	cfg := configuration{modelName: "Meeblip SE", portName: "test port", outputMIDIChannel: 1}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	if model.vp.Height != 2 {
		t.Fatalf("viewport height = %d, want 2", model.vp.Height)
	}

	// The viewport scrolls on the wheel button and the
	// press action; the deprecated event type is only
	// derived from them, so the messages set the two
	// fields the viewport reads.
	model.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	if model.vp.YOffset <= 0 {
		t.Errorf("wheel down left the viewport at offset %d", model.vp.YOffset)
	}
	model.Update(tea.MouseMsg{
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	})
	if model.vp.YOffset != 0 {
		t.Errorf("wheel up left the viewport at offset %d, want 0", model.vp.YOffset)
	}
}

func TestFocusMoveKeepsFocusedRowVisible(t *testing.T) {
	values := make([]int, 40)
	fields := make([]cc.ControlField, 40)
	for index := range fields {
		fields[index] = cc.ControlField{
			Name:       "Field" + string(rune('a'+index/26)) + string(rune('a'+index%26)),
			Controller: index,
			Value:      &values[index],
		}
	}
	cfg := configuration{modelName: "Meeblip SE", portName: "test port", outputMIDIChannel: 1}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	// Two columns of twenty rows inside the box:
	// jump to the last field, whose row is far
	// below the viewport.
	model.Update(tea.WindowSizeMsg{Width: 28, Height: 8})
	if model.columns != 2 {
		t.Fatalf("columns = %d, want 2", model.columns)
	}
	for range 39 {
		model.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if model.focusIndex != 39 {
		t.Fatalf("focus = %d, want 39", model.focusIndex)
	}
	row := model.focusIndex % model.rowCount()
	if row < model.vp.YOffset || row >= model.vp.YOffset+model.vp.Height {
		t.Errorf("focused row %d is outside the viewport offset %d height %d",
			row, model.vp.YOffset, model.vp.Height)
	}
}

func TestViewDrawsBoxes(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	view := model.View()
	// The grid sits in a rounded box: its top-left
	// corner, a side, and its bottom-left corner all
	// appear in the view.
	for _, want := range []string{"\u256d", "\u2502", "\u2570"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing the box character %q:\n%s", want, view)
		}
	}
}

func TestInitialStatusReportsNrpnFields(t *testing.T) {
	fields, _, _, err := loadModelFields("microKORG2", "")
	if err != nil {
		t.Fatalf("loadModelFields: %v", err)
	}
	cfg := configuration{modelName: "microKORG2", portName: "test port", outputMIDIChannel: 1}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	if !strings.Contains(model.status, "nrpn fields are not shown") {
		t.Errorf("status = %q, want the nrpn notice", model.status)
	}
}

func TestNewCLIFocusesTheFirstField(t *testing.T) {
	model, _, _ := newTestModel(t, "")
	if model.focusIndex != 0 || !model.inputs[0].Focused() {
		t.Errorf("initial focus = %d, want field 0", model.focusIndex)
	}
}

func TestNewCLISeededValuesShowInCells(t *testing.T) {
	fields := testFields()
	*fields[0].Value = 42
	*fields[3].Value = 127
	cfg := configuration{modelName: "Meeblip SE", portName: "test port", outputMIDIChannel: 1}
	model := newCLI(cfg, fields, &bytes.Buffer{})
	if got := model.inputs[0].Value(); got != "42" {
		t.Errorf("first cell shows %q, want 42", got)
	}
	if got := model.inputs[3].Value(); got != "127" {
		t.Errorf("fourth cell shows %q, want 127", got)
	}
}
