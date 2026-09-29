package main

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/sysex"
)

func TestValueDomainDrawsInsideItsBounds(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 10, Maximum: 12})
	random := rand.New(rand.NewSource(1))
	for sample := 0; sample < 100; sample++ {
		value := domain.draw(random)
		if !domain.contains(value) {
			t.Fatalf("drew %d, outside %s", value, domain)
		}
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
				if value := domain.drawDifferentFrom(random, current); value == current {
					t.Fatalf("%s redrew the current value %d", domain, current)
				}
			}
		}
	}
}

func TestValueDomainKeepsASingleValuedGene(t *testing.T) {
	// A one-value domain cannot produce a different candidate, so a mutation
	// must leave the gene alone instead of drawing until it differs.
	domain := newValueDomain(sysex.Domain{Minimum: 3, Maximum: 3})
	random := rand.New(rand.NewSource(3))
	if value := domain.drawDifferentFrom(random, 3); value != 3 {
		t.Fatalf("single-valued domain produced %d", value)
	}
	if _, ok := domain.nudge(3, random); ok {
		t.Fatal("single-valued domain offered a nudge")
	}
}

func TestValueDomainNudgeStepsAwayAtTheEdges(t *testing.T) {
	// The CC rule this replaces stepped down whenever the value was above the
	// minimum and up otherwise.
	domain := midiValueDomain()
	random := rand.New(rand.NewSource(4))
	tests := []struct {
		current int
		want    int
	}{
		{current: 0, want: 1},
		{current: 64, want: 63},
		{current: maxMIDIValue, want: maxMIDIValue - 1},
	}
	for _, test := range tests {
		value, ok := domain.nudge(test.current, random)
		if !ok || value != test.want {
			t.Fatalf("nudge(%d) = %d, %t, want %d, true", test.current, value, ok, test.want)
		}
	}
}

func TestValueDomainNudgeStaysInsideAOneofSet(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 0, Maximum: 5, Allowed: []int{0, 1, 5}})
	random := rand.New(rand.NewSource(5))
	for sample := 0; sample < 100; sample++ {
		for _, current := range domain.allowed {
			value, ok := domain.nudge(current, random)
			if !ok || value == current || !domain.contains(value) {
				t.Fatalf("nudge(%d) = %d, %t", current, value, ok)
			}
		}
	}
}

func TestValueDomainClampSnapsToTheNearestChoice(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 0, Maximum: 31, Allowed: []int{0, 7}})
	tests := []struct {
		value int
		want  int
	}{
		{value: -5, want: 0},
		{value: 3, want: 0},
		{value: 4, want: 7},
		{value: 99, want: 7},
	}
	for _, test := range tests {
		if got := domain.clamp(test.value); got != test.want {
			t.Fatalf("clamp(%d) = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestValueDomainClampBoundsARange(t *testing.T) {
	domain := newValueDomain(sysex.Domain{Minimum: 5, Maximum: 9})
	if got := domain.clamp(-1); got != 5 {
		t.Fatalf("clamp below the range is %d, want 5", got)
	}
	if got := domain.clamp(100); got != 9 {
		t.Fatalf("clamp above the range is %d, want 9", got)
	}
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
	if err == nil || !strings.Contains(err.Error(), "narrow") {
		t.Fatalf("got error %v, want it to name the offending gene", err)
	}
}

func TestPendingFixedGeneIsRejected(t *testing.T) {
	gen := &patchGenes{
		format: "test",
		genes:  []gene{{name: "waiting", policy: genePolicyFixedPendingSeed}},
	}
	if err := gen.validateFixedValues(); err == nil {
		t.Fatal("accepted a fixed gene that never received a value")
	}
}

func TestGenesRejectAnotherFormat(t *testing.T) {
	first := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 1}}}
	second := &patchGenes{format: dx7SingleFormatName, genes: []gene{{name: "a", value: 2}}}
	if err := first.crossover(second, rand.New(rand.NewSource(6))); err == nil {
		t.Fatal("combined gene lists from two formats")
	}
	if _, err := first.changesFrom(second); err == nil {
		t.Fatal("compared gene lists from two formats")
	}
}

func TestGenesRejectMismatchedLengths(t *testing.T) {
	first := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 1}, {name: "b", value: 2}}}
	second := &patchGenes{format: ccFormatID, genes: []gene{{name: "a", value: 3}}}
	if err := first.crossover(second, rand.New(rand.NewSource(7))); err == nil {
		t.Fatal("combined gene lists of different lengths")
	}
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
