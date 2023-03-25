package main

import (
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/theory"
)

type intervalKind struct {
	name string
	ans  string
	step int
}

var kinds = []intervalKind{
	{
		name: "unison",
		ans:  "1",
		step: (1 - 1) * 2,
	},
	{
		name: "second",
		ans:  "2",
		step: (2 - 1) * 2,
	},
	{
		name: "third",
		ans:  "3",
		step: (3 - 1) * 2,
	},
	{
		name: "fifth",
		ans:  "5",
		step: (5-1)*2 - 1,
	},
	{
		name: "octave",
		ans:  "8",
		step: 12,
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
	msgOn := []byte{midi.MakeNoteOn(midiChannel - 1), 0, 100}
	msgOff := []byte{midi.MakeNoteOff(midiChannel - 1), 0, 100}
	msgOn[1], msgOff[1] = byte(n1), byte(n1)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOn}); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)
	if err := aseq.Write(alsa.SeqEvent{sa, msgOff}); err != nil {
		panic(err)
	}
	time.Sleep(time.Second / 4)
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
	invlUp := flag.Bool("up", true, "intervals going up in pitch")
	invlDown := flag.Bool("down", true, "intervals going down in pitch")

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

	rand.Seed(time.Now().UnixNano())
	right, wrong := 0, 0
	for {
		kind := kinds[rand.Intn(len(kinds))]
		noteRange := *maxNote - *minNote
		n1 := *minNote + rand.Intn(noteRange)
		n2 := n1
		dir := kind.step
		if *invlDown {
			dir = -dir
		}
		if *invlUp == *invlDown {
			if rand.Intn(2) == 0 {
				dir = -dir
			}
		}
		n2 += dir
		if n2 >= *maxNote || n2 < *minNote {
			// retry
			continue
		}

		replays := 0
		for {
			playPair(aseq, sa, *midiChannel, n1, n2)
			input := ""
			fmt.Println("?")
			fmt.Scanf("%s", &input)
			switch input {
			case "r", ".":
				replays++
				fmt.Println("replay", replays)
				continue
			case kind.ans:
				right++
				fmt.Println("ok")
			default:
				wrong++
				fmt.Println("wrong")
			}
			break
		}
		fmt.Printf("%s: %s-%s (%d,%d); score %d/%d (%.0f%%)\n",
			kind.name, theory.MidiNoteName(n1), theory.MidiNoteName(n2),
			n1, n2,
			right, wrong,
			(100.0*float64(right))/float64(right+wrong))
	}
}
