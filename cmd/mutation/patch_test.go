package main

import (
	"math"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/chzchzchz/midispa/cc"
)

func newTestPatch(t *testing.T, modelName string) *Patch {
	t.Helper()
	patch, err := newPatchWithSemantics(modelName, nil)
	if err != nil {
		t.Fatalf("newPatchWithSemantics(%q): %v", modelName, err)
	}
	return patch
}

func TestPatchUsesCCModelMetadata(t *testing.T) {
	for _, modelName := range cc.ModelNames() {
		t.Run(modelName, func(t *testing.T) {
			patch := newTestPatch(t, modelName)
			if len(patch.genes) == 0 {
				t.Fatal("patch has no genes")
			}
			messages, err := patch.encode(0)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if len(messages) != len(patch.genes) {
				t.Fatalf("got %d messages for %d genes", len(messages), len(patch.genes))
			}
		})
	}
}

func TestPatchRejectsUnknownModel(t *testing.T) {
	if _, err := newPatchWithSemantics("unknown synth", nil); err == nil {
		t.Fatal("accepted an unknown model")
	}
}

func TestPatchAppliesSeedMessages(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	applied := patch.applyCCMessages([][]byte{
		{0xb5, 70, 11},
		{0x90, 60, 100},
		{0xb0, 70, 99},
		{0xb0, 127, 1},
	})
	if applied != 2 {
		t.Fatalf("applied %d seed values, want 2", applied)
	}
	if got := patch.genes[0].value; got != 99 {
		t.Fatalf("first gene is %d, want 99", got)
	}
}

func TestPatchAppliesDuplicateModelCCs(t *testing.T) {
	patch := newTestPatch(t, "WorldeEasyControl9")
	applied := patch.applyCCMessages([][]byte{{0xb0, 9, 55}})
	if applied != 2 {
		t.Fatalf("applied %d values for duplicate CC, want 2", applied)
	}
	for _, gene := range patch.genes {
		if patch.controllers[gene.name] == 9 && gene.value != 55 {
			t.Fatalf("duplicate CC 9 is %d, want 55", gene.value)
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
		if parent.genes[index].value != 0 {
			t.Fatalf("parent gene %d changed", index)
		}
		value := geneValues(t, mutation)[index].value
		if value < 0 || value > maxMIDIValue {
			t.Fatalf("gene %d is outside MIDI range: %d", index, value)
		}
		if value != 0 {
			changed++
		}
	}
	if changed < 1 || changed > maxMutatedGenes {
		t.Fatalf("changed %d genes, want 1-%d", changed, maxMutatedGenes)
	}
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
		if got, want := values[index].value, parent.genes[index].value; got != want {
			t.Fatalf("unchanged gene %d is %d, want %d", index, got, want)
		}
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
	if changed != settings.mutatedGenes {
		t.Fatalf("changed %d genes, want %d", changed, settings.mutatedGenes)
	}
}

func TestEvolutionSettingsValidation(t *testing.T) {
	valid := defaultEvolutionSettings()
	if err := valid.validateForModel(10); err != nil {
		t.Fatalf("valid settings: %v", err)
	}

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
			if err := settings.validateForModel(3); err == nil {
				t.Fatal("accepted invalid evolution settings")
			}
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
	if err != nil {
		t.Fatalf("geneChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(changes))
	}
	if changes[0].name != mutation.genes[0].name || changes[0].before != 10 || changes[0].after != 17 || changes[0].delta != 7 {
		t.Fatalf("unexpected positive change: %+v", changes[0])
	}
	if changes[1].name != mutation.genes[1].name || changes[1].before != 10 || changes[1].after != 6 || changes[1].delta != -4 {
		t.Fatalf("unexpected negative change: %+v", changes[1])
	}
}

func TestRandomizeUsesMIDIValues(t *testing.T) {
	patch := newTestPatch(t, "Pro VS Mini")
	patch.randomize(rand.New(rand.NewSource(2)))
	for index, gene := range patch.genes {
		value := gene.value
		if value < 0 || value > maxMIDIValue {
			t.Fatalf("gene %d is outside MIDI range: %d", index, value)
		}
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
	if err != nil {
		t.Fatalf("rankPatches: %v", err)
	}
	patch, score := ranked[0].patch, ranked[0].score
	if patch != second || score != 8 {
		t.Fatalf("selected score %d from %p, want score 8", score, patch)
	}
}

func TestMutationSelectionRejectsInvalidScores(t *testing.T) {
	patch := newTestPatch(t, "Volca Bass")
	if _, err := rankPatches([]scoredPatch{{patch: patch, score: 10}}); err == nil {
		t.Fatal("accepted score outside 0-9")
	}
	if _, err := rankPatches(nil); err == nil {
		t.Fatal("accepted an empty population")
	}
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
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
	genes := geneValues(t, engine.parent)
	if genes[0].value != 42 {
		t.Fatalf("seeded gene is %d, want 42", genes[0].value)
	}
	defaults := newTestPatch(t, model)
	exploring := 0
	for index, gene := range genes {
		if index > 0 && gene.value != defaults.genes[index].value {
			exploring++
		}
	}
	if exploring == 0 {
		t.Fatal("a partial seed left every other gene at its model default")
	}
}
