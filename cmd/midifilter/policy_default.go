//go:build !bpf
// +build !bpf

package main

import (
	"io"

	"github.com/chzchzchz/midispa/midi"
)

const defaultPolicyPath = ""

type defaultPolicy struct{}

func initPolicy(p string, w io.Writer) *defaultPolicy {
	if defaultPolicyPath != p {
		panic("non-default bpf policy")
	}
	return &defaultPolicy{}
}

func (p *defaultPolicy) handle(msg []byte) bool {
	// Drop clocks.
	return midi.IsClock(msg[0])
}
