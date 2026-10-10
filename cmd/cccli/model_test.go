package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/internal/fieldrules"
	"github.com/chzchzchz/midispa/midi"
)

// testFields builds a fixed field list whose values are
// addressable, so a test can set and read them the way the
// editor does through the model's own pointers.
func testFields() []cc.ControlField {
	values := []int{0, 0, 0, 0, 0}
	return []cc.ControlField{
		{Name: "Alpha", Controller: 10, Value: &values[0]},
		{Name: "Beta", Controller: 11, Value: &values[1]},
		{Name: "Gamma", Controller: 12, Value: &values[2]},
		{Name: "Delta", Controller: 13, Value: &values[3]},
		{Name: "Echo", Controller: 10, Value: &values[4]},
	}
}

// writeSeedSMF writes a Standard MIDI File holding the given
// control changes, all on the given one-based channel, so a
// test can build seeds the encoder would not produce itself.
func writeSeedSMF(t *testing.T, path string, channelNumber int, controls ...[2]int) {
	t.Helper()
	channelIndex := channelNumber - 1
	var writeErr error
	err := smfwriter.WriteFile(path, func(midiWriter smf.Writer) {
		if writeErr = midiWriter.Write(meta.Instrument("seed")); writeErr != nil {
			return
		}
		midiWriter.SetDelta(0)
		for _, control := range controls {
			change := channel.Channel(channelIndex).ControlChange(byte(control[0]), byte(control[1]))
			if writeErr = midiWriter.Write(change); writeErr != nil {
				return
			}
		}
	}, smfwriter.NumTracks(1), smfwriter.TimeFormat(smf.MetricTicks(ccTicksPerQuarter)))
	if writeErr != nil {
		t.Fatalf("write seed: %v", writeErr)
	}
	if err == smf.ErrFinished {
		return
	}
	if err != nil {
		t.Fatalf("write seed: %v", err)
	}
}

func TestApplySeed(t *testing.T) {
	fields := testFields()
	messages := [][]byte{
		{midi.MakeCC(0), 10, 42},
		{midi.MakeCC(0), 12, 7},
		{midi.MakeCC(0), 99, 100},
	}
	if applied := applySeed(fields, messages); applied != 3 {
		t.Errorf("applied = %d, want 3 (the duplicate controller sets two fields)", applied)
	}
	if *fields[0].Value != 42 || *fields[4].Value != 42 {
		t.Errorf("controller 10 set %d and %d, want 42 on both fields sharing it", *fields[0].Value, *fields[4].Value)
	}
	if *fields[2].Value != 7 {
		t.Errorf("controller 12 = %d, want 7", *fields[2].Value)
	}
	if *fields[1].Value != 0 || *fields[3].Value != 0 {
		t.Errorf("untouched fields changed: beta=%d delta=%d", *fields[1].Value, *fields[3].Value)
	}
}

func TestApplySeedLastMessageWins(t *testing.T) {
	fields := testFields()
	messages := [][]byte{
		{midi.MakeCC(0), 11, 1},
		{midi.MakeCC(0), 11, 99},
	}
	applySeed(fields, messages)
	if *fields[1].Value != 99 {
		t.Errorf("controller 11 = %d, want the last message's 99", *fields[1].Value)
	}
}

func TestApplySeedIgnoresChannel(t *testing.T) {
	fields := testFields()
	messages := [][]byte{
		{midi.MakeCC(9), 13, 55},
	}
	applySeed(fields, messages)
	if *fields[3].Value != 55 {
		t.Errorf("controller 13 = %d, want 55 from another channel", *fields[3].Value)
	}
}

func TestApplySeedIgnoresNonCC(t *testing.T) {
	fields := testFields()
	messages := [][]byte{
		{0x90, 60, 100},
		{midi.MakeCC(0), 10, 3},
	}
	// Controller 10 is shared by Alpha and Echo, so one
	// control change sets both fields.
	if applied := applySeed(fields, messages); applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	if *fields[0].Value != 3 || *fields[4].Value != 3 {
		t.Errorf("controller 10 = %d and %d, want 3 on both fields", *fields[0].Value, *fields[4].Value)
	}
}

func TestLoadSeedFromSMF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.mid")
	writeSeedSMF(t, path, 1, [2]int{10, 42}, [2]int{12, 7})

	fields := testFields()
	if err := loadSeed(path, fields); err != nil {
		t.Fatalf("loadSeed: %v", err)
	}
	if *fields[0].Value != 42 || *fields[2].Value != 7 {
		t.Errorf("seed set alpha=%d gamma=%d, want 42 and 7", *fields[0].Value, *fields[2].Value)
	}
}

func TestLoadSeedFromRawStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.raw")
	raw := []byte{
		midi.MakeCC(0), 11, 20,
		midi.MakeCC(0), 13, 21,
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	fields := testFields()
	if err := loadSeed(path, fields); err != nil {
		t.Fatalf("loadSeed: %v", err)
	}
	if *fields[1].Value != 20 || *fields[3].Value != 21 {
		t.Errorf("seed set beta=%d delta=%d, want 20 and 21", *fields[1].Value, *fields[3].Value)
	}
}

func TestLoadSeedRejectsSeedWithoutModelValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.mid")
	writeSeedSMF(t, path, 1, [2]int{99, 100})

	fields := testFields()
	if err := loadSeed(path, fields); err == nil {
		t.Error("a seed that sets no field was accepted")
	}
}

func TestLoadSeedRejectsUnreadableFile(t *testing.T) {
	fields := testFields()
	if err := loadSeed(filepath.Join(t.TempDir(), "missing.mid"), fields); err == nil {
		t.Error("a missing seed file was accepted")
	}
}

func TestReadSeedMessagesRejectsNeitherFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-midi")
	if err := os.WriteFile(path, []byte("definitely not midi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readSeedMessages(path); err == nil {
		t.Error("a file that is neither SMF nor raw MIDI was accepted")
	}
}

func TestReadCCRawRejectsEmptyStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.raw")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readCCRaw(path); err == nil {
		t.Error("an empty raw stream was accepted")
	}
}

func TestWriteCCSMFThenReadCCSMF(t *testing.T) {
	fields := testFields()
	*fields[0].Value = 42
	*fields[1].Value = 127
	*fields[2].Value = 0
	path := filepath.Join(t.TempDir(), "out.mid")

	if err := writeCCSMF(path, fields, "Test Model", 2); err != nil {
		t.Fatalf("writeCCSMF: %v", err)
	}
	messages, err := readCCSMF(path)
	if err != nil {
		t.Fatalf("readCCSMF: %v", err)
	}
	want := [][]byte{
		{midi.MakeCC(1), 10, 42},
		{midi.MakeCC(1), 11, 127},
		{midi.MakeCC(1), 12, 0},
		{midi.MakeCC(1), 13, 0},
		{midi.MakeCC(1), 10, 0},
	}
	if len(messages) != len(want) {
		t.Fatalf("read %d messages, want %d", len(messages), len(want))
	}
	for index, message := range messages {
		if !bytes.Equal(message, want[index]) {
			t.Errorf("message %d = %v, want %v", index, message, want[index])
		}
	}
}

func TestWriteCCSMFRejectsValueOutOfRange(t *testing.T) {
	fields := testFields()
	*fields[0].Value = 128
	path := filepath.Join(t.TempDir(), "out.mid")
	if err := writeCCSMF(path, fields, "Test Model", 1); err == nil {
		t.Error("a value of 128 was written")
	}
}

func TestWriteCCSMFRejectsChannelOutOfRange(t *testing.T) {
	fields := testFields()
	path := filepath.Join(t.TempDir(), "out.mid")
	if err := writeCCSMF(path, fields, "Test Model", 0); err == nil {
		t.Error("channel 0 was written")
	}
	if err := writeCCSMF(path, fields, "Test Model", 17); err == nil {
		t.Error("channel 17 was written")
	}
}

func TestApplyExcludes(t *testing.T) {
	rules := map[string]fieldrules.Rule{
		"Gamma": {Policy: fieldrules.PolicyExclude},
		"Delta": {Policy: fieldrules.PolicyFixed},
	}
	fields := testFields()
	kept, ignored, err := applyExcludes(fields, rules)
	if err != nil {
		t.Fatalf("applyExcludes: %v", err)
	}
	if ignored != 1 {
		t.Errorf("ignored %d fixed rules, want 1", ignored)
	}
	var names []string
	for _, field := range kept {
		names = append(names, field.Name)
	}
	// Delta's fixed rule is ignored, so it stays, in
	// declaration order after Beta.
	want := []string{"Alpha", "Beta", "Delta", "Echo"}
	if len(names) != len(want) {
		t.Fatalf("kept %v, want %v", names, want)
	}
	for index := range want {
		if names[index] != want[index] {
			t.Fatalf("kept %v, want %v in declaration order", names, want)
		}
	}
}

func TestApplyExcludesWithoutRules(t *testing.T) {
	fields := testFields()
	kept, ignored, err := applyExcludes(fields, nil)
	if err != nil {
		t.Fatalf("applyExcludes: %v", err)
	}
	if ignored != 0 || len(kept) != len(fields) {
		t.Errorf("kept %d fields, ignored %d, want %d and 0", len(kept), ignored, len(fields))
	}
}

func TestApplyExcludesRejectsUnknownName(t *testing.T) {
	rules := map[string]fieldrules.Rule{
		"NotAField": {Policy: fieldrules.PolicyExclude},
	}
	if _, _, err := applyExcludes(testFields(), rules); err == nil {
		t.Error("an unknown field name was accepted")
	}
}

func TestApplyExcludesRejectsExcludingEveryField(t *testing.T) {
	rules := map[string]fieldrules.Rule{}
	for _, field := range testFields() {
		rules[field.Name] = fieldrules.Rule{Policy: fieldrules.PolicyExclude}
	}
	if _, _, err := applyExcludes(testFields(), rules); err == nil {
		t.Error("a file that excludes every field was accepted")
	}
}

func TestLoadModelFields(t *testing.T) {
	fields, nrpnCount, _, err := loadModelFields("Meeblip SE", "")
	if err != nil {
		t.Fatalf("loadModelFields: %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("Meeblip SE has no cc fields")
	}
	if nrpnCount != 0 {
		t.Errorf("nrpn count = %d, want 0", nrpnCount)
	}
	for index := 1; index < len(fields); index++ {
		if fields[index].Controller < fields[index-1].Controller {
			t.Errorf("fields are not in declaration order at %d", index)
			break
		}
	}
}

func TestLoadModelFieldsAppliesRulesFile(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.json")
	content := `[
		{"FilterResonance": {"policy": "exclude"}},
		{"FilterCutoff": {"policy": "fixed"}}
	]`
	if err := os.WriteFile(rulesPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rules: %v", err)
	}

	fields, _, ignored, err := loadModelFields("Meeblip SE", rulesPath)
	if err != nil {
		t.Fatalf("loadModelFields: %v", err)
	}
	if ignored != 1 {
		t.Errorf("ignored %d fixed rules, want 1", ignored)
	}
	for _, field := range fields {
		if field.Name == "FilterResonance" {
			t.Error("an excluded field is still in the list")
		}
	}
	found := false
	for _, field := range fields {
		if field.Name == "FilterCutoff" {
			found = true
		}
	}
	if !found {
		t.Error("a fixed rule removed a field it should only have ignored")
	}
}

func TestNrpnFieldCount(t *testing.T) {
	params, err := cc.NewModelParams("microKORG2")
	if err != nil {
		t.Fatalf("NewModelParams: %v", err)
	}
	if count := nrpnFieldCount(params); count == 0 {
		t.Error("microKORG2 has no nrpn fields")
	}
}
