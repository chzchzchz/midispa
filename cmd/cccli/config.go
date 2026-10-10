package main

import (
	"flag"
	"fmt"
	"io"
)

const defaultMIDIChannelNumber = 1

// configuration is the parsed command line. The field rules path
// names the same file format cmd/mutation reads with
// --gene-semantics, so one rule file serves both tools.
type configuration struct {
	modelName         string
	portName          string
	outputMIDIChannel int
	filterMIDIChannel int
	inputPath         string
	output            string
	fieldRules        string
}

// parseConfiguration isolates flag state so configuration tests do
// not mutate process-global flags.
func parseConfiguration(arguments []string, output io.Writer) (configuration, error) {
	flags := flag.NewFlagSet("cccli", flag.ContinueOnError)
	flags.SetOutput(output)
	modelName := flags.String("model", "", "model name from cc/model.go")
	portName := flags.String("port", "", "MIDI output port")
	outputMIDIChannel := flags.Int("output-midi-channel", defaultMIDIChannelNumber, "MIDI channel control changes are sent on (1-16)")
	filterMIDIChannel := flags.Int("filter-midi-channel", 0, "optional MIDI channel a seed file is filtered to (1-16), or 0 to accept every channel")
	inputPath := flags.String("input", "", "optional seed file, SMF or raw MIDI, whose control changes populate the model")
	outputPath := flags.String("output", "", "path the save command writes the model to")
	fieldRulesPath := flags.String("field-rules", "", "optional JSON array of field rules, same format as cmd/mutation -gene-semantics")
	if err := flags.Parse(arguments); err != nil {
		return configuration{}, err
	}
	if flags.NArg() != 0 {
		return configuration{}, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return configuration{
		modelName:         *modelName,
		portName:          *portName,
		outputMIDIChannel: *outputMIDIChannel,
		filterMIDIChannel: *filterMIDIChannel,
		inputPath:         *inputPath,
		output:            *outputPath,
		fieldRules:        *fieldRulesPath,
	}, nil
}

// validateConfiguration judges the flags that mean the same thing
// whatever the model is. Everything that depends on which model is
// selected is checked once the model is loaded, so a misspelling
// names the thing it misspelled.
func validateConfiguration(config configuration) error {
	if config.modelName == "" {
		return fmt.Errorf("--model is required")
	}
	if config.portName == "" {
		return fmt.Errorf("--port is required")
	}
	if config.outputMIDIChannel < 1 || config.outputMIDIChannel > 16 {
		return fmt.Errorf("--output-midi-channel must be between 1 and 16")
	}
	// Zero means no filtering, so a filter is either off or a
	// channel of its own.
	if config.filterMIDIChannel < 0 || config.filterMIDIChannel > 16 {
		return fmt.Errorf("--filter-midi-channel must be between 1 and 16, or 0 to accept every channel")
	}
	return nil
}
