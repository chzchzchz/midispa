package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// Token-based input matches normal terminal line editing and also works with
// whitespace-separated scores from a pipe.
type judgeInput struct {
	reader *bufio.Reader
	source io.Reader
}

func newJudgeInput(source io.Reader) *judgeInput {
	return &judgeInput{reader: bufio.NewReader(source), source: source}
}

type judgeAnswerResult struct {
	answer string
	err    error
}

// io.Reader has no cancellation operation, so terminal input waits on a goroutine with a buffered result.
func readJudgeAnswer(ctx context.Context, input *judgeInput) (string, error) {
	result := make(chan judgeAnswerResult, 1)
	stopClose := func() bool { return false }
	if closer, ok := input.source.(io.Closer); ok {
		// Closing files and pipes unblocks the read goroutine when the session is canceled.
		stopClose = context.AfterFunc(ctx, func() { _ = closer.Close() })
	}
	defer stopClose()
	go func() {
		var answer string
		_, err := fmt.Fscan(input.reader, &answer)
		result <- judgeAnswerResult{answer: answer, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", context.Cause(ctx)
	case read := <-result:
		if err := context.Cause(ctx); err != nil {
			return "", err
		}
		return read.answer, read.err
	}
}

func judgeMutation(ctx context.Context, audition func() error, input *judgeInput, output io.Writer) (int, error) {
	if audition == nil {
		return 0, fmt.Errorf("audition function is nil")
	}
	for {
		if err := audition(); err != nil {
			return 0, err
		}
		for {
			if _, err := fmt.Fprint(output, "rank 0-9, or r to replay: "); err != nil {
				return 0, err
			}
			answer, err := readJudgeAnswer(ctx, input)
			if err != nil {
				return 0, err
			}
			switch {
			case len(answer) == 1 && answer[0] >= '0' && answer[0] <= '9':
				score := int(answer[0] - '0')
				if _, err := fmt.Fprintln(output, score); err != nil {
					return 0, err
				}
				return score, nil
			case answer == "r" || answer == "R":
				if _, err := fmt.Fprintln(output, "replaying"); err != nil {
					return 0, err
				}
				goto replay
			default:
				if _, err := fmt.Fprintf(output, "press 0-9 or r, not %q\n", answer); err != nil {
					return 0, err
				}
			}
		}
	replay:
	}
}
