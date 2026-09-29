package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
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
		if port != "test" {
			t.Fatalf("factory received port %q, want test", port)
		}
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
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got error %v, want context cancellation", err)
	}
	if !factoryCalled {
		t.Fatal("MIDI output factory was not called")
	}
	if len(writer.messages) == 0 {
		t.Fatal("injected output received no cleanup messages")
	}
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
	if err := runner.run(context.Background()); err != nil {
		t.Fatalf("runner: %v", err)
	}
	if len(auditioner.patches) < settings.roundSize {
		t.Fatalf("auditioned %d candidates, want at least %d", len(auditioner.patches), settings.roundSize)
	}
	if len(store.patches) != 1 || len(store.generations) != 1 || store.generations[0] != 0 {
		t.Fatalf("unexpected saves: patches=%d generations=%v", len(store.patches), store.generations)
	}
}
