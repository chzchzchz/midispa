package main

import (
	"flag"
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

// pulse per quarter note
const PPQN = 24.0

const CcBpmMsb = 16
const CcBpmLsb = 16 + 32

func main() {
	cnFlag := "midiclock"
	flag.StringVar(&cnFlag, "name", "midiclock", "midi client name")
	flag.StringVar(&cnFlag, "n", "midiclock", "midi client name")

	bpmFlag := flag.Float64("bpm", 120.0, "send clock signals at given bpm")
	randPctFlag := flag.Float64("randpct", 0.0, "percentage to swing the clock")

	runFlag := false
	flag.BoolVar(&runFlag, "run", false, "run at start")
	flag.BoolVar(&runFlag, "r", false, "run at start")

	flag.Parse()

	if !runFlag {
		log.Printf("started %q in stopped mode\n", cnFlag)
	}

	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(cnFlag)
	if err != nil {
		panic(err)
	}

	s := NewClockSequencer(aseq, bpmFlag, *randPctFlag)
	if runFlag {
		s.UpdateClock()
	}
	s.wg.Wait()
}
