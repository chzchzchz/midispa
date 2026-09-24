package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestJudgeMutationAcceptsRank(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, newJudgeInput(strings.NewReader("7")), &output)
	if err != nil {
		t.Fatalf("judgeMutation: %v", err)
	}
	if score != 7 || auditions != 1 {
		t.Fatalf("score %d after %d auditions, want 7 after 1", score, auditions)
	}
}

func TestJudgeMutationReplaysWholeMutation(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, newJudgeInput(strings.NewReader("r\n8")), &output)
	if err != nil {
		t.Fatalf("judgeMutation: %v", err)
	}
	if score != 8 || auditions != 2 {
		t.Fatalf("score %d after %d auditions, want 8 after 2", score, auditions)
	}
	if !strings.Contains(output.String(), "replaying") {
		t.Fatalf("missing replay confirmation in %q", output.String())
	}
}

func TestJudgeMutationIgnoresInvalidKeysWithoutReplay(t *testing.T) {
	auditions := 0
	var output strings.Builder
	score, err := judgeMutation(context.Background(), func() error {
		auditions++
		return nil
	}, newJudgeInput(strings.NewReader("x 4")), &output)
	if err != nil {
		t.Fatalf("judgeMutation: %v", err)
	}
	if score != 4 || auditions != 1 {
		t.Fatalf("score %d after %d auditions, want 4 after 1", score, auditions)
	}
}

func TestJudgeMutationPropagatesAuditionError(t *testing.T) {
	auditionErr := errors.New("audition failed")
	_, err := judgeMutation(context.Background(), func() error {
		return auditionErr
	}, newJudgeInput(strings.NewReader("1")), io.Discard)
	if !errors.Is(err, auditionErr) {
		t.Fatalf("got error %v, want %v", err, auditionErr)
	}
}

func TestJudgeMutationReturnsInputEOF(t *testing.T) {
	_, err := judgeMutation(context.Background(), func() error {
		return nil
	}, newJudgeInput(strings.NewReader("")), io.Discard)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("got error %v, want EOF", err)
	}
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
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got error %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("judge did not stop waiting for input")
	}
	inputWriter.Close()
}
