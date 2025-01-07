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

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func makeMsgChannel(aseq *alsa.Seq, input io.Reader) chan []byte {
	msgc := make(chan []byte, 64)
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
				msgc <- msg
			}
		}
	}()
	go func() {
		for {
			ev, err := aseq.Read()
			must(err)
			msgc <- ev.Data
		}
	}()
	return msgc
}

func main() {
	cnFlag := "retimer"
	flag.StringVar(&cnFlag, "name", "retimer", "midi client name")
	flag.StringVar(&cnFlag, "n", "retimer", "midi client name")

	bpmFlag := flag.Float64("bpm", 120.0, "retime clock signals at given bpm")
	outFlag := flag.String("out-port", "?", "output midi port name")

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
	msgc := makeMsgChannel(aseq, os.Stdin)

	var wg sync.WaitGroup
	wg.Add(2)
	contc := make(chan struct{}, 1)
	go func() {
		defer wg.Done()
		for msg := range msgc {
			cmd := msg[0]
			if cmd == midi.Clock {
				if len(msgc) < 32 {
					<-contc
				} else {
					log.Println("skipping clock")
				}
			} else {
				ev := alsa.SeqEvent{alsa.SubsSeqAddr, msg}
				aseq.Write(ev)
			}
		}
	}()
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
