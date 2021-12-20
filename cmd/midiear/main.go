package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/theory"
)

type intervalKind struct {
	name string
	step int
}

var kinds = []intervalKind{
	{
		name: "unison",
		step: (1 - 1) * 2,
	},
	{
		name: "second",
		step: (2 - 1) * 2,
	},
	{
		name: "third",
		step: (3 - 1) * 2,
	},
	{
		name: "fifth",
		step: (5-1)*2 - 1,
	},
	/*
		{
			name: "fourth",
			step: (4-1)*2 - 1,
		},
		{
			name: "sixth",
			step: (6-1)*2 - 1,
		},
		{
			name: "seventh-minor",
			step: (7-1)*2 - 2,
		},
		{
			name: "seventh-major",
			step: (7-1)*2 - 1,
		},
		{
			name: "tritone",
			step: (4 - 1) * 2,
		},
	*/
}

func playPair(aseq *alsa.Seq, sa alsa.SeqAddr, midiChannel, n1, n2 int) {
	mc := byte(midiChannel - 1)
	msgOn := []byte{0x90 | mc, 0, 100}
	msgOff := []byte{0x80 | mc, 0, 100}
	msgOn[1], msgOff[1] = byte(n1), byte(n1)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOn}); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOff}); err != nil {
		panic(err)
	}
	msgOn[1], msgOff[1] = byte(n2), byte(n2)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOn}); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOff}); err != nil {
		panic(err)
	}
}

func main() {
	portFlag := flag.String("p", "", "playback destination port")
	midiChannel := flag.Int("channel", 1, "midi channel 1-16")
	minNote := flag.Int("min", 48, "minimum midi note")
	maxNote := flag.Int("max", 72, "maximum midi note")

	flag.Parse()

	aseq, err := alsa.OpenSeq("midiear")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	if len(*portFlag) == 0 {
		panic("expected -p port flag")
	}
	sa, err := aseq.PortAddress(*portFlag)
	if err != nil {
		panic(err)
	}
	if err := aseq.OpenPortWrite(sa); err != nil {
		panic(err)
	}

	for {
		kind := kinds[rand.Intn(len(kinds))]
		noteRange := *maxNote - *minNote
		n1 := *minNote + rand.Intn(noteRange)
		n2 := n1 + kind.step
		for {
			playPair(aseq, sa, *midiChannel, n1, n2)
			input := ""
			fmt.Println("?")
			fmt.Scanf("%s", &input)
			if input != "r" {
				break
			}
		}
		fmt.Printf("%s: %s-%s (%d,%d)\n",
			kind.name, theory.MidiNoteName(n1), theory.MidiNoteName(n2), n1, n2)
	}
}
