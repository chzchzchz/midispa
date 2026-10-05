package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

var syncPort alsa.SeqAddr
var sharedMIDIDestination bool

const (
	midiChannelMax = 16
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
	if dev.Settle < 0 {
		return fmt.Errorf("device %q has a negative settle %v", dev.Name, time.Duration(dev.Settle))
	}
	if err := validatePatch(fmt.Sprintf("device %q", dev.Name), dev, dev.Patch); err != nil {
		return err
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
	what := fmt.Sprintf("device %q voice %d", dev.Name, index)
	// resolvePatch rather than voice.EffectivePatch: validation runs before the voice
	// bank exists, so the voice has no device backpointer to fall back to yet.
	if err := validatePatch(what, dev, resolvePatch(dev.Patch, voice.Patch)); err != nil {
		return err
	}
	return nil
}

// validChannel accepts the 1-16 a kit states a channel in, plus 0 for a voice that leaves
// it to its device. The protocol's 0-15 numbering never reaches this layer, so a kit and a
// log read the same way; protocolChannel converts at the wire.
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
		// A kit is loaded from wherever it sits on disk, so a relative Patch is
		// resolved against the file's own directory rather than the working one.
		setKitBaseDir(devices, filepath.Dir(path))
		return devices, nil
	}
	var device Device
	if err := json.Unmarshal(data, &device); err != nil {
		return nil, err
	}
	devices := []Device{device}
	setKitBaseDir(devices, filepath.Dir(path))
	return devices, nil
}

// setKitBaseDir records where each device's kit file lives.
func setKitBaseDir(devices []Device, dir string) {
	for index := range devices {
		devices[index].baseDir = dir
	}
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

// sequencerSession is what shutdown needs from the ALSA client. It is an interface so the
// shutdown path can be exercised without opening a port.
type sequencerSession interface {
	sequencerWriter
	Close() error
}

// eventReader is the blocking half of the ALSA client.
type eventReader interface {
	Read() (alsa.SeqEvent, error)
}

// shutdown stops playback, releases every note still sounding, blanks the unit, and closes
// the sequencer. It is the single way the program ends, so a note cannot be left on because
// the process went away by any other route. One failure is reported but does not stop the
// rest: a note still sounding matters more than a light that stayed on.
func shutdown(aseq sequencerSession) error {
	var firstErr error
	if err := stopPlayback(); err != nil {
		firstErr = err
	}
	if err := blankDevice(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := aseq.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// blankDevice leaves the unit dark. Shutting down without clearing it would leave the last
// frame lit, which reads as a sequencer that hung rather than one that stopped. It goes
// after the release because the release puts the step strip back, and a blackout suppresses
// the display output that would undo it.
func blankDevice() error {
	if patbank == nil {
		return nil
	}
	return patbank.f.Blackout()
}

// handleIncomingEvent applies one Fire event. A failure stops playback and is reported
// rather than ending the process: this runs on a goroutine, where a panic would take the
// whole program with it, and a transient write failure should not end a performance.
// Playback stops because nothing may keep sounding notes that are no longer tracked.
//
// The writer is an interface so this path can be exercised without opening a port, which
// is what a nil or failing handler used to need a real sequencer for.
func handleIncomingEvent(aseq sequencerWriter, ev alsa.SeqEvent) {
	err := processEvent(aseq, ev)
	if err == nil {
		return
	}
	logger.Error("event failed", "error", err, "data", fmt.Sprintf("% x", ev.Data))
	if stopErr := stopPlayback(); stopErr != nil {
		logger.Error("stopping after an event failure", "error", stopErr)
	}
}

// readFire pumps the Fire's events to the handler. Reading blocks in the ALSA library, so
// this owns the calling goroutine and returns only when the Fire stops answering, which is
// the one thing that can end the program from here.
func readFire(aseq eventReader, inc chan<- alsa.SeqEvent) error {
	for {
		ev, err := aseq.Read()
		if err != nil {
			return fmt.Errorf("reading the Fire: %w", err)
		}
		inc <- ev
	}
}

// openPorts resolves every port Fireloop writes to and opens them, so a wrong port name is
// reported before playback starts rather than as a panic later. It hands back the Fire port
// address rather than resolving it twice, so setup cannot fail a second time on a name that
// has already been found.
func openPorts(aseq *alsa.Seq, firePort string, devs []Device) (alsa.SeqAddr, error) {
	sa, err := aseq.PortAddress(firePort)
	if err != nil {
		return sa, fmt.Errorf("resolve Fire port %q: %w", firePort, err)
	}
	if err := aseq.OpenPortWrite(sa); err != nil {
		return sa, fmt.Errorf("open Fire port for writing: %w", err)
	}
	if err := aseq.OpenPortRead(sa); err != nil {
		return sa, fmt.Errorf("open Fire port for reading: %w", err)
	}
	if syncPort, err = aseq.CreatePortAddr("fireloop sync"); err != nil {
		return sa, fmt.Errorf("create sync port: %w", err)
	}
	for i, dev := range devs {
		dsa, err := aseq.PortAddress(dev.MidiPort)
		if err != nil {
			return sa, fmt.Errorf("resolve output port %q for %q: %w", dev.MidiPort, dev.Name, err)
		}
		if err := aseq.OpenPortWrite(dsa); err != nil {
			return sa, fmt.Errorf("open output port %q for writing: %w", dev.MidiPort, err)
		}
		devs[i].SeqAddr = dsa
	}
	return sa, nil
}

// openSequencer opens the ALSA client and the ports Fireloop needs, closing the client
// again if any of that fails, so a failed startup leaves nothing open behind it.
func openSequencer(firePort string, devs []Device) (*alsa.Seq, alsa.SeqAddr, error) {
	aseq, err := alsa.OpenSeq("fireloop")
	if err != nil {
		return nil, alsa.SeqAddr{}, fmt.Errorf("open sequencer: %w", err)
	}
	sa, err := openPorts(aseq, firePort, devs)
	if err != nil {
		aseq.Close()
		return nil, sa, err
	}
	return aseq, sa, nil
}

// processIncomingEvents applies the Fire's events one at a time, which is the only place the
// banks are changed, and saves the session when it is asked to leave.
//
// The save is here rather than wherever the request came from because this goroutine is the
// only reader of the banks. Saving from a signal handler would walk the pattern map while
// this one could be adding to it, and concurrent map iteration is a fatal error rather than
// a lost note.
//
// A panic anywhere below ends the process, so the session is written before the panic is
// passed on. That is the whole point of the save on the way out: a set should survive the
// run that made it whether the run was ended or fell over, and re-panicking keeps the
// failure loud instead of leaving the unit playing on with a half-applied edit.
func processIncomingEvents(events sequencerWriter, inc <-chan alsa.SeqEvent) {
	defer func() {
		if problem := recover(); problem != nil {
			saveSessionOnExit()
			panic(problem)
		}
	}()
	for ev := range inc {
		// An event carrying no data is the leave request rather than MIDI. Both handlers
		// already ignore anything that is not three bytes, so an empty event cannot
		// collide with real traffic.
		if len(ev.Data) == 0 {
			saveSessionOnExit()
			return
		}
		handleIncomingEvent(events, ev)
	}
}

// waitForLeave returns once the program has been asked to stop, either by a signal or by the
// Fire ceasing to answer. Reading blocks inside the ALSA library and cannot be interrupted,
// so the read gets its own goroutine and this one waits for whichever comes first.
func waitForLeave(aseq eventReader, inc chan<- alsa.SeqEvent) error {
	readErr := make(chan error, 1)
	go func() { readErr <- readFire(aseq, inc) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case received := <-signals:
		logger.Info("shutting down", "signal", received)
		return nil
	case err := <-readErr:
		return err
	}
}

func main() {
	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	var kits kitPaths
	flag.Var(&kits, "kit", "kit of devices to load; repeat to merge several kits")
	midiPort := flag.String("port", "FL STUDIO FIRE Jack 1", "midi port for akai fire")
	logLevel := flag.String("log-level", "info", "log verbosity: debug, info, warn or error")
	logFormat := flag.String("log-format", "text", "log format: text or json")
	flag.BoolVar(&sharedMIDIDestination, "shared-midi-destination", false, "broadcast MIDI output to all connected destinations")
	flag.StringVar(&statePath, "state", "", "session file: loaded at startup when it exists, saved on exit, saved and loaded from the panel")
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

	devs, err := loadKit(kits.all())
	if err != nil {
		return err
	}
	if err := validateDevices(devs); err != nil {
		return err
	}

	aseq, sa, err := openSequencer(*midiPort, devs)
	if err != nil {
		return err
	}
	// Every way out of here goes through shutdown, including the failures below it.
	defer func() {
		if err := shutdown(aseq); err != nil {
			logger.Error("shutdown", "error", err)
		}
	}()

	f := NewFire(func(b []byte) error {
		return aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: b})
	})

	vb := NewVoiceBank(devs)
	patbank = NewPatternBank(f, vb)
	if err := f.Off(); err != nil {
		return err
	}
	if err := patbank.Jump(1); err != nil {
		return err
	}
	songbank = NewSongBank(f, patbank)

	inc := make(chan alsa.SeqEvent, 4)
	processEvent = processPatternEvent

	if statePath != "" {
		stateKitPaths = kits.all()
		if err := loadStateFile(statePath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				// Starting empty when a set was on disk looks exactly like the set having
				// been deleted, so an unreadable state file stops startup instead.
				return fmt.Errorf("state %q: %w", statePath, err)
			}
			logger.Info("no state file to load, starting empty", "path", statePath)
		}
	}

	handled := make(chan struct{})
	go func() {
		defer close(handled)
		processIncomingEvents(aseq, inc)
	}()

	// Leaving is the only thing this function does from here: wait to be asked, hand the
	// request to the goroutine that owns the banks, and come back once it has saved. The
	// deferred shutdown then runs on the way out as it does for every other exit, so the
	// session is saved, everything sounding is released, the unit is blanked and the client
	// is closed by one path whichever way the program ended.
	leaveErr := waitForLeave(aseq, inc)
	inc <- alsa.SeqEvent{}
	<-handled
	return leaveErr
}
