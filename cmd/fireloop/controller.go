package main

import (
	"sync"
	"sync/atomic"
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
	// pageView is the page grid: a mode of the controller rather
	// than a view of a page, because no page is ever left on it.
	pageView
)

// screenWord carries the page and the view in one word, so a playback worker asks one
// atomic load what the unit is showing and can never see a page and a view that were never
// on screen together. The page travels in the high bits and the view in the low two.
func packScreen(page int, view viewMode) uint32 {
	return uint32(page)<<2 | uint32(view)
}

// screenPage unpacks the page a screen word carries.
func screenPage(word uint32) int {
	return int(word >> 2)
}

// screenView unpacks the view a screen word carries.
func screenView(word uint32) viewMode {
	return viewMode(word & 3)
}

// Controller owns everything a Fire event can change: the banks, the clipboard, the view
// the controls address, the modifiers under a held button, a tempo being typed, the session
// file, and the running playback.
//
// One goroutine calls Handle and nothing else touches these fields, so the controller needs
// no locking of its own. The exceptions are the tempo, which is left as a package-level
// atomic, and the screen the workers sample through screen, because the playback workers
// read both from their own goroutines and a controller field would have to be guarded to
// stay honest.
type Controller struct {
	patbank   *PatternBank
	songbank  *SongBank
	clipboard *Pattern

	// mode is which view the controls address, and pageIdx is the page
	// the controller is sitting on. The event loop is their one writer.
	// screen mirrors the pair for the playback workers, whose playheads
	// and pattern sources ask what is showing; a read of the fields from
	// those goroutines would race every view switch and page jump.
	mode viewMode
	// screen is mode and pageIdx as the workers see them. setScreen is its
	// only writer, the same arrangement the tempo's atomic serves.
	screen atomic.Uint32

	shift    bool
	alt      bool
	pending  int
	tapTimes []time.Time

	// running is the running set, or nil when nothing has been started. It
	// stays installed after its workers end, so the question is whether a
	// session is running and not whether this field is nil. One entry per
	// page the set plays, the page on screen included, is the whole of it:
	// there is no primary and no extras, only the page that is shown and
	// the pages that are not.
	running *RunningSet

	// pages is the page table, one entry per pad, and pageIdx is
	// the page the controller is sitting on. The current page's
	// live state is authoritative: its memento is written by the
	// save that precedes every jump and every save to disk, so
	// the table and the banks hold the same objects from the
	// first jump on.
	pages   map[int]*Page
	pageIdx int

	// prevMode is the view page mode was entered from, and the
	// view a jump from page mode records for the page being
	// left, because page mode is a mode of the controller
	// rather than a view of a page.
	prevMode viewMode

	// pagePad and pagePadTime track the double tap that jumps
	// to a page: the pad last pressed in page mode, and when.
	// pagePad is -1 when no press is being watched.
	pagePad     int
	pagePadTime time.Time

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
	controller := &Controller{
		patbank:     bank,
		pages:       map[int]*Page{1: newPage(vb)},
		pagePad:     -1,
		sessionPath: sessionPath,
	}
	controller.setScreen(1, patternView)
	controller.songbank = NewSongBank(f, bank)
	bank.controller = controller
	controller.updatePageIndicator()
	return controller
}

// playing reports whether a set is running, on any of its sessions.
func (c *Controller) playing() bool {
	return c.running != nil && c.running.playing()
}

// stopPlayback ends the running set, if there is one, and puts the pattern view back the
// way a stopped set leaves it. Stopping is idempotent, so the callers that stop before they
// edit and the ones that edit before they stop cannot fight over it.
func (c *Controller) stopPlayback() error {
	var firstErr error
	if c.running != nil {
		firstErr = c.running.stop()
		c.running = nil
	}
	if c.patbank == nil {
		return firstErr
	}
	c.patbank.clearPadState()
	// A playhead left lit on the step strip would outlive the playback that put it
	// there, so put it back.
	if err := c.patbank.clearStepPlayhead(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// seekSongBeat schedules a seek on the running set and reports the beat it seeks away
// from, which is the arrangement readout's before-and-after. Every session is sought, so
// all the pages a set plays stay in sync. It is the one thing a bank needs
// the running set for that is not stopping it, and it is why a bank can reach its
// owner for a seek without holding a handle of its own.
//
// Nothing is scheduled when nothing is playing, which the caller checks first; -1 is what
// that reports, since a live playhead is never behind zero.
func (c *Controller) seekSongBeat(beat float32) float32 {
	if c.running == nil {
		return -1
	}
	return c.running.seek(beat)
}

// Handle applies one Fire event to whichever view is showing. This is the whole dispatch:
// the mode decides which handler runs, so a test that drives Handle exercises the same
// choice production makes rather than naming a handler directly.
func (c *Controller) Handle(aseq sequencerWriter, ev alsa.SeqEvent) error {
	switch c.mode {
	case songView:
		return c.processSongEvent(aseq, ev)
	case pageView:
		return c.processPageEvent(aseq, ev)
	default:
		return c.processPatternEvent(aseq, ev)
	}
}

// setScreen records where the unit is looking. It is the one writer of the pair: the event
// loop asks c.mode and c.pageIdx, which it alone writes, and the playback workers ask
// showing(), which one atomic load answers for both.
func (c *Controller) setScreen(page int, view viewMode) {
	c.pageIdx = page
	c.mode = view
	c.screen.Store(packScreen(page, view))
}

// setPage moves to a page and keeps the view.
func (c *Controller) setPage(page int) {
	c.setScreen(page, c.mode)
}

// setView switches the view and keeps the page.
func (c *Controller) setView(view viewMode) {
	c.setScreen(c.pageIdx, view)
}

// showing is the screen as the playback workers see it, in one load: the
// page and the view that their playheads and pattern sources both ask
// about. The event loop asks c.mode and c.pageIdx, which it alone writes.
func (c *Controller) showing() (int, viewMode) {
	word := c.screen.Load()
	return screenPage(word), screenView(word)
}

// setMode moves between the pattern view and the arrangement. Playback stops because a
// set is built for the view it started on: each view plays its own selection, and a set
// that kept running across the switch would sound a selection the screen no longer shows.
// The edit modes are closed because each belongs to the view being left, and the
// modifiers are released because they act on whatever the view under them is.
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
	c.setView(mode)
	return c.patbank.pads.SetLed(NotePatternSong, songLight)
}

// redrawMode puts the mode lights back and repaints the view the
// controller is on. Every way out of page mode and every load ends
// here: the mode is the one record of which view's redraw is the
// right one, so the caller sets it and asks, rather than each
// caller carrying its own copy of the choice.
func (c *Controller) redrawMode() error {
	if err := c.restoreIndicators(); err != nil {
		return err
	}
	switch c.mode {
	case songView:
		return c.songbank.redraw()
	case pageView:
		return c.drawPageMode()
	default:
		return c.patbank.redraw()
	}
}
