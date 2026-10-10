package main

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/internal/fieldrules"
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
	semantics, err := fieldrules.Load(path)
	require.NoError(t, err, "fieldrules.Load")
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
			_, err := fieldrules.Load(writeSemanticsFile(t, test.contents))
			assert.Error(t, err, "accepted invalid semantics: %s", test.contents)
		})
	}
}

func TestPatchGeneSemanticsExcludeAndFix(t *testing.T) {
	fixedValue := 40
	semantics := map[string]fieldrules.Rule{
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
	semantics := map[string]fieldrules.Rule{
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
	semantics := map[string]fieldrules.Rule{
		"SoundController1": {Policy: "fixed", Value: &fixedValue},
	}
	mutation, err := newMutation(newTestCCFactory("Sound Controller"), defaultEvolutionSettings(), rand.New(rand.NewSource(4)), semantics, seedPath)
	require.NoError(t, err, "newMutation")
	genes := geneValues(t, mutation.parent)
	require.NotEmpty(t, genes, "seed overrode explicit fixed value")
	assert.Equal(t, 40, genes[0].value, "seed overrode explicit fixed value")
}

func TestFixedGeneWithoutValueOrSeedFails(t *testing.T) {
	semantics := map[string]fieldrules.Rule{
		"SoundController1": {Policy: "fixed"},
	}
	_, err := newMutation(newTestCCFactory("Sound Controller"), defaultEvolutionSettings(), rand.New(rand.NewSource(3)), semantics, "")
	assert.ErrorContains(t, err, "requires an explicit value or a seed value")
}

func TestPatchGeneSemanticsRejectsUnknownGene(t *testing.T) {
	semantics := map[string]fieldrules.Rule{
		"NotAGene": {Policy: "exclude"},
	}
	_, err := newPatchWithSemantics("Sound Controller", semantics)
	assert.Error(t, err, "accepted semantics for an unknown gene")
}

// dumpTarget is one model the dump can be asked for.
type dumpTarget struct {
	name      string
	format    string
	modelName string
}

// dumpTargets is every model this command can name: the two SysEx programs,
// which have no --model, and every CC model in cc/model.go. A model's
// parameters come from its own struct, so a model added later can carry a
// shape none of these checks have seen.
func dumpTargets() []dumpTarget {
	targets := make([]dumpTarget, 0, len(cc.ModelNames())+2)
	for _, formatName := range []string{dx7SingleFormatName, pro800FormatName} {
		targets = append(targets, dumpTarget{name: formatName, format: formatName})
	}
	for _, modelName := range cc.ModelNames() {
		targets = append(targets, dumpTarget{name: modelName, format: ccFormatName, modelName: modelName})
	}
	return targets
}

// dumpFile asks for a dump and returns where it landed, along with what it
// reported. Most of these checks go on to read the file back.
func dumpFile(t *testing.T, format, modelName string) (path, report string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "excludes.json")
	var output strings.Builder
	require.NoError(t, dumpExcludedGenes(configuration{format: format, modelName: modelName, dumpExcludes: path}, &output), "dumpExcludedGenes")
	return path, output.String()
}

// catalogFactory selects a format the way production does, which is the only
// way to reach a catalog without restating what each format needs.
func catalogFactory(t *testing.T, format, modelName string) patchFactory {
	t.Helper()
	factory, err := selectPatchFormat(configuration{format: format, modelName: modelName})
	require.NoError(t, err, "selectPatchFormat(%q)", format)
	return factory
}

// catalogGenes reads a format's full parameter list with no rules applied. It
// is the peer newTestPatch is for a CC model, and it is what a dump is
// measured against.
func catalogGenes(t *testing.T, format, modelName string) []gene {
	t.Helper()
	catalog, err := catalogFactory(t, format, modelName).newPatch(nil)
	require.NoError(t, err, "catalog for %q", format)
	return geneValues(t, catalog)
}

func TestDumpExcludedGenesWritesOneRulePerParameter(t *testing.T) {
	for _, target := range dumpTargets() {
		t.Run(target.name, func(t *testing.T) {
			// A dump is one question about a model, so a SysEx run must not be
			// asked for the seed dump a real run needs: pro800 would fail at
			// loadSeed without one.
			path, report := dumpFile(t, target.format, target.modelName)
			// One file holds one model's parameters, so the report has to say
			// which one it described, and how to use what it wrote.
			assert.Contains(t, report, target.name, "the report does not say which model was dumped")
			assert.Contains(t, report, "--gene-semantics", "the dump does not say how to use the file")

			// The dumped file has to satisfy the reader that a run will use, so
			// this loads it back rather than comparing text: a name the loader
			// would reject is a name the dump got wrong.
			semantics, err := fieldrules.Load(path)
			require.NoError(t, err, "the dumped file is not a gene semantics file")
			catalog := catalogGenes(t, target.format, target.modelName)
			require.NotEmpty(t, catalog, "model has no parameters")
			require.Len(t, semantics, len(catalog), "one rule per parameter")
			for _, current := range catalog {
				rule, listed := semantics[current.name]
				if assert.True(t, listed, "parameter %s is missing from the dump", current.name) {
					assert.Equal(t, fieldrules.PolicyExclude, rule.Policy, "parameter %s policy", current.name)
					assert.Nil(t, rule.Value, "parameter %s was dumped with a value", current.name)
				}
			}

			// What the file says it does: taken as rules, it leaves no
			// parameter to evolve. That is why the report tells the reader to
			// edit it, and it is the one property a run would show silently.
			emptied, err := catalogFactory(t, target.format, target.modelName).newPatch(semantics)
			require.NoError(t, err, "catalog with the dumped rules")
			assert.Zero(t, emptied.mutableGeneCount(), "an unedited dump left something to evolve")
		})
	}
}

func TestEditedDumpNarrowsTheSearchToWhatItKept(t *testing.T) {
	// The point of the dump is that its file becomes a --gene-semantics file,
	// so this edits one and reads it back the way a run would. Editing is
	// modelled as deleting entries, which leaves every parameter except the
	// wanted ones excluded: deleting a rule frees a parameter, so keeping two
	// means excluding the other eight, not excluding two.
	path, _ := dumpFile(t, ccFormatName, "Sound Controller")

	catalog := newTestPatch(t, "Sound Controller")
	require.Greater(t, len(catalog.genes), 2, "the model has too few parameters to edit")
	kept := map[string]bool{catalog.genes[0].name: true, catalog.genes[1].name: true}
	edited := make([]map[string]fieldrules.Rule, 0, len(catalog.genes))
	keptControllers := make([]int, 0, len(kept))
	for _, current := range catalog.genes {
		if kept[current.name] {
			keptControllers = append(keptControllers, catalog.controllers[current.name])
			continue
		}
		edited = append(edited, map[string]fieldrules.Rule{current.name: {Policy: fieldrules.PolicyExclude}})
	}
	require.NoError(t, writeGeneSemantics(path, edited), "writeGeneSemantics")

	semantics, err := fieldrules.Load(path)
	require.NoError(t, err, "the edited dump is not a gene semantics file")
	require.Len(t, semantics, len(catalog.genes)-len(kept), "one rule per edited parameter")
	searched, err := newPatchWithSemantics("Sound Controller", semantics)
	require.NoError(t, err, "catalog with the edited dump")
	genes := geneValues(t, searched)
	require.Len(t, genes, len(kept), "the edited dump did not narrow the search")
	for _, current := range genes {
		assert.True(t, kept[current.name], "parameter %s was freed by accident", current.name)
		assert.Contains(t, keptControllers, catalog.controllers[current.name], "parameter %s has no controller", current.name)
	}
}

func TestDumpExcludedGenesRejectsAConfigurationItCannotDump(t *testing.T) {
	path := filepath.Join(t.TempDir(), "excludes.json")
	tests := []struct {
		name   string
		config configuration
	}{
		{
			name:   "gene semantics alongside a dump",
			config: configuration{format: ccFormatName, modelName: "Sound Controller", geneSemantics: "rules.json", dumpExcludes: path},
		},
		{
			name:   "no model for the cc format",
			config: configuration{format: ccFormatName, dumpExcludes: path},
		},
		{
			name:   "model on a format without one",
			config: configuration{format: dx7SingleFormatName, modelName: "Volca Bass", dumpExcludes: path},
		},
		{
			name:   "unknown model",
			config: configuration{format: ccFormatName, modelName: "Not A Synth", dumpExcludes: path},
		},
		{
			name:   "unknown format",
			config: configuration{format: "dx7-bulk", dumpExcludes: path},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Error(t, dumpExcludedGenes(test.config, io.Discard), "accepted a dump configuration that cannot produce a file")
			assert.NoFileExists(t, path, "a rejected dump still wrote a file")
		})
	}
}

func TestDumpExcludedGenesReportsAnUnwritablePath(t *testing.T) {
	config := configuration{format: ccFormatName, modelName: "Sound Controller", dumpExcludes: filepath.Join(t.TempDir(), "no-such-directory", "excludes.json")}
	assert.ErrorContains(t, dumpExcludedGenes(config, io.Discard), "write gene semantics")
}

// failingWriter stands in for a closed pipe or a full device, where the report
// cannot go out.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("report failed")
}

func TestDumpExcludedGenesFailsWhenItsReportCannotBeWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "excludes.json")
	config := configuration{format: ccFormatName, modelName: "Sound Controller", dumpExcludes: path}
	// The file is written before the report, so a report that cannot go out
	// leaves a usable file behind and still has to fail: the caller asked for
	// both, and reporting success would say it had got them.
	assert.Error(t, dumpExcludedGenes(config, failingWriter{}), "a report that could not be written passed for success")
	assert.FileExists(t, path, "the dump file was not written before the report")
}

func TestDumpExcludedGenesKeepsDeclarationOrder(t *testing.T) {
	path, _ := dumpFile(t, dx7SingleFormatName, "")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read dumped semantics")

	var entries []map[string]fieldrules.Rule
	require.NoError(t, json.Unmarshal(data, &entries), "parse dumped semantics")
	require.NotEmpty(t, entries, "dumped semantics are empty")
	// Declaration order is what makes two dumps of the same model the same
	// file, and an editor's choice of what to keep stays stable across
	// re-dumps. A map would shuffle the names between runs, which nothing
	// would reject.
	catalog := catalogGenes(t, dx7SingleFormatName, "")
	require.Len(t, entries, len(catalog), "parameter count")
	for index, current := range catalog {
		require.Len(t, entries[index], 1, "entry %d names more than one parameter", index)
		for name := range entries[index] {
			assert.Equal(t, current.name, name, "parameter %d is out of declaration order", index)
		}
	}
}
