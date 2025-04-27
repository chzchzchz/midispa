//go:build !bpf
// +build !bpf

package main

import (
	"io"

	"github.com/chzchzchz/midispa/midi"
)

const defaultPolicyPath = ""

func initPolicy(p string, w io.Writer) {
	if defaultPolicyPath != p {
		panic("non-default bpf policy")
	}
}

func handlePolicy(msg []byte) bool {
	return midi.IsClock(msg[0])
}
