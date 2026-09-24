package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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

// Keep the signal in the cancellation cause so the main flow can clean up before returning its conventional exit status.
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
	return runMutation(ctx, config, os.Stdin, os.Stdout)
}

func failCommand(err error) {
	var termination terminationError
	if errors.As(err, &termination) {
		fmt.Fprintln(os.Stderr, "mutation:", termination)
		os.Exit(termination.exitCode())
	}
	fmt.Fprintln(os.Stderr, "mutation:", err)
	os.Exit(1)
}
