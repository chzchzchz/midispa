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
	assigns := mustLoadAssignments(os.Args[2])
	mcs := make(map[string]*MidiControls)
	for _, m := range dms {
		mcs[m.Device] = NewMidiControls(m.MidiParams())
	}
	for _, aa := range assigns {
		fmt.Printf("%+v\n", aa)
	}

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
	for _, a := range assigns {
		log.Println("opening", a.InDevice)
		c, p, err := aseq.PortAddress(a.InDevice)
		if err != nil {
			panic(err)
		}
		if err := aseq.OpenPort(c, p); err != nil {
			panic(err)
		}
	}
	log.Println("output device", assigns[0].OutDevice)
	outDev, err := alsa.OpenDeviceByName(assigns[0].OutDevice)
	if err != nil {
		panic(err)
	}
	defer outDev.Close()

	// Apply existing patch, if any.
	log.Println("applying old patch to ", assigns[0].OutDevice)
	outMcs := mcs[assigns[0].OutDevice]
	if outMcs == nil {
		panic("no out mcs" + assigns[0].OutDevice)
	}
	for _, msg := range outMcs.ToControlCodes() {
		msg[0] |= byte(dms[0].Channel - 1)
		log.Println("initializing", outMcs.Name(int(msg[1])), "=", int(msg[2]))
		if _, err := outDev.Write(msg); err != nil {
			panic(err)
		}
	}

	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		if len(ev.Data) != 3 || ev.Data[0]&0xf0 != 0xb0 {
			continue
		}
		cc, val := int(ev.Data[1]), int(ev.Data[2])
		inName := mcs[assigns[0].InDevice].Name(cc)
		if inName == "" {
			continue
		}
		if inName == "Record" && val == 0 {
			log.Println("saving presets to", os.Args[1])
			mustSaveDeviceModels(dms, os.Args[1])
			continue
		}
		outName := assigns[0].InToOut(inName)
		if outName == "" {
			continue
		}
		outCC := outMcs.CC(outName)
		outMcs.Set(outCC, val)
		ch := byte(dms[0].Channel - 1)
		log.Println(inName, "->", outName, "=", val, "; ch =", int(ch))
		if _, err := outDev.Write([]byte{0xb0 | ch, byte(outCC), byte(val)}); err != nil {
			panic(err)
		}
	}
}
