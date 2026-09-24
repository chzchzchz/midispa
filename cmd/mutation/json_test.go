package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchModelJSONUsesModelFieldNames(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	patch.genes[1].value = 42
	path := filepath.Join(t.TempDir(), "patch.json")
	if err := writePatchJSON(path, patch); err != nil {
		t.Fatalf("writePatchJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON: %v", err)
	}
	var model map[string]any
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	value, ok := model["SoundController2"].(float64)
	if !ok {
		t.Fatalf("JSON does not contain model field SoundController2: %s", data)
	}
	if int(value) != 42 {
		t.Fatalf("SoundController2 is %v, want 42", value)
	}
}

func TestPatchStoreWritesJSONSidecars(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	outputPath := filepath.Join(t.TempDir(), "best.mid")
	store := smfPatchStore{
		outputPath:  outputPath,
		midiChannel: defaultMIDIChannelNumber,
		jsonOutput:  true,
	}
	if err := store.save(patch, 3); err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, path := range []string{outputPath + ".0003.json", outputPath + ".json"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}
