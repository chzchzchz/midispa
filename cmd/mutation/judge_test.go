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
	}, newJudgeInput(strings.NewReader("7")), &output)
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
	}, newJudgeInput(strings.NewReader("r\n8")), &output)
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
	}, newJudgeInput(strings.NewReader("x 4")), &output)
	require.NoError(t, err, "judgeMutation")
	assert.Equal(t, 4, score)
	assert.Equal(t, 1, auditions)
}

func TestJudgeMutationPropagatesAuditionError(t *testing.T) {
	auditionErr := errors.New("audition failed")
	_, err := judgeMutation(context.Background(), func() error {
		return auditionErr
	}, newJudgeInput(strings.NewReader("1")), io.Discard)
	assert.ErrorIs(t, err, auditionErr)
}

func TestJudgeMutationReturnsInputEOF(t *testing.T) {
	_, err := judgeMutation(context.Background(), func() error {
		return nil
	}, newJudgeInput(strings.NewReader("")), io.Discard)
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
		}, newJudgeInput(inputReader), io.Discard)
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
