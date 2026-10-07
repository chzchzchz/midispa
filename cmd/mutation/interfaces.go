package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"regexp"

	"github.com/chzchzchz/midispa/track"
)

// patch is one candidate in a format-agnostic population. The engine only
// copies, changes, and compares bounded gene values; turning those values into
// MIDI traffic is the format's job, so the genetic engine never learns a
// vendor field name or a message shape.
type patch interface {
	formatID() string
	clone() patch
	randomize(*rand.Rand)
	mutate(*rand.Rand, evolutionSettings, bool)
	// geneStore exposes the shared value list, which is how crossover and
	// change reporting reach another candidate without knowing its format.
	geneStore() *patchGenes
	mutableGeneCount() int
	validateFixedValues() error
	// encodable reports whether this patch can be written to its wire format
	// at all. A record can hold a value that the message it uses has nowhere
	// to put, which no value range catches, and such a candidate is worth
	// discarding rather than failing the session over.
	encodable() error
	encode(channelIndex int) ([][]byte, error)
}

// patchFactory is where the command chooses a format. Seeding and output are
// format-specific operations that the engine must not reinterpret: a CC seed
// overlays controller values on a randomized patch, while a SysEx seed
// replaces the decoded program and therefore every gene value.
type patchFactory interface {
	newPatch(semantics map[string]geneSemantic) (patch, error)
	loadSeed(path string, target patch) error
	store(outputPath string) patchStore
	requiresSeed() bool
	supportsJSONDump() bool
	// acceptsModelName reports whether --model applies to this format, which
	// decides both that a CC run must name a model and that a SysEx run must
	// not.
	acceptsModelName() bool
	// outputExtension is the file suffix the format's dumps use. A format
	// declares its own rather than the command inferring one from its name,
	// so a future format is not judged by a convention meant for another.
	outputExtension() string
}

func crossoverGenes(child, other patch, random *rand.Rand, filter *regexp.Regexp) error {
	return child.geneStore().crossover(other.geneStore(), random, filter)
}

func geneChanges(candidate, reference patch) ([]geneChange, error) {
	return candidate.geneStore().changesFrom(reference.geneStore())
}

// The runner depends on these boundaries rather than ALSA, SMF, or SysEx
// implementations.
type auditioner interface {
	audition(context.Context, patch) error
}

type patchStore interface {
	path() string
	save(patch, int) error
}

// midiOutputFactory defers ALSA setup until the command has validated its configuration.
type midiOutputFactory func(string) (io.Writer, io.Closer, error)

type midiAuditioner struct {
	player   *midiPlayer
	playback *track.Pattern
}

func (audition midiAuditioner) audition(ctx context.Context, candidate patch) error {
	return audition.player.audition(ctx, candidate, audition.playback)
}

// ccFormatID and ccFormatName name the CC format, which is defined by a model
// from cc/model.go rather than by a wire message.
const ccFormatName = "cc"

// ccPatchFactory builds CC patches for one model. The MIDI channel is applied
// when a patch is encoded, not stored in a gene, so switching channels does not
// change the evolved values.
type ccPatchFactory struct {
	modelName   string
	midiChannel int
	jsonOutput  bool
}

func (factory ccPatchFactory) newPatch(semantics map[string]geneSemantic) (patch, error) {
	return newPatchWithSemantics(factory.modelName, semantics)
}

func (factory ccPatchFactory) loadSeed(path string, target patch) error {
	ccPatch, err := asCCPatch(target)
	if err != nil {
		return err
	}
	return loadSeedPatch(path, ccPatch)
}

func (factory ccPatchFactory) store(outputPath string) patchStore {
	return smfPatchStore{outputPath: outputPath, midiChannel: factory.midiChannel, jsonOutput: factory.jsonOutput}
}

func (factory ccPatchFactory) requiresSeed() bool { return false }

func (factory ccPatchFactory) supportsJSONDump() bool { return true }

func (factory ccPatchFactory) acceptsModelName() bool { return true }

func (factory ccPatchFactory) outputExtension() string { return smfOutputExtension }

// asCCPatch rejects a patch from another format instead of trusting the
// runner to pair a format with the store it came from.
func asCCPatch(target patch) (*Patch, error) {
	ccPatch, ok := target.(*Patch)
	if !ok {
		return nil, fmt.Errorf("CC handling requires a CC patch, got %s", target.formatID())
	}
	return ccPatch, nil
}
