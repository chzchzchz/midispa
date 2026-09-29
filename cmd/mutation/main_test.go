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
	if candidate == nil {
		t.Fatal("candidate is nil")
	}
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
	if err := runTestMutation(
		context.Background(),
		mutation,
		player,
		outputPath,
		strings.NewReader("9 0 8 7"),
		&output,
	); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(output.String(), "selected candidate with rank 9") {
		t.Fatalf("missing selected score in %q", output.String())
	}
	if !strings.Contains(output.String(), "SoundController") || !strings.Contains(output.String(), "delta ") {
		t.Fatalf("missing mutation change report in %q", output.String())
	}
	if _, err := os.Stat(outputPath + ".0000"); err != nil {
		t.Fatalf("stat generation zero: %v", err)
	}

	written := newTestPatch(t, "Sound Controller")
	if err := loadSeedPatch(outputPath, written); err != nil {
		t.Fatalf("load selected patch: %v", err)
	}
	expectedGenes := geneValues(t, expectedPatches[0].patch)
	for index := range expectedGenes {
		got := written.genes[index].value
		want := expectedGenes[index].value
		if got != want {
			t.Fatalf("selected gene %d is %d, want %d", index, got, want)
		}
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
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(output.String(), "candidate 3/3") || !strings.Contains(output.String(), "generation 0001") {
		t.Fatalf("population did not continue into a second generation: %q", output.String())
	}
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
			err := terminationError{signal: test.signal}
			if got := err.exitCode(); got != test.want {
				t.Fatalf("exit code is %d, want %d", got, test.want)
			}
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
	if err != nil {
		t.Fatalf("parseConfiguration: %v", err)
	}
	if config.modelName != "Volca Bass" || config.portName != "MIDI Out" || config.output != "best.mid" {
		t.Fatalf("unexpected configuration: %+v", config)
	}
	if config.settings.roundSize != 6 || config.settings.mutationRate != 0.8 || config.settings.parentDecay != 0.25 || config.midiChannel != defaultMIDIChannelNumber || config.rngSeed != 1234 || !config.jsonOutput {
		t.Fatalf("unexpected evolution settings: %+v", config.settings)
	}
}

func TestParseConfigurationDefaultsRNGSeed(t *testing.T) {
	config, err := parseConfiguration([]string{
		"--model", "Volca Bass",
		"--port", "MIDI Out",
		"--output", "best.mid",
	}, io.Discard)
	if err != nil {
		t.Fatalf("parseConfiguration: %v", err)
	}
	if config.rngSeed != unsetRNGSeed {
		t.Fatalf("default RNG seed is %d, want unset", config.rngSeed)
	}
}

func TestParseConfigurationRejectsPositionalArguments(t *testing.T) {
	if _, err := parseConfiguration([]string{"unexpected"}, io.Discard); err == nil {
		t.Fatal("accepted positional arguments")
	}
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
	if err := validateConfiguration(valid); err != nil {
		t.Fatalf("valid configuration: %v", err)
	}
	sysex := configuration{
		format:      dx7SingleFormatName,
		portName:    "MIDI Out",
		output:      "best.syx",
		seedPath:    "voice.syx",
		midiChannel: defaultMIDIChannelNumber,
		settings:    defaultEvolutionSettings(),
	}
	if err := validateConfiguration(sysex); err != nil {
		t.Fatalf("valid SysEx configuration: %v", err)
	}

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
			if err := validateConfiguration(test.config); err == nil {
				t.Fatal("accepted incomplete configuration")
			}
		})
	}

}
