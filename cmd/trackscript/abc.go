package main

import (
	"os"
	"os/exec"
)

func abc2midi(in, out string) error {
	// TODO: check dates / existence to avoid regen
	cmd := exec.Command("abc2midi", in, "-o", out)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	return err
}
