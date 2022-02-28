package main

import (
	"flag"

	"github.com/chzchzchz/midispa/alsa"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	inFlag := flag.String("i", "", "midi port for recording")
	outdirFlag := flag.String("o", "", "output directory")
	// todo midi output port for playback

	flag.Parse()

	if *outdirFlag == "" {
		panic("expects -o outdir")
	}

	aseq, err := alsa.OpenSeq("midirec")
	must(err)
	defer aseq.Close()

	sa, err := aseq.PortAddress(*inFlag)
	must(err)
	must(aseq.OpenPortRead(sa))
	s := NewSequencer(aseq, *outdirFlag)
	must(s.processEvents())
}
