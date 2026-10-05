package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testWriteCloser struct{}

func (testWriteCloser) Close() error {
	return nil
}

func TestRunMutationWithFactoryUsesInjectedOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &recordingMIDIWriter{failAt: -1}
	factoryCalled := false
	factory := func(port string) (io.Writer, io.Closer, error) {
		factoryCalled = true
		assert.Equal(t, "test", port, "factory received the wrong port")
		return writer, testWriteCloser{}, nil
	}
	config := configuration{
		format:      ccFormatName,
		modelName:   "Sound Controller",
		portName:    "test",
		output:      filepath.Join(t.TempDir(), "best.mid"),
		midiChannel: defaultMIDIChannelNumber,
		settings:    defaultEvolutionSettings(),
	}
	err := runMutationWithFactory(ctx, config, strings.NewReader(""), io.Discard, factory)
	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, factoryCalled, "MIDI output factory was not called")
	assert.NotEmpty(t, writer.messages, "injected output received no cleanup messages")
}

func TestRunMutationWithDumpExcludesNeverStartsASession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "excludes.json")
	// A dump is answered by the model alone, so a configuration that a run
	// would reject -- no port, no output, no seed, no evolution settings --
	// still has to produce its file, and must not reach the instrument.
	config := configuration{
		format:       ccFormatName,
		modelName:    "Volca Bass",
		dumpExcludes: path,
	}
	factory := func(string) (io.Writer, io.Closer, error) {
		return nil, nil, fmt.Errorf("a dump opened a MIDI output")
	}
	var output strings.Builder
	require.NoError(t, runMutationWithFactory(context.Background(), config, strings.NewReader(""), &output, factory), "runMutationWithFactory")
	assert.FileExists(t, path, "the dump wrote no file")
	assert.Contains(t, output.String(), "Volca Bass", "missing report")
	assert.Contains(t, output.String(), "excluded parameters", "missing report")

	semantics, err := loadGeneSemantics(path)
	require.NoError(t, err, "the dumped file is not a gene semantics file")
	catalog, err := newPatchWithSemantics("Volca Bass", nil)
	require.NoError(t, err, "catalog")
	assert.Len(t, semantics, len(catalog.genes), "one rule per parameter")
}

type recordingAuditioner struct {
	patches []patch
}

func (auditioner *recordingAuditioner) audition(_ context.Context, candidate patch) error {
	auditioner.patches = append(auditioner.patches, candidate)
	return nil
}

type recordingPatchStore struct {
	patches     []patch
	generations []int
}

func (store *recordingPatchStore) path() string {
	return "best.mid"
}

func (store *recordingPatchStore) save(candidate patch, generation int) error {
	store.patches = append(store.patches, candidate)
	store.generations = append(store.generations, generation)
	return nil
}

func TestMutationRunnerUsesHardwareIndependentInterfaces(t *testing.T) {
	parent := newTestPatch(t, "Sound Controller")
	settings := defaultEvolutionSettings()
	settings.roundSize = 3
	engine := newTestMutation(parent, settings, 12)
	auditioner := &recordingAuditioner{}
	store := &recordingPatchStore{}
	runner := mutationRunner{
		engine:     engine,
		auditioner: auditioner,
		store:      store,
		input:      strings.NewReader("9 8 7"),
		output:     io.Discard,
	}
	require.NoError(t, runner.run(context.Background()), "runner")
	assert.GreaterOrEqual(t, len(auditioner.patches), settings.roundSize, "auditioned candidates")
	require.Len(t, store.patches, 1, "unexpected number of saves")
	require.Len(t, store.generations, 1, "unexpected number of saves")
	assert.Equal(t, 0, store.generations[0], "saved generation")
}
