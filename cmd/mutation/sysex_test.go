package main

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
	dx7 "github.com/chzchzchz/midispa/sysex/yamaha/dx7"
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
	if err != nil {
		t.Fatalf("marshal test voice: %v", err)
	}
	return message
}

func newTestSysexPatch(t *testing.T) *SysexPatch {
	t.Helper()
	return newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), nil)
}

func newTestSysexPatchFrom(t *testing.T, message []byte, semantics map[string]geneSemantic) *SysexPatch {
	t.Helper()
	format := dx7Format{}
	candidate, err := newSysexPatch(format, format.NewRoot(), semantics)
	if err != nil {
		t.Fatalf("newSysexPatch: %v", err)
	}
	if err := candidate.loadMessage(message); err != nil {
		t.Fatalf("loadMessage: %v", err)
	}
	return candidate
}

func writeTestSeed(t *testing.T, message []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "voice.syx")
	if err := os.WriteFile(path, message, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return path
}

func TestSysexPatchReadsEveryTaggedVoiceField(t *testing.T) {
	candidate := newTestSysexPatch(t)
	if len(candidate.genes) < 100 {
		t.Fatalf("catalog has %d genes, want the whole voice", len(candidate.genes))
	}
	if candidate.formatID() != dx7SingleFormatName {
		t.Fatalf("format is %q, want %q", candidate.formatID(), dx7SingleFormatName)
	}
	// The seed supplies the values, so every gene starts from the file.
	byName := make(map[string]gene, len(candidate.genes))
	for _, gene := range candidate.genes {
		byName[gene.name] = gene
	}
	if got := byName["Osc[0].EgRate[1]"]; got.value != 20 {
		t.Fatalf("Osc[0].EgRate[1] seeded as %d, want 20", got.value)
	}
	if got := byName["Transpose"]; got.value != 12 {
		t.Fatalf("Transpose seeded as %d, want 12", got.value)
	}
}

func TestSysexPatchKeepsMetadataOutOfTheBudget(t *testing.T) {
	candidate := newTestSysexPatch(t)
	for _, gene := range candidate.genes {
		switch {
		case gene.name == "Channel", gene.name == "OperatorOn", strings.HasPrefix(gene.name, "VoiceName["):
			if gene.policy != genePolicyImmutable {
				t.Fatalf("%s is %v, want immutable", gene.name, gene.policy)
			}
		default:
			if gene.policy != genePolicyMutable {
				t.Fatalf("%s is %v, want mutable", gene.name, gene.policy)
			}
		}
	}
	if candidate.mutableGeneCount() == len(candidate.genes) {
		t.Fatal("no gene is excluded from the mutation budget")
	}
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
		if err != nil {
			t.Fatalf("encode generation %d: %v", generation, err)
		}
		if len(messages) != 1 {
			t.Fatalf("generation %d emitted %d messages, want 1", generation, len(messages))
		}
		if len(messages[0]) != 163 {
			t.Fatalf("generation %d emitted %d bytes, want 163", generation, len(messages[0]))
		}
		var decoded dx7.SingleVoice
		if err := decoded.UnmarshalBinary(messages[0]); err != nil {
			t.Fatalf("decode generation %d: %v", generation, err)
		}
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
	if _, err := candidate.encode(0); err == nil {
		t.Fatal("encoded a value the instrument would clamp")
	}
}

func TestSysexEncodeAppliesTheConfiguredChannel(t *testing.T) {
	format := dx7Format{channel: 9}
	candidate, err := newSysexPatch(format, format.NewRoot(), nil)
	if err != nil {
		t.Fatalf("newSysexPatch: %v", err)
	}
	if err := candidate.loadMessage(testSingleVoiceMessage(t, 0)); err != nil {
		t.Fatalf("loadMessage: %v", err)
	}
	messages, err := candidate.encode(0)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := messages[0][2]; got != 9 {
		t.Fatalf("emitted channel %d, want 9", got)
	}
}

func TestSysexPatchClonesWithoutSharingGenes(t *testing.T) {
	parent := newTestSysexPatch(t)
	child := parent.clone().(*SysexPatch)
	for index := range child.genes {
		if child.genes[index].policy == genePolicyMutable {
			child.genes[index].value = 7
		}
	}
	if parent.genes[0].value == 7 {
		t.Fatal("a clone changed its parent's genes")
	}
	changes, err := geneChanges(child, parent)
	if err != nil {
		t.Fatalf("geneChanges: %v", err)
	}
	if len(changes) == 0 {
		t.Fatal("a modified clone reported no changes")
	}
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
			format := sysexPatchFactory{format: dx7Format{}}
			if err := format.loadSeed(path, newTestSysexPatch(t)); err == nil {
				t.Fatal("accepted an unusable seed file")
			}
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
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	candidate := newTestSysexPatchFrom(t, message, nil)
	if _, err := candidate.encode(0); err != nil {
		t.Fatalf("encode a name the seed carried: %v", err)
	}
	seedPath := writeTestSeed(t, message)
	settings := defaultEvolutionSettings()
	settings.roundSize = minimumRoundSize
	engine, err := newMutation(sysexPatchFactory{format: dx7Format{}}, settings, rand.New(rand.NewSource(41)), nil, seedPath)
	if err != nil {
		t.Fatalf("a seed with a padded voice name was rejected: %v", err)
	}
	for _, gene := range geneValues(t, engine.parent) {
		if gene.name == "VoiceName[0]" && gene.value != 0 {
			t.Fatalf("padded voice name became %d", gene.value)
		}
	}
}

func TestSysexGeneSemanticsUseReflectedNames(t *testing.T) {
	fixed := 42
	semantics := map[string]geneSemantic{
		"Osc[0].EgRate[0]": {Policy: "exclude"},
		"Osc[0].EgRate[1]": {Policy: "fixed", Value: &fixed},
	}
	candidate := newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), semantics)
	for _, gene := range candidate.genes {
		switch gene.name {
		case "Osc[0].EgRate[0]":
			t.Fatal("excluded gene remains in the catalog")
		case "Osc[0].EgRate[1]":
			if gene.value != 42 || gene.policy != genePolicyFixed {
				t.Fatalf("fixed gene is %+v", gene)
			}
		}
	}
	if err := candidate.validateFixedValues(); err != nil {
		t.Fatalf("validateFixedValues: %v", err)
	}
}

func TestSysexFixedValueMustFitItsOwnField(t *testing.T) {
	outOfRange := 50
	notPrintable := 7
	tests := []struct {
		name      string
		semantics map[string]geneSemantic
	}{
		{name: "range", semantics: map[string]geneSemantic{"Transpose": {Policy: "fixed", Value: &outOfRange}}},
		{name: "ascii", semantics: map[string]geneSemantic{"VoiceName[0]": {Policy: "fixed", Value: &notPrintable}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := newTestSysexPatchFrom(t, testSingleVoiceMessage(t, 0), test.semantics)
			if err := candidate.validateFixedValues(); err == nil {
				t.Fatal("accepted a fixed value outside its field")
			}
		})
	}
}

func TestSysexPatchRejectsAnUnknownGeneName(t *testing.T) {
	semantics := map[string]geneSemantic{"NotAField": {Policy: "exclude"}}
	if _, err := newSysexPatch(dx7Format{}, (&dx7.SingleVoice{}), semantics); err == nil {
		t.Fatal("accepted semantics for a field that does not exist")
	}
}

func TestSysexStoreWritesDecodableDumps(t *testing.T) {
	candidate := newTestSysexPatch(t)
	outputPath := filepath.Join(t.TempDir(), "best.syx")
	store := sysexPatchStore{outputPath: outputPath}
	if store.path() != outputPath {
		t.Fatalf("store path is %q, want %q", store.path(), outputPath)
	}
	if err := store.save(candidate, 3); err != nil {
		t.Fatalf("save: %v", err)
	}
	for _, path := range []string{outputPath, outputPath + ".0003"} {
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var decoded dx7.SingleVoice
		if err := decoded.UnmarshalBinary(written); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
}

func TestSysexStoreRejectsAnotherFormat(t *testing.T) {
	store := sysexPatchStore{outputPath: filepath.Join(t.TempDir(), "best.syx")}
	if err := store.save(newTestPatch(t, "Sound Controller"), 0); err == nil {
		t.Fatal("wrote a CC patch as a SysEx dump")
	}
	ccStore := smfPatchStore{outputPath: filepath.Join(t.TempDir(), "best.mid")}
	if err := ccStore.save(newTestSysexPatch(t), 0); err == nil {
		t.Fatal("wrote a SysEx patch as an SMF")
	}
}

func TestSysexAuditionSendsOneMessageAndSettles(t *testing.T) {
	candidate := newTestSysexPatch(t)
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	if err := player.audition(context.Background(), candidate, nil); err != nil {
		t.Fatalf("audition: %v", err)
	}
	// Two cleanup messages, the patch, then the probe; the settle wait lands
	// between the patch and the probe.
	if len(*sleeps) == 0 || (*sleeps)[0] != defaultSysexSettle {
		t.Fatalf("first wait is %v, want the SysEx settle %v", (*sleeps)[0], defaultSysexSettle)
	}
	if writer.messages[2][0] != 0xf0 {
		t.Fatalf("third message is not a SysEx message: %v", writer.messages[2])
	}
	if writer.messages[3][0] != 0x90 {
		t.Fatalf("probe note did not follow the dump: %v", writer.messages[3])
	}
}

func TestSysexAuditionHonoursTheSettleFlag(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	player.sysexSettle = 250 * time.Millisecond
	if err := player.audition(context.Background(), newTestSysexPatch(t), nil); err != nil {
		t.Fatalf("audition: %v", err)
	}
	if got := (*sleeps)[0]; got != 250*time.Millisecond {
		t.Fatalf("settled for %v, want 250ms", got)
	}
}

func TestCCAuditionDoesNotSettle(t *testing.T) {
	player, _, sleeps := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: -1})
	if err := player.audition(context.Background(), newTestPatch(t, "Sound Controller"), nil); err != nil {
		t.Fatalf("audition: %v", err)
	}
	if len(*sleeps) == 0 || (*sleeps)[0] == defaultSysexSettle {
		t.Fatalf("a CC patch waited for a SysEx settle: %v", *sleeps)
	}
}

func TestSysexFormatSelection(t *testing.T) {
	if _, err := newPatchFactory(configuration{format: ccFormatName, modelName: "Volca Bass", output: "best.mid", midiChannel: 1}); err != nil {
		t.Fatalf("cc format: %v", err)
	}
	if _, err := newPatchFactory(configuration{format: dx7SingleFormatName, output: "best.syx", midiChannel: 1}); err != nil {
		t.Fatalf("dx7-single format: %v", err)
	}
	if _, err := newPatchFactory(configuration{format: "dx7-bulk", output: "best.syx", midiChannel: 1}); err == nil {
		t.Fatal("accepted a format that does not exist")
	}
	// A CC run is defined by a model, so the format requires the name rather
	// than the flag parser guessing it from --format.
	if _, err := newPatchFactory(configuration{format: ccFormatName, output: "best.mid", midiChannel: 1}); err == nil {
		t.Fatal("started a CC run without a model")
	}
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
			if err := validateConfiguration(test.config); err != nil {
				t.Fatalf("flag validation rejected a well-formed configuration: %v", err)
			}
			if _, err := newPatchFactory(test.config); err == nil {
				t.Fatal("accepted an output path the format would not write")
			}
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
	engine, err := newMutation(sysexPatchFactory{format: dx7Format{channel: seedChannel - 1}}, settings, rand.New(rand.NewSource(11)), nil, seedPath)
	if err != nil {
		t.Fatalf("newMutation: %v", err)
	}
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
	if err := runner.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(output.String(), "selected candidate with rank 9") {
		t.Fatalf("missing selection in %q", output.String())
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var decoded dx7.SingleVoice
	if err := decoded.UnmarshalBinary(written); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if reflect.DeepEqual(decoded, *testSingleVoice()) {
		t.Fatal("the champion is identical to the seed")
	}
	// Every candidate is a complete dump, and the probe note follows one once
	// the settle delay has passed.
	dumps := 0
	for index, message := range writer.messages {
		if message[0] != 0xf0 {
			continue
		}
		dumps++
		if message[2] != byte(seedChannel-1) {
			t.Fatalf("dump %d targets channel %d, want %d", index, message[2], seedChannel-1)
		}
		if index+1 >= len(writer.messages) || !midi.IsNoteOn(writer.messages[index+1][0]) {
			t.Fatalf("message %d was not followed by a probe note", index)
			continue
		}
		// The patch and the notes that audition it must agree, or the probe
		// sounds whatever the instrument still had loaded.
		if note := midi.Channel(writer.messages[index+1][0]); note != int(message[2]) {
			t.Fatalf("probe note on channel %d follows a dump for channel %d", note, message[2])
		}
	}
	// The run continues into a second generation before the judge input runs
	// out, so there are at least as many dumps as there were candidates.
	if dumps < settings.roundSize {
		t.Fatalf("sent %d dumps, want at least one per candidate (%d)", dumps, settings.roundSize)
	}
	if len(*sleeps) == 0 || (*sleeps)[0] != defaultSysexSettle {
		t.Fatalf("first wait is %v, want the settle %v", (*sleeps)[0], defaultSysexSettle)
	}
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
			if err := validateConfiguration(test.config); err != nil {
				t.Fatalf("flag validation rejected a well-formed configuration: %v", err)
			}
			if _, err := newPatchFactory(test.config); err == nil {
				t.Fatal("accepted an incompatible combination")
			}
		})
	}
}

func TestSysexImmutableFieldsSurviveMutation(t *testing.T) {
	// A field the format calls metadata must come out of a long run byte for
	// byte identical, or a changed-gene report could claim a change the
	// instrument would never hear.
	parent := newTestSysexPatch(t)
	settings := defaultEvolutionSettings()
	settings.mutatedGenes = 8
	settings.mutationSigma = 60
	engine := &Mutation{settings: settings, random: rand.New(rand.NewSource(31))}
	immutable := 0
	for generation := 0; generation < 30; generation++ {
		child := engine.mutatePatch(parent, generation%2 == 0)
		values := geneValues(t, child)
		for index, gene := range values {
			if parent.genes[index].policy == genePolicyMutable {
				continue
			}
			immutable++
			if gene.value != parent.genes[index].value {
				t.Fatalf("immutable gene %s changed from %d to %d", gene.name, parent.genes[index].value, gene.value)
			}
		}
	}
	if immutable == 0 {
		t.Fatal("no immutable gene was checked")
	}
}
