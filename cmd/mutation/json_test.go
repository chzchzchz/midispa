package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchModelJSONUsesModelFieldNames(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	patch.genes[1].value = 42
	path := filepath.Join(t.TempDir(), "patch.json")
	require.NoError(t, writePatchJSON(path, patch), "writePatchJSON")

	data, err := os.ReadFile(path)
	require.NoError(t, err, "read JSON")
	var model map[string]any
	require.NoError(t, json.Unmarshal(data, &model), "decode JSON")
	value, ok := model["SoundController2"].(float64)
	require.True(t, ok, "JSON does not contain model field SoundController2: %s", data)
	assert.Equal(t, 42, int(value), "SoundController2")
}

func TestPatchStoreWritesJSONSidecars(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	outputPath := filepath.Join(t.TempDir(), "best.mid")
	store := smfPatchStore{
		outputPath:  outputPath,
		midiChannel: defaultMIDIChannelNumber,
		jsonOutput:  true,
	}
	require.NoError(t, store.save(patch, 3), "save")
	for _, path := range []string{outputPath + ".0003.json", outputPath + ".json"} {
		assert.FileExists(t, path, "no JSON sidecar was written")
	}
}
