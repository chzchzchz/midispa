package main

import (
	"context"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMutation(parent patch, settings evolutionSettings, randomSeed int64) *Mutation {
	parentScore := 0
	return &Mutation{
		settings:    settings,
		random:      rand.New(rand.NewSource(randomSeed)),
		parent:      parent,
		parentScore: &parentScore,
	}
}

// geneValues reads a candidate's gene storage through the format-agnostic
// patch interface, which is what the engine produces after breeding.
func geneValues(t *testing.T, candidate patch) []gene {
	t.Helper()
	require.NotNil(t, candidate, "candidate is nil")
	return candidate.geneStore().genes
}

func newTestCCFactory(modelName string) patchFactory {
	return ccPatchFactory{modelName: modelName, midiChannel: defaultMIDIChannelNumber}
}

func runTestMutation(ctx context.Context, mutation *Mutation, player *midiPlayer, outputPath string, input io.Reader, output io.Writer) error {
	runner := mutationRunner{
		engine:     mutation,
		auditioner: midiAuditioner{player: player},
		store:      smfPatchStore{outputPath: outputPath, midiChannel: defaultMIDIChannelNumber},
		input:      input,
		output:     output,
	}
	return runner.run(ctx)
}

func TestMutationSelectsHighestRankAndWritesGenerationZero(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = index
	}
	expected := newTestMutation(parent, defaultEvolutionSettings(), 7)
	expectedPatches := expected.nextRound()

	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: -1})
	var output strings.Builder
	outputPath := filepath.Join(t.TempDir(), "best.mid")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 7)
	require.NoError(t, runTestMutation(
		context.Background(),
		mutation,
		player,
		outputPath,
		strings.NewReader("9 0 8 7"),
		&output,
	), "run")
	assert.Contains(t, output.String(), "selected candidate with rank 9", "missing selected score")
	assert.Contains(t, output.String(), "SoundController", "missing mutation change report")
	assert.Contains(t, output.String(), "delta ", "missing mutation change report")
	assert.FileExists(t, outputPath+".0000", "generation zero was not written")

	written := newTestPatch(t, "Sound Controller")
	require.NoError(t, loadSeedPatch(outputPath, written), "load selected patch")
	expectedGenes := geneValues(t, expectedPatches[0].patch)
	for index := range expectedGenes {
		assert.Equal(t, expectedGenes[index].value, written.genes[index].value, "selected gene %d", index)
	}
}

func TestMutationUsesConfiguredRoundSize(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	settings.mutatedGenes = 1
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: -1})
	var output strings.Builder
	mutation := newTestMutation(parent, settings, 8)
	err := runTestMutation(
		context.Background(),
		mutation,
		player,
		filepath.Join(t.TempDir(), "best.mid"),
		strings.NewReader("9 8 7 6"),
		&output,
	)
	require.NoError(t, err, "run")
	assert.Contains(t, output.String(), "candidate 3/3", "population did not continue into a second generation")
	assert.Contains(t, output.String(), "generation 0001", "population did not continue into a second generation")
}

func TestTerminationErrorExitCode(t *testing.T) {
	tests := []struct {
		name   string
		signal os.Signal
		want   int
	}{
		{name: "interrupt", signal: os.Interrupt, want: 130},
		{name: "terminate", signal: syscall.SIGTERM, want: 143},
		{name: "hangup", signal: syscall.SIGHUP, want: 129},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, terminationError{signal: test.signal}.exitCode())
		})
	}
}

func TestParseConfiguration(t *testing.T) {
	config, err := parseConfiguration([]string{
		"--model", "Volca Bass",
		"--port", "MIDI Out",
		"--output", "best.mid",
		"--round-size", "6",
		"--mutation-rate", "0.8",
		"--parent-decay", "0.25",
		"--rng-seed", "1234",
		"--json",
	}, io.Discard)
	require.NoError(t, err, "parseConfiguration")
	assert.Equal(t, "Volca Bass", config.modelName, "unexpected configuration: %+v", config)
	assert.Equal(t, "MIDI Out", config.portName, "unexpected configuration: %+v", config)
	assert.Equal(t, "best.mid", config.output, "unexpected configuration: %+v", config)
	assert.Equal(t, 6, config.settings.roundSize, "unexpected evolution settings: %+v", config.settings)
	assert.Equal(t, 0.8, config.settings.mutationRate, "unexpected evolution settings: %+v", config.settings)
	assert.Equal(t, 0.25, config.settings.parentDecay, "unexpected evolution settings: %+v", config.settings)
	assert.Equal(t, defaultMIDIChannelNumber, config.midiChannel, "unexpected evolution settings: %+v", config.settings)
	assert.Equal(t, int64(1234), config.rngSeed, "unexpected evolution settings: %+v", config.settings)
	assert.True(t, config.jsonOutput, "unexpected evolution settings: %+v", config.settings)
}

func TestParseConfigurationDefaultsRNGSeed(t *testing.T) {
	config, err := parseConfiguration([]string{
		"--model", "Volca Bass",
		"--port", "MIDI Out",
		"--output", "best.mid",
	}, io.Discard)
	require.NoError(t, err, "parseConfiguration")
	assert.Equal(t, unsetRNGSeed, config.rngSeed, "default RNG seed is not unset")
}

func TestParseConfigurationRejectsPositionalArguments(t *testing.T) {
	_, err := parseConfiguration([]string{"unexpected"}, io.Discard)
	assert.Error(t, err, "accepted positional arguments")
}

func TestParseConfigurationReadsDumpExcludes(t *testing.T) {
	config, err := parseConfiguration([]string{"--model", "Volca Bass", "--dump-excludes", "excludes.json"}, io.Discard)
	require.NoError(t, err, "parseConfiguration")
	// Nothing else is asked for, because a dump needs nothing else.
	assert.Equal(t, "excludes.json", config.dumpExcludes, "unexpected configuration: %+v", config)
	assert.Empty(t, config.portName, "unexpected configuration: %+v", config)
	assert.Empty(t, config.output, "unexpected configuration: %+v", config)
}

func TestValidateConfiguration(t *testing.T) {
	valid := configuration{
		format:      ccFormatName,
		modelName:   "Volca Bass",
		portName:    "MIDI Out",
		output:      "best.mid",
		midiChannel: defaultMIDIChannelNumber,
		settings:    defaultEvolutionSettings(),
	}
	assert.NoError(t, validateConfiguration(valid), "rejected a valid CC configuration")

	sysex := configuration{
		format:      dx7SingleFormatName,
		portName:    "MIDI Out",
		output:      "best.syx",
		seedPath:    "voice.syx",
		midiChannel: defaultMIDIChannelNumber,
		settings:    defaultEvolutionSettings(),
	}
	assert.NoError(t, validateConfiguration(sysex), "rejected a valid SysEx configuration")

	tests := []struct {
		name   string
		config configuration
	}{
		{name: "port", config: configuration{format: ccFormatName, modelName: "Volca Bass", output: "best.mid"}},
		{name: "output", config: configuration{format: ccFormatName, modelName: "Volca Bass", portName: "MIDI Out"}},
		{name: "midi channel", config: configuration{format: ccFormatName, modelName: "Volca Bass", portName: "MIDI Out", output: "best.mid", midiChannel: 17}},
		{name: "negative settle", config: configuration{format: ccFormatName, modelName: "Volca Bass", portName: "MIDI Out", output: "best.mid", sysexSettle: -1}},
		{name: "model with sysex", config: configuration{format: dx7SingleFormatName, modelName: "Volca Bass", portName: "MIDI Out", output: "best.syx"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, validateConfiguration(test.config), "accepted incomplete configuration")
		})
	}
}

func TestJudgeCommandsTuneLaterGenerations(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: -1})
	var output strings.Builder
	mutation := newTestMutation(parent, settings, 9)
	require.NoError(t, runTestMutation(
		context.Background(),
		mutation,
		player,
		filepath.Join(t.TempDir(), "best.mid"),
		strings.NewReader("9 9 ] 9 < 9 9 9 9 9 9"),
		&output,
	), "run")
	assert.Equal(t, 4, mutation.settings.roundSize, "round size key did not resize the next generation")
	assert.Equal(t, 0.9, mutation.settings.mutationRate, "rate key did not lower the next generation's mutation rate")
	assert.Contains(t, output.String(), "candidate 3/3", "first generation kept its size")
	assert.Contains(t, output.String(), "candidate 4/4", "second generation used the new size")
	assert.Contains(t, output.String(), "generation 0000: round size 3, mutated genes automatic, mutation rate 1.0", "missing initial settings line")
	assert.Contains(t, output.String(), "generation 0001: round size 4, mutated genes automatic, mutation rate 1.0", "missing round size settings line")
	assert.Contains(t, output.String(), "generation 0002: round size 4, mutated genes automatic, mutation rate 0.9", "missing retuned settings line")
	assert.Contains(t, output.String(), "round size 4", "missing round size report")
	assert.Contains(t, output.String(), "mutation rate 0.9", "missing mutation rate report")
}
