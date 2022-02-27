package main

import (
	"flag"
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

func writeMidiMsgs(aseq *alsa.Seq, sa alsa.SeqAddr, msgs [][]byte) error {
	for _, msg := range msgs {
		if err := aseq.Write(alsa.SeqEvent{sa, msg}); err != nil {
			return err
		}
	}
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	kitFlag := flag.String("kit", "kit.json", "kit of devices to load")
	midiPort := flag.String("port", "FL STUDIO FIRE MIDI 1", "midi port for akai fire")
	flag.Parse()

	aseq, err := alsa.OpenSeq("fireloop")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()
	sa, err := aseq.PortAddress(*midiPort)
	must(err)
	must(aseq.OpenPortWrite(sa))
	must(aseq.OpenPortRead(sa))
	must(aseq.CreatePort("fireloop sync")) // port 1 used to send start/stop events

	write := func(b []byte) error {
		return aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: b})
	}
	f := NewFire(write)

	log.Println("loading kit", *kitFlag)
	devs := mustLoadDevices(*kitFlag)
	for i, dev := range devs {
		log.Printf("opening %q for writing", dev.MidiPort)
		dsa, err := aseq.PortAddress(dev.MidiPort)
		if err != nil {
			panic(err)
		}
		devs[i].SeqAddr = dsa
		must(aseq.OpenPortWrite(dsa))
	}

	vb := NewVoiceBank(devs)

	patbank = NewPatternBank(f, vb)
	must(f.Off())
	must(patbank.Jump(1))

	songbank = NewSongBank(f, patbank)

	inc := make(chan alsa.SeqEvent, 4)
	processEvent = processPatternEvent
	go func() {
		for ev := range inc {
			must(processEvent(aseq, ev))
		}
	}()
	for {
		ev, err := aseq.Read()
		must(err)
		inc <- ev
	}
}
