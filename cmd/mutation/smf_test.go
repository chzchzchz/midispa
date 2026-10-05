package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"
)

func TestPatchSMFRoundTrip(t *testing.T) {
	source := newTestPatch(t, "Sound Controller")
	for index := range source.genes {
		source.genes[index].value = 20 + index
	}
	path := filepath.Join(t.TempDir(), "patch.mid")
	require.NoError(t, writePatchSMFForChannel(path, source, defaultMIDIChannelNumber), "writePatchSMF")

	loaded := newTestPatch(t, "Sound Controller")
	require.NoError(t, loadSeedPatch(path, loaded), "loadSeedPatch")
	for index := range source.genes {
		assert.Equal(t, source.genes[index].value, loaded.genes[index].value, "gene %d", index)
	}
}

func TestPatchSMFUsesConfiguredChannel(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	path := filepath.Join(t.TempDir(), "patch.mid")
	require.NoError(t, writePatchSMFForChannel(path, patch, 10), "writePatchSMFForChannel")
	messages, err := readPatchSMF(path)
	require.NoError(t, err, "readPatchSMF")
	for index, message := range messages {
		assert.Equal(t, 9, midi.Channel(message[0]), "message %d does not use MIDI channel 10", index)
	}
	assert.Error(t, writePatchSMFForChannel(path, patch, 0), "accepted MIDI channel below 1")
	assert.Error(t, writePatchSMFForChannel(path, patch, 17), "accepted MIDI channel above 16")
}

func TestWriteGenerationKeepsNumberedAndLatestOutputs(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	for index := range patch.genes {
		patch.genes[index].value = index + 1
	}
	outputPath := filepath.Join(t.TempDir(), "best.mid")
	store := smfPatchStore{outputPath: outputPath, midiChannel: defaultMIDIChannelNumber}
	require.NoError(t, store.save(patch, 3), "save")

	generationData, err := os.ReadFile(outputPath + ".0003")
	require.NoError(t, err, "read generation")
	latestData, err := os.ReadFile(outputPath)
	require.NoError(t, err, "read latest")
	assert.Equal(t, generationData, latestData, "latest output differs from numbered generation")
}

func TestLoadSeedPatchRejectsUnrelatedModel(t *testing.T) {
	source := newTestPatch(t, "Sound Controller")
	for index := range source.genes {
		source.genes[index].value = index
	}
	path := filepath.Join(t.TempDir(), "patch.mid")
	require.NoError(t, writePatchSMFForChannel(path, source, defaultMIDIChannelNumber), "writePatchSMF")

	target := newTestPatch(t, "Volca Bass")
	assert.Error(t, loadSeedPatch(path, target), "accepted a seed with no matching CC values")
}

func TestPatchStoreRejectsInvalidArguments(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	assert.Error(t, (smfPatchStore{}).save(patch, 0), "accepted an empty output path")
	assert.Error(t, (smfPatchStore{outputPath: "best.mid"}).save(patch, -1), "accepted a negative generation")
}

// writePartialSeedSMF writes a seed carrying only the given controller values.
// A recording made on a device that does not expose every controller produces
// exactly this kind of file.
func writePartialSeedSMF(t *testing.T, path string, messages [][]byte) {
	t.Helper()
	var writeErr error
	err := smfwriter.WriteFile(path, func(midiWriter smf.Writer) {
		midiWriter.SetDelta(0)
		for _, message := range messages {
			control := channel.Channel(0).ControlChange(message[1], message[2])
			if writeErr = midiWriter.Write(control); writeErr != nil {
				return
			}
		}
	}, smfwriter.NumTracks(1), smfwriter.TimeFormat(smf.MetricTicks(patchTicksPerQuarter)))
	require.NoError(t, writeErr, "write partial seed")
	if err != nil {
		require.ErrorIs(t, err, smf.ErrFinished, "write partial seed")
	}
}
