package main

import (
	"fmt"
	"log"
	"os"

	"github.com/chzchzchz/midispa/alsa"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: midicc devices.json assignments.json")
		os.Exit(1)
	}
	dms := mustLoadDeviceModels(os.Args[1])
	devs := make(map[string]*DeviceModel)
	mcs := make(map[string]*MidiControls)
	for i, m := range dms {
		devs[m.Device] = &dms[i]
		mcs[m.Device] = NewMidiControls(m.MidiParams())
	}

	assigns := mustLoadAssignments(os.Args[2])
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

	inDevs := make(map[string]struct{})
	for _, a := range assigns {
		inDevs[a.InDevice] = struct{}{}
	}

	// Open input controller from sequencer.
	for inDev := range inDevs {
		log.Printf("opening input device %q", inDev)
		sa, err := aseq.PortAddress(inDev)
		if err != nil {
			panic(err)
		}
		if err := aseq.OpenPort(sa.Client, sa.Port); err != nil {
			panic(err)
		}
	}
	log.Printf("opening output device %q", assigns[0].OutDevice)
	saOut, err := aseq.PortAddress(assigns[0].OutDevice)
	if err != nil {
		panic(err)
	}
	if err := aseq.OpenPortWrite(saOut); err != nil {
		panic(err)
	}

	// Apply existing patch, if any.
	outChan := devs[assigns[0].OutDevice].Channel
	log.Printf("applying old patch to %q on channel %d", assigns[0].OutDevice, outChan)
	outMcs := mcs[assigns[0].OutDevice]
	if outMcs == nil {
		panic("no out mcs" + assigns[0].OutDevice)
	}
	for _, msg := range outMcs.ToControlCodes() {
		msg[0] |= byte(outChan - 1)
		log.Println("initializing", outMcs.Name(int(msg[1])), "=", int(msg[2]))
		ev := alsa.SeqEvent{saOut, msg}
		if err := aseq.Write(ev); err != nil {
			panic(err)
		}
	}

	pgm := 0
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		cmd := ev.Data[0] & 0xf0

		if len(ev.Data) == 2 && cmd == 0xc0 {
			v := int(ev.Data[1])
			if v < len(assigns) {
				pgm = v
				log.Println("controller program", pgm, "is", assigns[pgm].Title)
				continue
			}
		}
		if len(ev.Data) != 3 || cmd != 0xb0 {
			continue
		}
		cc, val := int(ev.Data[1]), int(ev.Data[2])
		inName := mcs[assigns[pgm].InDevice].Name(cc)
		if inName == "" {
			continue
		}
		if inName == "Record" && val == 0 {
			log.Println("saving presets to", os.Args[1])
			mustSaveDeviceModels(dms, os.Args[1])
			continue
		}
		outName := assigns[pgm].InToOut(inName)
		if outName == "" {
			continue
		}
		log.Println(inName, "->", outName, "=", val, "; ch =", outChan)
		outCC := outMcs.CC(outName)
		if !outMcs.Set(outCC, val) {
			continue
		}
		ch := byte(outChan - 1)
		ev = alsa.SeqEvent{SeqAddr: saOut, Data: []byte{0xb0 | ch, byte(outCC), byte(val)}}
		if err := aseq.Write(ev); err != nil {
			panic(err)
		}
	}
}
