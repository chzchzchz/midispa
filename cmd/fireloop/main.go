package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/chzchzchz/midispa/alsa"
)

var syncPort alsa.SeqAddr

const (
	midiChannelMax = 16
	midiNoteMax    = 127
)

// Validate routing values before opening ports because invalid channels otherwise fail during playback.
func validateDevices(devices []Device) error {
	for index := range devices {
		if err := validateDevice(index, &devices[index]); err != nil {
			return err
		}
	}
	return nil
}

func validateDevice(index int, dev *Device) error {
	if dev.MidiPort == "" {
		return fmt.Errorf("device %d has an empty MidiPort", index)
	}
	if !validChannel(dev.Channel) {
		return fmt.Errorf("device %q has invalid channel %d", dev.Name, dev.Channel)
	}
	if len(dev.Voices) == 0 {
		return fmt.Errorf("device %q has no voices", dev.Name)
	}
	for voiceIndex := range dev.Voices {
		if err := validateVoice(dev, voiceIndex); err != nil {
			return err
		}
	}
	return nil
}

func validateVoice(dev *Device, index int) error {
	voice := &dev.Voices[index]
	if voice.Note != nil && (*voice.Note < 0 || *voice.Note > midiNoteMax) {
		return fmt.Errorf("device %q voice %d has invalid note %d", dev.Name, index, *voice.Note)
	}
	if !validChannel(voice.Channel) {
		return fmt.Errorf("device %q voice %d has invalid channel %d", dev.Name, index, voice.Channel)
	}
	if resolveChannel(dev.Channel, voice.Channel) == 0 {
		return fmt.Errorf("device %q voice %d has no MIDI channel", dev.Name, index)
	}
	return nil
}

func validChannel(channel int) bool {
	return channel >= 0 && channel <= midiChannelMax
}

type midiWriter interface {
	Write(alsa.SeqEvent) error
}

func isNilMidiWriter(aseq midiWriter) bool {
	if aseq == nil {
		return true
	}
	if seq, ok := aseq.(*alsa.Seq); ok {
		return seq == nil
	}
	return false
}

func writeMidiMsgs(aseq midiWriter, sa alsa.SeqAddr, msgs [][]byte) error {
	if isNilMidiWriter(aseq) {
		return nil
	}
	for _, msg := range msgs {
		if err := aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: msg}); err != nil {
			return err
		}
	}
	return nil
}

// loadDeviceFile accepts the preferred top-level device array and keeps single-device files working.
func loadDeviceFile(path string) ([]Device, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty kit file")
	}
	if data[0] == '[' {
		var devices []Device
		if err := json.Unmarshal(data, &devices); err != nil {
			return nil, err
		}
		return devices, nil
	}
	var device Device
	if err := json.Unmarshal(data, &device); err != nil {
		return nil, err
	}
	return []Device{device}, nil
}

// A kit file contains a device array, or a directory can contain JSON files with device arrays.
func loadDevices(path string) ([]Device, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		devices, err := loadDeviceFile(path)
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
		fileDevices, err := loadDeviceFile(filePath)
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
