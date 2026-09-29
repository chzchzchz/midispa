package main

import (
	"math/rand"
	"strings"
	"testing"
)

func TestNewMutationInitializesParentAndRound(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	mutation, err := newMutation(newTestCCFactory("Volca Bass"), settings, rand.New(rand.NewSource(5)), nil, "")
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
	if mutation.parent == nil {
		t.Fatal("mutation has no parent patch")
	}
	if mutation.parentScore == nil || *mutation.parentScore != 0 {
		t.Fatalf("initial parent score is %v, want 0", mutation.parentScore)
	}
	if mutation.generation != 0 {
		t.Fatalf("initial generation is %d, want 0", mutation.generation)
	}
	patches := mutation.nextRound()
	if len(patches) != settings.roundSize {
		t.Fatalf("round has %d patches, want %d", len(patches), settings.roundSize)
	}
}

func TestMutationBreedsPopulationFromMultipleParents(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.mutationRate = 0
	settings.crossoverRate = 1
	settings.roundSize = 4
	parent := newTestPatch(t, "Sound Controller")
	other := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 10
		other.genes[index].value = 100
	}
	parentScore := 7
	mutation := &Mutation{
		settings:    settings,
		random:      rand.New(rand.NewSource(9)),
		parent:      parent,
		parentScore: &parentScore,
	}
	population, err := mutation.breedPopulation([]scoredPatch{
		{patch: other, score: 8},
		{patch: parent, score: 7},
	})
	if err != nil {
		t.Fatalf("breedPopulation: %v", err)
	}
	if len(population) != settings.roundSize {
		t.Fatalf("population has %d patches, want %d", len(population), settings.roundSize)
	}
	elites := geneValues(t, population[0].patch)
	if elites[0].value != 10 || geneValues(t, population[1].patch)[0].value != 100 {
		t.Fatalf("elites were not preserved: %d, %d", elites[0].value, geneValues(t, population[1].patch)[0].value)
	}
	for index := 2; index < len(population); index++ {
		for geneIndex, gene := range geneValues(t, population[index].patch) {
			if gene.value != 10 && gene.value != 100 {
				t.Fatalf("population candidate %d gene %d came from neither parent: %d", index, geneIndex, gene.value)
			}
		}
	}
}

func TestRankWeightedSelectionFavorsHigherRanks(t *testing.T) {
	low := newTestPatch(t, "Sound Controller")
	high := newTestPatch(t, "Sound Controller")
	mutation := &Mutation{random: rand.New(rand.NewSource(13))}
	ranked := []scoredPatch{
		{patch: high, score: 9},
		{patch: low, score: 0},
	}
	highSelections := 0
	for sample := 0; sample < 1000; sample++ {
		if mutation.selectParent(ranked) == high {
			highSelections++
		}
	}
	if highSelections < 800 {
		t.Fatalf("higher-ranked patch selected %d/1000 times, want at least 800", highSelections)
	}
}

func TestParentDecayAllowsAgedChampionReplacement(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	candidate := newTestPatch(t, "Sound Controller")
	parentScore := 9
	mutation := &Mutation{
		settings:    defaultEvolutionSettings(),
		parent:      parent,
		parentScore: &parentScore,
	}

	if _, _, improved := mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}}); improved {
		t.Fatal("fresh rank-9 parent was replaced by rank 8")
	}
	if mutation.parentAge != 1 {
		t.Fatalf("parent age is %d after one stale round, want 1", mutation.parentAge)
	}
	if _, _, improved := mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}}); improved {
		t.Fatal("one-generation-old rank-9 parent was replaced by rank 8")
	}
	selected, score, improved := mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}})
	if !improved || selected != candidate || score != 8 {
		t.Fatalf("aged parent was not replaced: selected=%p score=%d improved=%t", selected, score, improved)
	}
	if mutation.parentAge != 0 {
		t.Fatalf("parent age is %d after replacement, want 0", mutation.parentAge)
	}
	if got, want := mutation.parentSelectionWeight(), 9.0; got != want {
		t.Fatalf("new parent selection weight is %v, want %v", got, want)
	}
}

func TestMutationCrossoverMixesGenes(t *testing.T) {
	mutation := &Mutation{settings: defaultEvolutionSettings(), random: rand.New(rand.NewSource(10))}
	parentA := newTestPatch(t, "Sound Controller")
	parentB := newTestPatch(t, "Sound Controller")
	for index := range parentA.genes {
		parentA.genes[index].value = 10
		parentB.genes[index].value = 100
	}
	child, err := mutation.crossover(parentA, parentB, true)
	if err != nil {
		t.Fatalf("crossover: %v", err)
	}
	for index, gene := range geneValues(t, child) {
		if gene.value != 10 && gene.value != 100 {
			t.Fatalf("gene %d came from neither parent: %d", index, gene.value)
		}
	}
}

func TestCrossoverRejectsAnotherFormat(t *testing.T) {
	mutation := &Mutation{settings: defaultEvolutionSettings(), random: rand.New(rand.NewSource(10))}
	ccPatch := newTestPatch(t, "Sound Controller")
	sysexPatch := newTestSysexPatch(t)
	if _, err := mutation.crossover(ccPatch, sysexPatch, true); err == nil {
		t.Fatal("combined genes from two formats")
	}
}

func TestGaussianMutationStaysInMIDIRange(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.mutatedGenes = 2
	settings.mutationSigma = 4
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 64
	}
	mutation := &Mutation{settings: settings, random: rand.New(rand.NewSource(11))}
	child := mutation.mutatePatch(parent, true)
	changed := 0
	for index, gene := range geneValues(t, child) {
		if gene.value < 0 || gene.value > maxMIDIValue {
			t.Fatalf("gene %d is outside MIDI range: %d", index, gene.value)
		}
		if gene.value != parent.genes[index].value {
			changed++
		}
	}
	if changed != settings.mutatedGenes {
		t.Fatalf("changed %d genes, want %d", changed, settings.mutatedGenes)
	}
}

func TestMutationWriteChanges(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	candidate := parent.clone().(*Patch)
	candidate.genes[0].value = parent.genes[0].value + 20
	var output strings.Builder
	runner := mutationRunner{engine: &Mutation{parent: parent}, output: &output}
	if err := runner.writeChanges(candidate, parent); err != nil {
		t.Fatalf("writeChanges: %v", err)
	}
	want := "  " + candidate.genes[0].name + ": 0 -> 20 (delta +20)\n"
	if output.String() != want {
		t.Fatalf("change report is %q, want %q", output.String(), want)
	}

	output.Reset()
	if err := runner.writeChanges(parent.clone(), parent); err != nil {
		t.Fatalf("writeChanges: %v", err)
	}
	if output.String() != "  changed genes: none (unchanged parent)\n" {
		t.Fatalf("unchanged report is %q", output.String())
	}
}

func TestMutationAdvancePreventsRegression(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	better := newTestPatch(t, "Sound Controller")
	equal := newTestPatch(t, "Sound Controller")
	parentScore := 7
	mutation := &Mutation{parent: parent, parentScore: &parentScore}
	advance := func(candidate patch, score int) (patch, int, bool, error) {
		ranked, err := rankPatches([]scoredPatch{{patch: candidate, score: score}})
		if err != nil {
			return nil, 0, false, err
		}
		selected, selectedScore, improved := mutation.advanceRanked(ranked)
		return selected, selectedScore, improved, nil
	}

	selected, score, improved, err := advance(better, 6)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if selected != parent || score != 7 || improved {
		t.Fatalf("worse round replaced parent: selected=%p score=%d improved=%t", selected, score, improved)
	}

	selected, score, improved, err = advance(better, 8)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if selected != better || score != 8 || !improved {
		t.Fatalf("better round did not replace parent: selected=%p score=%d improved=%t", selected, score, improved)
	}

	selected, score, improved, err = advance(equal, 8)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if selected != better || score != 8 || improved {
		t.Fatalf("tie replaced earlier parent: selected=%p score=%d improved=%t", selected, score, improved)
	}
}

func TestNewMutationRejectsInvalidState(t *testing.T) {
	if _, err := newMutation(newTestCCFactory("Volca Bass"), defaultEvolutionSettings(), nil, nil, ""); err == nil {
		t.Fatal("accepted a nil random source")
	}

	settings := defaultEvolutionSettings()
	model := newTestPatch(t, "Sound Controller")
	settings.mutatedGenes = len(model.genes) + 1
	if _, err := newMutation(newTestCCFactory(model.model), settings, rand.New(rand.NewSource(6)), nil, ""); err == nil {
		t.Fatal("accepted more mutated genes than the model has")
	}
	for _, roundSize := range []int{1, 2} {
		settings := defaultEvolutionSettings()
		settings.roundSize = roundSize
		if _, err := newMutation(newTestCCFactory(model.model), settings, rand.New(rand.NewSource(7)), nil, ""); err == nil {
			t.Fatalf("accepted round size %d without room for offspring", roundSize)
		}
	}
	if _, err := newMutation(sysexPatchFactory{format: newDX7Format(0)}, defaultEvolutionSettings(), rand.New(rand.NewSource(8)), nil, ""); err == nil {
		t.Fatal("started a SysEx run without a seed")
	}
}
