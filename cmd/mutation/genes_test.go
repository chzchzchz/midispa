package main

import (
	"math/rand"
	"regexp"
	"testing"

	"github.com/chzchzchz/midispa/sysex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueDomainDrawsInsideItsBounds(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 10, Maximum: 12})
	random := rand.New(rand.NewSource(1))
	for sample := 0; sample < 100; sample++ {
		value := domain.draw(random)
		assert.True(t, domain.contains(value), "drew %d, outside %s", value, domain)
	}
}

func TestValueDomainNeverRedrawsTheSameValue(t *testing.T) {
	domains := []valueDomain{
		midiValueDomain(),
		newValueDomain(sysex.Domain{Minimum: 0, Maximum: 1}),
		newValueDomain(sysex.Domain{Minimum: 0, Maximum: 2, Allowed: []int{0, 1}}),
	}
	random := rand.New(rand.NewSource(2))
	for _, domain := range domains {
		for _, current := range allowedValues(domain) {
			for sample := 0; sample < 50; sample++ {
				value := domain.drawDifferentFrom(random, current)
				assert.NotEqual(t, current, value, "%s redrew the current value", domain)
			}
		}
	}
}

func TestValueDomainKeepsASingleValuedGene(t *testing.T) {
	// A one-value domain cannot produce a different candidate, so a mutation
	// must leave the gene alone instead of drawing until it differs.
	domain := newValueDomain(sysex.Domain{Minimum: 3, Maximum: 3})
	random := rand.New(rand.NewSource(3))
	assert.Equal(t, 3, domain.drawDifferentFrom(random, 3), "single-valued domain produced another value")
	_, ok := domain.nudge(3, random)
	assert.False(t, ok, "single-valued domain offered a nudge")
}

func TestValueDomainNudgeStepsAwayAtTheEdges(t *testing.T) {
	// The CC rule this replaces stepped down whenever the value was above the
	// minimum and up otherwise.
	domain := midiValueDomain()
	random := rand.New(rand.NewSource(4))
	tests := []struct {
		name    string
		current int
		want    int
	}{
		{name: "minimum steps up", current: 0, want: 1},
		{name: "middle steps down", current: 64, want: 63},
		{name: "maximum steps down", current: maxMIDIValue, want: maxMIDIValue - 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, ok := domain.nudge(test.current, random)
			require.True(t, ok, "nudge(%d) produced nothing", test.current)
			assert.Equal(t, test.want, value, "nudge(%d)", test.current)
		})
	}
}

func TestValueDomainNudgeStaysInsideAOneofSet(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 0, Maximum: 5, Allowed: []int{0, 1, 5}})
	random := rand.New(rand.NewSource(5))
	for sample := 0; sample < 100; sample++ {
		for _, current := range domain.allowed {
			value, ok := domain.nudge(current, random)
			require.True(t, ok, "nudge(%d) produced nothing", current)
			assert.NotEqual(t, current, value, "nudge(%d) redrew the current value", current)
			assert.True(t, domain.contains(value), "nudge(%d) produced %d, outside %s", current, value, domain)
		}
	}
}

func TestValueDomainClampSnapsToTheNearestChoice(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 0, Maximum: 31, Allowed: []int{0, 7}})
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{name: "below the range", value: -5, want: 0},
		{name: "rounds down to the lower choice", value: 3, want: 0},
		{name: "rounds up to the upper choice", value: 4, want: 7},
		{name: "above the range", value: 99, want: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, domain.clamp(test.value), "clamp(%d)", test.value)
		})
	}
}

func TestValueDomainClampBoundsARange(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 5, Maximum: 9})
	assert.Equal(t, 5, domain.clamp(-1), "clamp below the range")
	assert.Equal(t, 9, domain.clamp(100), "clamp above the range")
}

func TestFixedGeneOutsideItsDomainIsRejected(t *testing.T) {
	gen := &patchGenes{
		format: "test",
		genes: []gene{
			{name: "wide", value: 4, policy: genePolicyFixed, domain: midiValueDomain()},
			{name: "narrow", value: 50, policy: genePolicyFixed, domain: newValueDomain(sysex.Domain{Minimum: 0, Maximum: 31})},
		},
	}
	err := gen.validateFixedValues()
	require.Error(t, err, "accepted a fixed gene outside its domain")
	assert.Contains(t, err.Error(), "narrow", "the error does not name the offending gene")
}

func TestPendingFixedGeneIsRejected(t *testing.T) {
	gen := &patchGenes{
		format: "test",
		genes:  []gene{{name: "waiting", policy: genePolicyFixedPendingSeed}},
	}
	assert.Error(t, gen.validateFixedValues(), "accepted a fixed gene that never received a value")
}

func TestGenesRejectAnotherFormat(t *testing.T) {
	first := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 1}}}
	second := &patchGenes{format: dx7SingleFormatName, genes: []gene{{name: "a", value: 2}}}
	assert.Error(t, first.crossover(second, rand.New(rand.NewSource(6)), nil), "combined gene lists from two formats")
	_, err := first.changesFrom(second)
	assert.Error(t, err, "compared gene lists from two formats")
}

func TestGenesRejectMismatchedLengths(t *testing.T) {
	first := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 1}, {name: "b", value: 2}}}
	second := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 3}}}
	assert.Error(t, first.crossover(second, rand.New(rand.NewSource(7)), nil), "combined gene lists of different lengths")
}

func TestMutateOnlyTakesMatchingGenes(t *testing.T) {
	gen := &patchGenes{
		format: ccFormatID,
		genes: []gene{
			{name: "Alpha", value: 10, policy: genePolicyMutable, domain: midiValueDomain()},
			{name: "Beta", value: 10, policy: genePolicyMutable, domain: midiValueDomain()},
			{name: "Gamma", value: 10, policy: genePolicyMutable, domain: midiValueDomain()},
		},
	}
	settings := evolutionSettings{mutationRate: 1, mutatedGenes: 3, mutateFilter: mustPattern(t, "Beta")}
	gen.mutate(rand.New(rand.NewSource(11)), settings, false)
	assert.Equal(t, 10, gen.genes[0].value, "gene outside the pattern mutated")
	assert.Equal(t, 10, gen.genes[2].value, "gene outside the pattern mutated")
	assert.NotEqual(t, 10, gen.genes[1].value, "gene inside the pattern was not mutated")
}

func TestCrossoverOnlyTakesMatchingGenes(t *testing.T) {
	alphaStable := true
	betaMoved := false
	for seed := int64(0); seed < 100; seed++ {
		child := &patchGenes{
			format: ccFormatID,
			genes: []gene{
				{name: "Alpha", value: 1, policy: genePolicyMutable, domain: midiValueDomain()},
				{name: "Beta", value: 1, policy: genePolicyMutable, domain: midiValueDomain()},
			},
		}
		other := &patchGenes{
			format: ccFormatID,
			genes: []gene{
				{name: "Alpha", value: 2, policy: genePolicyMutable, domain: midiValueDomain()},
				{name: "Beta", value: 2, policy: genePolicyMutable, domain: midiValueDomain()},
			},
		}
		require.NoError(t, child.crossover(other, rand.New(rand.NewSource(seed)), mustPattern(t, "Beta")))
		alphaStable = alphaStable && child.genes[0].value == 1
		betaMoved = betaMoved || child.genes[1].value == 2
	}
	assert.True(t, alphaStable, "gene outside the pattern crossed over")
	assert.True(t, betaMoved, "gene inside the pattern never crossed over")
}

// allowedValues lists the values a domain can actually produce, so the draw
// tests cover every one of them.
func allowedValues(domain valueDomain) []int {
	if len(domain.allowed) > 0 {
		return domain.allowed
	}
	if domain.minimum == domain.maximum {
		return []int{domain.minimum}
	}
	return []int{domain.minimum, domain.maximum}
}

// mustPattern compiles a test pattern, which fails the
// test when the pattern itself is invalid.
func mustPattern(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	compiled, err := regexp.Compile(pattern)
	require.NoError(t, err, "test pattern")
	return compiled
}
