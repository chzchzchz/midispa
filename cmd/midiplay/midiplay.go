package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/chzchzchz/midispa/alsa"
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

	pat, err := NewPattern(*fname)
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
	patDur := time.Duration(pat.lastTick) * tickDur
	if *verbose {
		fmt.Printf("bpm: %v (%v); tpb: %v\n", pat.BPM, beatdur, pat.TicksPerBeat)
		fmt.Printf("pat duration: %v\n", patDur)
	}
	for i := 0; i < *loop; i++ {
		if *verbose {
			fmt.Println("loop", i, start)
		}
		for _, m := range pat.msgs {
			wait(m.Tick)
			must(aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: m.Raw}))
		}
		wait(int(pat.lastTick))
		start, tick = start.Add(patDur), 0
	}
}
