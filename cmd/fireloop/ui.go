package main

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const defaultBPM = 139

// bpmFixed is the resolution of the 14-bit tempo midiclock programs over CC: one sixty-fourth
// of a beat per count. Fireloop has always stored whole beats, so the stored value becomes this
// fixed point and currentBPM divides it back out. It is midiclock's convention rather than the
// coarse-and-fine a general controller sends, and taking it exactly is what makes one knob mean
// the same thing in both programs.
const bpmFixed = 64

// The tempo is the one piece of state the controller does not own: the playback worker
// samples it from its own goroutine while the event loop is still setting it, so it has to
// stay readable from both without a lock that would sit on the step path.
var bpm atomic.Int64

func init() {
	setBPM(defaultBPM)
}

func currentBPM() int {
	return int(math.Round(float64(bpm.Load()) / bpmFixed))
}

// setBPM takes a whole beat and does not clamp, which is the existing convention rather than an
// oversight: every caller with a bounded value clamps it before calling.
func setBPM(value int) {
	bpm.Store(int64(value) * bpmFixed)
}

// setBPMHalf writes one half of the 14-bit tempo and leaves the other half standing, because
// the coarse and fine controllers arrive as two separate messages and nothing promises the
// coarse one comes first. Every MIDI event is handled on one goroutine in processIncomingEvents,
// so the read-modify-write needs no compare-and-swap loop; the atomic is there because the
// playback worker reads currentBPM from another.
func setBPMHalf(msb bool, value int) {
	raw := bpm.Load()
	if msb {
		raw = raw&0x7f | int64(value)<<7
	} else {
		raw = raw&^0x7f | int64(value)
	}
	bpm.Store(clampBPMRaw(raw))
}

// clampBPMRaw keeps a raw 14-bit tempo above the slowest beat the sequencer will play.
//
// Only a floor is needed, because the fourteen-bit format is its own ceiling: the largest raw
// two seven-bit halves can carry is 16383 counts, which is 255.98 BPM, and stateTempoMax is 299.
// A bound that cannot fire is a bound nothing tests, so the format's ceiling is stated here
// instead of written as a branch.
//
// Clamping the raw rather than rounding it to a beat and clamping that is what keeps currentBPM
// in range by construction, whatever the halves happened to hold. A floor clamp also rewrites
// the other half's bits — a raw of 0x0010 comes back as 0x0540, so the MSB is suddenly 10 —
// which is safe because the floor is itself a legal tempo and the coarse message that follows
// overwrites those bits. It is why the clamp lives here rather than inside setBPM, which takes
// a whole beat and has no half-formed value to repair.
func clampBPMRaw(raw int64) int64 {
	if low := int64(stateTempoMin * bpmFixed); raw < low {
		return low
	}
	return raw
}

func (c *Controller) exitPatternEditModes() error {
	if c.patbank.noteEditActive() {
		if err := c.patbank.setNoteEdit(false); err != nil {
			return err
		}
	}
	// Swing entry is left before length entry because both own the readout row, and a mode
	// switch that dropped only one of them would leave the other's number on the display.
	if c.patbank.swingEditActive() {
		if err := c.patbank.setSwingEntry(false); err != nil {
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
		NoteSnap:        LEDOff,
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
	if c.patbank.swingEditActive() {
		lights[NoteSnap] = LEDRed
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

// tempoEntryColumn indents the tempo entry so it reads as a number being typed rather than as
// another readout; a swing entry is not transient, so it takes column zero like the other
// modes' own readouts. Both are written to readoutRow.
const tempoEntryColumn = 4

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
	// Two taps a few milliseconds apart put a tempo far above anything the sequencer will
	// play, and the store takes whatever it is given, so the bound belongs here. The
	// readout shows the clamped value rather than the raw one, so the display never claims
	// a tempo nothing is playing.
	tempo := clampIndex(int(60.0/dur.Seconds()), stateTempoMin, stateTempoMax)
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
	// The same split the pattern view makes: a controller is not a button, even where their
	// numbers collide.
	if midi.IsCC(status) {
		if err := c.wakeBlackout(); err != nil {
			return err
		}
		return c.handleControlChange(aseq, ev)
	}
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	// A release is asked before the note-on test, because a pad is released by a note-off
	// as well as by a note-on with no velocity.
	if onGrid && isPadRelease(status, velocity) {
		return nil
	}
	if !midi.IsNoteOn(status) || velocity == 0 {
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

// fireControlChannel is the channel the Fire's own controls arrive on. A number from the
// device and a number for the software travel the same wire, so the channel is the only thing
// telling them apart, and it is tested before either handler looks at the number.
//
// It is counted the way midi.Channel counts, from zero, so this is MIDI channel 1 — which is
// also the channel a controller sends on by default, and the reason the software's own
// controls have to be aimed at MIDI channel 2 or above to be heard. Saying which one it is
// matters because the two numberings are both called "channel" and a user reading one and
// sending on the other gets silence.
//
// This was an assumption until it was checked against a Fire, which does send its knobs on
// MIDI channel 1 as counted here. It stays written as an assumption no test can settle: the
// tests that dispatch CCSelect and CCVolume call midi.MakeCC(0) themselves, which authors the
// answer rather than observing it, and no amount of reading this repository can do better.
// logIncoming reports the channel a control actually arrived on, which is how it was checked and
// how it would be checked again.
const fireControlChannel = 0

// Fireloop's own controls, as opposed to the device's in fire.go. The prefix is the
// distinction and not a naming habit: CC is what the Fire sends, Cc is what this program
// accepts, and a constant appearing in the wrong one of the two handlers below is a bug on
// sight. The numbers are midiclock's, so one controller drives both programs the same way.
const (
	// CcBpmMsb and CcBpmLsb are one 14-bit tempo split across two messages, not a coarse
	// and fine pair: the value is beats per minute in sixty-fourths, so 120 BPM is 7680
	// counts whose MSB is 60 and LSB is 0. A controller sending a plain 7-bit pair lands
	// somewhere else entirely, which is the trade for agreeing with midiclock.
	CcBpmMsb = 16
	CcBpmLsb = 48
	// CcSwing is midiclock's number, so a controller set up for it works here unchanged.
	CcSwing = 17
	// CcSwingAlt is the number a controller with a real swing knob on it sends. cc/model.go
	// knows it as Skulpt's swing; the repository also finds it on a PWM sweep, a wah, two
	// oscillators, a slider, filter Q and vibrato delay, so it is the second choice rather
	// than the first.
	CcSwingAlt = 78
)

// handleControlChange tells a number meant for this program from one meant for the device,
// before either handler looks at what the number is. Both sets arrive on one wire from one
// port, so this is the whole of the separation between them.
func (c *Controller) handleControlChange(aseq sequencerWriter, ev alsa.SeqEvent) error {
	controller, value := int(ev.Data[1]), int(ev.Data[2])
	channel := midi.Channel(ev.Data[0])
	if channel != fireControlChannel {
		return c.handleSequencerControl(controller, value, channel)
	}
	// The Fire's controls mean nothing on the arrangement screen, which is how they behave
	// today. Routing them through a shared handler must not start joging a track selection
	// that song mode has no concept of.
	if c.mode == songView {
		return nil
	}
	return c.handleDeviceControl(aseq, controller, value)
}

// handleDeviceControl is the CC set the Fire sends. It holds CC-prefixed numbers only, so
// nothing here can be moved by a controller that was aimed at the software.
func (c *Controller) handleDeviceControl(aseq sequencerWriter, controller, value int) error {
	switch controller {
	case CCSelect:
		dir, turning := encoderDirection(value)
		if !turning {
			return nil
		}
		// Swing entry comes before length mode: the two are mutually exclusive, and the
		// newest mode owning the knob is what a player expects of a mode they just entered.
		if c.patbank.swingEditActive() {
			return c.patbank.AdjustSwing(dir)
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
	case CCVolume:
		return c.patbank.AdjustVelocity(aseq, value)
	}
	return nil
}

// handleSequencerControl is the CC set fireloop accepts, off the device's channel. It holds
// Cc-prefixed numbers only, so the Fire's own knobs cannot reach it however they are moved.
func (c *Controller) handleSequencerControl(controller, value, channel int) error {
	switch controller {
	case CcBpmMsb, CcBpmLsb:
		setBPMHalf(controller == CcBpmMsb, value)
		logger.Debug("tempo cc",
			"msb", controller == CcBpmMsb,
			"value", value,
			"channel", channel,
			"bpm", currentBPM(),
		)
		return c.reportControlChange(fmt.Sprintf("Tempo %03d", currentBPM()))
	case CcSwing, CcSwingAlt:
		from := currentSwingPct()
		setSwingPct(ccSwingToSwingPct(value))
		logger.Debug("swing cc",
			"controller", controller,
			"value", value,
			"channel", channel,
			"from", from,
			"to", currentSwingPct(),
		)
		return c.reportControlChange(fmt.Sprintf("Swing %s%%", swingLabel(currentSwingPct())))
	}
	return nil
}

// reportControlChange says what a software control just did, on the screen the player is
// looking at. reportState writes the pattern view's readout row, which is the wrong screen in
// song mode, so the song bank prints its own row there instead — a knob that moves something
// invisible is a knob that looks broken.
//
// The separator is repainted alongside it because the swing appears in two places at once,
// and the readout changing while the label did not is exactly the disagreement printSwing
// exists to prevent.
func (c *Controller) reportControlChange(text string) error {
	if c.mode == songView {
		return c.songbank.PrintTempo()
	}
	// Swing entry owns the readout row, so a control that moved the value repaints that mode
	// the way the encoder does rather than writing a message over a number being typed. The
	// one rule is that anything moving the value repaints everywhere the value is shown.
	if c.patbank.swingEditActive() {
		if err := c.patbank.printSwingEntry(); err != nil {
			return err
		}
		return c.patbank.printSwing()
	}
	if err := c.reportState(text); err != nil {
		return err
	}
	return c.patbank.printSwing()
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
	// Swing entry owns the readout row the way length entry does, so a message written over
	// it would replace a number the player is in the middle of typing with a differently
	// worded one. The control it reports having changed has already said itself on the
	// separator.
	if c.patbank.swingEditActive() {
		return nil
	}
	return c.patbank.printReadout(0, fitOLEDText(text))
}

func (c *Controller) handlePatternGrid(aseq sequencerWriter, x, y, vel int) error {
	if vel == 0 {
		c.patbank.releasePad(y, x)
		return nil
	}
	if c.patbank.noteEditActive() {
		return c.patbank.handleNoteEditPad(aseq, y, x, vel, c.alt)
	}
	// Swing entry's keypad sits in front of the Shift branch, not beside it. Shift is not
	// part of this gesture — the mode already says which of the two is being entered — so
	// placed after, Shift plus a pad would build a tempo number that the mode's own release
	// then applied as a swing.
	if c.patbank.swingEditActive() {
		return c.typeNumber(x, y, "swing entry", swingEntryColumn, "Swing: %02d")
	}
	if c.shift {
		return c.typeNumber(x, y, "tempo entry", tempoEntryColumn, "Tempo: %03d")
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
	// A control change is never a pad and never a button, and the second byte alone cannot
	// say which it is: NoteShift is 48 and so is CcBpmLsb, and CC 78 lands inside the grid
	// pads' own range. The status byte is the only thing that tells the two apart, so it is
	// asked before the number is looked at at all.
	if midi.IsCC(status) {
		if err := c.wakeBlackout(); err != nil {
			return err
		}
		return c.handleControlChange(aseq, ev)
	}
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	// A release is asked before the note-on test, because a pad is released by a note-off
	// as well as by a note-on with no velocity.
	if onGrid && isPadRelease(status, velocity) {
		c.patbank.releasePad(y, x)
		return nil
	}
	if !midi.IsNoteOn(status) || velocity == 0 {
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
	case NoteShift, NoteAlt, NoteMute1, NoteMute2, NoteMute3, NoteMute4:
		return c.handleEditing(note)
	case NoteOverview, NoteSnap, NoteMode, NoteGridLeft, NoteGridRight,
		NotePatternUp, NotePatternDown, NoteRecord, NotePatternSong,
		NoteBrowser, NoteAccent:
		return c.handleNavigation(aseq, note)
	}
	// Every button the unit has is bound above, so reaching this means a number nobody
	// bound arrived. Saying so once is better than a control that looks handled and is not.
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

// handleEditing answers the controls that change notes or hold a modifier: Shift, Alt and
// the four track rows. The encoders and the volume knob are not here: they arrive as
// control changes, and handleDeviceControl answers them before this dispatch is reached at
// all. Listing them again by note number would be a second copy of the same rule, and the
// copy that has drifted is the one nothing reaches.
func (c *Controller) handleEditing(note int) error {
	switch note {
	case NoteShift:
		c.shift = !c.shift
		logger.Debug("shift", "on", c.shift, "alt", c.alt)
		if !c.shift {
			if err := c.patbank.pads.SetLed(NoteShift, 0); err != nil {
				return err
			}
			// Shift is inert inside swing entry — the mode already says which of the
			// two is being entered — so it must not touch the digits the pads are typing
			// there. Treating them as a stale tempo entry threw away a number the player
			// had only just read off the display.
			if c.patbank.swingEditActive() {
				break
			}
			if c.pending > 20 && c.pending < 300 {
				setBPM(c.pending)
			}
			// Dropped rather than left standing: a digit run that turned out to be out
			// of range used to survive the release and be applied by the next Shift
			// gesture, which answers one entry with another one's number.
			if c.pending != 0 {
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
	// Snap enters swing entry, which unlike length mode leaves the set running: a swing
	// changes nothing in any pattern, so stopping to move a groove control would make the
	// control useless on a running groove. With Shift it is the same control's off and on
	// instead — straight, or the last groove — because reaching for the value while the set
	// runs should not require entering a mode and leaving it again.
	case NoteSnap:
		if c.shift {
			return c.patbank.ToggleSwingOnOff()
		}
		return c.patbank.ToggleSwingEntry()
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
