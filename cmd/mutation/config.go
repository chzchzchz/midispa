package main

import (
	"flag"
	"fmt"
	"io"
)

const (
	defaultMIDIChannelNumber = 1
	unsetRNGSeed             = int64(-1)
)

type configuration struct {
	modelName     string
	portName      string
	seedPath      string
	output        string
	playback      string
	geneSemantics string
	midiChannel   int
	jsonOutput    bool
	rngSeed       int64
	settings      evolutionSettings
}

// parseConfiguration isolates flag state so configuration tests do not mutate process-global flags.
func parseConfiguration(arguments []string, output io.Writer) (configuration, error) {
	flags := flag.NewFlagSet("mutation", flag.ContinueOnError)
	flags.SetOutput(output)
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
	midiChannel := flags.Int("midi-channel", defaultMIDIChannelNumber, "MIDI channel for generated CCs, probe notes, and patch files (1-16)")
	jsonOutput := flags.Bool("json", false, "write a model JSON dump alongside each SMF output")
	rngSeed := flags.Int64("rng-seed", unsetRNGSeed, "optional random seed for reproducible mutation runs")
	if err := flags.Parse(arguments); err != nil {
		return configuration{}, err
	}
	if flags.NArg() != 0 {
		return configuration{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return configuration{
		modelName:     *modelName,
		portName:      *portName,
		seedPath:      *seedPath,
		output:        *outputPath,
		playback:      *playbackPath,
		geneSemantics: *geneSemanticsPath,
		midiChannel:   *midiChannel,
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
	if config.modelName == "" {
		return fmt.Errorf("--model is required")
	}
	if config.portName == "" {
		return fmt.Errorf("--port is required")
	}
	if config.output == "" {
		return fmt.Errorf("--output is required")
	}
	if config.midiChannel < 1 || config.midiChannel > 16 {
		return fmt.Errorf("--midi-channel must be between 1 and 16")
	}
	return nil
}
