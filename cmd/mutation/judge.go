package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// Token-based input matches normal terminal line editing and also works with
// whitespace-separated scores from a pipe.
type judgeInput struct {
	reader *bufio.Reader
	source io.Reader
	// interactive reports whether a judge types at the source
	// in real time, which is what lets an answer stop the
	// melody that is currently playing.
	interactive bool
}

func newJudgeInput(source io.Reader) *judgeInput {
	return &judgeInput{reader: bufio.NewReader(source), source: source, interactive: isInteractive(source)}
}

// isInteractive reports whether the source is a terminal a
// judge types at. A piped score is scripted rather than a
// reaction to the melody, so only a terminal can interrupt
// an audition.
func isInteractive(source io.Reader) bool {
	file, ok := source.(*os.File)
	return ok && isatty.IsTerminal(file.Fd())
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
	focusMutation(pattern string) ([]string, error)
}

// judgePrompt advertises every key the evaluation loop accepts.
const judgePrompt = "rank 0-9, r to replay, [ ] round size, - + genes, < > rate, pattern: "

func judgeMutation(ctx context.Context, audition func(ctx context.Context) error, controls evolutionControls, input *judgeInput, output io.Writer) (int, error) {
	if audition == nil {
		return 0, fmt.Errorf("audition function is nil")
	}
	for {
		answer, err := auditionAndRead(ctx, audition, input, output)
		if err != nil {
			return 0, err
		}
		for {
			handled, err := applyJudgeCommand(answer, controls, output)
			if err != nil {
				return 0, err
			}
			if !handled {
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
			if _, err := fmt.Fprint(output, judgePrompt); err != nil {
				return 0, err
			}
			if answer, err = readJudgeAnswer(ctx, input); err != nil {
				return 0, err
			}
		}
	replay:
	}
}

// auditionAndRead auditions the candidate while reading the
// judge's first answer, so a value entered during the melody
// stops it and still counts as the answer. The prompt is
// printed once the audition has settled, so an interrupted
// audition and a completed one lead the same transcript.
func auditionAndRead(ctx context.Context, audition func(ctx context.Context) error, input *judgeInput, output io.Writer) (string, error) {
	if !input.interactive {
		// A scripted score is not a judge reacting to the
		// melody, so the audition plays out in full.
		if err := audition(ctx); err != nil {
			return "", err
		}
		if _, err := fmt.Fprint(output, judgePrompt); err != nil {
			return "", err
		}
		return readJudgeAnswer(ctx, input)
	}
	// The audition runs under its own context, which the
	// entered value cancels; the session's context stays
	// alive for the rest of the round.
	auditionCtx, stopAudition := context.WithCancel(ctx)
	defer stopAudition()
	auditionDone := make(chan error, 1)
	answerDone := make(chan judgeAnswerResult, 1)
	go func() { auditionDone <- audition(auditionCtx) }()
	go func() {
		answer, err := readJudgeAnswer(ctx, input)
		answerDone <- judgeAnswerResult{answer: answer, err: err}
	}()
	select {
	case err := <-auditionDone:
		if err != nil {
			return "", err
		}
		if _, err := fmt.Fprint(output, judgePrompt); err != nil {
			return "", err
		}
		result := <-answerDone
		return result.answer, result.err
	case result := <-answerDone:
		// The entered value is the interruption, so a
		// canceled audition is expected rather than a fault.
		stopAudition()
		if err := <-auditionDone; err != nil && !errors.Is(err, context.Canceled) {
			return "", err
		}
		if _, err := fmt.Fprint(output, judgePrompt); err != nil {
			return "", err
		}
		return result.answer, result.err
	}
}

// applyJudgeCommand handles every answer that tunes the search
// rather than ranking a candidate: the six tuning keys, and any
// longer answer as a gene name pattern. It reports whether the
// answer was one of them; a key at a setting's bound or a
// pattern that matches nothing is printed as feedback so the
// session keeps waiting for a rank.
func applyJudgeCommand(answer string, controls evolutionControls, output io.Writer) (bool, error) {
	// A judge without controls, which the tests exercise, treats
	// every tuning key as invalid input rather than panicking.
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
		// A one character answer is a typo rather than a
		// pattern, so it keeps the invalid-input message.
		if len(answer) < 2 {
			return false, nil
		}
		var matched []string
		if matched, err = controls.focusMutation(answer); err == nil {
			message = fmt.Sprintf("gene pattern %q: %s", answer, strings.Join(matched, ", "))
		}
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
