package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/midi"
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
	if err := writePatchSMFForChannel(path, source, defaultMIDIChannelNumber); err != nil {
		t.Fatalf("writePatchSMF: %v", err)
	}

	loaded := newTestPatch(t, "Sound Controller")
	if err := loadSeedPatch(path, loaded); err != nil {
		t.Fatalf("loadSeedPatch: %v", err)
	}
	for index := range source.genes {
		if got, want := loaded.genes[index].value, source.genes[index].value; got != want {
			t.Fatalf("gene %d is %d, want %d", index, got, want)
		}
	}
}

func TestPatchSMFUsesConfiguredChannel(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	path := filepath.Join(t.TempDir(), "patch.mid")
	if err := writePatchSMFForChannel(path, patch, 10); err != nil {
		t.Fatalf("writePatchSMFForChannel: %v", err)
	}
	messages, err := readPatchSMF(path)
	if err != nil {
		t.Fatalf("readPatchSMF: %v", err)
	}
	for index, message := range messages {
		if channel := midi.Channel(message[0]); channel != 9 {
			t.Fatalf("message %d uses MIDI channel %d, want 10", index, channel+1)
		}
	}
	if err := writePatchSMFForChannel(path, patch, 0); err == nil {
		t.Fatal("accepted MIDI channel below 1")
	}
	if err := writePatchSMFForChannel(path, patch, 17); err == nil {
		t.Fatal("accepted MIDI channel above 16")
	}
}

func TestWriteGenerationKeepsNumberedAndLatestOutputs(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	for index := range patch.genes {
		patch.genes[index].value = index + 1
	}
	outputPath := filepath.Join(t.TempDir(), "best.mid")
	store := smfPatchStore{outputPath: outputPath, midiChannel: defaultMIDIChannelNumber}
	if err := store.save(patch, 3); err != nil {
		t.Fatalf("save: %v", err)
	}

	generationPath := outputPath + ".0003"
	generationData, err := os.ReadFile(generationPath)
	if err != nil {
		t.Fatalf("read generation: %v", err)
	}
	latestData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read latest: %v", err)
	}
	if !bytes.Equal(generationData, latestData) {
		t.Fatal("latest output differs from numbered generation")
	}
}

func TestLoadSeedPatchRejectsUnrelatedModel(t *testing.T) {
	source := newTestPatch(t, "Sound Controller")
	for index := range source.genes {
		source.genes[index].value = index
	}
	path := filepath.Join(t.TempDir(), "patch.mid")
	if err := writePatchSMFForChannel(path, source, defaultMIDIChannelNumber); err != nil {
		t.Fatalf("writePatchSMF: %v", err)
	}

	target := newTestPatch(t, "Volca Bass")
	if err := loadSeedPatch(path, target); err == nil {
		t.Fatal("accepted a seed with no matching CC values")
	}
}

func TestPatchStoreRejectsInvalidArguments(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	if err := (smfPatchStore{}).save(patch, 0); err == nil {
		t.Fatal("accepted an empty output path")
	}
	if err := (smfPatchStore{outputPath: "best.mid"}).save(patch, -1); err == nil {
		t.Fatal("accepted a negative generation")
	}
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
	if writeErr != nil {
		t.Fatalf("write partial seed: %v", writeErr)
	}
	if err != nil && !errors.Is(err, smf.ErrFinished) {
		t.Fatalf("write partial seed: %v", err)
	}
}
