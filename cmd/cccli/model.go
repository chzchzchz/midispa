package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/internal/fieldrules"
	"github.com/chzchzchz/midispa/midi"
)

const maxMIDIValue = 127

// loadModelFields builds the model's ordered CC fields and applies
// the field rules. Excluded fields are gone from the returned list,
// so they are never shown, seeded or saved. The second result counts
// the model's nrpn-tagged fields, which the editor reports but does
// not render; the third counts the fixed rules it ignored.
func loadModelFields(modelName, rulesPath string) ([]cc.ControlField, int, int, error) {
	params, err := cc.NewModelParams(modelName)
	if err != nil {
		return nil, 0, 0, err
	}
	fields, err := cc.ControlFields(params)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(fields) == 0 {
		return nil, 0, 0, fmt.Errorf("model %q has no cc fields", modelName)
	}
	nrpnCount := nrpnFieldCount(params)
	var rules map[string]fieldrules.Rule
	if rulesPath != "" {
		if rules, err = fieldrules.Load(rulesPath); err != nil {
			return nil, 0, 0, err
		}
	}
	fields, ignoredFixed, err := applyExcludes(fields, rules)
	if err != nil {
		return nil, 0, 0, err
	}
	return fields, nrpnCount, ignoredFixed, nil
}

// applyExcludes drops every field named by an exclude rule and counts
// the fixed rules it ignored: an editor has no fixed values, so a
// field a rule would freeze stays editable and the rule is reported
// rather than applied. A name the model does not declare is a
// misspelling and an error, as is a file that would leave the model
// with no fields at all.
func applyExcludes(fields []cc.ControlField, rules map[string]fieldrules.Rule) ([]cc.ControlField, int, error) {
	if len(rules) == 0 {
		return fields, 0, nil
	}
	known := make(map[string]bool, len(fields))
	for _, field := range fields {
		known[field.Name] = true
	}
	ignored := 0
	for name, rule := range rules {
		if !known[name] {
			return nil, 0, fmt.Errorf("field %q is not present in the model", name)
		}
		if rule.Policy == fieldrules.PolicyFixed {
			ignored++
		}
	}
	kept := make([]cc.ControlField, 0, len(fields))
	for _, field := range fields {
		if rule, hasRule := rules[field.Name]; hasRule && rule.Policy == fieldrules.PolicyExclude {
			continue
		}
		kept = append(kept, field)
	}
	if len(kept) == 0 {
		return nil, 0, fmt.Errorf("field rules exclude every field of the model")
	}
	return kept, ignored, nil
}

// applySeed applies the seed's messages in recording order, so the
// last message for a controller wins, the same precedence the
// hardware gives a stream of control changes. Matching is on the
// controller number only, and controllers the model does not know
// are ignored.
func applySeed(fields []cc.ControlField, messages [][]byte) int {
	applied := 0
	for _, message := range messages {
		if len(message) < 3 || !midi.IsCC(message[0]) {
			continue
		}
		for index := range fields {
			if fields[index].Controller != int(message[1]) {
				continue
			}
			*fields[index].Value = int(message[2])
			applied++
		}
	}
	return applied
}

// loadSeed reads a seed file, trying SMF framing first and a raw MIDI
// stream second, and applies it to the fields. A seed that sets no
// field is rejected, the same check a mutation run makes, because a
// model left at zero is indistinguishable from an editor that failed
// to load.
func loadSeed(path string, fields []cc.ControlField) error {
	messages, err := readSeedMessages(path)
	if err != nil {
		return fmt.Errorf("read seed %q: %w", path, err)
	}
	if applySeed(fields, messages) == 0 {
		return fmt.Errorf("seed %q has no CC values for the model", path)
	}
	return nil
}

// nrpnFieldCount counts the model's nrpn-tagged fields, which the
// editor does not render: each needs three messages to set, so it
// belongs to a later change than this one.
func nrpnFieldCount(params any) int {
	nrpnFields, err := cc.NRPNFields(params)
	if err != nil {
		return 0
	}
	return len(nrpnFields)
}
