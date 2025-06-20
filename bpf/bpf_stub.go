//go:build !bpf

package bpf

import (
	"io"
)

type BPF struct{}

func NewBPF(p string, w io.Writer) *BPF { panic("ubpf not enabled") }
func (bpf *BPF) Run(dat []byte) int     { panic("ubpf not enabled") }
