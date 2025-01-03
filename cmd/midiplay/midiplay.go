package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fname := flag.String("input", "in.mid", "input midi filename")
	midiPort := flag.String("port", "MIDI4x4 MIDI Out 1", "midi port for output")
	loop := flag.Int("loop", 1, "number of times to loop")
	verbose := flag.Bool("verbose", false, "verbose mode")
	flag.Parse()

	pat, err := track.NewPattern(*fname)
	if err != nil {
		panic(err)
	}

	aseq, err := alsa.OpenSeq("midiplay")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()
	sa, err := aseq.PortAddress(*midiPort)
	must(err)
	must(aseq.OpenPortWrite(sa))
	must(aseq.OpenPortRead(sa))

	tickDur, tick := pat.TickDuration(), 0
	beatdur := time.Duration(float64(time.Second) * (1.0 / (float64(pat.BPM) / 60.0)))
	tickDurF := float64(tickDur)
	start := time.Now()
	wait := func(t int) {
		if t <= tick {
			return
		}
		curTick := float64(time.Since(start)) / tickDurF
		diff := float64(t) - curTick
		sleepTime := time.Duration(diff * tickDurF)
		//fmt.Printf("%v: %v\n", t, sleepTime)
		time.Sleep(sleepTime)
		tick = t
	}
	if *verbose {
		fmt.Printf("bpm: %v (%v); tpb: %v\n", pat.BPM, beatdur, pat.TicksPerBeat)
		fmt.Printf("pat duration: %v\n", pat.Duration())
	}

	var usedChannels [16]bool
	for _, m := range pat.Msgs {
		cmd := m.Raw[0]
		if midi.Message(cmd) == midi.NoteOn {
			usedChannels[midi.Channel(cmd)] = true
		}
	}

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
			if usedChannels[i] {
				msg := []byte{midi.MakeCC(i), midi.AllNotesOff, 0x7f}
				aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: msg})
			}
		}
		os.Exit(1)
	}()

	for i := 0; i < *loop; i++ {
		if *verbose {
			fmt.Println("loop", i, start)
		}
		for _, m := range pat.Msgs {
			wait(m.Tick)
			must(aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: m.Raw}))
		}
		wait(int(pat.LastTick))
		start, tick = start.Add(pat.Duration()), 0
	}
}
