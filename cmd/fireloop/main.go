package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
)

var syncPort alsa.SeqAddr
var sharedMIDIDestination bool

const (
	midiChannelMax = 16
	midiNoteMax    = 127
	// defaultKitPath is the kit loaded when -kit is not given.
	defaultKitPath = "kit.json"
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
		return fmt.Errorf("device %d (%q) has an empty MidiPort", index, dev.Name)
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

func midiDestination(destination alsa.SeqAddr) alsa.SeqAddr {
	if sharedMIDIDestination {
		return alsa.SubsSeqAddr
	}
	return destination
}

func writeMidiMsgs(aseq midiWriter, sa alsa.SeqAddr, msgs [][]byte) error {
	if isNilMidiWriter(aseq) {
		return nil
	}
	sa = midiDestination(sa)
	for _, msg := range msgs {
		logOutbound("", sa, msg)
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

// loadKit merges the devices from every kit path in the order the paths are given, so
// the voice list on the pad grid follows the command line rather than the file names.
func loadKit(paths []string) ([]Device, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no kit specified")
	}
	var devices []Device
	for _, path := range paths {
		pathDevices, err := loadDevices(path)
		if err != nil {
			return nil, err
		}
		devices = append(devices, pathDevices...)
	}
	return devices, nil
}

// kitPaths collects repeated -kit arguments, which lets several kit files or directories
// be merged. The flag package cannot repeat a string flag on its own, hence the Value.
type kitPaths struct {
	paths []string
	given bool
}

func (k *kitPaths) String() string {
	if len(k.paths) == 0 {
		return defaultKitPath
	}
	return strings.Join(k.paths, ",")
}

// Set drops the default kit the first time the flag appears. Appending to it instead
// would load the default alongside whatever the command line asked for, which fails
// outright when no kit.json sits in the working directory.
func (k *kitPaths) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty kit path")
	}
	if !k.given {
		k.paths = nil
		k.given = true
	}
	k.paths = append(k.paths, value)
	return nil
}

// all returns the kit paths to load: the ones given on the command line, or the default
// when there are none.
func (k *kitPaths) all() []string {
	if len(k.paths) == 0 {
		return []string{defaultKitPath}
	}
	return k.paths
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	var kits kitPaths
	flag.Var(&kits, "kit", "kit of devices to load; repeat to merge several kits")
	midiPort := flag.String("port", "FL STUDIO FIRE Jack 1", "midi port for akai fire")
	logLevel := flag.String("log-level", "info", "log verbosity: debug, info, warn or error")
	logFormat := flag.String("log-format", "text", "log format: text or json")
	flag.BoolVar(&sharedMIDIDestination, "shared-midi-destination", false, "broadcast MIDI output to all connected destinations")
	flag.Parse()

	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		log.Printf("unknown log level %q, using info", *logLevel)
	}
	if !strings.EqualFold(*logFormat, logFormatJSON) && !strings.EqualFold(*logFormat, "text") {
		log.Printf("unknown log format %q, using text", *logFormat)
	}
	setLogger(newLogger(level, *logFormat))
	logger.Info("fireloop starting",
		"kits", kits.String(), "port", *midiPort, "shared", sharedMIDIDestination,
		"level", level.String(), "format", strings.ToLower(*logFormat))

	log.Println("loading kit", kits.String())
	devs, err := loadKit(kits.all())
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
