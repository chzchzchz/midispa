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

// evolutionControls lets the judge tune the search while the engine
// keeps every setting inside its validated bounds.
type evolutionControls interface {
	adjustRoundSize(delta int) (int, error)
	adjustMutatedGenes(delta int) (int, error)
	adjustMutationRate(delta int) (float64, error)
}

// judgePrompt advertises every key the evaluation loop accepts.
const judgePrompt = "rank 0-9, r to replay, [ ] round size, - + genes, < > rate: "

func judgeMutation(ctx context.Context, audition func() error, controls evolutionControls, input *judgeInput, output io.Writer) (int, error) {
	if audition == nil {
		return 0, fmt.Errorf("audition function is nil")
	}
	for {
		if err := audition(); err != nil {
			return 0, err
		}
		for {
			if _, err := fmt.Fprint(output, judgePrompt); err != nil {
				return 0, err
			}
			answer, err := readJudgeAnswer(ctx, input)
			if err != nil {
				return 0, err
			}
			handled, err := applyEvolutionCommand(answer, controls, output)
			if err != nil {
				return 0, err
			}
			if handled {
				continue
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

// applyEvolutionCommand handles the keys that tune the search rather
// than rank a candidate. It reports whether the answer was one of
// them; a key at a setting's bound is printed as feedback so the
// session keeps waiting for a rank.
func applyEvolutionCommand(answer string, controls evolutionControls, output io.Writer) (bool, error) {
	if controls == nil {
		return false, nil
	}
	var message string
	var err error
	switch answer {
	case "[":
		var size int
		if size, err = controls.adjustRoundSize(-1); err == nil {
			message = fmt.Sprintf("round size %d", size)
		}
	case "]":
		var size int
		if size, err = controls.adjustRoundSize(1); err == nil {
			message = fmt.Sprintf("round size %d", size)
		}
	case "-":
		var count int
		if count, err = controls.adjustMutatedGenes(-1); err == nil {
			message = mutatedGenesMessage(count)
		}
	case "+":
		var count int
		if count, err = controls.adjustMutatedGenes(1); err == nil {
			message = mutatedGenesMessage(count)
		}
	case "<":
		var rate float64
		if rate, err = controls.adjustMutationRate(-1); err == nil {
			message = fmt.Sprintf("mutation rate %.1f", rate)
		}
	case ">":
		var rate float64
		if rate, err = controls.adjustMutationRate(1); err == nil {
			message = fmt.Sprintf("mutation rate %.1f", rate)
		}
	default:
		return false, nil
	}
	if err != nil {
		message = err.Error()
	}
	if _, printErr := fmt.Fprintln(output, message); printErr != nil {
		return true, printErr
	}
	return true, nil
}

// mutatedGenesMessage names the automatic range when the count is
// unset, which is how the flag default reads back to the judge.
func mutatedGenesMessage(count int) string {
	if count == 0 {
		return "mutated genes automatic"
	}
	return fmt.Sprintf("mutated genes %d", count)
}
