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
	clockPort := flag.String("c", "midiclock", "clock midi port")
	inPort := flag.String("i", "W-BW61 MIDI 1", "input midi port")
	outPort := flag.String("o", "MIDI4x4 MIDI Out 1", "output midi port")
	loopDir := flag.String("dir", "midi", "loop bank directory")
	flag.Parse()

	aseq, err := alsa.OpenSeq("midiloop")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	// Open Ports.
	sa, err := aseq.PortAddress(*clockPort)
	must(err)
	must(aseq.OpenPortRead(sa))

	if *inPort != "" {
		log.Printf("midiloop opened input %s", *inPort)
		sa, err = aseq.PortAddress(*inPort)
		must(err)
		must(aseq.OpenPortRead(sa))
	}

	if *outPort != "" {
		sa, err = aseq.PortAddress(*outPort)
		must(err)
		must(aseq.OpenPortWrite(sa))
	}

	// Turn off notes on termination.
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
		for i := 0; i < 16; i++ {
			msg := []byte{midi.MakeCC(i), midi.AllNotesOff, 0x7f}
			aseq.Write(alsa.MakeEvent(msg))
		}
		os.Exit(1)
	}()

	// Consume midi data.
	b := NewBank(*loopDir)
	s := NewSequencer(aseq, b)
	s.Run()
}
