package main

import (
	"sync"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

// viewMode is which of the two views the controls are addressing. It replaces a function
// pointer that was swapped from a handler while a second goroutine dispatched through it,
// which meant the mode lived in two places at once: whichever handler was installed, and
// whatever the last switch had set. One field is the one record.
type viewMode int

const (
	patternView viewMode = iota
	songView
)

// Controller owns everything a Fire event can change: the banks, the clipboard, the view
// the controls address, the modifiers under a held button, a tempo being typed, the session
// file, and the running playback.
//
// One goroutine calls Handle and nothing else touches these fields, so the controller needs
// no locking of its own. The exception is the tempo, which is left as a package-level atomic
// because the playback worker samples it from its own goroutine and a controller field would
// have to be guarded to stay honest.
type Controller struct {
	patbank   *PatternBank
	songbank  *SongBank
	clipboard *Pattern

	mode     viewMode
	shift    bool
	alt      bool
	pending  int
	tapTimes []time.Time

	// playback is the handle for the running set, or nil when nothing is playing.
	playback playbackStopFunc

	// sessionPath is the file the set is saved to and loaded from, and sessionKit is the
	// kit it is measured against. Both stay empty until run wires the flag, so the panel
	// gestures report that there is nowhere to save rather than writing somewhere unasked.
	sessionPath string
	sessionKit  []string

	// exitSave keeps the save on the way out to one write. The signal path and a panic
	// unwind can both reach it, and a second write would only add another timestamp to a
	// file the user is about to read.
	exitSave sync.Once

	// saves counts the publishes still in flight, so the save on the way out can wait for
	// them: an older snapshot that lands after a newer one would leave the file showing
	// less than the set the user is leaving with.
	saves sync.WaitGroup
}

// newController builds the banks and owns them. Building them here rather than handing it
// banks that already exist is what makes the ownership real: a bank can stop playback from
// inside an edit that rewrites what the worker reads, so it has to reach its owner, and a
// bank handed out before its owner existed would have to guard against that.
//
// The session path is a parameter rather than a field to set afterwards because it comes
// from the command line: a controller built and then configured is one where a whole
// configuration can be missed without anything noticing.
func newController(f *Fire, vb *VoiceBank, sessionPath string) *Controller {
	bank := NewPatternBank(f, vb)
	controller := &Controller{patbank: bank, mode: patternView, sessionPath: sessionPath}
	controller.songbank = NewSongBank(f, bank)
	bank.controller = controller
	return controller
}

// Handle applies one Fire event to whichever view is showing. This is the whole dispatch:
// the mode decides which handler runs, so a test that drives Handle exercises the same
// choice production makes rather than naming a handler directly.
func (c *Controller) Handle(aseq sequencerWriter, ev alsa.SeqEvent) error {
	if c.mode == songView {
		return c.processSongEvent(aseq, ev)
	}
	return c.processPatternEvent(aseq, ev)
}

// setMode moves between the pattern view and the arrangement. Playback stops because the
// other view has no sequencer for it, the edit modes are closed because each belongs to the
// view being left, and the modifiers are released because they act on whatever the view under
// them is.
func (c *Controller) setMode(mode viewMode, songLight int) error {
	if err := c.stopPlayback(); err != nil {
		return err
	}
	if err := c.exitPatternEditModes(); err != nil {
		return err
	}
	if err := c.releaseModifiers(); err != nil {
		return err
	}
	c.mode = mode
	return c.patbank.f.SetLed(NotePatternSong, songLight)
}
