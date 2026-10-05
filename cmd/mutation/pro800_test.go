package main

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/sysex/behringer/pro800"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "encode")
	return messages[0]
}

func writePro800Seed(t *testing.T, message []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "voice.syx")
	require.NoError(t, os.WriteFile(path, message, 0o600), "write seed")
	return path
}

func newTestPro800Patch(t *testing.T) *SysexPatch {
	t.Helper()
	format := newPro800Format()
	candidate, err := newSysexPatch(format, format.NewRoot(), nil)
	require.NoError(t, err, "newSysexPatch")
	require.NoError(t, candidate.loadMessage(pro800Message(t, probePatchData())), "loadMessage")
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
	assert.Equal(t, pro800FormatName, format.ID(), "format ID")
	message := pro800Message(t, probePatchData())
	decoded, err := format.Decode(message)
	require.NoError(t, err, "Decode")
	data, ok := decoded.(*pro800.PatchData)
	require.True(t, ok, "Decode returned %T", decoded)
	// Everything the record carries has to survive, including the fields the
	// catalog leaves alone.
	assert.Equal(t, 42, data.Address, "decoded address")
	assert.Equal(t, "PROBE", data.Patch.Name, "decoded name")
	assert.Equal(t, 1000, data.Patch.OscA.Volume, "decoded oscillator A volume")
	assert.Equal(t, 200, data.Patch.OscB.Fine, "decoded oscillator B fine")
	assert.Equal(t, int32(12345), data.Patch.Tuning[0], "decoded tuning")
	_, err = format.Encode(&dx7StyleRoot{})
	assert.Error(t, err, "encoded a program from another format")
}

// dx7StyleRoot stands in for a program a different format produced.
type dx7StyleRoot struct{ Channel int }

func TestPro800CatalogFollowsTheStructTags(t *testing.T) {
	candidate := newTestPro800Patch(t)
	// A field the record marks as not a sound parameter is not a gene. That
	// is declared on the Pro800 struct, so this test names no field the
	// command would otherwise have to know about. Both directions live in
	// one table because they are one decision per field: a field listed on
	// the wrong side is the same mistake whichever way round it is.
	fields := []struct {
		name     string
		wantGene bool
	}{
		{name: "Address", wantGene: false},
		{name: "Patch.Version", wantGene: false},
		{name: "Patch.Reserved", wantGene: false},
		{name: "Patch.Name", wantGene: false},
		{name: "Patch.LfoAftertouchPresent", wantGene: false},
		// A 6E dump has nowhere to put these, so a run seeded from one
		// could not evolve them. Leaving them out for both layouts keeps
		// what a run can evolve independent of the layout it started from.
		{name: "Patch.VoiceSpread", wantGene: false},
		{name: "Patch.TrackingReference", wantGene: false},
		{name: "Patch.GlideMode", wantGene: false},
		{name: "Patch.PitchBend.Range", wantGene: false},
		{name: "Patch.OscA.Volume", wantGene: true},
		{name: "Patch.OscA.PitchMode", wantGene: true},
		{name: "Patch.Filter.Cutoff", wantGene: true},
		{name: "Patch.LfoAftertouch", wantGene: true},
		// The fine tuning and the sync switch exist only on the second
		// oscillator, so they are evolvable there and absent from the first
		// because the record has nowhere to put them.
		{name: "Patch.OscB.Fine", wantGene: true},
		{name: "Patch.OscB.Sync", wantGene: true},
		{name: "Patch.Tuning[0]", wantGene: true},
		{name: "Patch.Tuning[11]", wantGene: true},
		// The mod wheel's own range is in both layouts and stays evolvable,
		// which is why the skip has to sit on the pitch bend field rather
		// than on every field called Range.
		{name: "Patch.ModWheel.Range", wantGene: true},
		{name: "Patch.PitchBend.Target", wantGene: true},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			gene := geneNamed(candidate.genes, field.name)
			if field.wantGene {
				assert.NotNil(t, gene, "%s is a sound parameter and should be a gene", field.name)
				return
			}
			assert.Nil(t, gene, "%s is not a sound parameter and must not be a gene", field.name)
		})
	}
	assert.Equal(t, len(candidate.genes), candidate.mutableGeneCount(), "every catalogued gene should be mutable")
	// The embedded envelope inside the filter is flattened, so its fields
	// carry no extra segment and a report reads the way a panel does.
	if geneNamed(candidate.genes, "Patch.Filter.Envelope.Attack") == nil {
		if geneNamed(candidate.genes, "Patch.Filter.Attack") == nil {
			require.Fail(t, "the filter's envelope was not reached")
		}
	}
}

func TestPro800GeneValuesComeFromTheSeed(t *testing.T) {
	candidate := newTestPro800Patch(t)
	if volume := geneNamed(candidate.genes, "Patch.OscA.Volume"); assert.NotNil(t, volume, "no oscillator volume gene") {
		assert.Equal(t, 1000, volume.value, "oscillator volume seeded")
	}
	if tuning := geneNamed(candidate.genes, "Patch.Tuning[0]"); assert.NotNil(t, tuning, "no tuning gene") {
		assert.Equal(t, 12345, tuning.value, "tuning seeded")
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
	_, err := (newPro800Format()).Encode(older)
	assert.Error(t, err, "encoded an aftertouch amount the layout has no room for")
	assert.NoError(t, newTestPro800Patch(t).encodable(), "a seeded patch should be writable")
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed cannot be written back", "want the seed to be rejected as unwritable")
	assert.Zero(t, remaining, "the parent was never asked whether it can be written")
}

func TestPro800RunSkipsACandidateItCannotWrite(t *testing.T) {
	// A mutation can leave a value the dump layout has nowhere to put. That
	// candidate is dropped and the rest of the round is still judged.
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(3)), nil, seedPath)
	require.NoError(t, err, "newMutation")
	parent, ok := engine.parent.(*SysexPatch)
	require.True(t, ok, "parent is %T", engine.parent)
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
	require.NoError(t, runner.run(context.Background()), "a candidate that cannot be written ended the run")
	assert.Contains(t, output.String(), "skipped: value has no place in this dump layout",
		"the unwritable candidate was not reported")
	assert.Contains(t, output.String(), "selected candidate with rank 9", "the round was not ranked")
	// A dump is still a SysEx message, so the settle delay still applies.
	require.NotEmpty(t, *sleeps, "the run never waited before sending the dump")
	assert.Equal(t, defaultSysexSettle, (*sleeps)[0], "first wait")
	dumps := 0
	for _, message := range writer.messages {
		if message[0] == 0xf0 {
			dumps++
		}
	}
	// Two candidates were judged, and the run starts a second generation
	// before the input runs out, so at least two dumps is the floor here.
	assert.GreaterOrEqual(t, dumps, 2, "sent %d dumps, want one per writable candidate", dumps)
}

func TestPro800RunWritesDecodableGenerations(t *testing.T) {
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	outputPath := filepath.Join(t.TempDir(), "best.syx")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(11)), nil, seedPath)
	require.NoError(t, err, "newMutation")
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
	require.NoError(t, runner.run(context.Background()), "run")
	written, err := os.ReadFile(outputPath)
	require.NoError(t, err, "read output")
	_, err = (newPro800Format()).Decode(written)
	assert.NoError(t, err, "decode output")
	assert.NotEqual(t, pro800Message(t, probePatchData()), written, "the champion is identical to the seed")
}

func TestPro800GeneSemanticsUseReflectedNames(t *testing.T) {
	fixed := 4000
	format := newPro800Format()
	candidate, err := newSysexPatch(format, format.NewRoot(), map[string]geneSemantic{
		"Patch.Filter.Cutoff": {Policy: "fixed", Value: &fixed},
	})
	require.NoError(t, err, "newSysexPatch")
	require.NoError(t, candidate.loadMessage(pro800Message(t, probePatchData())), "loadMessage")
	if cutoff := geneNamed(candidate.genes, "Patch.Filter.Cutoff"); assert.NotNil(t, cutoff, "no filter cutoff gene") {
		assert.Equal(t, fixed, cutoff.value, "fixed gene value")
		assert.Equal(t, genePolicyFixed, cutoff.policy, "fixed gene policy")
	}
	assert.NoError(t, candidate.validateFixedValues(), "validateFixedValues")
	// A field the record marks as not a sound parameter cannot be named.
	_, err = newSysexPatch(format, format.NewRoot(), map[string]geneSemantic{
		"Patch.OscA.Fine": {Policy: "fixed", Value: &fixed},
	})
	assert.Error(t, err, "accepted a rule for a field that is not a gene")
}

func TestPro800FormatSelection(t *testing.T) {
	_, err := newPatchFactory(configuration{format: pro800FormatName, output: "best.syx", midiChannel: 1})
	assert.NoError(t, err, "pro800 format")
	withModel := configuration{format: pro800FormatName, modelName: "Volca Bass", portName: "test", output: "best.syx", midiChannel: 1}
	assert.NoError(t, validateConfiguration(withModel), "flag validation rejected a well-formed configuration")
	_, err = newPatchFactory(withModel)
	assert.Error(t, err, "accepted --model for a format without one")
	withJSON := configuration{format: pro800FormatName, portName: "test", output: "best.syx", midiChannel: 1, jsonOutput: true}
	_, err = newPatchFactory(withJSON)
	assert.Error(t, err, "accepted --json for a format without a model")
	_, err = newPatchFactory(configuration{format: pro800FormatName, portName: "test", output: "best.mid", midiChannel: 1})
	assert.Error(t, err, "accepted an output path the format would not write")
}

func TestPro800RunSaysWhyARoundEmptied(t *testing.T) {
	// A round can lose every candidate to a value the message cannot carry,
	// for instance by excluding every other gene in a 6E run. The error has
	// to name that rather than blaming the engine for producing nothing.
	seedPath := writePro800Seed(t, pro800Message(t, probePatchData()))
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newPro800Format()}, settings, rand.New(rand.NewSource(3)), nil, seedPath)
	require.NoError(t, err, "newMutation")
	parent, ok := engine.parent.(*SysexPatch)
	require.True(t, ok, "parent is %T", engine.parent)
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the 3 candidates could be written to the instrument",
		"want the round to explain itself")
}
