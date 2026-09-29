package main

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	"github.com/chzchzchz/midispa/sysex"
)

// A drawn value must differ from the value it replaces, otherwise a mutation
// spends one of its slots without producing a candidate the judge can hear.
// The attempt count bounds that retry; a domain wide enough to have a
// different value almost always finds one on the first draw.
const redrawAttempts = 16

// ccFormatID names the patch format a gene list belongs to. Two candidates
// only combine when their gene lists mean the same thing.
const ccFormatID = "cc"

type genePolicy uint8

const (
	genePolicyMutable genePolicy = iota
	genePolicyFixedPendingSeed
	genePolicyFixed
)

// Gene policy encodes the valid lifecycle states so a fixed gene cannot be
// confused with a mutable gene before its seed value is applied.
type gene struct {
	name   string
	value  int
	policy genePolicy
	domain valueDomain
}

type geneChange struct {
	name   string
	before int
	after  int
	delta  int
}

// valueDomain is the engine's view of one field's constraint. A CC controller
// takes its bound from the MIDI data model; a SysEx field takes it from the
// program struct tags. Every rule about drawing, clamping, and validating a
// candidate lives here, so the two formats cannot drift apart.
//
// It deliberately mirrors sysex.Domain rather than reusing it. sysex.Domain
// describes what a decoded field may hold, which is a question the protocol
// package can answer for any vendor. How to search inside that constraint is
// the engine's business, and drawing needs a random source that a codec has no
// reason to hold. The cost is that contains and String are written twice, so a
// new kind of constraint has to be added to both types; merging them would
// either put a random source in the protocol package or leave these as free
// functions on someone else's type, which is the lesser evil.
type valueDomain struct {
	minimum int
	maximum int
	allowed []int
}

func newValueDomain(domain sysex.Domain) valueDomain {
	return valueDomain{minimum: domain.Minimum, maximum: domain.Maximum, allowed: domain.Allowed}
}

// midiValueDomain bounds a controller value to the 7-bit MIDI data byte range.
func midiValueDomain() valueDomain {
	return valueDomain{minimum: 0, maximum: maxMIDIValue}
}

func (domain valueDomain) String() string {
	if len(domain.allowed) == 0 {
		return fmt.Sprintf("%d..%d", domain.minimum, domain.maximum)
	}
	parts := make([]string, 0, len(domain.allowed))
	for _, allowed := range domain.allowed {
		parts = append(parts, strconv.Itoa(allowed))
	}
	return strings.Join(parts, ",")
}

func (domain valueDomain) contains(value int) bool {
	if len(domain.allowed) == 0 {
		return value >= domain.minimum && value <= domain.maximum
	}
	return slices.Contains(domain.allowed, value)
}

// hasAlternative reports whether the domain can produce more than one value,
// which is what makes a no-op draw impossible to avoid.
func (domain valueDomain) hasAlternative() bool {
	if len(domain.allowed) > 0 {
		return len(domain.allowed) > 1
	}
	return domain.minimum < domain.maximum
}

func (domain valueDomain) draw(random *rand.Rand) int {
	if len(domain.allowed) > 0 {
		return domain.allowed[random.Intn(len(domain.allowed))]
	}
	return domain.minimum + random.Intn(domain.maximum-domain.minimum+1)
}

// drawDifferentFrom draws a value other than current, or returns current when
// the domain holds a single legal value and no alternative exists.
func (domain valueDomain) drawDifferentFrom(random *rand.Rand, current int) int {
	if !domain.hasAlternative() {
		return current
	}
	for attempt := 0; attempt < redrawAttempts; attempt++ {
		if value := domain.draw(random); value != current {
			return value
		}
	}
	return current
}

// clamp brings a proposed value into the domain. A oneof snaps to the nearest
// permitted value so a Gaussian step out of a two-value field lands on a legal
// choice rather than between two.
func (domain valueDomain) clamp(value int) int {
	if len(domain.allowed) == 0 {
		if value < domain.minimum {
			return domain.minimum
		}
		if value > domain.maximum {
			return domain.maximum
		}
		return value
	}
	nearest := domain.allowed[0]
	for _, allowed := range domain.allowed[1:] {
		if absInt(allowed-value) < absInt(nearest-value) {
			nearest = allowed
		}
	}
	return nearest
}

// nudge returns the closest other legal value, which is how a mutation that
// would otherwise be a no-op still produces a candidate. The second result is
// false when the domain has no alternative to offer.
func (domain valueDomain) nudge(current int, random *rand.Rand) (int, bool) {
	if len(domain.allowed) > 0 {
		position := slices.Index(domain.allowed, current)
		if position < 0 {
			return domain.allowed[random.Intn(len(domain.allowed))], true
		}
		// Drawing a position other than the current one is what makes a
		// single draw enough however small the set is.
		pick := random.Intn(len(domain.allowed) - 1)
		if pick >= position {
			pick++
		}
		return domain.allowed[pick], true
	}
	if current > domain.minimum {
		return current - 1, true
	}
	if current < domain.maximum {
		return current + 1, true
	}
	return current, false
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// patchGenes is the value store every patch format shares. A parent and the
// candidates mutated from it each hold their own copy of the slice, so
// storing a gene list once and cloning it per candidate is what keeps a
// champion from changing underneath a running population.
type patchGenes struct {
	format string
	genes  []gene
}

func (gen *patchGenes) formatID() string {
	return gen.format
}

// geneStore returns the embedded value list, which is how format-agnostic
// crossover and change reporting reach another candidate's genes without
// knowing which format produced it.
func (gen *patchGenes) geneStore() *patchGenes {
	return gen
}

func (gen *patchGenes) cloneGenes() *patchGenes {
	return &patchGenes{format: gen.format, genes: append([]gene(nil), gen.genes...)}
}

func (gen *patchGenes) randomize(random *rand.Rand) {
	for index := range gen.genes {
		if gen.genes[index].policy != genePolicyMutable {
			continue
		}
		gen.genes[index].value = gen.genes[index].domain.draw(random)
	}
}

// mutate changes a bounded number of mutable genes in place. Initial
// candidates draw uniformly, while descendants take a bounded Gaussian step so
// that neighboring candidates stay comparable by ear.
func (gen *patchGenes) mutate(random *rand.Rand, settings evolutionSettings, gaussian bool) {
	if random.Float64() >= settings.mutationRate {
		return
	}
	mutable := make([]int, 0, len(gen.genes))
	for index := range gen.genes {
		if gen.genes[index].policy == genePolicyMutable {
			mutable = append(mutable, index)
		}
	}
	if len(mutable) == 0 {
		return
	}
	count := mutationCount(random, settings, len(mutable))
	for _, selected := range random.Perm(len(mutable))[:count] {
		mutateGene(&gen.genes[mutable[selected]], random, settings, gaussian)
	}
}

// mutationCount resolves the automatic 1..maxMutatedGenes range and clamps an
// explicit request to the number of genes that can actually change.
func mutationCount(random *rand.Rand, settings evolutionSettings, mutableCount int) int {
	count := settings.mutatedGenes
	if count == 0 {
		maximum := maxMutatedGenes
		if mutableCount < maximum {
			maximum = mutableCount
		}
		return 1 + random.Intn(maximum)
	}
	if count > mutableCount {
		return mutableCount
	}
	return count
}

func mutateGene(target *gene, random *rand.Rand, settings evolutionSettings, gaussian bool) {
	if gaussian && random.Float64() >= mutationExplorationRate {
		value := target.domain.clamp(target.value + int(math.Round(random.NormFloat64()*settings.mutationSigma)))
		if value == target.value {
			if nudged, ok := target.domain.nudge(target.value, random); ok {
				value = nudged
			}
		}
		target.value = value
		return
	}
	target.value = target.domain.drawDifferentFrom(random, target.value)
}

// crossover takes each mutable gene from the other parent with even odds.
func (gen *patchGenes) crossover(other *patchGenes, random *rand.Rand) error {
	if err := gen.requireSameFormat(other); err != nil {
		return err
	}
	for index := range gen.genes {
		if gen.genes[index].policy == genePolicyMutable && random.Intn(2) == 0 {
			gen.genes[index].value = other.genes[index].value
		}
	}
	return nil
}

func (gen *patchGenes) changesFrom(other *patchGenes) ([]geneChange, error) {
	if err := gen.requireSameFormat(other); err != nil {
		return nil, err
	}
	changes := make([]geneChange, 0)
	for index := range gen.genes {
		before := other.genes[index].value
		after := gen.genes[index].value
		if before == after {
			continue
		}
		changes = append(changes, geneChange{
			name:   gen.genes[index].name,
			before: before,
			after:  after,
			delta:  after - before,
		})
	}
	return changes, nil
}

func (gen *patchGenes) requireSameFormat(other *patchGenes) error {
	if gen.format != other.format {
		return fmt.Errorf("cannot combine %s and %s genes", gen.format, other.format)
	}
	if len(gen.genes) != len(other.genes) {
		return fmt.Errorf("%s gene lists differ in length: %d and %d", gen.format, len(gen.genes), len(other.genes))
	}
	return nil
}

func (gen *patchGenes) mutableGeneCount() int {
	count := 0
	for index := range gen.genes {
		if gen.genes[index].policy == genePolicyMutable {
			count++
		}
	}
	return count
}

// hasSeedConfigurableGenes reports whether a seed can still supply a value for
// any gene, which is how the CC loader decides an empty seed is a mistake.
func (gen *patchGenes) hasSeedConfigurableGenes() bool {
	for index := range gen.genes {
		if gen.genes[index].policy != genePolicyFixed {
			return true
		}
	}
	return false
}

// validateFixedValues rejects a fixed gene that never received a value and one
// that received a value its own field could not hold. The gene name is quoted
// because a rejected fixed value is otherwise invisible at the point the
// candidate would have been auditioned. A mutable gene's value came from the
// wire, which the format's own decoder has already validated.
func (gen *patchGenes) validateFixedValues() error {
	for index := range gen.genes {
		current := &gen.genes[index]
		if current.policy == genePolicyFixedPendingSeed {
			return fmt.Errorf("fixed gene %q requires an explicit value or a seed value", current.name)
		}
		if current.policy == genePolicyFixed && !current.domain.contains(current.value) {
			return fmt.Errorf("fixed gene %q value %d is outside %s", current.name, current.value, current.domain)
		}
	}
	return nil
}
