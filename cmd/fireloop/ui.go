package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const defaultBPM = 139

// The tempo is the one piece of state the controller does not own: the playback worker
// samples it from its own goroutine while the event loop is still setting it, so it has to
// stay readable from both without a lock that would sit on the step path.
var bpm atomic.Int64

func init() {
	bpm.Store(defaultBPM)
}

func currentBPM() int {
	return int(bpm.Load())
}

func setBPM(value int) {
	bpm.Store(int64(value))
}

func (c *Controller) exitPatternEditModes() error {
	if c.patbank.noteEditActive() {
		if err := c.patbank.setNoteEdit(false); err != nil {
			return err
		}
	}
	if c.patbank.lengthEditActive() {
		return c.patbank.setLengthMode(false)
	}
	return nil
}

// releaseModifiers drops the engaged modifier buttons together with their lights, so a
// mode switch cannot carry Alt or Shift into the mode that follows. An entry still on
// the display is dropped as well, since it only means something while Shift is held.
func (c *Controller) releaseModifiers() error {
	c.alt = false
	c.shift = false
	c.pending = 0
	if err := c.patbank.pads.SetLed(NoteAlt, LEDOff); err != nil {
		return err
	}
	return c.patbank.pads.SetLed(NoteShift, LEDOff)
}

// restoreIndicators re-applies the button lights, which a blackout turned off, so each
// light again reports the state behind it. The view comes from the controller's own mode,
// which is the one record of it rather than something a caller had to pass in and keep in
// step with.
func (c *Controller) restoreIndicators() error {
	lights := map[int]int{
		NoteAlt:         LEDOff,
		NoteShift:       LEDOff,
		NoteRecord:      LEDOff,
		NoteMode:        LEDOff,
		NoteOverview:    LEDOff,
		NotePatternSong: LEDOff,
		CCMuteLED1:      LEDOff,
		CCMuteLED2:      LEDOff,
		CCMuteLED3:      LEDOff,
		CCMuteLED4:      LEDOff,
	}
	if c.alt {
		lights[NoteAlt] = LEDYellow
	}
	if c.shift {
		lights[NoteShift] = LEDRed
	}
	if c.mode == songView {
		// The mode light reports which view is showing, so a wake from blackout has to
		// put it back as well, or the display claims to be in a mode it is not in.
		lights[NotePatternSong] = LEDGreen
	}
	if c.clipboard != nil {
		lights[NoteRecord] = LEDGreen
	}
	if c.patbank.noteEditActive() {
		lights[NoteMode] = LEDGreen
	}
	if c.patbank.lengthEditActive() {
		lights[NoteOverview] = LEDRed
	}
	if c.patbank.selTrackRow >= 1 && c.patbank.selTrackRow <= padRows {
		lights[CCMuteLED1+c.patbank.selTrackRow-1] = LEDGreen
	}
	for control, value := range lights {
		if err := c.patbank.pads.SetLed(control, value); err != nil {
			return err
		}
	}
	return nil
}

// wakeBlackout restores what the blackout cleared. A blackout is a display state: the
// controls kept their state, so the lights and the view come back to match them. Only
// the pattern view can be blacked out, so only that one is redrawn. The redraw is used
// rather than Jump because the playback worker reads the selected pattern index, and a
// blackout can happen while a pattern is playing.
func (c *Controller) wakeBlackout() error {
	if !c.patbank.pads.Wake() {
		return nil
	}
	logger.Info("wake from blackout")
	if err := c.restoreIndicators(); err != nil {
		return err
	}
	return c.patbank.redraw()
}

// tapTempoWindow is how long a tap stays usable, which is a minimum of twenty beats per
// minute. It is a variable so a test can exercise the reset path in milliseconds.
var tapTempoWindow = time.Minute / 20

// tempoDisplayRow is where an in-progress tempo entry is shown. It shares the bottom row
// with the length and chromatic status, which are transient readouts too.
const tempoDisplayRow = lengthDisplayRow

func (c *Controller) tapTempo() error {
	// TODO: have this use the pads instead
	if len(c.tapTimes) > 0 {
		// Reset if the last tap was too long ago to be part of the same tempo.
		last := c.tapTimes[len(c.tapTimes)-1]
		if time.Since(last) > tapTempoWindow {
			c.tapTimes = nil
		}
	}
	if len(c.tapTimes) > 4 {
		c.tapTimes = c.tapTimes[1:]
	}
	c.tapTimes = append(c.tapTimes, time.Now())
	if len(c.tapTimes) == 1 {
		return nil
	}
	var dur time.Duration
	for i := 1; i < len(c.tapTimes); i++ {
		dur += c.tapTimes[i].Sub(c.tapTimes[i-1])
	}
	dur /= time.Duration(len(c.tapTimes) - 1)
	tempo := int(60.0 / dur.Seconds())
	setBPM(tempo)
	s := fmt.Sprintf("Tempo: %03d", tempo)
	return c.patbank.printText(4, 3, s, false)
}

func (c *Controller) handleSongGrid(x, y int) error {
	if x >= 12 {
		return c.songbank.SelectPatternSlot((x - 12) + (y * 4))
	}
	if c.shift {
		return c.songbank.JumpMeasure(x, y)
	}
	return c.songbank.ToggleMeasure(x, y)
}

func (c *Controller) toggleAlt() error {
	c.alt = !c.alt
	logger.Debug("alt", "on", c.alt, "shift", c.shift)
	if c.alt {
		return c.patbank.pads.SetLed(NoteAlt, LEDYellow)
	}
	return c.patbank.pads.SetLed(NoteAlt, 0)
}

// The two handlers below are what a Fire event reaches once the controller has picked the
// view. The writer is an interface so a test can drive the real handler with a stub;
// starting playback also needs the sync port, which is why they take the wider interface
// rather than a plain MIDI writer.
func (c *Controller) processSongEvent(aseq sequencerWriter, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	logIncoming(int(ev.Data[1]), int(status), velocity)
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	if onGrid {
		if isPadRelease(status, velocity) {
			return nil
		}
	} else if !(midi.IsCC(status) || midi.IsNoteOn(status)) {
		return nil
	}
	if midi.IsNoteOn(status) && velocity == 0 {
		return nil
	}
	// A release must not end a blackout, so the wake waits for a real press.
	if err := c.wakeBlackout(); err != nil {
		return err
	}
	if onGrid {
		return c.handleSongGrid(x, y)
	}
	switch int(ev.Data[1]) {
	case NotePlay:
		if !c.playing() {
			c.startPlayback(aseq, c.songbank.newPlayback())
		}
	case NoteStop:
		return c.stopPlayback()
	case NoteShift:
		c.shift = !c.shift
		if c.shift {
			return c.songbank.pads.SetLed(NoteShift, LEDRed)
		} else {
			return c.songbank.pads.SetLed(NoteShift, 0)
		}
	// In song mode these controls navigate viewports; pads still edit the arrangement.
	case NotePatternUp:
		if c.shift {
			return c.songbank.MovePatternSelection(1)
		}
		return c.songbank.ScrollPatterns(1)
	case NotePatternDown:
		if c.shift {
			return c.songbank.MovePatternSelection(-1)
		}
		return c.songbank.ScrollPatterns(-1)
	case NoteGridLeft:
		if c.shift {
			return c.songbank.ScrollMeasures(-measureFinePageSize)
		}
		return c.songbank.ScrollMeasures(-measurePageSize)
	case NoteGridRight:
		if c.shift {
			return c.songbank.ScrollMeasures(measureFinePageSize)
		}
		return c.songbank.ScrollMeasures(measurePageSize)
	case NotePatternSong:
		if err := c.setMode(patternView, LEDOff); err != nil {
			return err
		}
		return c.patbank.redraw()
	case NoteBrowser, NoteAccent:
		return c.handleStateButton(int(ev.Data[1]))
	}
	return nil
}

func (c *Controller) handlePatternMute(n int) error {
	if c.alt {
		// Clearing a row removes the notes it holds, which the pattern cannot do while it
		// is being played, so that one stops playback. ClearTrackRow stops it and only stops
		// it for a row that holds a track, which is the only case where the set changes.
		// Alt stays engaged, so a run of rows can be cleared without pressing it again.
		return c.patbank.ClearTrackRow(n)
	}
	return c.patbank.SelectTrackRow(n)
}

// handleStateButton binds the two unclaimed buttons to the state file. Both gestures take
// Shift: Browser and Accent mean nothing on their own, and the modifier keeps a plain
// press from writing over a set or replacing one mid-performance. Alt is not a candidate
// because Shift plus Alt is the blackout.
func (c *Controller) handleStateButton(note int) error {
	if !c.shift {
		return nil
	}
	switch note {
	case NoteBrowser:
		return c.saveSession()
	case NoteAccent:
		return c.loadSession()
	}
	return nil
}

// saveSession takes the session as it stands and writes it. It only reads the banks, so it
// is safe while a pattern is playing, which is when it is most wanted.
//
// Only the snapshot happens here. The write and its readout go to their own goroutine so
// the press that asked for them is not held for the fsync.
func (c *Controller) saveSession() error {
	if c.sessionPath == "" {
		return c.reportState("No -state path")
	}
	c.saveOffLoop()
	return nil
}

// reportSave writes what a publish produced: the same log lines and the same readout row a
// synchronous save did, in the same words.
func (c *Controller) reportSave(report stateReport, err error) error {
	if err != nil {
		logger.Error("state save failed", "path", c.sessionPath, "error", err)
		return c.reportState(stateFailureText("Save failed", err))
	}
	logger.Info("state saved", append([]any{"path", c.sessionPath}, report.logAttrs()...)...)
	return c.reportState(report.saveText())
}

// loadSession replaces the running session with the one on disk. A load stops playback
// and repaints the view it was asked from, so what the unit shows afterwards is the set
// that was loaded rather than the one that was there.
func (c *Controller) loadSession() error {
	if c.sessionPath == "" {
		return c.reportState("No -state path")
	}
	report, err := c.loadState(c.sessionPath)
	if err != nil {
		logger.Error("state load failed", "path", c.sessionPath, "error", err)
		return c.reportState(stateFailureText("Load failed", err))
	}
	logger.Info("state loaded", append([]any{"path", c.sessionPath}, report.logAttrs()...)...)
	// The repaint belongs here rather than in loadState, which is deliberately kept from
	// knowing the view: which one is showing is the controller's business, and it is what
	// says which bank has to be repainted and which mode light put back.
	if err := c.restoreIndicators(); err != nil {
		return err
	}
	if c.mode == songView {
		err = c.songbank.redraw()
	} else {
		err = c.patbank.redraw()
	}
	if err != nil {
		return err
	}
	return c.reportState(report.loadText())
}

// reportState writes a transient result on the readout row, the row the length and
// chromatic readouts use. Anything already there is a readout too, so a message is
// replaced by the next redraw rather than left to go stale.
func (c *Controller) reportState(text string) error {
	if err := c.patbank.clearTextRows(lengthDisplayRow, 1); err != nil {
		return err
	}
	return c.patbank.printText(lengthDisplayRow, 0, fitOLEDText(text), false)
}

func (c *Controller) handlePatternGrid(aseq sequencerWriter, x, y, vel int) error {
	if vel == 0 {
		c.patbank.releasePad(y, x)
		return nil
	}
	if c.patbank.noteEditActive() {
		return c.patbank.handleNoteEditPad(aseq, y, x, vel, c.alt)
	}
	if c.shift {
		c.pending *= 10
		if c.pending > 999 {
			c.pending = 0
		}
		addend := (3*y + ((x % 4) % 3)) + 1
		if y == 3 {
			addend = 0
		}
		c.pending += addend
		logger.Debug("tempo entry", "pending", c.pending, "padX", x, "padY", y)
		// Only the row the number goes on is cleared. Wiping the whole display here
		// looked like a blackout: the screen went dark while the pads stayed lit.
		if err := c.patbank.clearTextRows(tempoDisplayRow, 1); err != nil {
			return err
		}
		return c.patbank.printText(tempoDisplayRow, 4, fmt.Sprintf("Tempo: %03d", c.pending), false)
	}
	if handled, err := c.patbank.handleChromaticStepPress(y, x); handled {
		return err
	}
	patEv, err := c.patbank.ToggleEvent(y, x, vel)
	if err != nil {
		return err
	}
	if c.playing() || patEv.Velocity == 0 {
		return nil
	}
	return writeMidiMsgs(aseq, eventDestination(patEv), patEv.ToMidi())
}

func (c *Controller) processPatternEvent(aseq sequencerWriter, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	logIncoming(int(ev.Data[1]), int(status), velocity)
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	if onGrid {
		if isPadRelease(status, velocity) {
			c.patbank.releasePad(y, x)
			return nil
		}
		if !midi.IsNoteOn(status) {
			return nil
		}
	} else if !(midi.IsCC(status) || midi.IsNoteOn(status)) {
		return nil
	}
	if midi.IsNoteOn(status) && velocity == 0 {
		return nil
	}
	// A release must not end a blackout, so the wake waits for a real press.
	if err := c.wakeBlackout(); err != nil {
		return err
	}
	if onGrid {
		return c.handlePatternGrid(aseq, x, y, velocity)
	}
	note := int(ev.Data[1])
	// The dispatcher only decides which family a control belongs to; each family then
	// answers for its own controls. Splitting them here is what keeps a button from being
	// a case buried in a switch that also has to work out what kind of event arrived.
	// Every bound control is named here, so a control added to a handler without being
	// added here shows up as unbound rather than being quietly swallowed by whatever
	// family happens to be the last one tried.
	switch note {
	case NotePlay, NoteStop, NoteTap:
		return c.handleTransport(aseq, note)
	case NoteShift, NoteAlt, NoteMute1, NoteMute2, NoteMute3, NoteMute4, CCVolume, CCSelect:
		return c.handleEditing(aseq, ev, note)
	case NoteOverview, NoteMode, NoteGridLeft, NoteGridRight,
		NotePatternUp, NotePatternDown, NoteRecord, NotePatternSong,
		NoteBrowser, NoteAccent:
		return c.handleNavigation(aseq, note)
	}
	// The unit has more buttons than the program binds. Saying so once is better than a
	// control that looks handled and is not.
	logger.Debug("unbound control", "note", note)
	return nil
}

// handleTransport answers Play, Stop and Tap: the controls that start, end or set the tempo
// of a set rather than changing anything about what is written down.
func (c *Controller) handleTransport(aseq sequencerWriter, note int) error {
	switch note {
	case NotePlay:
		if c.clipboard != nil {
			// Copy and paste. SetPattern stops the set for itself, because it is the thing
			// about to replace what the worker reads.
			if err := c.patbank.SetPattern(c.clipboard); err != nil {
				return err
			}
			c.clipboard = nil
			return c.patbank.pads.SetLed(NoteRecord, LEDOff)
		}
		if !c.playing() {
			c.startPlayback(aseq, c.patbank.newPlayback())
		}
		return nil
	case NoteStop:
		if c.alt {
			// Clear pattern. Alt stays engaged, so the track buttons keep scrolling.
			return c.patbank.SetPattern(&Pattern{})
		}
		return c.stopPlayback()
	case NoteTap:
		return c.tapTempo()
	default:
		return nil
	}
}

// handleEditing answers the controls that change notes or hold a modifier: Shift, Alt, the
// four track rows, and the two encoders.
func (c *Controller) handleEditing(aseq sequencerWriter, ev alsa.SeqEvent, note int) error {
	velocity := int(ev.Data[2])
	switch note {
	case NoteShift:
		c.shift = !c.shift
		logger.Debug("shift", "on", c.shift, "alt", c.alt)
		if !c.shift {
			if err := c.patbank.pads.SetLed(NoteShift, 0); err != nil {
				return err
			}
			if c.pending > 20 && c.pending < 300 {
				setBPM(c.pending)
				c.pending = 0
				return c.patbank.redraw()
			}
		} else {
			return c.patbank.pads.SetLed(NoteShift, LEDRed)
		}
	case NoteAlt:
		if c.shift {
			// Blackout. The controls keep their state, so the next press brings the
			// lights and screen back to it, the Alt light included.
			logger.Info("blackout")
			return c.patbank.pads.Blackout()
		}
		return c.toggleAlt()
	case NoteMute1:
		return c.handlePatternMute(1)
	case NoteMute2:
		return c.handlePatternMute(2)
	case NoteMute3:
		return c.handlePatternMute(3)
	case NoteMute4:
		return c.handlePatternMute(4)
	case CCVolume:
		return c.patbank.AdjustVelocity(aseq, velocity)
	case CCSelect:
		dir, turning := encoderDirection(int(ev.Data[2]))
		if !turning {
			return nil
		}
		if c.patbank.lengthEditActive() {
			return c.patbank.AdjustLength(dir)
		}
		if c.patbank.noteEditActive() {
			// In note-edit mode the knob moves the palette by an octave rather than the
			// track's voice, so the pitch being chosen stays on screen while it moves.
			return c.patbank.ShiftPaletteOctave(dir)
		}
		return c.patbank.JogSelect(dir)
	}
	// Shift with nothing typed into it is the one control that has nothing to do, and
	// saying so is cheaper than making every path through it return.
	return nil
}

// handleNavigation answers everything that moves the view without changing a note: the
// pattern buttons, the step cursor, the two edit modes, and the way into the arrangement.
// These are the controls that stay live while a set plays, because none of them writes
// state the playback worker reads.
func (c *Controller) handleNavigation(aseq sequencerWriter, note int) error {
	switch note {
	// Overview exposes the per-pattern length while the encoder changes steps. It stops
	// the set first, because a length edit moves where the playhead should be.
	case NoteOverview:
		if err := c.stopPlayback(); err != nil {
			return err
		}
		return c.patbank.ToggleLengthMode()
	case NoteMode:
		return c.patbank.ToggleNoteMode()
	case NoteGridLeft:
		return c.patbank.MoveStepCursor(-1)
	case NoteGridRight:
		return c.patbank.MoveStepCursor(1)
	case NotePatternUp:
		// Alt reuses the pattern buttons for the track window, which is why Alt
		// stays lit until it is pressed again.
		if c.alt {
			return c.patbank.ScrollTracks(1)
		}
		// Jump stops the set for a selection that moves, and not for one that is
		// already at the end of the list. Going nowhere changes nothing playing.
		return c.patbank.Jump(1)
	case NotePatternDown:
		if c.alt {
			return c.patbank.ScrollTracks(-1)
		}
		return c.patbank.Jump(-1)
	case NoteRecord:
		if c.clipboard != nil {
			c.clipboard = nil
			return c.patbank.pads.SetLed(NoteRecord, LEDOff)
		}
		c.clipboard = c.patbank.CurrentPattern().Copy()
		return c.patbank.pads.SetLed(NoteRecord, LEDGreen)
	case NotePatternSong:
		if err := c.setMode(songView, LEDGreen); err != nil {
			return err
		}
		return c.songbank.redraw()
	default:
		// Browser and Accent take Shift to reach the session file, and do nothing
		// without it.
		return c.handleStateButton(note)
	}
}
