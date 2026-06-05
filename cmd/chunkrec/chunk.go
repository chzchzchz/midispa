package main

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/chzchzchz/midispa/wav"
)

type Chunk struct {
	start            time.Time
	prev             Window
	windows          []Window
	winSamples       int
	minWindowsToSave int
	rate             int
	saveDir          string
	savec            chan Window
}

func NewChunk(winSamples, minWindowsToSave, rate int, saveDir string) *Chunk {
	return &Chunk{
		winSamples:       winSamples,
		minWindowsToSave: minWindowsToSave,
		rate:             rate,
		saveDir:          saveDir,
		savec:            make(chan Window, 4),
	}
}

func (c *Chunk) save(windows []Window) {
	fstr := time.DateOnly + "_" + time.TimeOnly
	fname := c.start.Format(fstr) + ".wav"
	path := filepath.Join(c.saveDir, fname)
	log.Println("saving", path)

	// Normalize
	wmin, wmax := float32(1e10), float32(-1e10)
	for _, w := range windows {
		for _, s := range w {
			if s < wmin {
				wmin = s
			}
			if s > wmax {
				wmax = s
			}
		}
	}
	d := wmax - wmin
	for _, w := range windows {
		for i := range w {
			w[i] = 2.0*((w[i]-wmin)/d) - 1.0
		}
	}

	wf, err := wav.OpenWriter(path, c.rate)
	if err != nil {
		panic(err)
	}

	// Write out
	for _, w := range windows {
		if err := wf(w); err != nil {
			panic(err)
		}
	}
	if err := wf(nil); err != nil {
		panic(err)
	}
	log.Println("saved", path)
}

func (c *Chunk) saver() {
	var wg sync.WaitGroup
	for w := range c.savec {
		// len(w) == 0 for fast saving
		if !w.silent() {
			c.windows = append(c.windows, w)
			if len(c.windows) == 1 {
				log.Println("begin recording")
				c.start = time.Now()
			}
			continue
		}
		if len(c.windows) < c.minWindowsToSave {
			// Not enough windows to save.
			c.windows, c.prev = nil, w
			continue
		}

		// Append silence at beginning and end.
		windows := c.windows
		windows = append(windows, w)
		if c.prev != nil {
			windows = append([]Window{c.prev}, windows...)
		}
		c.windows, c.prev = nil, nil

		// Save it off to the side.
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.save(windows)
		}()
	}
	wg.Wait()
}
