package main

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMutationInitializesParentAndRound(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	mutation, err := newMutation(newTestCCFactory("Volca Bass"), settings, rand.New(rand.NewSource(5)), nil, "")
	require.NoError(t, err, "newMutation")
	assert.NotNil(t, mutation.parent, "mutation has no parent patch")
	require.NotNil(t, mutation.parentScore, "mutation has no parent score")
	assert.Zero(t, *mutation.parentScore, "initial parent score")
	assert.Zero(t, mutation.generation, "initial generation")
	assert.Len(t, mutation.nextRound(), settings.roundSize, "round size")
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
	require.NoError(t, err, "breedPopulation")
	require.Len(t, population, settings.roundSize, "population size")
	elites := geneValues(t, population[0].patch)
	assert.Equal(t, 10, elites[0].value, "first elite was not preserved")
	assert.Equal(t, 100, geneValues(t, population[1].patch)[0].value, "second elite was not preserved")
	for index := 2; index < len(population); index++ {
		for geneIndex, gene := range geneValues(t, population[index].patch) {
			assert.Contains(t, []int{10, 100}, gene.value,
				"population candidate %d gene %d came from neither parent", index, geneIndex)
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
	assert.GreaterOrEqual(t, highSelections, 800, "higher-ranked patch selected times out of 1000")
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

	_, _, improved := mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}})
	assert.False(t, improved, "fresh rank-9 parent was replaced by rank 8")
	assert.Equal(t, 1, mutation.parentAge, "parent age after one stale round")
	_, _, improved = mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}})
	assert.False(t, improved, "one-generation-old rank-9 parent was replaced by rank 8")
	selected, score, improved := mutation.advanceRanked([]scoredPatch{{patch: candidate, score: 8}})
	assert.True(t, improved, "aged parent was not replaced")
	assert.Same(t, candidate, selected, "aged parent was not replaced")
	assert.Equal(t, 8, score, "aged parent was not replaced")
	assert.Zero(t, mutation.parentAge, "parent age after replacement")
	assert.Equal(t, 9.0, mutation.parentSelectionWeight(), "new parent selection weight")
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
	require.NoError(t, err, "crossover")
	for index, gene := range geneValues(t, child) {
		assert.Contains(t, []int{10, 100}, gene.value, "gene %d came from neither parent", index)
	}
}

func TestCrossoverRejectsAnotherFormat(t *testing.T) {
	mutation := &Mutation{settings: defaultEvolutionSettings(), random: rand.New(rand.NewSource(10))}
	ccPatch := newTestPatch(t, "Sound Controller")
	sysexPatch := newTestSysexPatch(t)
	_, err := mutation.crossover(ccPatch, sysexPatch, true)
	assert.Error(t, err, "combined genes from two formats")
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
		assert.GreaterOrEqual(t, gene.value, 0, "gene %d is below the MIDI range", index)
		assert.LessOrEqual(t, gene.value, maxMIDIValue, "gene %d is above the MIDI range", index)
		if gene.value != parent.genes[index].value {
			changed++
		}
	}
	assert.Equal(t, settings.mutatedGenes, changed, "changed genes")
}

func TestMutationWriteChanges(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	candidate := parent.clone().(*Patch)
	candidate.genes[0].value = parent.genes[0].value + 20
	var output strings.Builder
	runner := mutationRunner{engine: &Mutation{parent: parent}, output: &output}
	require.NoError(t, runner.writeChanges(candidate, parent), "writeChanges")
	want := "  " + candidate.genes[0].name + ": 0 -> 20 (delta +20)\n"
	assert.Equal(t, want, output.String(), "change report")

	output.Reset()
	require.NoError(t, runner.writeChanges(parent.clone(), parent), "writeChanges")
	assert.Equal(t, "  changed genes: none (unchanged parent)\n", output.String(), "unchanged report")
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
	require.NoError(t, err, "advance")
	assert.Same(t, parent, selected, "worse round replaced parent")
	assert.Equal(t, 7, score, "worse round replaced parent")
	assert.False(t, improved, "worse round replaced parent")

	selected, score, improved, err = advance(better, 8)
	require.NoError(t, err, "advance")
	assert.Same(t, better, selected, "better round did not replace parent")
	assert.Equal(t, 8, score, "better round did not replace parent")
	assert.True(t, improved, "better round did not replace parent")

	selected, score, improved, err = advance(equal, 8)
	require.NoError(t, err, "advance")
	assert.Same(t, better, selected, "tie replaced earlier parent")
	assert.Equal(t, 8, score, "tie replaced earlier parent")
	assert.False(t, improved, "tie replaced earlier parent")
}

func TestNewMutationRejectsInvalidState(t *testing.T) {
	_, err := newMutation(newTestCCFactory("Volca Bass"), defaultEvolutionSettings(), nil, nil, "")
	assert.Error(t, err, "accepted a nil random source")

	settings := defaultEvolutionSettings()
	model := newTestPatch(t, "Sound Controller")
	settings.mutatedGenes = len(model.genes) + 1
	_, err = newMutation(newTestCCFactory(model.model), settings, rand.New(rand.NewSource(6)), nil, "")
	assert.Error(t, err, "accepted more mutated genes than the model has")
	for _, roundSize := range []int{1, 2} {
		settings := defaultEvolutionSettings()
		settings.roundSize = roundSize
		_, err = newMutation(newTestCCFactory(model.model), settings, rand.New(rand.NewSource(7)), nil, "")
		assert.Error(t, err, "accepted round size %d without room for offspring", roundSize)
	}
	_, err = newMutation(sysexPatchFactory{format: newDX7Format(0)}, defaultEvolutionSettings(), rand.New(rand.NewSource(8)), nil, "")
	assert.Error(t, err, "started a SysEx run without a seed")
}
