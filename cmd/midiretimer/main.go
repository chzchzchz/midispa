package main

import (
	"flag"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// pulse per quarter note
const PPQN = 24.0
const OutputBufferSize = 16
const InputBufferSize = 64

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func makeInputChannel(aseq *alsa.Seq, input io.Reader) chan []byte {
	inc := make(chan []byte, InputBufferSize)
	go func() {
		rbuf := make([]byte, 512)
		framebuf := make([]byte, 0, 512)
		for {
			n, err := input.Read(rbuf)
			if err != nil {
				return
			}
			framebuf = append(framebuf, rbuf[:n]...)
			msgs, nbytes := midi.Frame(framebuf)
			framebuf = framebuf[nbytes:]
			for _, msg := range msgs {
				inc <- msg
			}
		}
	}()
	go func() {
		for {
			ev, err := aseq.Read()
			must(err)
			if !ev.IsControl() {
				inc <- ev.Data
			}
		}
	}()
	return inc
}

func main() {
	cnFlag := "retimer"
	flag.StringVar(&cnFlag, "name", "retimer", "midi client name")
	flag.StringVar(&cnFlag, "n", "retimer", "midi client name")

	bpmFlag := flag.Float64("bpm", 120.0, "retime clock signals at given bpm")
	outFlag := flag.String("out-port", "?", "output midi port name")
	replyClocksFlag := flag.Bool("reply-clocks", false, "reply to clocks")

	flag.Parse()

	aseq, err := alsa.OpenSeq(cnFlag)
	must(err)

	sa, err := aseq.PortAddress(*outFlag)
	must(err)
	must(aseq.OpenPortWrite(sa))

	curBpmInt := int(*bpmFlag * 64.0)
	clockDur := int64(0)
	updateClockDur := func() {
		if curBpmInt < 64 {
			return
		}
		cps := ((float64(curBpmInt) / 64.0) / 60.0) * PPQN
		dur := time.Duration(float64(time.Second) / cps)
		atomic.StoreInt64(&clockDur, int64(dur))
		log.Printf("bpm = %v\n", float64(curBpmInt)/64.0)
	}
	updateClockDur()
	inc := makeInputChannel(aseq, os.Stdin)

	clockMsg := []byte{midi.Clock}

	// Write output midi messages to stdout
	outc := make(chan []byte, OutputBufferSize)
	if *replyClocksFlag {
		go func() {
			for msgs := range outc {
				// NB: unbuffered
				os.Stdout.Write(msgs)
			}
		}()
	}

	// Read in midi messages from stdin, wait and forward.
	var wg sync.WaitGroup
	wg.Add(2)
	contc := make(chan struct{}, 1)
	go func() {
		defer wg.Done()
		for msg := range inc {
			cmd := msg[0]
			if cmd == midi.Clock {
				if len(inc) < InputBufferSize/2 {
					<-contc
				} else {
					log.Println("skipping clock")
				}
				if *replyClocksFlag {
					outc <- clockMsg
				}
			} else {
				ev := alsa.SeqEvent{alsa.SubsSeqAddr, msg}
				aseq.Write(ev)
			}
		}
	}()
	// Retime clocks.
	go func() {
		defer wg.Done()
		nextClock := time.Now()
		for {
			contc <- struct{}{}
			var nextDur time.Duration
			for {
				nextDur = time.Duration(atomic.LoadInt64(&clockDur))
				if nextDur != 0 {
					break
				}
				nextClock = time.Now()
			}
			nextClock = nextClock.Add(nextDur)
			time.Sleep(time.Until(nextClock))
		}
	}()
	wg.Wait()
}
