package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJudgeMutationAcceptsRank(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, nil, newJudgeInput(strings.NewReader("7")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 7, score)
	assert.Equal(t, 1, auditions)
}

func TestJudgeMutationReplaysWholeMutation(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, nil, newJudgeInput(strings.NewReader("r\n8")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 8, score)
	assert.Equal(t, 2, auditions)
	assert.Contains(t, output.String(), "replaying", "missing replay confirmation")
}

func TestJudgeMutationIgnoresInvalidKeysWithoutReplay(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, nil, newJudgeInput(strings.NewReader("x 4")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 4, score)
	assert.Equal(t, 1, auditions)
}

func TestJudgeMutationPropagatesAuditionError(t *testing.T) {
	auditionErr := errors.New("audition failed")
	_, err := judgeMutation(context.Background(), func() error {
		return auditionErr
	}, nil, newJudgeInput(strings.NewReader("1")), io.Discard)
	assert.ErrorIs(t, err, auditionErr)
}

func TestJudgeMutationReturnsInputEOF(t *testing.T) {
	_, err := judgeMutation(context.Background(), func() error {
		return nil
	}, nil, newJudgeInput(strings.NewReader("")), io.Discard)
	assert.ErrorIs(t, err, io.EOF)
}

func TestJudgeMutationStopsWaitingForInputOnCancellation(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	defer inputReader.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := judgeMutation(ctx, func() error {
			return nil
		}, nil, newJudgeInput(inputReader), io.Discard)
		result <- err
	}()
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled, "judge did not stop waiting for input")
	case <-time.After(time.Second):
		require.Fail(t, "judge did not stop waiting for input")
	}
	inputWriter.Close()
}

func newJudgeTestMutation(t *testing.T, settings evolutionSettings) *Mutation {
	t.Helper()
	return newTestMutation(newTestPatch(t, "Sound Controller"), settings, 3)
}

func TestJudgeMutationAdjustsRoundSize(t *testing.T) {
	mutation := newJudgeTestMutation(t, defaultEvolutionSettings())
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error { return nil }, mutation, newJudgeInput(strings.NewReader("] 8")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 8, score)
	assert.Equal(t, defaultRoundSize+1, mutation.settings.roundSize, "round size was not adjusted")
	assert.Contains(t, output.String(), "round size 5", "missing round size report")
}

func TestJudgeMutationAdjustsMutatedGenes(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.mutatedGenes = 1
	mutation := newJudgeTestMutation(t, settings)
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error { return nil }, mutation, newJudgeInput(strings.NewReader("- 6")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 6, score)
	assert.Zero(t, mutation.settings.mutatedGenes, "mutated genes was not adjusted")
	assert.Contains(t, output.String(), "mutated genes automatic", "missing automatic genes report")
}

func TestJudgeMutationAdjustsMutationRate(t *testing.T) {
	mutation := newJudgeTestMutation(t, defaultEvolutionSettings())
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error { return nil }, mutation, newJudgeInput(strings.NewReader("< 5")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 5, score)
	assert.Equal(t, 0.9, mutation.settings.mutationRate, "mutation rate was not adjusted")
	assert.Contains(t, output.String(), "mutation rate 0.9", "missing mutation rate report")
}

func TestJudgeMutationReportsTuningBounds(t *testing.T) {
	settings := defaultEvolutionSettings()
	settings.roundSize = minimumRoundSize
	settings.mutatedGenes = 0
	settings.mutationRate = 0
	mutation := newJudgeTestMutation(t, settings)
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error { return nil }, mutation, newJudgeInput(strings.NewReader("[ - < 3")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 3, score)
	assert.Contains(t, output.String(), "round size is already 3", "missing round size bound report")
	assert.Contains(t, output.String(), "mutated genes is already automatic", "missing genes bound report")
	assert.Contains(t, output.String(), "mutation rate is already 0.0", "missing rate bound report")
}

func TestJudgeMutationIgnoresTuningKeysWithoutControls(t *testing.T) {
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error { return nil }, nil, newJudgeInput(strings.NewReader("] 4")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 4, score)
	assert.Contains(t, output.String(), `not "]"`, "tuning key was not reported as invalid")
}
