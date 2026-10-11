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
	// pageView is the page grid: a mode of the controller rather
	// than a view of a page, because no page is ever left on it.
	pageView
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

	// playback is the running set, or nil when nothing has been started. It stays
	// installed after its worker ends, so the question is whether the session is running
	// and not whether this field is nil.
	playback *PlaybackSession

	// extra holds the sessions of the other enabled pages while a
	// song-mode set plays more than one page: the primary in
	// playback draws the visible arrangement, and these play the
	// same song index from their own pages with nothing to draw.
	// They are stopped by the same stopPlayback that ends the
	// primary, so a set has one place to ask about and one place
	// to end.
	extra []*PlaybackSession

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
		mode:        patternView,
		pages:       map[int]*Page{1: newPage(vb)},
		pageIdx:     1,
		pagePad:     -1,
		sessionPath: sessionPath,
	}
	controller.songbank = NewSongBank(f, bank)
	bank.controller = controller
	controller.updatePageIndicator()
	return controller
}

// startPlayback installs and starts a set the bank built. The bank says what to play and
// how to draw it; the controller owns the fact that something is, which is what lets every
// guard and every stop read one answer rather than three that could disagree.
//
// Its callers ask first that nothing is playing, so there is nothing to stop here. That is
// also what makes replacing the field rather than stopping it safe: whatever was installed
// either never started or has already ended, and an ended session has no worker left to
// join.
func (c *Controller) startPlayback(aseq sequencerWriter, playback *Playback) {
	session := newPlaybackSession(playback)
	c.playback = session
	session.Start(aseq)
}

// playing reports whether a set is running, on the primary
// or on any extra session. A worker only ends when it is
// stopped or its write fails, and a failed write can end one
// session's worker while another's keeps running, so the
// guards have to ask about all of them: starting a new set
// replaces the extras, which would orphan a still-running one
// with nothing left that can stop it.
func (c *Controller) playing() bool {
	if c.playback != nil && c.playback.Running() {
		return true
	}
	for _, session := range c.extra {
		if session.Running() {
			return true
		}
	}
	return false
}

// stopPlayback ends the running set, if there is one, and puts the pattern view back the
// way a stopped set leaves it. Stopping is idempotent, so the callers that stop before they
// edit and the ones that edit before they stop cannot fight over it.
func (c *Controller) stopPlayback() error {
	var firstErr error
	if session := c.playback; session != nil {
		c.playback = nil
		if err := session.Stop(); err != nil {
			firstErr = err
		}
	}
	for _, session := range c.extra {
		if err := session.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.extra = nil
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
// from, which is the arrangement readout's before-and-after. Every installed session is
// sought, so all the pages a set plays stay in sync. It is the one thing a bank needs
// the running set for that is not stopping it, and it is why a bank can reach its
// owner for a seek without holding a handle of its own.
//
// Nothing is scheduled when nothing is playing, which the caller checks first; -1 is what
// that reports, since a live playhead is never behind zero.
func (c *Controller) seekSongBeat(beat float32) float32 {
	if c.playback == nil {
		return -1
	}
	oldSongBeat := c.playback.SeekSongBeat(beat)
	for _, session := range c.extra {
		session.SeekSongBeat(beat)
	}
	return oldSongBeat
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
