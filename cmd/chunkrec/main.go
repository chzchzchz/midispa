package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"unsafe"

	j "github.com/xthexder/go-jack"

	//"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
)

const SampleRate = 48000
const WindowSamples = 1024

func makeCallback(c *Chunk) func([]j.AudioSample) int {
	now := NewWindow(c.winSamples)
	return func(s []j.AudioSample) int {
		dat := *(*[]float32)(unsafe.Pointer(&s))
		if len(dat)+len(now) > cap(now) {
			fmt.Println(len(dat), len(now), cap(now))
			panic("oops size")
		}
		now = append(now, dat...)
		if now.full() {
			c.savec <- now
			now = NewWindow(c.winSamples)
		}
		return 0
	}
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
	pcIn := jack.PortConfig{
		ClientName:    *cnFlag + "-record",
		PortName:      "in",
		MatchName:     strings.Split(*portsFlag, ","),
		AudioCallback: makeCallback(chunk),
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
