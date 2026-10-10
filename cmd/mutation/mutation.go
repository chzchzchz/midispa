package main

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"

	"github.com/chzchzchz/midispa/internal/fieldrules"
)

const (
	defaultMutationRate     = 1.0
	defaultMutationSigma    = 16.0
	defaultMutatedGenes     = 0
	defaultCrossoverRate    = 0.7
	mutationExplorationRate = 0.1
	mutationRateStep        = 0.1
	defaultRoundSize        = 4
	defaultParentDecay      = 0.5
	maxMutatedGenes         = 3
	maxRoundSize            = 32
	geneticEliteCount       = 2
	minimumRoundSize        = geneticEliteCount + 1
)

// evolutionSettings controls broad exploration, local refinement, selection
// decay, population size, and output routing. A zero gene count preserves the
// original automatic range of one through maxMutatedGenes; MIDI channels remain
// one-based at the command boundary.
type evolutionSettings struct {
	mutationRate  float64
	mutationSigma float64
	mutatedGenes  int
	crossoverRate float64
	roundSize     int
	parentDecay   float64
	// mutateFilter restricts mutation and crossover to genes
	// whose names match, which the judge sets for one
	// generation at a time. It is session state rather than
	// configuration: no flag sets it and validate ignores it.
	mutateFilter *regexp.Regexp
}

func defaultEvolutionSettings() evolutionSettings {
	return evolutionSettings{
		mutationRate:  defaultMutationRate,
		mutationSigma: defaultMutationSigma,
		mutatedGenes:  defaultMutatedGenes,
		crossoverRate: defaultCrossoverRate,
		roundSize:     defaultRoundSize,
		parentDecay:   defaultParentDecay,
	}
}

func (settings evolutionSettings) validate() error {
	if math.IsNaN(settings.mutationRate) || math.IsInf(settings.mutationRate, 0) || settings.mutationRate < 0 || settings.mutationRate > 1 {
		return fmt.Errorf("--mutation-rate must be between 0 and 1")
	}
	if math.IsNaN(settings.mutationSigma) || math.IsInf(settings.mutationSigma, 0) || settings.mutationSigma < 0 {
		return fmt.Errorf("--mutation-sigma must not be negative")
	}
	if settings.mutatedGenes < 0 {
		return fmt.Errorf("--mutated-genes must not be negative")
	}
	if math.IsNaN(settings.crossoverRate) || settings.crossoverRate < 0 || settings.crossoverRate > 1 {
		return fmt.Errorf("--crossover-rate must be between 0 and 1")
	}
	if math.IsNaN(settings.parentDecay) || math.IsInf(settings.parentDecay, 0) || settings.parentDecay < 0 {
		return fmt.Errorf("--parent-decay must not be negative or non-finite")
	}
	if settings.roundSize < minimumRoundSize {
		return fmt.Errorf("--round-size must be at least %d", minimumRoundSize)
	}
	return nil
}

func (settings evolutionSettings) validateForModel(geneCount int) error {
	if err := settings.validate(); err != nil {
		return err
	}
	if settings.mutatedGenes > geneCount {
		return fmt.Errorf("--mutated-genes is %d, but model has only %d mutable genes", settings.mutatedGenes, geneCount)
	}
	return nil
}

type populationCandidate struct {
	patch     patch
	reference patch
}

type scoredPatch struct {
	patch patch
	score int
}

// Mutation owns the genetic population while a patch stays one candidate in
// whatever format the run selected. The parent is the highest-ranked patch
// seen so far and remains an elite so later generations cannot regress below
// the current champion.
type Mutation struct {
	settings    evolutionSettings
	random      *rand.Rand
	parent      patch
	parentScore *int
	parentAge   int
	population  []populationCandidate
	generation  int
}

func newMutation(factory patchFactory, settings evolutionSettings, random *rand.Rand, semantics map[string]fieldrules.Rule, seedPath string) (*Mutation, error) {
	if random == nil {
		return nil, fmt.Errorf("random source is nil")
	}
	if factory == nil {
		return nil, fmt.Errorf("patch factory is nil")
	}
	parent, err := factory.newPatch(semantics)
	if err != nil {
		return nil, err
	}
	if err := settings.validateForModel(parent.mutableGeneCount()); err != nil {
		return nil, err
	}
	// The parent is randomized first and the seed laid over it, so a seed
	// that carries only some values still leaves the rest exploring instead of
	// pinned to a model default. A SysEx seed replaces every value, so the
	// draws it discards cost nothing.
	parent.randomize(random)
	if seedPath == "" {
		if factory.requiresSeed() {
			return nil, fmt.Errorf("format %q requires --seed", parent.formatID())
		}
	} else if err := factory.loadSeed(seedPath, parent); err != nil {
		return nil, err
	}
	if err := parent.validateFixedValues(); err != nil {
		return nil, err
	}
	// A seed that cannot be written back is not a starting point, and
	// finding that out here beats discarding every candidate in generation
	// zero with no way to rank any of them.
	if err := parent.encodable(); err != nil {
		return nil, fmt.Errorf("seed cannot be written back: %w", err)
	}
	parentScore := 0
	return &Mutation{settings: settings, random: random, parent: parent, parentScore: &parentScore}, nil
}

// Small mutations make neighboring candidates easier to compare by ear than
// fully random patches would be.
func (mutation *Mutation) mutatePatch(parent patch, gaussian bool) patch {
	child := parent.clone()
	child.mutate(mutation.random, mutation.settings, gaussian)
	return child
}

// Initial candidates explore broadly from the seed or random baseline; later
// generations refine inherited values with bounded Gaussian mutations.
func (mutation *Mutation) initialPopulation() []populationCandidate {
	population := make([]populationCandidate, 0, mutation.settings.roundSize)
	population = append(population, populationCandidate{patch: mutation.parent.clone(), reference: mutation.parent})
	for len(population) < mutation.settings.roundSize {
		population = append(population, populationCandidate{
			patch:     mutation.mutatePatch(mutation.parent, false),
			reference: mutation.parent,
		})
	}
	return population
}

func (mutation *Mutation) nextRound() []populationCandidate {
	if mutation.population == nil {
		mutation.population = mutation.initialPopulation()
	}
	return mutation.population
}

// adjustRoundSize resizes the population of every generation bred
// after this one. The bounds keep a generation small enough to
// compare by ear but large enough to sample the search space.
func (mutation *Mutation) adjustRoundSize(delta int) (int, error) {
	return adjustIntSetting(&mutation.settings.roundSize, delta, minimumRoundSize, maxRoundSize, "round size")
}

// adjustMutatedGenes moves how many genes a descendant mutation
// touches. Zero keeps the automatic one through maxMutatedGenes
// range, so stepping down from one returns to it and stepping up
// from it pins the count to one.
func (mutation *Mutation) adjustMutatedGenes(delta int) (int, error) {
	mutable := mutation.parent.mutableGeneCount()
	current := mutation.settings.mutatedGenes
	updated := current + delta
	switch {
	case updated < 0, updated > mutable && current == 0:
		return current, fmt.Errorf("mutated genes is already automatic")
	case updated > mutable:
		return current, fmt.Errorf("mutated genes is already %d", current)
	}
	mutation.settings.mutatedGenes = updated
	return updated, nil
}

// adjustMutationRate scales how likely a descendant is mutated at
// all. Steps round to one decimal so repeated keys echo stable
// values instead of drifting through floating point remainders.
func (mutation *Mutation) adjustMutationRate(delta int) (float64, error) {
	current := mutation.settings.mutationRate
	updated := math.Round((current+float64(delta)*mutationRateStep)*10) / 10
	if updated < 0 || updated > 1 {
		return current, fmt.Errorf("mutation rate is already %.1f", current)
	}
	mutation.settings.mutationRate = updated
	return updated, nil
}

// adjustIntSetting moves a bounded integer setting by one step.
// The bound message names the setting and its current value,
// which is how the judge reports a key pressed at a limit.
func adjustIntSetting(current *int, delta, minimum, maximum int, name string) (int, error) {
	updated := *current + delta
	if updated < minimum || updated > maximum {
		return *current, fmt.Errorf("%s is already %d", name, *current)
	}
	*current = updated
	return updated, nil
}

// focusMutation restricts the next generation's mutations and
// crossover to genes whose names match pattern, and reports
// the mutable genes it selected. breedPopulation clears
// the restriction once the focused generation is bred.
func (mutation *Mutation) focusMutation(pattern string) ([]string, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("gene pattern: %w", err)
	}
	matched := mutation.parent.geneStore().matchingGeneNames(compiled)
	if len(matched) == 0 {
		return nil, fmt.Errorf("no mutable genes match %q", pattern)
	}
	mutation.settings.mutateFilter = compiled
	return matched, nil
}

func (mutation *Mutation) crossover(parentA, parentB patch, enabled bool) (patch, error) {
	child := parentA.clone()
	if !enabled {
		return child, nil
	}
	if err := crossoverGenes(child, parentB, mutation.random, mutation.settings.mutateFilter); err != nil {
		return nil, err
	}
	return child, nil
}

// Rank-weighted roulette selection uses the full rank as fitness. Adding one
// keeps the lowest rank eligible for occasional exploration.
func rankWeight(score int) float64 {
	return float64(score + 1)
}

func (mutation *Mutation) effectiveParentScore() float64 {
	if mutation.parentScore == nil {
		return 0
	}
	return float64(*mutation.parentScore) - float64(mutation.parentAge)*mutation.settings.parentDecay
}

func (mutation *Mutation) parentSelectionWeight() float64 {
	weight := mutation.effectiveParentScore() + 1
	if weight < 0 {
		return 0
	}
	return weight
}

func (mutation *Mutation) selectionWeight(candidate scoredPatch) float64 {
	if candidate.patch == mutation.parent {
		return mutation.parentSelectionWeight()
	}
	return rankWeight(candidate.score)
}

func (mutation *Mutation) selectParent(ranked []scoredPatch) patch {
	totalWeight := 0.0
	for _, candidate := range ranked {
		totalWeight += mutation.selectionWeight(candidate)
	}
	threshold := mutation.random.Float64() * totalWeight
	for _, candidate := range ranked {
		threshold -= mutation.selectionWeight(candidate)
		if threshold <= 0 {
			return candidate.patch
		}
	}
	return ranked[len(ranked)-1].patch
}

// rankPatches sorts scored in place; stable sorting keeps equal ranks in
// candidate order for deterministic selection.
func rankPatches(scored []scoredPatch) ([]scoredPatch, error) {
	if len(scored) == 0 {
		return nil, fmt.Errorf("cannot select from an empty mutation set")
	}
	for _, candidate := range scored {
		if candidate.patch == nil {
			return nil, fmt.Errorf("mutation is nil")
		}
		if candidate.score < 0 || candidate.score > 9 {
			return nil, fmt.Errorf("mutation score %d is outside 0-9", candidate.score)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	return scored, nil
}

// selectionPool re-adds the current parent when it is not among the ranked
// candidates so weighted selection always considers the champion.
func (mutation *Mutation) selectionPool(ranked []scoredPatch) []scoredPatch {
	pool := append([]scoredPatch(nil), ranked...)
	if mutation.parentScore == nil {
		return pool
	}
	for _, candidate := range pool {
		if candidate.patch == mutation.parent {
			return pool
		}
	}
	parent := scoredPatch{patch: mutation.parent, score: *mutation.parentScore}
	return append([]scoredPatch{parent}, pool...)
}

// Rank-weighted selection preserves diversity, crossover combines compatible
// patches, and the remaining children receive local mutations.
func (mutation *Mutation) breedPopulation(ranked []scoredPatch) ([]populationCandidate, error) {
	// A pattern focuses one generation, so clearing it here
	// keeps a stray pattern from reaching later generations.
	defer func() { mutation.settings.mutateFilter = nil }()
	pool := mutation.selectionPool(ranked)
	population := make([]populationCandidate, 0, mutation.settings.roundSize)
	appendElite := func(candidate patch) {
		if len(population) >= mutation.settings.roundSize {
			return
		}
		population = append(population, populationCandidate{patch: candidate.clone(), reference: candidate})
	}
	appendElite(mutation.parent)
	for _, candidate := range ranked {
		if len(population) >= geneticEliteCount {
			break
		}
		if candidate.patch != mutation.parent {
			appendElite(candidate.patch)
		}
	}
	for len(population) < mutation.settings.roundSize {
		parentA := mutation.selectParent(pool)
		parentB := mutation.selectParent(pool)
		child, err := mutation.crossover(parentA, parentB, mutation.random.Float64() < mutation.settings.crossoverRate)
		if err != nil {
			return nil, err
		}
		child = mutation.mutatePatch(child, true)
		population = append(population, populationCandidate{patch: child, reference: parentA})
	}
	return population, nil
}

// Equal scores retain the earlier patch until the parent decays enough for a
// new candidate to replace it.
func (mutation *Mutation) advanceRanked(ranked []scoredPatch) (patch, int, bool) {
	candidate := ranked[0]
	if mutation.parentScore != nil {
		effectiveParentScore := mutation.effectiveParentScore()
		replace := float64(candidate.score) > effectiveParentScore
		if !replace && mutation.settings.parentDecay > 0 && mutation.parentAge > 0 && float64(candidate.score) >= effectiveParentScore {
			replace = true
		}
		if !replace {
			mutation.parentAge++
			return mutation.parent, *mutation.parentScore, false
		}
	}
	mutation.parent = candidate.patch
	parentScore := candidate.score
	mutation.parentScore = &parentScore
	mutation.parentAge = 0
	return mutation.parent, parentScore, true
}
