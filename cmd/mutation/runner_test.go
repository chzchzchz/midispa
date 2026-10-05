package main

import (
	"context"
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
