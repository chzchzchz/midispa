package main

import (
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
	return cmd.Run()
}
