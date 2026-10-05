package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSemanticsFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "semantics.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600), "write semantics")
	return path
}

func TestLoadGeneSemantics(t *testing.T) {
	path := writeSemanticsFile(t, `[
		{"SoundController1":{"policy":"exclude"}},
		{"SoundController2":{"policy":"fixed","value":40}},
		{"SoundController3":{"policy":"fixed"}}
	]`)
	semantics, err := loadGeneSemantics(path)
	require.NoError(t, err, "loadGeneSemantics")
	require.Len(t, semantics, 3, "loaded semantics")
	assert.Equal(t, "exclude", semantics["SoundController1"].Policy, "unexpected exclude rule")
	if assert.NotNil(t, semantics["SoundController2"].Value, "unexpected fixed value") {
		assert.Equal(t, 40, *semantics["SoundController2"].Value, "unexpected fixed value")
	}
	assert.Nil(t, semantics["SoundController3"].Value, "fixed rule unexpectedly has a value")
}

func TestLoadGeneSemanticsRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{name: "unknown policy", contents: `[{"SoundController1":{"policy":"unknown"}}]`},
		{name: "exclude with a value", contents: `[{"SoundController1":{"policy":"exclude","value":1}}]`},
		{name: "repeated gene", contents: `[{"SoundController1":{"policy":"exclude"}},{"SoundController1":{"policy":"fixed"}}]`},
		{name: "several rules at once", contents: `[{"SoundController1":{"policy":"exclude"},"SoundController2":{"policy":"fixed"}}]`},
		{name: "not json", contents: `not-json`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadGeneSemantics(writeSemanticsFile(t, test.contents))
			assert.Error(t, err, "accepted invalid semantics: %s", test.contents)
		})
	}
}

func TestPatchGeneSemanticsExcludeAndFix(t *testing.T) {
	fixedValue := 40
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "exclude"},
		"SoundController2": {Policy: "fixed", Value: &fixedValue},
	}
	patch, err := newPatchWithSemantics("Sound Controller", semantics)
	require.NoError(t, err, "newPatchWithSemantics")
	for _, gene := range patch.genes {
		switch gene.name {
		case "SoundController1":
			require.Fail(t, "excluded gene remains in patch")
		case "SoundController2":
			assert.Equal(t, 40, gene.value, "fixed gene value")
			assert.Equal(t, genePolicyFixed, gene.policy, "fixed gene policy")
		}
	}

	mutation := &Mutation{settings: defaultEvolutionSettings(), random: rand.New(rand.NewSource(1))}
	child := mutation.mutatePatch(patch, true)
	for _, gene := range geneValues(t, child) {
		if gene.name == "SoundController2" {
			assert.Equal(t, 40, gene.value, "fixed gene mutated")
		}
	}
	messages, err := child.encode(0)
	require.NoError(t, err, "encode")
	for _, message := range messages {
		assert.NotEqual(t, byte(70), message[1], "excluded gene was emitted")
		if message[1] == 71 {
			assert.Equal(t, byte(40), message[2], "fixed gene emitted as %d", message[2])
		}
	}
	outputPath := filepath.Join(t.TempDir(), "semantic-patch.mid")
	require.NoError(t, writePatchSMFForChannel(outputPath, child.(*Patch), 1), "write semantic patch")
	writtenMessages, err := readPatchSMF(outputPath)
	require.NoError(t, err, "read semantic patch")
	for _, message := range writtenMessages {
		if message[1] == 70 || (message[1] == 71 && message[2] != 40) {
			assert.Fail(t, "SMF output violated gene semantics", "%v", message)
		}
	}
}

func TestFixedGeneUsesSeedValue(t *testing.T) {
	seedPatch := newTestPatch(t, "Sound Controller")
	seedPatch.genes[0].value = 40
	seedPath := filepath.Join(t.TempDir(), "seed.mid")
	require.NoError(t, writePatchSMFForChannel(seedPath, seedPatch, 1), "write seed")
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed"},
	}
	mutation, err := newMutation(newTestCCFactory("Sound Controller"), defaultEvolutionSettings(), rand.New(rand.NewSource(2)), semantics, seedPath)
	require.NoError(t, err, "newMutation")
	genes := geneValues(t, mutation.parent)
	require.NotEmpty(t, genes, "fixed seed value was not preserved")
	assert.Equal(t, 40, genes[0].value, "fixed seed value was not preserved")
	assert.Equal(t, genePolicyFixed, genes[0].policy, "fixed seed value was not preserved")
}

func TestExplicitFixedValueOverridesSeed(t *testing.T) {
	seedPatch := newTestPatch(t, "Sound Controller")
	seedPatch.genes[0].value = 90
	seedPath := filepath.Join(t.TempDir(), "seed.mid")
	require.NoError(t, writePatchSMFForChannel(seedPath, seedPatch, 1), "write seed")
	fixedValue := 40
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed", Value: &fixedValue},
	}
	mutation, err := newMutation(newTestCCFactory("Sound Controller"), defaultEvolutionSettings(), rand.New(rand.NewSource(4)), semantics, seedPath)
	require.NoError(t, err, "newMutation")
	genes := geneValues(t, mutation.parent)
	require.NotEmpty(t, genes, "seed overrode explicit fixed value")
	assert.Equal(t, 40, genes[0].value, "seed overrode explicit fixed value")
}

func TestFixedGeneWithoutValueOrSeedFails(t *testing.T) {
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed"},
	}
	_, err := newMutation(newTestCCFactory("Sound Controller"), defaultEvolutionSettings(), rand.New(rand.NewSource(3)), semantics, "")
	assert.ErrorContains(t, err, "requires an explicit value or a seed value")
}

func TestPatchGeneSemanticsRejectsUnknownGene(t *testing.T) {
	semantics := map[string]geneSemantic{
		"NotAGene": {Policy: "exclude"},
	}
	_, err := newPatchWithSemantics("Sound Controller", semantics)
	assert.Error(t, err, "accepted semantics for an unknown gene")
}
