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

const (
	midiChannelMax = 16
	// defaultKitPath is the kit loaded when -kit is not given.
	defaultKitPath = "kit.json"
)

// The shutdown settle is a variable rather than a constant so the wait
// can be retuned from the command line: how much patience the unit needs
// before a shutdown is clean is a property of the hardware it is talking
// to, and a flag answers that without a rebuild.
var (
	// shutdownSettleLinger is how long shutdown waits after blanking the
	// unit and before the sequencer closes. A blackout is several dozen
	// commands at once, and the unit takes them in at its own pace; a
	// client that goes away with the burst still in flight leaves the last
	// lit frame standing, which is what a shutdown with no wait produced.
	shutdownSettleLinger = 250 * time.Millisecond
)

// Validate routing values before opening ports because invalid channels otherwise fail during playback.
func validateDevices(devices []Device) error {
	// A name is how the palette, the log and the display talk about a device, so two of
	// them answering to one name is a kit nobody can read back. A shared port or channel is
	// not the same thing: a kit may put two devices on one destination deliberately, so
	// only the name is checked here.
	names := make(map[string]int, len(devices))
	for index := range devices {
		if err := validateDevice(index, &devices[index]); err != nil {
			return err
		}
		name := devices[index].Name
		if first, taken := names[name]; taken {
			return fmt.Errorf("device %d (%q) has the same name as device %d", index, name, first)
		}
		names[name] = index
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

// isNilMidiWriter reports whether the sequencer is absent: a nil interface, or a nil
// *alsa.Seq that has been boxed into a non-nil one. openSequencer hands back a nil pointer
// on failure, and assigning that into an interface-typed field makes it look present.
//
// It takes the value rather than one of alsa's interfaces because callers reach the
// sequencer through several of them and what it inspects is the same either way. Naming
// one would mean this check had to be repeated for the others.
func isNilMidiWriter(aseq any) bool {
	if aseq == nil {
		return true
	}
	if seq, ok := aseq.(*alsa.Seq); ok {
		return seq == nil
	}
	return false
}

func writeMidiMsgs(aseq alsa.EventWriter, sa alsa.SeqAddr, msgs [][]byte) error {
	if isNilMidiWriter(aseq) {
		return nil
	}
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

// shutdown stops playback, releases every note still sounding, blanks the unit, and closes
// the sequencer. It is the single way the program ends, so a note cannot be left on because
// the process went away by any other route. One failure is reported but does not stop the
// rest: a note still sounding matters more than a light that stayed on.
//
// Closing is all it asks of the client, so that is all it takes. The notes are released
// through the playback that is already running rather than written from here, so this does
// not need the writing half at all.
//
// The settle after the blackout exists because the unit is the one thing
// on the path that runs at its own pace: a blackout is several dozen
// commands at once, and a client that goes away before the unit has taken
// the whole burst in leaves the last lit frame standing. A wait before the
// blackout was tried and bought nothing, so it is gone.
func shutdown(c *Controller, aseq alsa.Closer) error {
	var firstErr error
	if err := c.stopPlayback(); err != nil {
		firstErr = err
	}
	if err := blankDevice(c); err != nil && firstErr == nil {
		firstErr = err
	}
	logger.Debug("shutdown: blackout written, letting it settle",
		"duration", shutdownSettleLinger)
	time.Sleep(shutdownSettleLinger)
	if err := aseq.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	logger.Debug("shutdown: sequencer closed")
	return firstErr
}

// blankDevice leaves the unit dark. Shutting down without clearing it would leave the last
// frame lit, which reads as a sequencer that hung rather than one that stopped. It goes
// after the release because the release puts the step strip back, and a blackout suppresses
// the display output that would undo it.
func blankDevice(c *Controller) error {
	return c.patbank.pads.Blackout()
}

// handleIncomingEvent applies one Fire event. A failure stops playback and is reported
// rather than ending the process: this runs on a goroutine, where a panic would take the
// whole program with it, and a transient write failure should not end a performance.
// Playback stops because nothing may keep sounding notes that are no longer tracked.
//
// The writer is an interface so this path can be exercised without opening a port, which
// is what a nil or failing handler used to need a real sequencer for.
func (c *Controller) handleIncomingEvent(aseq sequencerWriter, ev alsa.SeqEvent) {
	err := c.Handle(aseq, ev)
	if err == nil {
		return
	}
	logger.Error("event failed", "error", err, "data", fmt.Sprintf("% x", ev.Data))
	if stopErr := c.stopPlayback(); stopErr != nil {
		logger.Error("stopping after an event failure", "error", stopErr)
	}
}

// readFire pumps the Fire's events to the handler. Reading blocks in the ALSA library, so
// this owns the calling goroutine and returns only when the Fire stops answering, which is
// the one thing that can end the program from here.
func readFire(aseq alsa.EventReader, inc chan<- alsa.SeqEvent) error {
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
func (c *Controller) processIncomingEvents(events sequencerWriter, inc <-chan alsa.SeqEvent) {
	defer func() {
		if problem := recover(); problem != nil {
			c.saveSessionOnExit()
			panic(problem)
		}
	}()
	for ev := range inc {
		// An event carrying no data is the leave request rather than MIDI. Both handlers
		// already ignore anything that is not three bytes, so an empty event cannot
		// collide with real traffic.
		if len(ev.Data) == 0 {
			c.saveSessionOnExit()
			return
		}
		c.handleIncomingEvent(events, ev)
	}
}

// waitForLeave returns once the program has been asked to stop, either by a signal or by the
// Fire ceasing to answer. Reading blocks inside the ALSA library and cannot be interrupted,
// so the read gets its own goroutine and this one waits for whichever comes first.
func waitForLeave(aseq alsa.EventReader, inc chan<- alsa.SeqEvent) error {
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
	stateFile := flag.String("state", "", "session file: loaded at startup when it exists, saved on exit, saved and loaded from the panel")
	flag.DurationVar(&shutdownSettleLinger, "shutdown-settle", shutdownSettleLinger, "time shutdown waits after blanking the unit and before the sequencer closes, so the unit has taken the whole blackout burst in")
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
		"kits", kits.String(), "port", *midiPort,
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
	f := NewFire(func(b []byte) error {
		return aseq.Write(alsa.SeqEvent{SeqAddr: sa, Data: b})
	})

	vb := NewVoiceBank(devs)
	// The banks are built by the controller that owns them, so neither is ever handed out
	// before it can reach its owner.
	ctrl := newController(f, vb, *stateFile)
	// Every way out of here goes through shutdown, including the failures below it. It
	// takes the controller rather than reaching for globals because stopping playback is
	// the controller's, and shutdown is the one path that has to be able to ask.
	defer func() {
		if err := shutdown(ctrl, aseq); err != nil {
			logger.Error("shutdown", "error", err)
		}
	}()

	if err := f.Off(); err != nil {
		return err
	}
	if err := ctrl.patbank.Jump(1); err != nil {
		return err
	}

	inc := make(chan alsa.SeqEvent, 4)

	if ctrl.sessionPath != "" {
		ctrl.sessionKit = kits.all()
		if err := ctrl.loadStateFile(ctrl.sessionPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				// Starting empty when a set was on disk looks exactly like the set having
				// been deleted, so an unreadable state file stops startup instead.
				return fmt.Errorf("state %q: %w", ctrl.sessionPath, err)
			}
			logger.Info("no state file to load, starting empty", "path", ctrl.sessionPath)
		}
	}

	handled := make(chan struct{})
	go func() {
		defer close(handled)
		ctrl.processIncomingEvents(aseq, inc)
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
