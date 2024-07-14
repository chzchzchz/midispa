package main

import (
	"fmt"
	"os"
	"os/exec"
)

func abc2midi(in, out string) error {
	sInput, err := os.Stat(in)
	if err != nil {
		return err
	}
	sOutput, errOut := os.Stat(out)
	if errOut == nil && sInput.ModTime().Before(sOutput.ModTime()) {
		return nil
	}
	cmd := exec.Command("abc2midi", in, "-o", out)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	sOutput, err = os.Stat(out)
	if err != nil {
		return err
	}
	if sOutput.ModTime().Before(sInput.ModTime()) {
		return fmt.Errorf("%s not updated", out)
	}
	return nil
}
