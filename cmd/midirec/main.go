package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	inFlag := flag.String("i", "", "midi port for recording")
	ssFlag := flag.Bool("s", false, "single shot mode (record to file)")
	mFlag := flag.String("m", "", "midi port for clock master")
	qFlag := flag.Bool("q", false, "quantize to match measure start")
	outdirFlag := flag.String("o", "", "output directory or file")
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

	var s *Sequencer
	if *ssFlag {
		sigc := make(chan os.Signal, 1)
		defer close(sigc)
		signal.Notify(sigc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		go func() {
			if _, ok := <-sigc; !ok {
				return
			}
			ev := alsa.SeqEvent{aseq.SeqAddr, []byte{midi.Stop}}
			aseq.Write(ev)
		}()
		s = NewSingleShotSequencer(aseq, *outdirFlag)
	} else {
		s = NewSequencer(aseq, *outdirFlag)
	}
	s.matchMeasure = *qFlag
	must(s.processEvents())
}
