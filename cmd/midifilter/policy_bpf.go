//go:build bpf
// +build bpf

package main

import (
	"io"

	"github.com/chzchzchz/midispa/bpf"
)

const defaultPolicyPath = "examples/clock.elf"

var bpfPolicy *bpf.BPF

func initPolicy(p string, w io.Writer) {
	bpfPolicy = bpf.NewBPF(p, w)
	if bpfPolicy == nil {
		panic("could not load bpf file " + p)
	}
}

func handlePolicy(msg []byte) bool {
	ret := bpfPolicy.Run(msg)
	return ret == bpf.DROP
}
