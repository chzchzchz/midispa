package main

import (
	"fmt"
	"math/rand"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/midi"
)

const maxMIDIValue = 127

type genePolicy uint8

const (
	genePolicyMutable genePolicy = iota
	genePolicyFixedPendingSeed
	genePolicyFixed
)

// Gene policy encodes the valid lifecycle states so a fixed gene cannot be
// confused with a mutable gene before its seed value is applied.
type patchGene struct {
	name       string
	controller int
	value      int
	policy     genePolicy
}

// Patch is one MIDI patch. Its model storage is independently allocated so a
// parent and the patches mutated from it cannot alias one another.
type Patch struct {
	model string
	genes []patchGene
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
	genes := make([]patchGene, 0, len(fields))
	knownNames := make(map[string]bool, len(fields))
	for _, field := range fields {
		knownNames[field.Name] = true
		rule, hasRule := semantics[field.Name]
		if hasRule && rule.Policy == "exclude" {
			continue
		}
		policy := genePolicyMutable
		if hasRule {
			policy = genePolicyFixed
			if rule.Value == nil {
				policy = genePolicyFixedPendingSeed
			}
		}
		gene := patchGene{
			name:       field.Name,
			controller: field.Controller,
			value:      *field.Value,
			policy:     policy,
		}
		if hasRule && rule.Value != nil {
			gene.value = *rule.Value
		}
		genes = append(genes, gene)
	}
	for name := range semantics {
		if !knownNames[name] {
			return nil, fmt.Errorf("gene %q is not present in model %q", name, modelName)
		}
	}
	return &Patch{model: modelName, genes: genes}, nil
}

type geneChange struct {
	name   string
	before int
	after  int
	delta  int
}

func (patch *Patch) changesFrom(parent *Patch) []geneChange {
	changes := make([]geneChange, 0)
	for index := range patch.genes {
		before := parent.genes[index].value
		after := patch.genes[index].value
		if before == after {
			continue
		}
		changes = append(changes, geneChange{
			name:   patch.genes[index].name,
			before: before,
			after:  after,
			delta:  after - before,
		})
	}
	return changes
}

func (patch *Patch) randomize(random *rand.Rand) {
	for index := range patch.genes {
		if patch.genes[index].policy == genePolicyMutable {
			patch.genes[index].value = random.Intn(maxMIDIValue + 1)
		}
	}
}

func (patch *Patch) clone() *Patch {
	return &Patch{
		model: patch.model,
		genes: append([]patchGene(nil), patch.genes...),
	}
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
			if patch.genes[index].controller != int(message[1]) {
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

func (patch *Patch) mutableGeneCount() int {
	count := 0
	for _, gene := range patch.genes {
		if gene.policy == genePolicyMutable {
			count++
		}
	}
	return count
}

func (patch *Patch) hasSeedConfigurableGenes() bool {
	for _, gene := range patch.genes {
		if gene.policy != genePolicyFixed {
			return true
		}
	}
	return false
}

func (patch *Patch) validateFixedValues() error {
	for _, gene := range patch.genes {
		if gene.policy == genePolicyFixedPendingSeed {
			return fmt.Errorf("fixed gene %q requires an explicit value or a seed value", gene.name)
		}
	}
	return nil
}

func (patch *Patch) controlChanges(channel int) ([][]byte, error) {
	if channel < 0 || channel > 15 {
		return nil, fmt.Errorf("MIDI channel %d is outside 1-16", channel+1)
	}
	messages := make([][]byte, 0, len(patch.genes))
	status := midi.MakeCC(channel)
	for index := range patch.genes {
		value := patch.genes[index].value
		if value < 0 || value > maxMIDIValue {
			return nil, fmt.Errorf("CC %d value %d is outside 0-127", patch.genes[index].controller, value)
		}
		messages = append(messages, []byte{status, byte(patch.genes[index].controller), byte(value)})
	}
	return messages, nil
}
