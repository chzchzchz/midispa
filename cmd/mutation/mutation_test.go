package main

import (
	"fmt"
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

func TestAdjustRoundSizeClampsAtBounds(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 3)

	size, err := mutation.adjustRoundSize(1)
	require.NoError(t, err, "adjustRoundSize")
	assert.Equal(t, defaultRoundSize+1, size, "round size step")
	assert.Equal(t, defaultRoundSize+1, mutation.settings.roundSize, "round size was not stored")

	mutation.settings.roundSize = maxRoundSize
	_, err = mutation.adjustRoundSize(1)
	assert.EqualError(t, err, "round size is already 32", "missing upper bound report")

	mutation.settings.roundSize = minimumRoundSize
	_, err = mutation.adjustRoundSize(-1)
	assert.EqualError(t, err, "round size is already 3", "missing lower bound report")
}

func TestAdjustMutatedGenesWalksAutomaticToMutableCount(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 3)
	mutable := parent.mutableGeneCount()

	count, err := mutation.adjustMutatedGenes(1)
	require.NoError(t, err, "adjustMutatedGenes")
	assert.Equal(t, 1, count, "stepping up from automatic pinned the count to one")

	mutation.settings.mutatedGenes = 1
	count, err = mutation.adjustMutatedGenes(-1)
	require.NoError(t, err, "adjustMutatedGenes")
	assert.Zero(t, count, "stepping down from one returned to automatic")

	mutation.settings.mutatedGenes = mutable
	_, err = mutation.adjustMutatedGenes(1)
	assert.EqualError(t, err, fmt.Sprintf("mutated genes is already %d", mutable), "missing upper bound report")

	mutation.settings.mutatedGenes = 0
	_, err = mutation.adjustMutatedGenes(-1)
	assert.EqualError(t, err, "mutated genes is already automatic", "missing automatic bound report")
}

func TestAdjustMutationRateStepsAndClamps(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 3)

	rate, err := mutation.adjustMutationRate(-1)
	require.NoError(t, err, "adjustMutationRate")
	assert.Equal(t, 0.9, rate, "mutation rate step")
	assert.Equal(t, 0.9, mutation.settings.mutationRate, "mutation rate was not stored")

	rate, err = mutation.adjustMutationRate(1)
	require.NoError(t, err, "adjustMutationRate")
	assert.Equal(t, 1.0, rate, "mutation rate step back to the ceiling")

	mutation.settings.mutationRate = 0
	_, err = mutation.adjustMutationRate(-1)
	assert.EqualError(t, err, "mutation rate is already 0.0", "missing lower bound report")

	mutation.settings.mutationRate = 1
	_, err = mutation.adjustMutationRate(1)
	assert.EqualError(t, err, "mutation rate is already 1.0", "missing upper bound report")
}

func TestFocusMutationSelectsMatchingGenes(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 3)

	matched, err := mutation.focusMutation("SoundController5")
	require.NoError(t, err, "focusMutation")
	assert.Equal(t, []string{"SoundController5"}, matched, "pattern selected unexpected genes")
	require.NotNil(t, mutation.settings.mutateFilter, "filter was not stored")
	assert.Equal(t, "SoundController5", mutation.settings.mutateFilter.String(), "unexpected filter pattern")
}

func TestFocusMutationRejectsUnusablePatterns(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	mutation := newTestMutation(parent, defaultEvolutionSettings(), 3)

	_, err := mutation.focusMutation("99[")
	assert.Error(t, err, "accepted an invalid pattern")
	assert.Nil(t, mutation.settings.mutateFilter, "invalid pattern stored a filter")

	_, err = mutation.focusMutation("NoSuchGene")
	assert.EqualError(t, err, `no mutable genes match "NoSuchGene"`, "missing zero match report")
	assert.Nil(t, mutation.settings.mutateFilter, "zero match pattern stored a filter")
}

func TestBreedPopulationFocusesAndClearsThePattern(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	for index := range parent.genes {
		parent.genes[index].value = 50
	}
	settings := defaultEvolutionSettings()
	settings.mutationRate = 1
	settings.mutatedGenes = 10
	settings.crossoverRate = 0
	mutation := newTestMutation(parent, settings, 4)

	_, err := mutation.focusMutation("SoundController5")
	require.NoError(t, err, "focusMutation")
	population, err := mutation.breedPopulation([]scoredPatch{{patch: parent.clone(), score: 9}})
	require.NoError(t, err, "breedPopulation")
	assert.Nil(t, mutation.settings.mutateFilter, "breedPopulation did not clear the pattern")

	for index, candidate := range population {
		for geneIndex, gene := range geneValues(t, candidate.patch) {
			if geneIndex == 4 {
				continue
			}
			assert.Equal(t, 50, gene.value, "candidate %d gene %d changed outside the pattern", index, geneIndex)
		}
	}
}
