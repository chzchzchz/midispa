package main

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/internal/fieldrules"
	"github.com/chzchzchz/midispa/midi"
	dx7 "github.com/chzchzchz/midispa/sysex/yamaha/dx7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSingleVoice is a valid one-voice program with a few non-default values,
// so a mutation is visible in the encoded message.
func testSingleVoice() *dx7.SingleVoice {
	voice := &dx7.SingleVoice{}
	voice.Algorithm = 5
	voice.Transpose = 12
	voice.Osc[0].EgRate = [4]int{10, 20, 30, 40}
	voice.Osc[0].OutLevel = 99
	copy(voice.VoiceName[:], "INIT VOICE")
	return voice
}

func testSingleVoiceMessage(t *testing.T, channel int) []byte {
	t.Helper()
	voice := testSingleVoice()
	voice.Channel = channel
	message, err := voice.MarshalBinary()
	require.NoError(t, err, "marshal test voice")
	return message
}

func newTestSysexPatch(t *testing.T) *SysexPatch {
	t.Helper()
	return newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), nil)
}

func newTestSysexPatchFrom(t *testing.T, message []byte, semantics map[string]fieldrules.Rule) *SysexPatch {
	t.Helper()
	format := newDX7Format(0)
	candidate, err := newSysexPatch(format, format.NewRoot(), semantics)
	require.NoError(t, err, "newSysexPatch")
	require.NoError(t, candidate.loadMessage(message), "loadMessage")
	return candidate
}

func writeTestSeed(t *testing.T, message []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "voice.syx")
	require.NoError(t, os.WriteFile(path, message, 0o600), "write seed")
	return path
}

func TestSysexPatchReadsEveryTaggedVoiceField(t *testing.T) {
	candidate := newTestSysexPatch(t)
	assert.GreaterOrEqual(t, len(candidate.genes), 100, "catalog has %d genes, want the whole voice", len(candidate.genes))
	assert.Equal(t, dx7SingleFormatName, candidate.formatID(), "format")
	// The seed supplies the values, so every gene starts from the file.
	byName := make(map[string]gene, len(candidate.genes))
	for _, gene := range candidate.genes {
		byName[gene.name] = gene
	}
	assert.Equal(t, 20, byName["Osc[0].EgRate[1]"].value, "Osc[0].EgRate[1] seeded, want 20")
	assert.Equal(t, 12, byName["Transpose"].value, "Transpose seeded, want 12")
}

// A field the record marks as not a sound parameter is not in the catalog at
// all, so it can never be reported as changed and can never be named in a
// gene rule. This is a property of the DX7 struct tags, not of the format.
func TestSysexPatchOmitsMetadata(t *testing.T) {
	candidate := newTestSysexPatch(t)
	for _, gene := range candidate.genes {
		assert.NotEqual(t, "Channel", gene.name, "%s is not a sound parameter and must not be a gene", gene.name)
		assert.False(t, strings.HasPrefix(gene.name, "VoiceName["),
			"%s is not a sound parameter and must not be a gene", gene.name)
		assert.Equal(t, genePolicyMutable, gene.policy, "%s policy", gene.name)
	}
	assert.Equal(t, len(candidate.genes), candidate.mutableGeneCount(), "every catalogued gene should be mutable")
}

func TestSysexEncodeRoundTripsThroughTheCodec(t *testing.T) {
	candidate := newTestSysexPatch(t)
	settings := defaultEvolutionSettings()
	settings.mutatedGenes = 5
	settings.mutationSigma = 40
	engine := &Mutation{settings: settings, random: rand.New(rand.NewSource(3))}
	for generation := 0; generation < 20; generation++ {
		child := engine.mutatePatch(candidate, generation%2 == 0)
		messages, err := child.encode(0)
		require.NoError(t, err, "encode generation %d", generation)
		require.Len(t, messages, 1, "generation %d emitted %d messages, want 1", generation, len(messages))
		assert.Len(t, messages[0], 163, "generation %d emitted %d bytes, want 163", generation, len(messages[0]))
		var decoded dx7.SingleVoice
		assert.NoError(t, decoded.UnmarshalBinary(messages[0]), "decode generation %d", generation)
	}
}

func TestSysexEncodeRejectsAnOutOfDomainValue(t *testing.T) {
	candidate := newTestSysexPatch(t)
	// Bypass the mutation engine to prove the codec is the validation gate.
	for index := range candidate.genes {
		if candidate.genes[index].name == "Transpose" {
			candidate.genes[index].value = 120
		}
	}
	_, err := candidate.encode(0)
	assert.Error(t, err, "encoded a value the instrument would clamp")
}

func TestSysexEncodeAppliesTheConfiguredChannel(t *testing.T) {
	format := newDX7Format(9)
	candidate, err := newSysexPatch(format, format.NewRoot(), nil)
	require.NoError(t, err, "newSysexPatch")
	require.NoError(t, candidate.loadMessage(testSingleVoiceMessage(t, 0)), "loadMessage")
	messages, err := candidate.encode(0)
	require.NoError(t, err, "encode")
	assert.Equal(t, byte(9), messages[0][2], "emitted channel")
}

func TestSysexPatchClonesWithoutSharingGenes(t *testing.T) {
	parent := newTestSysexPatch(t)
	child := parent.clone().(*SysexPatch)
	for index := range child.genes {
		if child.genes[index].policy == genePolicyMutable {
			child.genes[index].value = 7
		}
	}
	assert.NotEqual(t, 7, parent.genes[0].value, "a clone changed its parent's genes")
	changes, err := geneChanges(child, parent)
	require.NoError(t, err, "geneChanges")
	assert.NotEmpty(t, changes, "a modified clone reported no changes")
}

func TestSysexSeedRejectsUnusableDumps(t *testing.T) {
	valid := testSingleVoiceMessage(t, 0)
	tests := []struct {
		name    string
		message []byte
	}{
		{name: "empty", message: nil},
		{name: "short", message: valid[:len(valid)-1]},
		{name: "trailing bytes", message: append(append([]byte(nil), valid...), 0x00)},
		{name: "two messages", message: append(append([]byte(nil), valid...), valid...)},
		{name: "other manufacturer", message: func() []byte {
			other := append([]byte(nil), valid...)
			other[1] = 0x42
			return other
		}()},
		{name: "bad checksum", message: func() []byte {
			corrupt := append([]byte(nil), valid...)
			corrupt[len(corrupt)-2] ^= 0x01
			return corrupt
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestSeed(t, test.message)
			format := sysexPatchFactory{format: newDX7Format(0)}
			assert.Error(t, format.loadSeed(path, newTestSysexPatch(t)), "accepted an unusable seed file")
		})
	}
}

func TestSysexSeedAcceptsANonPrintableVoiceName(t *testing.T) {
	// A dump may pad a name with control bytes; the wire range is permissive
	// even though the generator draws printable text, and the name is pinned
	// to whatever the seed carried rather than judged against that generator
	// domain.
	voice := testSingleVoice()
	for index := range voice.VoiceName {
		voice.VoiceName[index] = 0
	}
	message, err := voice.MarshalBinary()
	require.NoError(t, err, "marshal")
	candidate := newTestSysexPatchFrom(t, message, nil)
	_, err = candidate.encode(0)
	assert.NoError(t, err, "encode a name the seed carried")
	seedPath := writeTestSeed(t, message)
	settings := defaultEvolutionSettings()
	settings.roundSize = minimumRoundSize
	engine, err := newMutation(sysexPatchFactory{format: newDX7Format(0)}, settings, rand.New(rand.NewSource(41)), nil, seedPath)
	require.NoError(t, err, "a seed with a padded voice name was rejected")
	for _, gene := range geneValues(t, engine.parent) {
		if gene.name == "VoiceName[0]" {
			assert.Equal(t, 0, gene.value, "padded voice name became %d", gene.value)
		}
	}
}

func TestSysexGeneSemanticsUseReflectedNames(t *testing.T) {
	fixed := 42
	semantics := map[string]fieldrules.Rule{
		"Osc[0].EgRate[0]": {Policy: "exclude"},
		"Osc[0].EgRate[1]": {Policy: "fixed", Value: &fixed},
	}
	candidate := newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), semantics)
	for _, gene := range candidate.genes {
		switch gene.name {
		case "Osc[0].EgRate[0]":
			require.Fail(t, "excluded gene remains in the catalog")
		case "Osc[0].EgRate[1]":
			assert.Equal(t, 42, gene.value, "fixed gene value")
			assert.Equal(t, genePolicyFixed, gene.policy, "fixed gene policy")
		}
	}
	assert.NoError(t, candidate.validateFixedValues(), "validateFixedValues")
}

func TestSysexFixedValueMustFitItsOwnField(t *testing.T) {
	// A DX7 field carries its own bounds, so a value that is a legal MIDI
	// data byte can still be outside what the instrument accepts.
	outOfRange := 50
	candidate := newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), map[string]fieldrules.Rule{
		"Transpose": {Policy: "fixed", Value: &outOfRange},
	})
	assert.Error(t, candidate.validateFixedValues(), "accepted a fixed value outside its field")
}

func TestSysexRuleCannotNameASkippedField(t *testing.T) {
	// A field the record marks as not a sound parameter is not in the
	// catalog, so a rule naming it is a mistake rather than a silent no-op.
	_, err := newSysexPatch(newDX7Format(0), (&dx7.SingleVoice{}), map[string]fieldrules.Rule{
		"VoiceName[0]": {Policy: "fixed"},
	})
	assert.Error(t, err, "accepted a rule for a field that is not a gene")
}

func TestSysexPatchRejectsAnUnknownGeneName(t *testing.T) {
	semantics := map[string]fieldrules.Rule{"NotAField": {Policy: "exclude"}}
	_, err := newSysexPatch(newDX7Format(0), (&dx7.SingleVoice{}), semantics)
	assert.Error(t, err, "accepted semantics for a field that does not exist")
}

func TestSysexStoreWritesDecodableDumps(t *testing.T) {
	candidate := newTestSysexPatch(t)
	outputPath := filepath.Join(t.TempDir(), "best.syx")
	store := sysexPatchStore{outputPath: outputPath}
	assert.Equal(t, outputPath, store.path(), "store path")
	require.NoError(t, store.save(candidate, 3), "save")
	for _, path := range []string{outputPath, outputPath + ".0003"} {
		written, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		var decoded dx7.SingleVoice
		assert.NoError(t, decoded.UnmarshalBinary(written), "decode %s", path)
	}
}

func TestSysexStoreRejectsAnotherFormat(t *testing.T) {
	store := sysexPatchStore{outputPath: filepath.Join(t.TempDir(), "best.syx")}
	assert.Error(t, store.save(newTestPatch(t, "Sound Controller"), 0), "wrote a CC patch as a SysEx dump")
	ccStore := smfPatchStore{outputPath: filepath.Join(t.TempDir(), "best.mid")}
	assert.Error(t, ccStore.save(newTestSysexPatch(t), 0), "wrote a SysEx patch as an SMF")
}

func TestSysexAuditionSendsOneMessageAndSettles(t *testing.T) {
	candidate := newTestSysexPatch(t)
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	require.NoError(t, player.audition(context.Background(), candidate, nil), "audition")
	// Two cleanup messages, the patch, then the probe; the settle wait lands
	// between the patch and the probe.
	require.NotEmpty(t, *sleeps, "the audition never waited")
	assert.Equal(t, defaultSysexSettle, (*sleeps)[0], "first wait")
	require.GreaterOrEqual(t, len(writer.messages), 4, "the audition sent too few messages")
	assert.Equal(t, byte(0xf0), writer.messages[2][0], "third message is not a SysEx message")
	assert.Equal(t, byte(0x90), writer.messages[3][0], "probe note did not follow the dump")
}

func TestSysexAuditionHonoursTheSettleFlag(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	player.sysexSettle = 250 * time.Millisecond
	require.NoError(t, player.audition(context.Background(), newTestSysexPatch(t), nil), "audition")
	require.NotEmpty(t, *sleeps, "the audition never waited")
	assert.Equal(t, 250*time.Millisecond, (*sleeps)[0], "settled for the wrong duration")
}

func TestCCAuditionDoesNotSettle(t *testing.T) {
	player, _, sleeps := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: -1})
	require.NoError(t, player.audition(context.Background(), newTestPatch(t, "Sound Controller"), nil), "audition")
	// The original check failed on an empty wait list as well as on a settle
	// wait, so an audition that never waited did not pass this either.
	require.NotEmpty(t, *sleeps, "a CC audition recorded no waits at all")
	assert.NotEqual(t, defaultSysexSettle, (*sleeps)[0], "a CC patch waited for a SysEx settle")
}

func TestSysexFormatSelection(t *testing.T) {
	_, err := newPatchFactory(configuration{format: ccFormatName, modelName: "Volca Bass", output: "best.mid", midiChannel: 1})
	assert.NoError(t, err, "cc format")
	_, err = newPatchFactory(configuration{format: dx7SingleFormatName, output: "best.syx", midiChannel: 1})
	assert.NoError(t, err, "dx7-single format")
	_, err = newPatchFactory(configuration{format: "dx7-bulk", output: "best.syx", midiChannel: 1})
	assert.Error(t, err, "accepted a format that does not exist")
	// A CC run is defined by a model, so the format requires the name rather
	// than the flag parser guessing it from --format.
	_, err = newPatchFactory(configuration{format: ccFormatName, output: "best.mid", midiChannel: 1})
	assert.Error(t, err, "started a CC run without a model")
}

// The output suffix is a property of the selected format, so a run is rejected
// by the format that would write the file rather than by its name.
func TestOutputExtensionComesFromTheFormat(t *testing.T) {
	tests := []struct {
		name   string
		config configuration
	}{
		{
			name:   "cc writing a raw dump",
			config: configuration{format: ccFormatName, modelName: "Volca Bass", portName: "test", output: "best.syx", midiChannel: 1},
		},
		{
			name:   "sysex writing an smf",
			config: configuration{format: dx7SingleFormatName, portName: "test", output: "best.mid", midiChannel: 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NoError(t, validateConfiguration(test.config), "flag validation rejected a well-formed configuration")
			_, err := newPatchFactory(test.config)
			assert.Error(t, err, "accepted an output path the format would not write")
		})
	}
}

func TestSysexRunWritesDecodableGenerations(t *testing.T) {
	// A channel other than the default, so the dump and the probe notes are
	// checked against a value that cannot pass by accident.
	const seedChannel = 5
	seed := testSingleVoiceMessage(t, 0)
	seedPath := writeTestSeed(t, seed)
	outputPath := filepath.Join(t.TempDir(), "best.syx")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine, err := newMutation(sysexPatchFactory{format: newDX7Format(seedChannel - 1)}, settings, rand.New(rand.NewSource(11)), nil, seedPath)
	require.NoError(t, err, "newMutation")
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayerForChannel(writer, seedChannel)
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
	assert.Contains(t, output.String(), "selected candidate with rank 9", "missing selection")
	written, err := os.ReadFile(outputPath)
	require.NoError(t, err, "read output")
	var decoded dx7.SingleVoice
	require.NoError(t, decoded.UnmarshalBinary(written), "decode output")
	assert.NotEqual(t, *testSingleVoice(), decoded, "the champion is identical to the seed")
	// Every candidate is a complete dump, and the probe note follows one once
	// the settle delay has passed.
	dumps := 0
	for index, message := range writer.messages {
		if message[0] != 0xf0 {
			continue
		}
		dumps++
		assert.Equal(t, byte(seedChannel-1), message[2], "dump %d targets the wrong channel", index)
		if index+1 >= len(writer.messages) {
			require.Fail(t, "message %d was not followed by a probe note", index)
		}
		assert.True(t, midi.IsNoteOn(writer.messages[index+1][0]), "message %d was not followed by a probe note", index)
		// The patch and the notes that audition it must agree, or the probe
		// sounds whatever the instrument still had loaded.
		assert.Equal(t, int(message[2]), midi.Channel(writer.messages[index+1][0]),
			"probe note after dump %d plays on a different channel", index)
	}
	// The run continues into a second generation before the judge input runs
	// out, so there are at least as many dumps as there were candidates.
	assert.GreaterOrEqual(t, dumps, settings.roundSize, "sent %d dumps, want at least one per candidate (%d)", dumps, settings.roundSize)
	require.NotEmpty(t, *sleeps, "the run never waited before sending the dump")
	assert.Equal(t, defaultSysexSettle, (*sleeps)[0], "first wait")
}

func TestSysexRunRejectsInvalidCombinations(t *testing.T) {
	base := configuration{
		format:      dx7SingleFormatName,
		portName:    "test",
		output:      "best.syx",
		seedPath:    writeTestSeed(t, testSingleVoiceMessage(t, 0)),
		midiChannel: defaultMIDIChannelNumber,
		settings:    defaultEvolutionSettings(),
	}
	// Every rule below belongs to the format, so none of them is a property
	// of the flag values on their own.
	withModel := base
	withModel.modelName = "Volca Bass"
	withJSON := base
	withJSON.jsonOutput = true
	withSMFOutput := base
	withSMFOutput.output = "best.mid"
	tests := []struct {
		name   string
		config configuration
	}{
		{name: "model", config: withModel},
		{name: "json dump", config: withJSON},
		{name: "smf output", config: withSMFOutput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.NoError(t, validateConfiguration(test.config), "flag validation rejected a well-formed configuration")
			_, err := newPatchFactory(test.config)
			assert.Error(t, err, "accepted an incompatible combination")
		})
	}
}
