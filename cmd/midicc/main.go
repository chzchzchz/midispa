package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/midi"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: midicc devices.json assignments.json")
		os.Exit(1)
	}
	dms := mustLoadDeviceModels(os.Args[1])
	devs := make(map[string]*DeviceModel)
	mcs := make(cc.MidiControlsMap)
	for i, m := range dms {
		devs[m.Device] = &dms[i]
		if mc := cc.NewMidiControlsCC(m.MidiParams()); mc != nil {
			mcs[m.Device] = append(mcs[m.Device], mc)
		}
		if mc := cc.NewMidiControlsNote(m.MidiParams()); mc != nil {
			mcs[m.Device] = append(mcs[m.Device], mc)
		}
	}

	assigns := mustLoadAssignments(os.Args[2])
	log.Printf("loaded %d assignments", len(assigns))

	aseq, err := alsa.OpenSeq("midicc")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()
	if len(os.Args) < 2 {
		if devs, err := aseq.Devices(); err == nil {
			for _, dev := range devs {
				fmt.Printf("%+v\n", dev)
			}
		}
		os.Exit(1)
	}

	// Open input controller from sequencer.
	inDevs := make(map[string]struct{})
	for _, a := range assigns {
		inDevs[a.InDevice] = struct{}{}
	}
	in2sa := make(map[string]alsa.SeqAddr)
	for inDev := range inDevs {
		log.Printf("opening input device %q", inDev)
		sa, err := aseq.PortAddress(inDev)
		if err != nil {
			panic(err)
		}
		if err := aseq.OpenPortRead(sa); err != nil {
			panic(err)
		}
		if err := aseq.OpenPortWrite(sa); err != nil {
			log.Printf("warning: could not writeback to %q", inDev)
		}
		in2sa[inDev] = sa
	}

	log.Printf("opening output device %q", assigns[0].OutDevice)
	saOut, err := aseq.PortAddress(assigns[0].OutDevice)
	if err != nil {
		panic(err)
	}
	if err := aseq.OpenPortWrite(saOut); err != nil {
		panic(err)
	}
	for i := range assigns {
		assigns[i].saOut = saOut
		assigns[i].saIn = in2sa[assigns[i].InDevice]
	}

	turnOffButtons := func(a Assignments) {
		for _, mc := range mcs[a.InDevice] {
			if !midi.IsNoteOn(mc.Cmd) {
				continue
			}
			// TODO: have input channel.
			msg := []byte{midi.NoteOn, 0, 0}
			ev := alsa.SeqEvent{a.saIn, msg}
			for _, name := range mc.Names() {
				if _, ok := a.in2out[name]; !ok {
					continue
				}
				cc := mc.CC(name)
				if cc < 0 {
					panic("did not have cc for " + name)
				}
				msg[1] = byte(cc)
				if err := aseq.Write(ev); err != nil {
					log.Printf("failed to turn off %q: %v", name, err)
				}
			}
		}
	}
	turnOffAllButtons := func() {
		for _, a := range assigns {
			turnOffButtons(a)
		}
	}
	sigc := make(chan os.Signal, 1)
	go func() {
		<-sigc
		signal.Stop(sigc)
		turnOffAllButtons()
		os.Exit(0)
	}()
	defer func() {
		signal.Stop(sigc)
		close(sigc)
		turnOffAllButtons()
	}()
	signal.Notify(sigc, os.Interrupt)

	savef := func() {
		log.Println("saving presets to", os.Args[1])
		mustSaveDeviceModels(dms, os.Args[1])
	}
	outChan := devs[assigns[0].OutDevice].Channel
	s := Seq{aseq: aseq, savef: savef, mcs: mcs, outChan: outChan, assigns: assigns}
	s.applyPatches()
	s.run()
}
