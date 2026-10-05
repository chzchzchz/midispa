package main

import (
	"math"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/cc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPatch(t *testing.T, modelName string) *Patch {
	t.Helper()
	patch, err := newPatchWithSemantics(modelName, nil)
	require.NoError(t, err, "newPatchWithSemantics(%q)", modelName)
	return patch
}

func TestPatchUsesCCModelMetadata(t *testing.T) {
	for _, modelName := range cc.ModelNames() {
		t.Run(modelName, func(t *testing.T) {
			patch := newTestPatch(t, modelName)
			assert.NotEmpty(t, patch.genes, "patch has no genes")
			messages, err := patch.encode(0)
			require.NoError(t, err, "encode")
			assert.Len(t, messages, len(patch.genes), "got %d messages for %d genes", len(messages), len(patch.genes))
		})
	}
}

func TestPatchRejectsUnknownModel(t *testing.T) {
	_, err := newPatchWithSemantics("unknown synth", nil)
	assert.Error(t, err, "accepted an unknown model")
}

func TestPatchAppliesSeedMessages(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	applied := patch.applyCCMessages([][]byte{
		{0xb5, 70, 11},
		{0x90, 60, 100},
		{0xb0, 70, 99},
		{0xb0, 127, 1},
	})
	assert.Equal(t, 2, applied, "applied seed values")
	assert.Equal(t, 99, patch.genes[0].value, "first gene")
}

func TestPatchAppliesDuplicateModelCCs(t *testing.T) {
	patch := newTestPatch(t, "WorldeEasyControl9")
	applied := patch.applyCCMessages([][]byte{{0xb0, 9, 55}})
	assert.Equal(t, 2, applied, "applied values for duplicate CC")
	for _, gene := range patch.genes {
		if patch.controllers[gene.name] == 9 {
			assert.Equal(t, 55, gene.value, "duplicate CC 9")
		}
	}
}

func TestMutationChangesOnlyCopiedGenes(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 0
	}
	mutationEngine := newTestMutation(parent, defaultEvolutionSettings(), 1)
	mutation := mutationEngine.mutatePatch(parent, false)
	changed := 0
	for index := range parent.genes {
		assert.Zero(t, parent.genes[index].value, "parent gene %d changed", index)
		value := geneValues(t, mutation)[index].value
		assert.GreaterOrEqual(t, value, 0, "gene %d is below the MIDI range", index)
		assert.LessOrEqual(t, value, maxMIDIValue, "gene %d is above the MIDI range", index)
		if value != 0 {
			changed++
		}
	}
	assert.GreaterOrEqual(t, changed, 1, "changed genes, want 1-%d", maxMutatedGenes)
	assert.LessOrEqual(t, changed, maxMutatedGenes, "changed genes, want 1-%d", maxMutatedGenes)
}

func TestMutationRateCanKeepCandidateUnchanged(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = index
	}
	settings := defaultEvolutionSettings()
	settings.mutationRate = 0
	mutationEngine := newTestMutation(parent, settings, 3)
	mutation := mutationEngine.mutatePatch(parent, false)
	values := geneValues(t, mutation)
	for index := range parent.genes {
		assert.Equal(t, parent.genes[index].value, values[index].value, "unchanged gene %d", index)
	}
}

func TestMutatedGenesControlsExactCount(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 0
	}
	settings := defaultEvolutionSettings()
	settings.mutatedGenes = 2
	mutationEngine := newTestMutation(parent, settings, 4)
	mutation := mutationEngine.mutatePatch(parent, false)
	values := geneValues(t, mutation)
	changed := 0
	for index := range parent.genes {
		if values[index].value != parent.genes[index].value {
			changed++
		}
	}
	assert.Equal(t, settings.mutatedGenes, changed, "changed genes")
}

func TestEvolutionSettingsValidation(t *testing.T) {
	valid := defaultEvolutionSettings()
	assert.NoError(t, valid.validateForModel(10), "valid settings")

	tests := []struct {
		name   string
		update func(*evolutionSettings)
	}{
		{name: "negative rate", update: func(settings *evolutionSettings) { settings.mutationRate = -0.1 }},
		{name: "high rate", update: func(settings *evolutionSettings) { settings.mutationRate = 1.1 }},
		{name: "NaN rate", update: func(settings *evolutionSettings) { settings.mutationRate = math.NaN() }},
		{name: "negative sigma", update: func(settings *evolutionSettings) { settings.mutationSigma = -1 }},
		{name: "NaN sigma", update: func(settings *evolutionSettings) { settings.mutationSigma = math.NaN() }},
		{name: "infinite sigma", update: func(settings *evolutionSettings) { settings.mutationSigma = math.Inf(1) }},
		{name: "negative genes", update: func(settings *evolutionSettings) { settings.mutatedGenes = -1 }},
		{name: "negative crossover", update: func(settings *evolutionSettings) { settings.crossoverRate = -0.1 }},
		{name: "high crossover", update: func(settings *evolutionSettings) { settings.crossoverRate = 1.1 }},
		{name: "NaN crossover", update: func(settings *evolutionSettings) { settings.crossoverRate = math.NaN() }},
		{name: "negative parent decay", update: func(settings *evolutionSettings) { settings.parentDecay = -0.1 }},
		{name: "NaN parent decay", update: func(settings *evolutionSettings) { settings.parentDecay = math.NaN() }},
		{name: "zero round", update: func(settings *evolutionSettings) { settings.roundSize = 0 }},
		{name: "too many genes", update: func(settings *evolutionSettings) { settings.mutatedGenes = 4 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := defaultEvolutionSettings()
			test.update(&settings)
			assert.Error(t, settings.validateForModel(3), "accepted invalid evolution settings")
		})
	}
}

func TestPatchChangesFromParent(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 10
	}
	mutation := parent.clone().(*Patch)
	mutation.genes[0].value = 17
	mutation.genes[1].value = 6
	changes, err := geneChanges(mutation, parent)
	require.NoError(t, err, "geneChanges")
	require.Len(t, changes, 2, "got %d changes, want 2", len(changes))
	assert.Equal(t, mutation.genes[0].name, changes[0].name, "unexpected positive change: %+v", changes[0])
	assert.Equal(t, 10, changes[0].before, "unexpected positive change: %+v", changes[0])
	assert.Equal(t, 17, changes[0].after, "unexpected positive change: %+v", changes[0])
	assert.Equal(t, 7, changes[0].delta, "unexpected positive change: %+v", changes[0])
	assert.Equal(t, mutation.genes[1].name, changes[1].name, "unexpected negative change: %+v", changes[1])
	assert.Equal(t, 10, changes[1].before, "unexpected negative change: %+v", changes[1])
	assert.Equal(t, 6, changes[1].after, "unexpected negative change: %+v", changes[1])
	assert.Equal(t, -4, changes[1].delta, "unexpected negative change: %+v", changes[1])
}

func TestRandomizeUsesMIDIValues(t *testing.T) {
	patch := newTestPatch(t, "Pro VS Mini")
	patch.randomize(rand.New(rand.NewSource(2)))
	for index, gene := range patch.genes {
		assert.GreaterOrEqual(t, gene.value, 0, "gene %d is below the MIDI range", index)
		assert.LessOrEqual(t, gene.value, maxMIDIValue, "gene %d is above the MIDI range", index)
	}
}

func TestMutationSelectsHighestScore(t *testing.T) {
	first := newTestPatch(t, "Volca Bass")
	second := newTestPatch(t, "Volca Bass")
	third := newTestPatch(t, "Volca Bass")
	ranked, err := rankPatches([]scoredPatch{
		{patch: first, score: 3},
		{patch: second, score: 8},
		{patch: third, score: 5},
	})
	require.NoError(t, err, "rankPatches")
	assert.Same(t, second, ranked[0].patch, "selected patch")
	assert.Equal(t, 8, ranked[0].score, "selected score")
}

func TestMutationSelectionRejectsInvalidScores(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	_, err := rankPatches([]scoredPatch{{patch: patch, score: 10}})
	assert.Error(t, err, "accepted score outside 0-9")
	_, err = rankPatches(nil)
	assert.Error(t, err, "accepted an empty population")
}

func TestPartialSeedKeepsARandomBaseline(t *testing.T) {
	// A seed overlays a randomized parent, so controllers the file does not
	// carry keep exploring instead of sitting at a model default.
	const model = "Sound Controller"
	seeded := newTestPatch(t, model)
	controller := seeded.controllers[seeded.genes[0].name]
	seedPath := filepath.Join(t.TempDir(), "seed.mid")
	writePartialSeedSMF(t, seedPath, [][]byte{{0xb0, byte(controller), 42}})

	engine, err := newMutation(newTestCCFactory(model), defaultEvolutionSettings(), rand.New(rand.NewSource(21)), nil, seedPath)
	require.NoError(t, err, "newMutation")
	genes := geneValues(t, engine.parent)
	require.NotEmpty(t, genes, "no genes after seeding")
	assert.Equal(t, 42, genes[0].value, "seeded gene")
	defaults := newTestPatch(t, model)
	exploring := 0
	for index, gene := range genes {
		if index > 0 && gene.value != defaults.genes[index].value {
			exploring++
		}
	}
	assert.NotZero(t, exploring, "a partial seed left every other gene at its model default")
}
