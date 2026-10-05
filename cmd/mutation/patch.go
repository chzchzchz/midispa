package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/midi"
)

const maxMIDIValue = 127

// Patch is one CC patch for a selected model. Its gene store is
// independently allocated so a parent and the patches mutated from it cannot
// alias one another. The controller table is read-only after construction and
// is shared by every clone, so a gene stays a name, a value, a policy, and a
// domain.
type Patch struct {
	model       string
	controllers map[string]int
	patchGenes
}

func newPatchWithSemantics(modelName string, semantics map[string]geneSemantic) (*Patch, error) {
	params, err := cc.NewModelParams(modelName)
	if err != nil {
		return nil, err
	}
	fields, err := cc.ControlFields(params)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("model %q has no cc fields", modelName)
	}
	genes := make([]gene, 0, len(fields))
	controllers := make(map[string]int, len(fields))
	knownNames := make(map[string]bool, len(fields))
	for _, field := range fields {
		knownNames[field.Name] = true
		controllers[field.Name] = field.Controller
		rule, hasRule := semantics[field.Name]
		if hasRule && rule.Policy == excludePolicy {
			continue
		}
		policy := genePolicyMutable
		if hasRule {
			policy = genePolicyFixed
			if rule.Value == nil {
				policy = genePolicyFixedPendingSeed
			}
		}
		// A controller value is a MIDI data byte, so its bound comes from the
		// data model rather than from a struct tag.
		current := gene{
			name:   field.Name,
			value:  *field.Value,
			policy: policy,
			domain: midiValueDomain(),
		}
		if hasRule && rule.Value != nil {
			current.value = *rule.Value
		}
		genes = append(genes, current)
	}
	for name := range semantics {
		if !knownNames[name] {
			return nil, fmt.Errorf("gene %q is not present in model %q", name, modelName)
		}
	}
	return &Patch{
		model:       modelName,
		controllers: controllers,
		patchGenes:  patchGenes{format: ccFormatID, genes: genes},
	}, nil
}

func (patch *Patch) clone() patch {
	return &Patch{model: patch.model, controllers: patch.controllers, patchGenes: *patch.cloneGenes()}
}

// Messages are applied in recording order, so the last occurrence of a
// controller has the same precedence it would have on the hardware.
func (patch *Patch) applyCCMessages(messages [][]byte) int {
	applied := 0
	for _, message := range messages {
		if len(message) < 3 || !midi.IsCC(message[0]) {
			continue
		}
		for index := range patch.genes {
			if patch.controllers[patch.genes[index].name] != int(message[1]) {
				continue
			}
			if patch.genes[index].policy == genePolicyFixed {
				continue
			}
			patch.genes[index].value = int(message[2])
			if patch.genes[index].policy == genePolicyFixedPendingSeed {
				patch.genes[index].policy = genePolicyFixed
			}
			applied++
		}
	}
	return applied
}

// encodable always succeeds: every controller value comes from a 7-bit MIDI
// data byte, so there is no way to build a CC patch the instrument could not
// be sent. The check exists for formats whose records are wider than their
// message.
func (patch *Patch) encodable() error { return nil }

// encode turns the patch into the channel messages that reproduce it on the
// instrument. The channel is zero-based here and one-based at the flag.
func (patch *Patch) encode(channelIndex int) ([][]byte, error) {
	if channelIndex < 0 || channelIndex > 15 {
		return nil, fmt.Errorf("MIDI channel %d is outside 1-16", channelIndex+1)
	}
	messages := make([][]byte, 0, len(patch.genes))
	status := midi.MakeCC(channelIndex)
	for index := range patch.genes {
		value := patch.genes[index].value
		if value < 0 || value > maxMIDIValue {
			return nil, fmt.Errorf("CC %d value %d is outside 0-127", patch.controllers[patch.genes[index].name], value)
		}
		messages = append(messages, []byte{status, byte(patch.controllers[patch.genes[index].name]), byte(value)})
	}
	return messages, nil
}
