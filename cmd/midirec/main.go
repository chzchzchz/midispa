package main

import (
	"flag"
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	inFlag := flag.String("i", "", "midi port for recording")
	mFlag := flag.String("m", "", "midi port for clock master")
	outdirFlag := flag.String("o", "", "output directory")
	// todo midi output port for playback

	flag.Parse()

	if *outdirFlag == "" {
		panic("expects -o outdir")
	}

	aseq, err := alsa.OpenSeq("midirec")
	must(err)
	defer aseq.Close()

	log.Printf("opening midi input %q", *inFlag)
	sa, err := aseq.PortAddress(*inFlag)
	must(err)
	must(aseq.OpenPortRead(sa))

	if *mFlag != "" {
		log.Printf("opening midi master %q", *mFlag)
		sa, err = aseq.PortAddress(*mFlag)
		must(err)
		must(aseq.OpenPortRead(sa))
	}

	s := NewSequencer(aseq, *outdirFlag)
	must(s.processEvents())
}
