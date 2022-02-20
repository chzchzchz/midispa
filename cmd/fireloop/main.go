package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var shiftOn = false
var altOn = false
var pendingNumber = 0
var bpm = 139
var cancelPlayback context.CancelFunc

func handleMute(n int) error {
	if altOn {
		patbank.ClearTrackRow(n)
		return patbank.Jump(0)
	}
	return patbank.SelectTrackRow(n)
}

func processEvent(aseq *alsa.Seq, ev alsa.SeqEvent) error {
	fmt.Printf("%+v\n", ev)
	if len(ev.Data) != 3 {
		return nil
	}

	cmd := ev.Data[0] & 0xf0
	if cmd == 0x80 {
		// note-off
		switch int(ev.Data[1]) {
		case NoteShift:
			shiftOn = false
			if pendingNumber > 20 && pendingNumber < 300 {
				bpm = pendingNumber
				pendingNumber = 0
				return patbank.Jump(0)
			}
		case NoteAlt:
			altOn = false
			return patbank.f.SetLed(NoteAlt, 0)
		}
		return nil
	}

	// cc / note on only
	if cmd != 0x90 && cmd != 0xb0 {
		return nil
	}
	if x, y, ok := Note2Grid(int(ev.Data[1])); ok {
		if shiftOn {
			pendingNumber *= 10
			if pendingNumber > 999 {
				pendingNumber = 0
			}
			addend := (3*y + ((x % 4) % 3)) + 1
			if y == 3 {
				addend = 0
			}
			pendingNumber += addend
			if err := patbank.f.ClearOLED(); err != nil {
				return err
			}
			s := fmt.Sprintf("Tempo: %03d", pendingNumber)
			return patbank.f.Print(4, 3, s)
		}
		patEv, err := patbank.ToggleEvent(y, x, int(ev.Data[2]))
		if err != nil {
			return err
		}
		if cancelPlayback != nil {
			return nil
		}
		if patEv.Velocity > 0 {
			return writeMidiMsgs(aseq, patEv.device.SeqAddr, patEv.ToMidi())
		}
		return nil
	}
	switch int(ev.Data[1]) {
	case NoteShift:
		shiftOn = true
		return nil
	case NotePatternUp:
		return patbank.Jump(1)
	case NotePatternDown:
		return patbank.Jump(-1)
	case NoteAlt:
		altOn = true
		if err := patbank.f.SetLed(NoteAlt, 1); err != nil {
			return err
		}
		if shiftOn {
			return patbank.f.Off()
		}
	case NoteMute1:
		return handleMute(1)
	case NoteMute2:
		return handleMute(2)
	case NoteMute3:
		return handleMute(3)
	case NoteMute4:
		return handleMute(4)
	case CCSelect:
		dir := 1
		if int(ev.Data[2]) == EncoderLeft {
			dir = -1
		}
		return patbank.JogSelect(dir)
	case NoteStop:
		if cancelPlayback != nil {
			cancelPlayback()
			cancelPlayback = nil
		}
		return nil
	case NotePlay:
		if cancelPlayback == nil {
			startSequencer(aseq)
		}
		return nil
	}
	return nil
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

	inc := make(chan alsa.SeqEvent, 4)
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
