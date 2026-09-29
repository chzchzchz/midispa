package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/chzchzchz/midispa/track"
)

// mutationRunner owns the interactive session while keeping the genetic engine
// independent of terminal, ALSA, and SMF details.
type mutationRunner struct {
	engine     *Mutation
	auditioner auditioner
	store      patchStore
	input      io.Reader
	output     io.Writer
}

func (runner *mutationRunner) run(ctx context.Context) error {
	if runner.engine == nil {
		return fmt.Errorf("mutation engine is nil")
	}
	if runner.auditioner == nil {
		return fmt.Errorf("candidate auditioner is nil")
	}
	if runner.store == nil {
		return fmt.Errorf("patch store is nil")
	}
	judgeInput := newJudgeInput(runner.input)
	for {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		population := runner.engine.nextRound()
		scored := make([]scoredPatch, 0, len(population))
		for index, candidate := range population {
			patch := candidate.patch
			if _, err := fmt.Fprintf(runner.output, "generation %04d, candidate %d/%d\n", runner.engine.generation, index+1, len(population)); err != nil {
				return err
			}
			if err := runner.writeChanges(patch, candidate.reference); err != nil {
				return err
			}
			// A record can be wider than the message it is written as, so a
			// candidate can hold a value with nowhere to go. Dropping it
			// costs one slot in this round; failing would cost the session.
			if err := patch.encodable(); err != nil {
				if _, err := fmt.Fprintf(runner.output, "  skipped: %v\n", err); err != nil {
					return err
				}
				continue
			}
			score, err := judgeMutation(ctx, func() error {
				return runner.auditioner.audition(ctx, patch)
			}, judgeInput, runner.output)
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("judge generation %04d candidate %d: %w", runner.engine.generation, index+1, err)
			}
			scored = append(scored, scoredPatch{patch: patch, score: score})
		}

		if len(scored) == 0 && len(population) > 0 {
			// Every candidate in the round held a value the message could
			// not carry. That is a different fault from an engine that
			// produced nothing, and the error should say which happened
			// rather than leaving the reader hunting in the engine.
			return fmt.Errorf("generation %04d: none of the %d candidates could be written to the instrument", runner.engine.generation, len(population))
		}
		ranked, err := rankPatches(scored)
		if err != nil {
			return err
		}
		best, score, improved := runner.engine.advanceRanked(ranked)
		if err := runner.store.save(best, runner.engine.generation); err != nil {
			return err
		}
		selection := "kept parent"
		if improved {
			selection = "selected candidate"
		}
		if _, err := fmt.Fprintf(runner.output, "%s with rank %d, wrote %s.%0*d and %s\n", selection, score, runner.store.path(), generationDigitsWidth, runner.engine.generation, runner.store.path()); err != nil {
			return err
		}
		nextPopulation, err := runner.engine.breedPopulation(ranked)
		if err != nil {
			return err
		}
		runner.engine.population = nextPopulation
		runner.engine.generation++
	}
}

func (runner *mutationRunner) writeChanges(candidate, reference patch) error {
	if reference == nil {
		reference = runner.engine.parent
	}
	changes, err := geneChanges(candidate, reference)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		_, err := fmt.Fprintln(runner.output, "  changed genes: none (unchanged parent)")
		return err
	}
	for _, change := range changes {
		if _, err := fmt.Fprintf(runner.output, "  %s: %d -> %d (delta %+d)\n", change.name, change.before, change.after, change.delta); err != nil {
			return err
		}
	}
	return nil
}

func runMutation(ctx context.Context, config configuration, input io.Reader, output io.Writer) error {
	return runMutationWithFactory(ctx, config, input, output, openMIDIOutput)
}

// runMutationWithFactory keeps hardware construction injectable so orchestration
// can be exercised without opening ALSA.
func runMutationWithFactory(ctx context.Context, config configuration, input io.Reader, output io.Writer, factory midiOutputFactory) error {
	if err := validateConfiguration(config); err != nil {
		return err
	}
	if factory == nil {
		return fmt.Errorf("MIDI output factory is nil")
	}
	randomSeed := time.Now().UnixNano()
	if config.rngSeed != unsetRNGSeed {
		randomSeed = config.rngSeed
	}
	random := rand.New(rand.NewSource(randomSeed))
	semantics, err := loadGeneSemantics(config.geneSemantics)
	if err != nil {
		return err
	}
	format, err := newPatchFactory(config)
	if err != nil {
		return err
	}
	engine, err := newMutation(format, config.settings, random, semantics, config.seedPath)
	if err != nil {
		return err
	}

	var playback *track.Pattern
	if config.playback != "" {
		playback, err = track.NewPattern(config.playback)
		if err != nil {
			return fmt.Errorf("read playback %q: %w", config.playback, err)
		}
	}

	midiWriter, midiCloser, err := factory(config.portName)
	if err != nil {
		return err
	}
	defer midiCloser.Close()
	player := newMIDIPlayerForChannel(midiWriter, config.midiChannel)
	player.sysexSettle = config.sysexSettle
	runner := mutationRunner{
		engine:     engine,
		auditioner: midiAuditioner{player: player, playback: playback},
		store:      format.store(config.output),
		input:      input,
		output:     output,
	}
	runErr := runner.run(ctx)
	cleanupErr := player.resetChannels(player.auditionChannels(playback))
	return errors.Join(runErr, cleanupErr)
}
