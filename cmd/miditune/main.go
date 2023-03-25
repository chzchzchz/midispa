package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
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
			cc := midi.MakeCC(*chanFlag + i - 1)
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
		cmd := midi.Message(ev.Data[0])

		// TODO: support bulk tuning sysex
		// TODO: patches should match; broadcast bank select, program select
		switch cmd {
		case midi.NoteOn:
			ch := getCh()
			note2Ch[int(ev.Data[1])] = ch
			if ch < 1 {
				// eat note
				continue
			}
			cc := midi.MakeCC(ch - 1)
			// select coarse tuning
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 100, 2}}))
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 101, 0}}))
			// change cents; 0x40
			v := rand.Intn(50) - 40
			must(aseq.Write(alsa.SeqEvent{ev.SeqAddr, []byte{cc, 6, byte(0x40 + v)}}))
			ev.Data[0] = midi.MakeNoteOn(ch - 1)
		case midi.NoteOff:
			n := int(ev.Data[1])
			ch := note2Ch[n]
			delete(note2Ch, n)
			if ch < 1 {
				continue
			}
			freeCh(ch)
			ev.Data[0] = midi.MakeNoteOff(ch - 1)
		case midi.CC:
			for i := 0; i < *maxChansFlag; i++ {
				ev.Data[0] = midi.MakeCC(i + *chanFlag - 1)
				must(aseq.Write(ev))
			}
			continue
		}
		must(aseq.Write(ev))
	}
}
