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
const CcSwing = 17

func must(err error) {
	if err != nil {
		panic(err)
	}
}

const (
	minSwingPct = 0.1
	maxSwingPct = 99.9
)

// validSwingPct reports whether v is in the hard-validated swing range.
func validSwingPct(v float64) bool {
	return v >= minSwingPct && v <= maxSwingPct
}

func main() {
	cnFlag := "midiclock"
	flag.StringVar(&cnFlag, "name", "midiclock", "midi client name")
	flag.StringVar(&cnFlag, "n", "midiclock", "midi client name")

	inFlag := flag.String("i", "", "input midi port")

	bpmFlag := flag.Float64("bpm", 120.0, "send clock signals at given bpm")
	swingPctFlag := flag.Float64("swingpct", 50.0, "deterministic swing ratio (50 = straight, 66 = swing)")
	randPctFlag := flag.Float64("randpct", 0.0, "percentage to swing the clock")

	runFlag := false
	flag.BoolVar(&runFlag, "run", false, "run at start")
	flag.BoolVar(&runFlag, "r", false, "run at start")

	flag.Parse()

	if !validSwingPct(*swingPctFlag) {
		log.Fatalf("swingpct must be in [%.1f, %.1f], got %v", minSwingPct, maxSwingPct, *swingPctFlag)
	}

	if !runFlag {
		log.Printf("%q started in stopped mode\n", cnFlag)
	}

	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(cnFlag)
	must(err)
	if *inFlag != "" {
		log.Printf("%q opened input %s", cnFlag, *inFlag)
		sa, err := aseq.PortAddress(*inFlag)
		must(err)
		must(aseq.OpenPortRead(sa))
	}

	s := NewClockSequencer(aseq, bpmFlag, *swingPctFlag, *randPctFlag)
	if runFlag {
		s.UpdateClock()
	}
	s.wg.Wait()
}
