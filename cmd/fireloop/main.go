package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/util"
)

var syncPort alsa.SeqAddr

const (
	midiChannelMax = 16
	midiNoteMax    = 127
)

// Validate routing values before opening ports because invalid channels otherwise fail during playback.
func validateDevices(devices []Device) error {
	for i, dev := range devices {
		if dev.MidiPort == "" {
			return fmt.Errorf("device %d has an empty MidiPort", i)
		}
		if dev.Channel < 0 || dev.Channel > midiChannelMax {
			return fmt.Errorf("device %q has invalid channel %d", dev.Name, dev.Channel)
		}
		if len(dev.Voices) == 0 {
			return fmt.Errorf("device %q has no voices", dev.Name)
		}
		for j, voice := range dev.Voices {
			if voice.Note < 0 || voice.Note > midiNoteMax {
				return fmt.Errorf("device %q voice %d has invalid note %d", dev.Name, j, voice.Note)
			}
			if voice.Channel < 0 || voice.Channel > midiChannelMax {
				return fmt.Errorf("device %q voice %d has invalid channel %d", dev.Name, j, voice.Channel)
			}
			if dev.Channel == 0 && voice.Channel == 0 {
				return fmt.Errorf("device %q voice %d has no MIDI channel", dev.Name, j)
			}
		}
	}
	return nil
}

func writeMidiMsgs(aseq *alsa.Seq, sa alsa.SeqAddr, msgs [][]byte) error {
	for _, msg := range msgs {
		if err := aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: msg}); err != nil {
			return err
		}
	}
	return nil
}

// A kit can be one JSON file or a directory of JSON files, so multi-device
// setups can keep each device's routing configuration separate.
func loadDevices(path string) ([]Device, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		devices, err := util.LoadJSONFile[Device](path)
		if err != nil {
			return nil, fmt.Errorf("load kit %q: %w", path, err)
		}
		if len(devices) == 0 {
			return nil, fmt.Errorf("kit %q contains no devices", path)
		}
		return devices, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var devices []Device
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		filePath := filepath.Join(path, entry.Name())
		fileDevices, err := util.LoadJSONFile[Device](filePath)
		if err != nil {
			return nil, fmt.Errorf("load kit %q: %w", filePath, err)
		}
		devices = append(devices, fileDevices...)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("kit directory %q contains no devices", path)
	}
	sort.SliceStable(devices, func(i, j int) bool {
		return devices[i].Name < devices[j].Name
	})
	return devices, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	kitFlag := flag.String("kit", "kit.json", "kit of devices to load")
	midiPort := flag.String("port", "FL STUDIO FIRE Jack 1", "midi port for akai fire")
	flag.Parse()

	log.Println("loading kit", *kitFlag)
	devs, err := loadDevices(*kitFlag)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateDevices(devs); err != nil {
		log.Fatal(err)
	}

	aseq, err := alsa.OpenSeq("fireloop")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()
	sa, err := aseq.PortAddress(*midiPort)
	must(err)
	must(aseq.OpenPortWrite(sa))
	must(aseq.OpenPortRead(sa))
	syncPort, err = aseq.CreatePortAddr("fireloop sync")
	must(err)

	write := func(b []byte) error {
		return aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: b})
	}
	f := NewFire(write)

	for i, dev := range devs {
		log.Printf("opening %q for writing", dev.MidiPort)
		dsa, err := aseq.PortAddress(dev.MidiPort)
		if err != nil {
			log.Fatalf("resolve output port %q: %v", dev.MidiPort, err)
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
