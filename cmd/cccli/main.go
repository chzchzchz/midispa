package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chzchzchz/midispa/alsa"
)

type terminationError struct {
	signal os.Signal
}

func (err terminationError) Error() string {
	return fmt.Sprintf("%s signal received", err.signal)
}

func (err terminationError) exitCode() int {
	if signal, ok := err.signal.(syscall.Signal); ok {
		return 128 + int(signal)
	}
	return 1
}

// Keep the signal in the cancellation cause so the main flow can
// clean up before returning its conventional exit status. Ctrl+c is
// bubbletea's own quit, so the context carries only the signals a
// TUI does not handle itself.
func newTerminationContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		select {
		case received := <-signals:
			cancel(terminationError{signal: received})
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(signals)
		cancel(nil)
	}
}

func main() {
	if err := runCommand(); err != nil {
		failCommand(err)
	}
}

func runCommand() error {
	config, err := parseConfiguration(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, stop := newTerminationContext(context.Background())
	defer stop()
	return runEditor(ctx, config)
}

func failCommand(err error) {
	var termination terminationError
	if errors.As(err, &termination) {
		fmt.Fprintln(os.Stderr, "cccli:", termination)
		os.Exit(termination.exitCode())
	}
	fmt.Fprintln(os.Stderr, "cccli:", err)
	os.Exit(1)
}

// runEditor wires the pieces in the order the flags are validated:
// the model and its rules load, the seed populates it, and the port
// opens before the TUI starts, so a startup failure is a plain
// error rather than a half-open editor.
func runEditor(ctx context.Context, config configuration) error {
	if err := validateConfiguration(config); err != nil {
		return err
	}
	fields, _, ignoredFixed, err := loadModelFields(config.modelName, config.fieldRules)
	if err != nil {
		return err
	}
	if ignoredFixed > 0 {
		// A fixed rule says a field must not change, which an
		// editor has no way to honor; the field stays editable
		// and the rule is dropped, so say what the file asked
		// for and did not get.
		fmt.Fprintf(os.Stderr, "cccli: ignoring %d fixed field rules\n", ignoredFixed)
	}
	if config.inputPath != "" {
		if err := loadSeed(config.inputPath, fields, config.filterMIDIChannel); err != nil {
			return err
		}
	}
	sequence, output, err := openMIDI(config.portName)
	if err != nil {
		return err
	}
	defer sequence.Close()
	if err := openMIDIInput(sequence, config.inputPort); err != nil {
		return err
	}

	model := newCLI(config, fields, output)
	// The termination signals are cccli's to catch: the context
	// above turns them into a quit carrying the signal's exit
	// status. Bubble Tea catches the interrupt and the term
	// signal as well and turns each into a quit message of its
	// own, and its handler races the context's: once the context
	// has quit the program, the message send blocks forever and
	// the program's shutdown hangs. WithoutSignals leaves the
	// signals to cccli.
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithoutSignals())
	// A termination signal has to reach the program from outside
	// Update, which only sees terminal messages, so quitting on
	// ctx.Done is a goroutine rather than a key handler.
	go func() {
		<-ctx.Done()
		program.Quit()
	}()
	// Live input runs beside the program: the reader goroutine
	// forwards every control change as an inputMsg, and the
	// program's Send is safe from any goroutine and stops
	// blocking once the program has exited. The read cannot be
	// interrupted, so the goroutine stays blocked in the
	// sequencer the editor closes on its way out, and the
	// process exit reclaims it, the arrangement fireloop's read
	// makes too.
	go func() {
		readInput(sequence, func(ev alsa.SeqEvent) {
			program.Send(inputMsg{message: ev.Data})
		})
	}()
	_, err = program.Run()
	// A signal quit the program through the context, whose cause
	// names the signal, so the caller turns it into the
	// conventional exit status; a quit from the keyboard leaves
	// the cause unset and the program's own error stands.
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return err
}
