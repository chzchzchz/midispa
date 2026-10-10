package main

import (
	"bytes"
	"errors"
	"flag"
	"testing"
)

func TestParseConfigurationDefaults(t *testing.T) {
	config, err := parseConfiguration(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if config.outputMIDIChannel != defaultMIDIChannelNumber {
		t.Errorf("output midi channel = %d, want default %d", config.outputMIDIChannel, defaultMIDIChannelNumber)
	}
	if config.filterMIDIChannel != 0 {
		t.Errorf("filter midi channel = %d, want 0, every channel", config.filterMIDIChannel)
	}
	if config.modelName != "" || config.portName != "" || config.inputPath != "" || config.output != "" || config.fieldRules != "" {
		t.Errorf("unexpected non-empty defaults: %+v", config)
	}
}

func TestParseConfigurationValues(t *testing.T) {
	config, err := parseConfiguration([]string{
		"--model", "Meeblip SE",
		"--port", "hw:1",
		"--output-midi-channel", "10",
		"--filter-midi-channel", "5",
		"--input", "seed.mid",
		"--output", "out.mid",
		"--field-rules", "rules.json",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := configuration{
		modelName:         "Meeblip SE",
		portName:          "hw:1",
		outputMIDIChannel: 10,
		filterMIDIChannel: 5,
		inputPath:         "seed.mid",
		output:            "out.mid",
		fieldRules:        "rules.json",
	}
	if config != want {
		t.Errorf("config = %+v, want %+v", config, want)
	}
}

func TestParseConfigurationRejectsPositionalArguments(t *testing.T) {
	if _, err := parseConfiguration([]string{"Meeblip SE"}, &bytes.Buffer{}); err == nil {
		t.Error("positional argument accepted")
	}
}

func TestParseConfigurationHelpIsNotAnError(t *testing.T) {
	_, err := parseConfiguration([]string{"--help"}, &bytes.Buffer{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("err = %v, want flag.ErrHelp", err)
	}
}

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config configuration
		want   string
	}{
		{"no model", configuration{portName: "hw:1"}, "--model is required"},
		{"no port", configuration{modelName: "Meeblip SE"}, "--port is required"},
		{"output channel below range", configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: 0}, "--output-midi-channel must be between 1 and 16"},
		{"output channel above range", configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: 17}, "--output-midi-channel must be between 1 and 16"},
		{"filter channel below range", configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: 1, filterMIDIChannel: -1}, "--filter-midi-channel must be between 1 and 16, or 0 to accept every channel"},
		{"filter channel above range", configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: 1, filterMIDIChannel: 17}, "--filter-midi-channel must be between 1 and 16, or 0 to accept every channel"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateConfiguration(test.config)
			if err == nil {
				t.Fatalf("validate accepted %+v", test.config)
			}
			if err.Error() != test.want {
				t.Errorf("err = %q, want %q", err, test.want)
			}
		})
	}
}

func TestValidateConfigurationAcceptsBounds(t *testing.T) {
	for _, channel := range []int{1, 16} {
		config := configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: channel}
		if err := validateConfiguration(config); err != nil {
			t.Errorf("channel %d: %v", channel, err)
		}
	}
	for _, channel := range []int{0, 1, 16} {
		config := configuration{modelName: "Meeblip SE", portName: "hw:1", outputMIDIChannel: 1, filterMIDIChannel: channel}
		if err := validateConfiguration(config); err != nil {
			t.Errorf("filter channel %d: %v", channel, err)
		}
	}
}

func TestValidateConfigurationRejectsUnknownModel(t *testing.T) {
	config := configuration{modelName: "No Such Model", portName: "hw:1", outputMIDIChannel: 1}
	fields, _, _, err := loadModelFields(config.modelName, "")
	if err == nil {
		t.Fatalf("unknown model loaded %d fields", len(fields))
	}
}
