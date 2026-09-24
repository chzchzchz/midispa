package main

import (
	"fmt"
	"os"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"
	"gitlab.com/gomidi/midi/smf/smfwriter"
)

const (
	patchBPM              = 120
	patchTicksPerQuarter  = 96
	generationDigitsWidth = 4
)

// smfPatchStore preserves numbered generation history and refreshes the latest
// output after each completed generation.
type smfPatchStore struct {
	outputPath  string
	midiChannel int
	jsonOutput  bool
}

func (store smfPatchStore) path() string {
	return store.outputPath
}

func (store smfPatchStore) save(patch *Patch, generation int) error {
	if store.outputPath == "" {
		return fmt.Errorf("output path is empty")
	}
	if generation < 0 {
		return fmt.Errorf("generation %d is negative", generation)
	}
	generationPath := fmt.Sprintf("%s.%0*d", store.outputPath, generationDigitsWidth, generation)
	if err := store.write(generationPath, patch); err != nil {
		return fmt.Errorf("write generation: %w", err)
	}
	if err := store.write(store.outputPath, patch); err != nil {
		return fmt.Errorf("write latest output: %w", err)
	}
	return nil
}

func (store smfPatchStore) write(path string, patch *Patch) error {
	if err := writePatchSMFForChannel(path, patch, store.midiChannel); err != nil {
		return err
	}
	if store.jsonOutput {
		return writePatchJSON(path+".json", patch)
	}
	return nil
}

// Seed patches need every controller, including initial CC 7, 10, 91, and 93
// values that track.NewPattern intentionally omits.
func readPatchSMF(path string) ([][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := smfreader.New(file)
	if err := reader.ReadHeader(); err != nil {
		return nil, err
	}

	messages := make([][]byte, 0)
	for {
		message, err := reader.Read()
		if err == smf.ErrFinished {
			break
		}
		if err != nil {
			return nil, err
		}
		if control, ok := message.(channel.ControlChange); ok {
			messages = append(messages, append([]byte(nil), control.Raw()...))
		}
	}
	return messages, nil
}

func loadSeedPatch(path string, patch *Patch) error {
	messages, err := readPatchSMF(path)
	if err != nil {
		return fmt.Errorf("read seed %q: %w", path, err)
	}
	if patch.applyCCMessages(messages) == 0 && patch.hasSeedConfigurableGenes() {
		return fmt.Errorf("seed %q has no CC values for model %q", path, patch.model)
	}
	return nil
}

// MIDI channel numbers remain one-based at the user boundary and are converted
// only where the MIDI library requires zero-based values.
func writePatchSMFForChannel(path string, patch *Patch, channelNumber int) error {
	if channelNumber < 1 || channelNumber > 16 {
		return fmt.Errorf("MIDI channel %d is outside 1-16", channelNumber)
	}
	channelIndex := channelNumber - 1
	messages, messageErr := patch.controlChanges(channelIndex)
	if messageErr != nil {
		return messageErr
	}

	var writeErr error
	err := smfwriter.WriteFile(path, func(midiWriter smf.Writer) {
		if writeErr = midiWriter.Write(meta.Instrument(patch.model)); writeErr != nil {
			return
		}
		timeSignature := meta.TimeSig{
			Numerator:                4,
			Denominator:              4,
			ClocksPerClick:           24,
			DemiSemiQuaverPerQuarter: 8,
		}
		if writeErr = midiWriter.Write(timeSignature); writeErr != nil {
			return
		}
		if writeErr = midiWriter.Write(meta.BPM(patchBPM)); writeErr != nil {
			return
		}
		midiWriter.SetDelta(0)
		for _, message := range messages {
			control := channel.Channel(channelIndex).ControlChange(message[1], message[2])
			if writeErr = midiWriter.Write(control); writeErr != nil {
				return
			}
		}
	}, smfwriter.NumTracks(1), smfwriter.TimeFormat(smf.MetricTicks(patchTicksPerQuarter)))
	if writeErr != nil {
		return writeErr
	}
	if err == smf.ErrFinished {
		return nil
	}
	return err
}
