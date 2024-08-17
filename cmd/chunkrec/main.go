package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	j "github.com/xthexder/go-jack"

	//"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
	"github.com/chzchzchz/midispa/wav"
)

const SampleRate = 48000
const WindowSamples = 1024

var SilenceCutoff = float32(0.001)

type Chunk struct {
	start            time.Time
	prev             Window
	now              Window
	windows          []Window
	winSamples       int
	minWindowsToSave int
	rate             int
	saveDir          string
	savec            chan Window
}

type Window []float32

func NewWindow(samples int) Window {
	return make([]float32, 0, samples)
}

func (w Window) full() bool { return len(w) == cap(w) }

func (w Window) silent() bool {
	sum := float32(0.0)
	for _, v := range w {
		if v < 0 {
			v = -v
		}
		sum = sum + v
	}
	avg := sum / float32(len(w))
	return avg < SilenceCutoff
}

func (w Window) reset() {
}

func NewChunk(winSamples, minWindowsToSave, rate int, saveDir string) *Chunk {
	return &Chunk{
		now:              NewWindow(winSamples),
		winSamples:       winSamples,
		minWindowsToSave: minWindowsToSave,
		rate:             rate,
		saveDir:          saveDir,
		savec:            make(chan Window, 4),
	}
}

func (c *Chunk) update(dat []float32) {
	if len(dat)+len(c.now) > cap(c.now) {
		fmt.Println(len(dat), len(c.now), cap(c.now))
		panic("oops size")
	}
	c.now = append(c.now, dat...)
	if c.now.full() {
		c.savec <- c.now
		c.now = NewWindow(c.winSamples)
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

func main() {
	savePathFlag := flag.String("wav-path", "./chunks", "path to data directory")
	cnFlag := flag.String("clientname", "chunkrec", "jack client name")
	portsFlag := flag.String("port", "system:capture_7", "jack source ports for recording")
	silenceCutoffFlag := flag.Float64("silence-cutoff", 0.001, "silence cutoff value")
	windowSamplesFlag := flag.Int("window-samples",
		WindowSamples*(SampleRate/(2*WindowSamples)),
		"number of samples per window")
	minWindowSaveFlag := flag.Int("min-save-windows", 2, "minimum windows to save")

	// NB: Set sink server via JACK_DEFAULT_SERVER
	flag.Parse()

	SilenceCutoff = float32(*silenceCutoffFlag)
	if err := os.MkdirAll(*savePathFlag, 0755); err != nil {
		panic(err)
	}

	chunk := NewChunk(*windowSamplesFlag, *minWindowSaveFlag, SampleRate, *savePathFlag)

	recCallback := func(s []j.AudioSample) int {
		x := *(*[]float32)(unsafe.Pointer(&s))
		chunk.update(x)
		return 0
	}
	pcIn := jack.PortConfig{
		ClientName:    *cnFlag + "-record",
		PortName:      "in",
		MatchName:     strings.Split(*portsFlag, ","),
		AudioCallback: recCallback,
	}
	rp, err := jack.NewReadPort(pcIn)
	if err != nil {
		panic(err)
	}

	defer func() {
		rp.Close()
		close(chunk.savec)
	}()

	chunk.saver()

	// MMC control to start / stop recording / drop / delete playback
	// CC control for normalization
	//s.midiLoop(aseq)
}
