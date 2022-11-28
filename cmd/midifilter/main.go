package main

import (
	"flag"
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

// Filters midi clocks

func main() {
	cnFlag := flag.String("name", "midifilter", "midi client name")
	flag.Parse()
	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	outc := make(chan alsa.SeqEvent, 16)
	go func() {
		defer close(outc)
		for ev := range outc {
			// log.Printf("event: %+v", ev)
			ev.SeqAddr = alsa.SubsSeqAddr
			if err := aseq.Write(ev); err != nil {
				log.Printf("write failed: %v", err)
				panic(err)
			}
		}
	}()
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		cmd := ev.Data[0]
		switch {
		case cmd < 0x80: // internal message
		case cmd >= 0xF8 && cmd <= 0xFC: // clock..stop
		default:
			outc <- ev
		}
	}
}
