//go:build bpf
// +build bpf

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/chzchzchz/midispa/bpf"
)

const defaultPolicyPath = "examples/clock.elf"

type bpfPolicy struct {
	b    *bpf.BPF
	path string
	wr   io.Writer

	newb atomic.Pointer[bpf.BPF]
}

func initPolicy(p string, w io.Writer) *bpfPolicy {
	b := bpf.NewBPF(p, w)
	if b == nil {
		panic("could not load bpf file " + p)
	}
	bp := &bpfPolicy{b: b, path: p, wr: w}
	go bp.refresh()
	return bp
}

func (bp *bpfPolicy) handle(msg []byte) bool {
	if newb := bp.newb.Load(); newb != nil {
		bp.b = newb
		bp.newb.Store(nil)
	}
	ret := bp.b.Run(msg)
	return ret == bpf.DROP
}

func (bp *bpfPolicy) refresh() {
	w, err := fsnotify.NewWatcher()
	must(err)
	defer w.Close()
	w.Add(filepath.Dir(bp.path))
	var lastModTime time.Time
	for {
		var ev fsnotify.Event
		select {
		case ev = <-w.Events:
		case err := <-w.Errors:
			must(err)
			return
		}
		if ev.Has(fsnotify.Remove) {
			continue
		}
		s, err := os.Stat(bp.path)
		if err != nil {
			continue
		}
		newModTime := s.ModTime()
		if lastModTime == newModTime {
			continue
		}
		b := bpf.NewBPF(bp.path, bp.wr)
		if b == nil {
			log.Printf("could not load bpf file %q", bp.path)
			continue
		}
		log.Printf("updated bpf file %q", bp.path)
		lastModTime = newModTime
		bp.newb.Store(b)
	}

}
