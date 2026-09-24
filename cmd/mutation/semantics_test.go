package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSemanticsFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "semantics.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write semantics: %v", err)
	}
	return path
}

func TestLoadGeneSemantics(t *testing.T) {
	path := writeSemanticsFile(t, `[
		{"SoundController1":{"policy":"exclude"}},
		{"SoundController2":{"policy":"fixed","value":40}},
		{"SoundController3":{"policy":"fixed"}}
	]`)
	semantics, err := loadGeneSemantics(path)
	if err != nil {
		t.Fatalf("loadGeneSemantics: %v", err)
	}
	if len(semantics) != 3 {
		t.Fatalf("loaded %d semantics, want 3", len(semantics))
	}
	if semantics["SoundController1"].Policy != "exclude" {
		t.Fatalf("unexpected exclude rule: %+v", semantics["SoundController1"])
	}
	if semantics["SoundController2"].Value == nil || *semantics["SoundController2"].Value != 40 {
		t.Fatalf("unexpected fixed value: %+v", semantics["SoundController2"])
	}
	if semantics["SoundController3"].Value != nil {
		t.Fatalf("fixed rule unexpectedly has a value: %+v", semantics["SoundController3"])
	}
}

func TestLoadGeneSemanticsRejectsInvalidRules(t *testing.T) {
	tests := []string{
		`[{"SoundController1":{"policy":"unknown"}}]`,
		`[{"SoundController1":{"policy":"exclude","value":1}}]`,
		`[{"SoundController1":{"policy":"fixed","value":128}}]`,
		`[{"SoundController1":{"policy":"exclude"}},{"SoundController1":{"policy":"fixed"}}]`,
		`[{"SoundController1":{"policy":"exclude"},"SoundController2":{"policy":"fixed"}}]`,
		`not-json`,
	}
	for _, contents := range tests {
		if _, err := loadGeneSemantics(writeSemanticsFile(t, contents)); err == nil {
			t.Fatalf("accepted invalid semantics: %s", contents)
		}
	}
}

func TestPatchGeneSemanticsExcludeAndFix(t *testing.T) {
	fixedValue := 40
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "exclude"},
		"SoundController2": {Policy: "fixed", Value: &fixedValue},
	}
	patch, err := newPatchWithSemantics("Sound Controller", semantics)
	if err != nil {
		t.Fatalf("newPatchWithSemantics: %v", err)
	}
	for _, gene := range patch.genes {
		switch gene.name {
		case "SoundController1":
			t.Fatal("excluded gene remains in patch")
		case "SoundController2":
			if gene.value != 40 || gene.policy != genePolicyFixed {
				t.Fatalf("fixed gene is %+v", gene)
			}
		}
	}

	mutation := &Mutation{settings: defaultEvolutionSettings(), random: rand.New(rand.NewSource(1))}
	child := mutation.mutatePatch(patch, true)
	for _, gene := range child.genes {
		if gene.name == "SoundController2" && gene.value != 40 {
			t.Fatalf("fixed gene mutated to %d", gene.value)
		}
	}
	messages, err := child.controlChanges(0)
	if err != nil {
		t.Fatalf("controlChanges: %v", err)
	}
	for _, message := range messages {
		if message[1] == 70 {
			t.Fatal("excluded gene was emitted")
		}
		if message[1] == 71 && message[2] != 40 {
			t.Fatalf("fixed gene emitted as %d", message[2])
		}
	}
	outputPath := filepath.Join(t.TempDir(), "semantic-patch.mid")
	if err := writePatchSMFForChannel(outputPath, child, 1); err != nil {
		t.Fatalf("write semantic patch: %v", err)
	}
	writtenMessages, err := readPatchSMF(outputPath)
	if err != nil {
		t.Fatalf("read semantic patch: %v", err)
	}
	for _, message := range writtenMessages {
		if message[1] == 70 || (message[1] == 71 && message[2] != 40) {
			t.Fatalf("SMF output violated gene semantics: %v", message)
		}
	}
}

func TestFixedGeneUsesSeedValue(t *testing.T) {
	seedPatch := newTestPatch(t, "Sound Controller")
	seedPatch.genes[0].value = 40
	seedPath := filepath.Join(t.TempDir(), "seed.mid")
	if err := writePatchSMFForChannel(seedPath, seedPatch, 1); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed"},
	}
	mutation, err := newMutationWithSemantics("Sound Controller", defaultEvolutionSettings(), rand.New(rand.NewSource(2)), semantics, seedPath)
	if err != nil {
		t.Fatalf("newMutationWithSemantics: %v", err)
	}
	if len(mutation.parent.genes) == 0 || mutation.parent.genes[0].value != 40 || mutation.parent.genes[0].policy != genePolicyFixed {
		t.Fatalf("fixed seed value was not preserved: %+v", mutation.parent.genes)
	}
}

func TestExplicitFixedValueOverridesSeed(t *testing.T) {
	seedPatch := newTestPatch(t, "Sound Controller")
	seedPatch.genes[0].value = 90
	seedPath := filepath.Join(t.TempDir(), "seed.mid")
	if err := writePatchSMFForChannel(seedPath, seedPatch, 1); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	fixedValue := 40
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed", Value: &fixedValue},
	}
	mutation, err := newMutationWithSemantics("Sound Controller", defaultEvolutionSettings(), rand.New(rand.NewSource(4)), semantics, seedPath)
	if err != nil {
		t.Fatalf("newMutationWithSemantics: %v", err)
	}
	if mutation.parent.genes[0].value != 40 {
		t.Fatalf("seed overrode explicit fixed value: %d", mutation.parent.genes[0].value)
	}
}

func TestFixedGeneWithoutValueOrSeedFails(t *testing.T) {
	semantics := map[string]geneSemantic{
		"SoundController1": {Policy: "fixed"},
	}
	_, err := newMutationWithSemantics("Sound Controller", defaultEvolutionSettings(), rand.New(rand.NewSource(3)), semantics, "")
	if err == nil || !strings.Contains(err.Error(), "requires an explicit value or a seed value") {
		t.Fatalf("got error %v", err)
	}
}

func TestPatchGeneSemanticsRejectsUnknownGene(t *testing.T) {
	semantics := map[string]geneSemantic{
		"NotAGene": {Policy: "exclude"},
	}
	if _, err := newPatchWithSemantics("Sound Controller", semantics); err == nil {
		t.Fatal("accepted semantics for an unknown gene")
	}
}
