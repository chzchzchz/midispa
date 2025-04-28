package main

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/track"
	"github.com/fsnotify/fsnotify"
)

type cachePattern struct {
	pattern *track.Pattern
	time    time.Time
}

type Bank struct {
	dir   string
	cache map[int]*cachePattern
	mu    sync.Mutex
	errc  chan error
}

func NewBank(dir string) *Bank {
	b := &Bank{
		dir:   dir,
		cache: make(map[int]*cachePattern),
		errc:  make(chan error, 1),
	}
	go func() { b.errc <- b.watch() }()
	return b
}

func (b *Bank) Reader(patNum int) Reader {
	// This only blocks watch(), so safe to hold while doing io.
	// Holding the lock for the file load means there won't be any toctou
	// issues that might lead to a pattern remaining resident after its
	// file is removed.
	b.mu.Lock()
	pat, ok := b.cache[patNum]
	if !ok {
		path := filepath.Join(b.dir, fmt.Sprintf("%d.mid", patNum))
		if p, err := track.NewPattern(path); err == nil {
			pat = &cachePattern{p, time.Now()}
			log.Printf("loaded pattern %d with %d ticks", patNum, p.LastTick)
		}
		b.cache[patNum] = pat
	}
	b.mu.Unlock()
	if pat == nil || pat.pattern == nil {
		// Default to 4 beat empty pattern.
		return NewEmptyReader(PPQN * BeatsPerMeasure)
	}
	var r Reader
	r = NewPatternReader(pat.pattern)
	if lastTick := pat.pattern.LastTick; lastTick%PPQN != 0 {
		// Round partially weighted toward next beat.
		r = NewExactTickReader(r, int(PPQN*((lastTick+3*PPQN/4)/PPQN)))
	}
	return r
}

func (b *Bank) watch() error {
	w, err := fsnotify.NewWatcher()
	must(err)
	defer func() {
		w.Close()
		close(b.errc)
	}()
	w.Add(b.dir)
	for {
		var ev fsnotify.Event
		select {
		case ev = <-w.Events:
		case err := <-w.Errors:
			return err
		}
		base := filepath.Base(ev.Name)
		var patNum int
		fmt.Sscanf(base, "%d.mid", &patNum)
		b.mu.Lock()
		if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Create) {
			// Expire cache.
			delete(b.cache, patNum)
		}
		b.mu.Unlock()
	}
}
