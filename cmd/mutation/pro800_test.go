package main

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/sysex/behringer/pro800"
)

// probePatchData is a patch with a few non-default values, in the newer dump
// layout so every field on the record has somewhere to go.
func probePatchData() *pro800.PatchData {
	data := &pro800.PatchData{Address: 42}
	data.Patch.Version = int(pro800.Version6F)
	data.Patch.Name = "PROBE"
	data.Patch.OscA.Volume = 1000
	data.Patch.OscB.Fine = 200
	data.Patch.Filter.Cutoff = 3000
	data.Patch.Tuning[0] = 12345
	return data
}

func pro800Message(t *testing.T, data *pro800.PatchData) []byte {
	t.Helper()
	messages, err := newPro800Format().Encode(data)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return messages[0]
}

func writePro800Seed(t *testing.T, message []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "voice.syx")
	if err := os.WriteFile(path, message, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return path
}

func newTestPro800Patch(t *testing.T) *SysexPatch {
	t.Helper()
	format := newPro800Format()
	candidate, err := newSysexPatch(format, format.NewRoot(), nil)
	if err != nil {
		t.Fatalf("newSysexPatch: %v", err)
	}
	if err := candidate.loadMessage(pro800Message(t, probePatchData())); err != nil {
		t.Fatalf("loadMessage: %v", err)
	}
	return candidate
}

func geneNamed(genes []gene, name string) *gene {
	for index := range genes {
		if genes[index].name == name {
			return &genes[index]
		}
	}
	return nil
}

func TestPro800FormatRoundTrips(t *testing.T) {
	format := newPro800Format()
	if format.ID() != pro800FormatName {
		t.Fatalf("format is %q, want %q", format.ID(), pro800FormatName)
	}
	message := pro800Message(t, probePatchData())
	decoded, err := format.Decode(message)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	data, ok := decoded.(*pro800.PatchData)
	if !ok {
		t.Fatalf("Decode returned %T", decoded)
	}
	// Everything the record carries has to survive, including the fields the
	// catalog leaves alone.
	if data.Address != 42 || data.Patch.Name != "PROBE" {
		t.Fatalf("decoded address %d name %q", data.Address, data.Patch.Name)
	}
	if data.Patch.OscA.Volume != 1000 || data.Patch.OscB.Fine != 200 || data.Patch.Tuning[0] != 12345 {
		t.Fatalf("decoded values did not survive: %+v", data.Patch)
	}
	if _, err := format.Encode(&dx7StyleRoot{}); err == nil {
		t.Fatal("encoded a program from another format")
	}
}

// dx7StyleRoot stands in for a program a different format produced.
type dx7StyleRoot struct{ Channel int }

func TestPro800CatalogFollowsTheStructTags(t *testing.T) {
	candidate := newTestPro800Patch(t)
	// A field the record marks as not a sound parameter is not a gene. That
	// is declared on the Pro800 struct, so this test names no field the
	// command would otherwise have to know about.
	for _, name := range []string{
		"Address",
		"Patch.Version",
		"Patch.Reserved",
		"Patch.Name",
		"Patch.LfoAftertouchPresent",
		// A 6E dump has nowhere to put these, so a run seeded from one
		// could not evolve them. Leaving them out for both layouts keeps
		// what a run can evolve independent of the layout it started from.
		"Patch.VoiceSpread",
		"Patch.TrackingReference",
		"Patch.GlideMode",
		"Patch.PitchBend.Range",
	} {
		if geneNamed(candidate.genes, name) != nil {
			t.Fatalf("%s is not a sound parameter and must not be a gene", name)
		}
	}
	for _, name := range []string{
		"Patch.OscA.Volume",
		"Patch.OscA.PitchMode",
		"Patch.Filter.Cutoff",
		"Patch.LfoAftertouch",
		// The fine tuning and the sync switch exist only on the second
		// oscillator, so they are evolvable there and absent from the first
		// because the record has nowhere to put them.
		"Patch.OscB.Fine",
		"Patch.OscB.Sync",
		"Patch.Tuning[0]",
		"Patch.Tuning[11]",
		// The mod wheel's own range is in both layouts and stays evolvable,
		// which is why the skip has to sit on the pitch bend field rather
		// than on every field called Range.
		"Patch.ModWheel.Range",
		"Patch.PitchBend.Target",
	} {
		if geneNamed(candidate.genes, name) == nil {
			t.Fatalf("%s is a sound parameter and should be a gene", name)
		}
	}
	if candidate.mutableGeneCount() != len(candidate.genes) {
		t.Fatal("every catalogued gene should be mutable")
	}
	// The embedded envelope inside the filter is flattened, so its fields
	// carry no extra segment and a report reads the way a panel does.
	if geneNamed(candidate.genes, "Patch.Filter.Envelope.Attack") == nil {
		if geneNamed(candidate.genes, "Patch.Filter.Attack") == nil {
			t.Fatal("the filter's envelope was not reached")
		}
	}
}

func TestPro800GeneValuesComeFromTheSeed(t *testing.T) {
	candidate := newTestPro800Patch(t)
	volume := geneNamed(candidate.genes, "Patch.OscA.Volume")
	if volume == nil || volume.value != 1000 {
		t.Fatalf("oscillator volume seeded as %+v", volume)
	}
	tuning := geneNamed(candidate.genes, "Patch.Tuning[0]")
	if tuning == nil || tuning.value != 12345 {
		t.Fatalf("tuning seeded as %+v", tuning)
	}
}

func TestPro800EncodeRejectsAnUnwritableRecord(t *testing.T) {
	// A 6E dump carries the aftertouch amount only when a flag says it is
	// there, so an amount without the flag is a value the message cannot
	// hold however it is set.
	older := probePatchData()
	older.Patch.Version = int(pro800.Version6E)
	older.Patch.LfoAftertouchPresent = false
	older.Patch.LfoAftertouch = 500
	if _, err := (newPro800Format()).Encode(older); err == nil {
		t.Fatal("encoded an aftertouch amount the layout has no room for")
	}
	candidate := newTestPro800Patch(t)
	if err := candidate.encodable(); err != nil {
		t.Fatalf("a seeded patch should be writable: %v", err)
	}
}

// A Pro800 seed that decodes but cannot be written back cannot be built from
// this package: the encoder refuses to produce one, and the operator fields a
// byte diff would have to aim at are not laid out at a uniform stride. The
// discard paths are therefore exercised through a stand-in, which is also the
// only way a future format can reach them without a hand-built file.
type unwritablePatch struct {
	*SysexPatch
	remaining *int
	reason    error
}

func (candidate unwritablePatch) encodable() error {
	if candidate.remaining != nil && *candidate.remaining > 0 {
		*candidate.remaining--
		return candidate.reason
	}
	return candidate.SysexPatch.encodable()
}

func (candidate unwritablePatch) clone() patch {
	return unwritablePatch{SysexPatch: candidate.SysexPatch.clone().(*SysexPatch), remaining: candidate.remaining, reason: candidate.reason}
}

type unwritableFactory struct {
	patchFactory
	candidate *SysexPatch
	remaining *int
	reason    error
}

func (factory unwritableFactory) newPatch(map[string]geneSemantic) (patch, error) {
	return unwritablePatch{SysexPatch: factory.candidate, remaining: factory.remaining, reason: factory.reason}, nil
}

// loadSeed is a no-op because the stand-in hands out a decoded patch rather
// than a format that reads one.
func (factory unwritableFactory) loadSeed(string, patch) error { return nil }

func TestPro800RunRejectsASeedItCannotWriteBack(t *testing.T) {
	// A seed the codec will not write back is not a starting point, and
	// finding that out at startup beats discarding every candidate of the
	// first round and then failing with an empty population.
	remaining := 1
	factory := unwritableFactory{
		patchFactory: sysexPatchFactory{format: newPro800Format()},
		candidate:    newTestPro800Patch(t),
		remaining:    &remaining,
		reason:       errors.New("value has no place in this dump layout"),
	}
	_, err := newMutation(factory, defaultEvolutionSettings(), rand.New(rand.NewSource(3)), nil, "seed.syx")
	if err == nil || !strings.Contains(err.Error(), "seed cannot be written back") {
		t.Fatalf("got error %v, want the seed to be rejected as unwritable", err)
	}
	if remaining != 0 {
		t.Fatal("the parent was never asked whether it can be written")
	}
}

func TestPro800RunSkipsACandidateItCannotWrite(t *testing.T) {
	// A mutation can leave a value the dump layout has nowhere to put. That
	// candidate is dropped and the rest of the round is still judged.
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(3)), nil, seedPath)
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
	parent, ok := engine.parent.(*SysexPatch)
	if !ok {
		t.Fatalf("parent is %T", engine.parent)
	}
	remaining := 1
	engine.population = []populationCandidate{
		{
			patch:     unwritablePatch{SysexPatch: parent, remaining: &remaining, reason: errors.New("value has no place in this dump layout")},
			reference: parent,
		},
		{patch: parent.clone(), reference: parent},
		{patch: parent.clone(), reference: parent},
	}
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	var output strings.Builder
	runner := mutationRunner{
		engine:     engine,
		auditioner: midiAuditioner{player: player},
		store:      sysexPatchStore{outputPath: filepath.Join(t.TempDir(), "best.syx")},
		input:      strings.NewReader("9 8"),
		output:     &output,
	}
	if err := runner.run(context.Background()); err != nil {
		t.Fatalf("a candidate that cannot be written ended the run: %v", err)
	}
	if !strings.Contains(output.String(), "skipped: value has no place in this dump layout") {
		t.Fatalf("the unwritable candidate was not reported: %q", output.String())
	}
	if !strings.Contains(output.String(), "selected candidate with rank 9") {
		t.Fatalf("the round was not ranked: %q", output.String())
	}
	// A dump is still a SysEx message, so the settle delay still applies.
	if len(*sleeps) == 0 || (*sleeps)[0] != defaultSysexSettle {
		t.Fatalf("first wait is %v, want the settle %v", (*sleeps)[0], defaultSysexSettle)
	}
	dumps := 0
	for _, message := range writer.messages {
		if message[0] == 0xf0 {
			dumps++
		}
	}
	// Two candidates were judged, and the run starts a second generation
	// before the input runs out, so at least two dumps is the floor here.
	if dumps < 2 {
		t.Fatalf("sent %d dumps, want one per writable candidate", dumps)
	}
}

func TestPro800RunWritesDecodableGenerations(t *testing.T) {
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	outputPath := filepath.Join(t.TempDir(), "best.syx")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(11)), nil, seedPath)
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, _ := newFakeMIDIPlayer(writer)
	var output strings.Builder
	runner := mutationRunner{
		engine:     engine,
		auditioner: midiAuditioner{player: player},
		store:      sysexPatchStore{outputPath: outputPath},
		// The second candidate is a mutation, so ranking it highest keeps a
		// changed champion rather than the untouched parent.
		input:  strings.NewReader("1 9 2"),
		output: &output,
	}
	if err := runner.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if _, err := (newPro800Format()).Decode(written); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if reflect.DeepEqual(written, pro800Message(t, probePatchData())) {
		t.Fatal("the champion is identical to the seed")
	}
}

func TestPro800GeneSemanticsUseReflectedNames(t *testing.T) {
	fixed := 4000
	format := newPro800Format()
	candidate, err := newSysexPatch(format, format.NewRoot(), map[string]geneSemantic{
		"Patch.Filter.Cutoff": {Policy: "fixed", Value: &fixed},
	})
	if err != nil {
		t.Fatalf("newSysexPatch: %v", err)
	}
	if err := candidate.loadMessage(pro800Message(t, probePatchData())); err != nil {
		t.Fatalf("loadMessage: %v", err)
	}
	cutoff := geneNamed(candidate.genes, "Patch.Filter.Cutoff")
	if cutoff == nil || cutoff.value != fixed || cutoff.policy != genePolicyFixed {
		t.Fatalf("fixed gene is %+v", cutoff)
	}
	if err := candidate.validateFixedValues(); err != nil {
		t.Fatalf("validateFixedValues: %v", err)
	}
	// A field the record marks as not a sound parameter cannot be named.
	if _, err := newSysexPatch(format, format.NewRoot(), map[string]geneSemantic{
		"Patch.OscA.Fine": {Policy: "fixed", Value: &fixed},
	}); err == nil {
		t.Fatal("accepted a rule for a field that is not a gene")
	}
}

func TestPro800FormatSelection(t *testing.T) {
	if _, err := newPatchFactory(configuration{format: pro800FormatName, output: "best.syx", midiChannel: 1}); err != nil {
		t.Fatalf("pro800 format: %v", err)
	}
	withModel := configuration{format: pro800FormatName, modelName: "Volca Bass", portName: "test", output: "best.syx", midiChannel: 1}
	if err := validateConfiguration(withModel); err != nil {
		t.Fatalf("flag validation rejected a well-formed configuration: %v", err)
	}
	if _, err := newPatchFactory(withModel); err == nil {
		t.Fatal("accepted --model for a format without one")
	}
	withJSON := configuration{format: pro800FormatName, portName: "test", output: "best.syx", midiChannel: 1, jsonOutput: true}
	if _, err := newPatchFactory(withJSON); err == nil {
		t.Fatal("accepted --json for a format without a model")
	}
	if _, err := newPatchFactory(configuration{format: pro800FormatName, portName: "test", output: "best.mid", midiChannel: 1}); err == nil {
		t.Fatal("accepted an output path the format would not write")
	}
}

func TestPro800RunSaysWhyARoundEmptied(t *testing.T) {
	// A round can lose every candidate to a value the message cannot carry,
	// for instance by excluding every other gene in a 6E run. The error has
	// to name that rather than blaming the engine for producing nothing.
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(3)), nil, seedPath)
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
	parent, ok := engine.parent.(*SysexPatch)
	if !ok {
		t.Fatalf("parent is %T", engine.parent)
	}
	remaining := settings.roundSize
	population := make([]populationCandidate, 0, settings.roundSize)
	for range settings.roundSize {
		population = append(population, populationCandidate{
			patch:     unwritablePatch{SysexPatch: parent, remaining: &remaining, reason: errors.New("value has no place in this dump layout")},
			reference: parent,
		})
	}
	engine.population = population
	runner := mutationRunner{
		engine:     engine,
		auditioner: &recordingAuditioner{},
		store:      &recordingPatchStore{},
		input:      strings.NewReader(""),
		output:     io.Discard,
	}
	err = runner.run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "none of the 3 candidates could be written to the instrument") {
		t.Fatalf("got error %v, want the round to explain itself", err)
	}
}
