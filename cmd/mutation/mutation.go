package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

const (
	defaultMutationRate     = 1.0
	defaultMutationSigma    = 16.0
	defaultMutatedGenes     = 0
	defaultCrossoverRate    = 0.7
	mutationExplorationRate = 0.1
	defaultRoundSize        = 4
	defaultParentDecay     = 0.5
	maxMutatedGenes         = 3
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
	patch     *Patch
	reference *Patch
}

type scoredPatch struct {
	patch *Patch
	score int
}

// Mutation owns the genetic population while Patch remains one MIDI patch.
// The parent is the highest-ranked patch seen so far and remains an elite so
// later generations cannot regress below the current champion.
type Mutation struct {
	settings    evolutionSettings
	random      *rand.Rand
	parent      *Patch
	parentScore *int
	parentAge   int
	population  []populationCandidate
	generation  int
}

func newMutationWithSemantics(modelName string, settings evolutionSettings, random *rand.Rand, semantics map[string]geneSemantic, seedPath string) (*Mutation, error) {
	if random == nil {
		return nil, fmt.Errorf("random source is nil")
	}
	parent, err := newPatchWithSemantics(modelName, semantics)
	if err != nil {
		return nil, err
	}
	if err := settings.validateForModel(parent.mutableGeneCount()); err != nil {
		return nil, err
	}
	parent.randomize(random)
	if seedPath != "" {
		if err := loadSeedPatch(seedPath, parent); err != nil {
			return nil, err
		}
	}
	if err := parent.validateFixedValues(); err != nil {
		return nil, err
	}
	parentScore := 0
	return &Mutation{settings: settings, random: random, parent: parent, parentScore: &parentScore}, nil
}

// Small mutations make neighboring candidates easier to compare by ear than
// fully random patches would be.
func (mutation *Mutation) mutatePatch(parent *Patch, gaussian bool) *Patch {
	patch := parent.clone()
	if mutation.random.Float64() >= mutation.settings.mutationRate {
		return patch
	}

	mutableGenes := make([]int, 0, len(patch.genes))
	for index, gene := range patch.genes {
		if gene.policy == genePolicyMutable {
			mutableGenes = append(mutableGenes, index)
		}
	}
	if len(mutableGenes) == 0 {
		return patch
	}
	mutationCount := mutation.settings.mutatedGenes
	if mutationCount == 0 {
		maximum := maxMutatedGenes
		if len(mutableGenes) < maximum {
			maximum = len(mutableGenes)
		}
		mutationCount = 1 + mutation.random.Intn(maximum)
	}
	if mutationCount > len(mutableGenes) {
		mutationCount = len(mutableGenes)
	}
	for _, selected := range mutation.random.Perm(len(mutableGenes))[:mutationCount] {
		gene := &patch.genes[mutableGenes[selected]]
		var value int
		if gaussian && mutation.random.Float64() >= mutationExplorationRate {
			value = gene.value + int(math.Round(mutation.random.NormFloat64()*mutation.settings.mutationSigma))
			if value < 0 {
				value = 0
			}
			if value > maxMIDIValue {
				value = maxMIDIValue
			}
			if value == gene.value {
				if value > 0 {
					value--
				} else {
					value++
				}
			}
		} else {
			value = mutation.random.Intn(maxMIDIValue + 1)
			for gene.value == value {
				value = mutation.random.Intn(maxMIDIValue + 1)
			}
		}
		gene.value = value
	}
	return patch
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

func (mutation *Mutation) crossover(parentA, parentB *Patch, enabled bool) *Patch {
	child := parentA.clone()
	if !enabled {
		return child
	}
	for index := range child.genes {
		if child.genes[index].policy == genePolicyMutable && mutation.random.Intn(2) == 0 {
			child.genes[index].value = parentB.genes[index].value
		}
	}
	return child
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

func (mutation *Mutation) selectParent(ranked []scoredPatch) *Patch {
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
func (mutation *Mutation) breedPopulation(ranked []scoredPatch) []populationCandidate {
	pool := mutation.selectionPool(ranked)
	population := make([]populationCandidate, 0, mutation.settings.roundSize)
	appendElite := func(patch *Patch) {
		if len(population) >= mutation.settings.roundSize {
			return
		}
		population = append(population, populationCandidate{patch: patch.clone(), reference: patch})
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
		child := mutation.crossover(parentA, parentB, mutation.random.Float64() < mutation.settings.crossoverRate)
		child = mutation.mutatePatch(child, true)
		population = append(population, populationCandidate{patch: child, reference: parentA})
	}
	return population
}

// Equal scores retain the earlier patch until the parent decays enough for a
// new candidate to replace it.
func (mutation *Mutation) advanceRanked(ranked []scoredPatch) (*Patch, int, bool) {
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
