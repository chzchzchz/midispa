package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/chzchzchz/midispa/bpf"
)

type Filter struct {
	f    *bpf.BPF
	path string
	args []string
	mw   msgWriter
}

type msgWriter struct {
	msgs [][]byte
}

func (m *msgWriter) Write(dat []byte) (int, error) {
	dat2 := make([]byte, len(dat))
	copy(dat2, dat)
	m.msgs = append(m.msgs, dat2)
	return len(dat), nil
}

func (f *Filter) compile() {
	if f.f != nil {
		return
	}

	sInput, err := os.Stat(f.path)
	if err != nil {
		panic("could not stat " + f.path + ": " + err.Error())
	}

	elf := f.path + "_" + strings.Join(f.args, "_") + ".elf"
	sElf, err := os.Stat(elf)
	needCompile := err != nil || sInput.ModTime().After(sElf.ModTime())
	if needCompile {
		// Outdated / missing elf file
		args := []string{"-O2", "--target=bpf", "-c", "-o", elf, f.path}
		args = append(args, f.args...)
		cmd := exec.Command("clang", args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err = cmd.Run(); err != nil {
			panic("clang failed on " + strings.Join(args, " ") + ": " + err.Error())
		}
	}
	if f.f = bpf.NewBPF(elf, &f.mw); f.f == nil {
		panic("could not load BPF for " + elf)
	}
}

func (f *Filter) Apply(dat []byte) [][]byte {
	f.compile()
	f.mw.msgs = nil
	ret := f.f.Run(dat)
	if ret == bpf.DROP {
		return f.mw.msgs
	}
	return append([][]byte{dat}, f.mw.msgs...)
}
