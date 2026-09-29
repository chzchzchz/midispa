package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	defaultMIDIChannelNumber = 1
	unsetRNGSeed             = int64(-1)
)

type configuration struct {
	format        string
	modelName     string
	portName      string
	seedPath      string
	output        string
	playback      string
	geneSemantics string
	midiChannel   int
	sysexSettle   time.Duration
	jsonOutput    bool
	rngSeed       int64
	settings      evolutionSettings
}

// parseConfiguration isolates flag state so configuration tests do not mutate process-global flags.
func parseConfiguration(arguments []string, output io.Writer) (configuration, error) {
	flags := flag.NewFlagSet("mutation", flag.ContinueOnError)
	flags.SetOutput(output)
	patchFormat := flags.String("format", ccFormatName, "patch format to evolve (cc, dx7-single)")
	modelName := flags.String("model", "", "model name from cc/model.go")
	portName := flags.String("port", "", "MIDI output port")
	seedPath := flags.String("seed", "", "optional SMF file containing the seed patch")
	outputPath := flags.String("output", "", "path for the latest and numbered best patches")
	playbackPath := flags.String("playback", "", "optional SMF file to play while judging")
	geneSemanticsPath := flags.String("gene-semantics", "", "optional JSON array of excluded and fixed-value gene rules")
	mutationRate := flags.Float64("mutation-rate", defaultMutationRate, "probability that a candidate is changed (0-1)")
	mutationSigma := flags.Float64("mutation-sigma", defaultMutationSigma, "standard deviation for Gaussian descendant mutations")
	mutatedGenes := flags.Int("mutated-genes", defaultMutatedGenes, "distinct genes changed per mutation; 0 chooses 1-3 automatically")
	crossoverRate := flags.Float64("crossover-rate", defaultCrossoverRate, "probability that a child combines genes from two selected patches (0-1)")
	roundSize := flags.Int("round-size", defaultRoundSize, "number of candidates judged per generation (at least 3)")
	parentDecay := flags.Float64("parent-decay", defaultParentDecay, "champion rank decay per generation")
	midiChannel := flags.Int("midi-channel", defaultMIDIChannelNumber, "MIDI channel for generated messages, probe notes, and patch files (1-16)")
	sysexSettle := flags.Duration("sysex-settle", defaultSysexSettle, "delay after a SysEx patch message before playing a note")
	jsonOutput := flags.Bool("json", false, "write a model JSON dump alongside each SMF output")
	rngSeed := flags.Int64("rng-seed", unsetRNGSeed, "optional random seed for reproducible mutation runs")
	if err := flags.Parse(arguments); err != nil {
		return configuration{}, err
	}
	if flags.NArg() != 0 {
		return configuration{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return configuration{
		format:        *patchFormat,
		modelName:     *modelName,
		portName:      *portName,
		seedPath:      *seedPath,
		output:        *outputPath,
		playback:      *playbackPath,
		geneSemantics: *geneSemanticsPath,
		midiChannel:   *midiChannel,
		sysexSettle:   *sysexSettle,
		jsonOutput:    *jsonOutput,
		rngSeed:       *rngSeed,
		settings: evolutionSettings{
			mutationRate:  *mutationRate,
			mutationSigma: *mutationSigma,
			mutatedGenes:  *mutatedGenes,
			crossoverRate: *crossoverRate,
			roundSize:     *roundSize,
			parentDecay:   *parentDecay,
		},
	}, nil
}

func validateConfiguration(config configuration) error {
	if config.portName == "" {
		return fmt.Errorf("--port is required")
	}
	if config.output == "" {
		return fmt.Errorf("--output is required")
	}
	if config.midiChannel < 1 || config.midiChannel > 16 {
		return fmt.Errorf("--midi-channel must be between 1 and 16")
	}
	if config.sysexSettle < 0 {
		return fmt.Errorf("--sysex-settle must not be negative")
	}
	return nil
}

// newPatchFactory selects the format once, so the engine, the store, and the
// seed loader all agree on which patch they are working with.
//
// Every rule that depends on what a format is lives here, asked of the format
// itself. validateConfiguration only judges flags that mean the same thing to
// all formats, so no check has to guess a format's needs from its name.
func newPatchFactory(config configuration) (patchFactory, error) {
	format, err := selectPatchFormat(config)
	if err != nil {
		return nil, err
	}
	switch {
	case format.acceptsModelName() && config.modelName == "":
		return nil, fmt.Errorf("--model is required for format %q", config.format)
	case !format.acceptsModelName() && config.modelName != "":
		return nil, fmt.Errorf("--model does not apply to format %q", config.format)
	}
	if config.jsonOutput && !format.supportsJSONDump() {
		// The JSON dump is a rebuilt CC model struct, so offering it for a
		// format without a model would write a file about a different patch.
		return nil, fmt.Errorf("--json does not apply to format %q", config.format)
	}
	if extension := format.outputExtension(); !strings.HasSuffix(config.output, extension) {
		return nil, fmt.Errorf("--output must end in %s for format %q", extension, config.format)
	}
	return format, nil
}

func selectPatchFormat(config configuration) (patchFactory, error) {
	switch config.format {
	case ccFormatName:
		return ccPatchFactory{modelName: config.modelName, midiChannel: config.midiChannel, jsonOutput: config.jsonOutput}, nil
	default:
		return newSysexPatchFactory(config.format, config.midiChannel)
	}
}
