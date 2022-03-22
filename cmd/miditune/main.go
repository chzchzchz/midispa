package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"

	"github.com/chzchzchz/midispa/alsa"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	nameFlag := flag.String("n", "miditune", "port name")
	chanFlag := flag.Int("c", 1, "start channel")
	maxChansFlag := flag.Int("m", 4, "number of channels to use")

	flag.Parse()
	aseq, err := alsa.OpenSeq(*nameFlag)
	must(err)
	defer aseq.Close()
	log.Printf("listening on %q", *nameFlag)

	note2Ch := make(map[int]int)
	usedCh := make([]bool, *maxChansFlag)
	getCh := func() int {
		for i, used := range usedCh {
			if !used {
				usedCh[i] = true
				return *chanFlag + i
			}
		}
		return 0
	}
	freeCh := func(ch int) {
		idx := ch - *chanFlag
		if usedCh[idx] == false {
			panic("freeing unused channel")
		}
		usedCh[idx] = false
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		<-sigc
		log.Println("reseting tuning and exiting")
		for i := 0; i < *maxChansFlag; i++ {
			cc := byte(0xb0 | (*chanFlag + i - 1))
			must(aseq.Write(alsa.SeqEvent{alsa.SubsSeqAddr, []byte{cc, 100, 2}}))
			must(aseq.Write(alsa.SeqEvent{alsa.SubsSeqAddr, []byte{cc, 101, 0}}))
			must(aseq.Write(alsa.SeqEvent{alsa.SubsSeqAddr, []byte{cc, 6, 0x40}}))
		}
		os.Exit(0)
	}()

	for {
		ev, err := aseq.Read()
		must(err)
		if ev.Data[0]&0x80 == 0 {
			log.Printf("skipping data %v", ev)
			continue
		}
		ev.SeqAddr = alsa.SubsSeqAddr
		cmd := ev.Data[0] & 0xf0

		// TODO: support bulk tuning sysex
		// TODO: patches should match; broadcast bank select, program select
		switch cmd {
		case 0x90: // note on
			ch := getCh()
			note2Ch[int(ev.Data[1])] = ch
			if ch < 1 {
				// eat note
				continue
			}
			cc := byte(0xb0 | (ch - 1))
			// select coarse tuning
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 100, 2}}))
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 101, 0}}))
			// change cents; 0x40
			v := rand.Intn(50) - 40
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 6, byte(0x40 + v)}}))
			ev.Data[0] = byte(0x90 | (ch - 1))
		case 0x80: // note off
			n := int(ev.Data[1])
			ch := note2Ch[n]
			delete(note2Ch, n)
			if ch < 1 {
				continue
			}
			freeCh(ch)
			ev.Data[0] = byte(0x80 | (ch - 1))
		case 0xb0:
			for i := 0; i < *maxChansFlag; i++ {
				ev.Data[0] = byte(0xb0 | (i + *chanFlag - 1))
				must(aseq.Write(ev))
			}
			continue
		}
		must(aseq.Write(ev))
	}
}
