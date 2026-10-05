package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// eventProcessFunc applies one event to the banks. The writer is an interface so a test can
// drive the real handler with a stub; starting playback also needs the sync port, which is
// why this is the wider interface rather than a plain MIDI writer.
type eventProcessFunc func(sequencerWriter, alsa.SeqEvent) error

const defaultBPM = 139

var processEvent eventProcessFunc
var shiftOn = false
var altOn = false
var pendingNumber = 0
var bpm atomic.Int64
var playbackStop playbackStopFunc
var patternClipboard *Pattern
var songbank *SongBank
var patbank *PatternBank

var tapTempoTimes []time.Time

func init() {
	bpm.Store(defaultBPM)
}

func currentBPM() int {
	return int(bpm.Load())
}

func setBPM(value int) {
	bpm.Store(int64(value))
}

func stopPlayback() error {
	var firstErr error
	if playbackStop != nil {
		if err := playbackStop(); err != nil {
			firstErr = err
		}
		playbackStop = nil
	}
	if patbank != nil {
		if playback := patbank.playback; playback != nil {
			if err := playback.releaseAll(playback.writer); err != nil && firstErr == nil {
				firstErr = err
			}
			patbank.playback = nil
		}
		patbank.clearPadState()
		// A playhead left lit on the step strip would outlive the playback that put it
		// there, so put it back.
		if err := patbank.clearStepPlayhead(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if songbank != nil {
		if playback := songbank.playback; playback != nil {
			if err := playback.releaseAll(playback.writer); err != nil && firstErr == nil {
				firstErr = err
			}
			songbank.playback = nil
		}
	}
	return firstErr
}

// switchMode moves between the pattern view and the arrangement. Playback stops because
// the other view has no sequencer for it, the edit modes are closed because each belongs
// to the view being left, and the modifiers are released because they act on whatever the
// view under them is.
func switchMode(next eventProcessFunc, songLight int) error {
	if err := stopPlayback(); err != nil {
		return err
	}
	if err := exitPatternEditModes(); err != nil {
		return err
	}
	if err := releaseModifiers(); err != nil {
		return err
	}
	processEvent = next
	if err := patbank.f.SetLed(NotePatternSong, songLight); err != nil {
		return err
	}
	return nil
}

func exitPatternEditModes() error {
	if patbank == nil {
		return nil
	}
	if patbank.editingNote {
		if err := patbank.setNoteEdit(false); err != nil {
			return err
		}
	}
	if patbank.editingLength {
		return patbank.setLengthMode(false)
	}
	return nil
}

// releaseModifiers drops the engaged modifier buttons together with their lights, so a
// mode switch cannot carry Alt or Shift into the mode that follows. An entry still on
// the display is dropped as well, since it only means something while Shift is held.
func releaseModifiers() error {
	altOn = false
	shiftOn = false
	pendingNumber = 0
	if patbank == nil {
		return nil
	}
	if err := patbank.f.SetLed(NoteAlt, LEDOff); err != nil {
		return err
	}
	return patbank.f.SetLed(NoteShift, LEDOff)
}

// restoreIndicators re-applies the button lights, which a blackout turned off, so each
// light again reports the state behind it. The caller knows which view is showing, so the
// mode is passed in rather than looked up: the handler that runs is the record of the mode,
// and a second record of it could disagree with this one.
func restoreIndicators(song bool) error {
	if patbank == nil {
		return nil
	}
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
	if altOn {
		lights[NoteAlt] = LEDYellow
	}
	if shiftOn {
		lights[NoteShift] = LEDRed
	}
	if song {
		// The mode light reports which view is showing, so a wake from blackout has to
		// put it back as well, or the display claims to be in a mode it is not in.
		lights[NotePatternSong] = LEDGreen
	}
	if patternClipboard != nil {
		lights[NoteRecord] = LEDGreen
	}
	if patbank.editingNote {
		lights[NoteMode] = LEDGreen
	}
	if patbank.editingLength {
		lights[NoteOverview] = LEDRed
	}
	if patbank.selTrackRow >= 1 && patbank.selTrackRow <= padRows {
		lights[CCMuteLED1+patbank.selTrackRow-1] = LEDGreen
	}
	for control, value := range lights {
		if err := patbank.f.SetLed(control, value); err != nil {
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
func wakeBlackout(song bool) error {
	if patbank == nil {
		return nil
	}
	if !patbank.f.Wake() {
		return nil
	}
	logger.Info("wake from blackout")
	if err := restoreIndicators(song); err != nil {
		return err
	}
	return patbank.redraw()
}

// tapTempoWindow is how long a tap stays usable, which is a minimum of twenty beats per
// minute. It is a variable so a test can exercise the reset path in milliseconds.
var tapTempoWindow = time.Minute / 20

// tempoDisplayRow is where an in-progress tempo entry is shown. It shares the bottom row
// with the length and chromatic status, which are transient readouts too.
const tempoDisplayRow = lengthDisplayRow

func tapTempo() error {
	// TODO: have this use the pads instead
	if len(tapTempoTimes) > 0 {
		// Reset if the last tap was too long ago to be part of the same tempo.
		last := tapTempoTimes[len(tapTempoTimes)-1]
		if time.Since(last) > tapTempoWindow {
			tapTempoTimes = nil
		}
	}
	if len(tapTempoTimes) > 4 {
		tapTempoTimes = tapTempoTimes[1:]
	}
	tapTempoTimes = append(tapTempoTimes, time.Now())
	if len(tapTempoTimes) == 1 {
		return nil
	}
	var dur time.Duration
	for i := 1; i < len(tapTempoTimes); i++ {
		dur += tapTempoTimes[i].Sub(tapTempoTimes[i-1])
	}
	dur /= time.Duration(len(tapTempoTimes) - 1)
	tempo := int(60.0 / dur.Seconds())
	setBPM(tempo)
	s := fmt.Sprintf("Tempo: %03d", tempo)
	return patbank.f.Print(4, 3, s)
}

func handleSongGrid(x, y int) error {
	if x >= 12 {
		return songbank.SelectPatternSlot((x - 12) + (y * 4))
	}
	if shiftOn {
		return songbank.JumpMeasure(x, y)
	}
	return songbank.ToggleMeasure(x, y)
}

func toggleAlt() error {
	altOn = !altOn
	logger.Debug("alt", "on", altOn, "shift", shiftOn)
	if altOn {
		return patbank.f.SetLed(NoteAlt, LEDYellow)
	}
	return patbank.f.SetLed(NoteAlt, 0)
}

func processSongEvent(aseq sequencerWriter, ev alsa.SeqEvent) error {
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
	if err := wakeBlackout(true); err != nil {
		return err
	}
	if onGrid {
		return handleSongGrid(x, y)
	}
	switch int(ev.Data[1]) {
	case NotePlay:
		if playbackStop == nil {
			playbackStop = songbank.startSequencer(aseq)
		}
	case NoteStop:
		return stopPlayback()
	case NoteShift:
		shiftOn = !shiftOn
		if shiftOn {
			return songbank.f.SetLed(NoteShift, LEDRed)
		} else {
			return songbank.f.SetLed(NoteShift, 0)
		}
	// In song mode these controls navigate viewports; pads still edit the arrangement.
	case NotePatternUp:
		if shiftOn {
			return songbank.MovePatternSelection(1)
		}
		return songbank.ScrollPatterns(1)
	case NotePatternDown:
		if shiftOn {
			return songbank.MovePatternSelection(-1)
		}
		return songbank.ScrollPatterns(-1)
	case NoteGridLeft:
		if shiftOn {
			return songbank.ScrollMeasures(-measureFinePageSize)
		}
		return songbank.ScrollMeasures(-measurePageSize)
	case NoteGridRight:
		if shiftOn {
			return songbank.ScrollMeasures(measureFinePageSize)
		}
		return songbank.ScrollMeasures(measurePageSize)
	case NotePatternSong:
		if err := switchMode(processPatternEvent, LEDOff); err != nil {
			return err
		}
		return patbank.Jump(0)
	case NoteBrowser, NoteAccent:
		return handleStateButton(int(ev.Data[1]), true)
	}
	return nil
}

func handlePatternMute(n int) error {
	if altOn {
		// Clearing a row removes the notes it holds, which the pattern cannot do while it
		// is being played, so that one stops playback. Alt stays engaged, so a run of rows
		// can be cleared without pressing it again.
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.ClearTrackRow(n)
	}
	return patbank.SelectTrackRow(n)
}

// handleStateButton binds the two unclaimed buttons to the state file. Both gestures take
// Shift: Browser and Accent mean nothing on their own, and the modifier keeps a plain
// press from writing over a set or replacing one mid-performance. Alt is not a candidate
// because Shift plus Alt is the blackout. song is the view the caller is running, which a
// load needs in order to repaint the right one.
func handleStateButton(note int, song bool) error {
	if !shiftOn {
		return nil
	}
	switch note {
	case NoteBrowser:
		return saveSession()
	case NoteAccent:
		return loadSession(song)
	}
	return nil
}

// saveSession writes the session and reports on the readout row. It only reads the banks,
// so it is safe while a pattern is playing, which is when it is most wanted.
func saveSession() error {
	if statePath == "" {
		return reportState("No -state path")
	}
	report, err := saveState(statePath, patbank, songbank, stateKitPaths)
	if err != nil {
		logger.Error("state save failed", "path", statePath, "error", err)
		return reportState(stateFailureText("Save failed", err))
	}
	logger.Info("state saved", append([]any{"path", statePath}, report.logAttrs()...)...)
	return reportState(report.saveText())
}

// loadSession replaces the running session with the one on disk. A load stops playback
// and repaints the view it was asked from, so what the unit shows afterwards is the set
// that was loaded rather than the one that was there.
func loadSession(song bool) error {
	if statePath == "" {
		return reportState("No -state path")
	}
	report, err := loadState(statePath, patbank, songbank, stateKitPaths)
	if err != nil {
		logger.Error("state load failed", "path", statePath, "error", err)
		return reportState(stateFailureText("Load failed", err))
	}
	logger.Info("state loaded", append([]any{"path", statePath}, report.logAttrs()...)...)
	// The repaint belongs here rather than in loadState, which is where the mode is
	// known: this is only ever reached from the handler for the view in question.
	if err := restoreIndicators(song); err != nil {
		return err
	}
	if song {
		err = songbank.Jump(0)
	} else {
		err = patbank.Jump(0)
	}
	if err != nil {
		return err
	}
	return reportState(report.loadText())
}

// reportState writes a transient result on the readout row, the row the length and
// chromatic readouts use. Anything already there is a readout too, so a message is
// replaced by the next redraw rather than left to go stale.
func reportState(text string) error {
	if patbank == nil {
		return nil
	}
	if err := patbank.clearTextRows(lengthDisplayRow, 1); err != nil {
		return err
	}
	return patbank.printText(lengthDisplayRow, 0, fitOLEDText(text), false)
}

func handlePatternGrid(aseq sequencerWriter, x, y, vel int) error {
	if vel == 0 {
		patbank.releasePad(y, x)
		return nil
	}
	if patbank.editingNote {
		return patbank.handleNoteEditPad(aseq, y, x, vel)
	}
	if shiftOn {
		pendingNumber *= 10
		if pendingNumber > 999 {
			pendingNumber = 0
		}
		addend := (3*y + ((x % 4) % 3)) + 1
		if y == 3 {
			addend = 0
		}
		pendingNumber += addend
		logger.Debug("tempo entry", "pending", pendingNumber, "padX", x, "padY", y)
		// Only the row the number goes on is cleared. Wiping the whole display here
		// looked like a blackout: the screen went dark while the pads stayed lit.
		if err := patbank.clearTextRows(tempoDisplayRow, 1); err != nil {
			return err
		}
		return patbank.printText(tempoDisplayRow, 4, fmt.Sprintf("Tempo: %03d", pendingNumber), false)
	}
	if handled, err := patbank.handleChromaticStepPress(y, x); handled {
		return err
	}
	patEv, err := patbank.ToggleEvent(y, x, vel)
	if err != nil {
		return err
	}
	if playbackStop != nil || patEv.Velocity == 0 {
		return nil
	}
	return writeMidiMsgs(aseq, eventDestination(patEv), patEv.ToMidi())
}

func processPatternEvent(aseq sequencerWriter, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	logIncoming(int(ev.Data[1]), int(status), velocity)
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	if onGrid {
		if isPadRelease(status, velocity) {
			patbank.releasePad(y, x)
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
	if err := wakeBlackout(false); err != nil {
		return err
	}
	if onGrid {
		return handlePatternGrid(aseq, x, y, velocity)
	}
	switch int(ev.Data[1]) {
	case NoteShift:
		shiftOn = !shiftOn
		logger.Debug("shift", "on", shiftOn, "alt", altOn)
		if !shiftOn {
			if err := patbank.f.SetLed(NoteShift, 0); err != nil {
				return err
			}
			if pendingNumber > 20 && pendingNumber < 300 {
				setBPM(pendingNumber)
				pendingNumber = 0
				return patbank.Jump(0)
			}
		} else {
			return patbank.f.SetLed(NoteShift, LEDRed)
		}
	// Overview exposes the per-pattern length while the encoder changes steps.
	case NoteOverview:
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.ToggleLengthMode()
	case NoteMode:
		return patbank.ToggleNoteMode()
	case NoteGridLeft:
		return patbank.MoveStepCursor(-1)
	case NoteGridRight:
		return patbank.MoveStepCursor(1)
	case NotePatternUp:
		// Alt reuses the pattern buttons for the track window, which is why Alt
		// stays lit until it is pressed again.
		if altOn {
			return patbank.ScrollTracks(1)
		}
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.Jump(1)
	case NotePatternDown:
		if altOn {
			return patbank.ScrollTracks(-1)
		}
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.Jump(-1)
	case NoteAlt:
		if shiftOn {
			// Blackout. The controls keep their state, so the next press brings the
			// lights and screen back to it, the Alt light included.
			logger.Info("blackout")
			return patbank.f.Blackout()
		}
		return toggleAlt()
	case NoteMute1:
		return handlePatternMute(1)
	case NoteMute2:
		return handlePatternMute(2)
	case NoteMute3:
		return handlePatternMute(3)
	case NoteMute4:
		return handlePatternMute(4)
	case CCVolume:
		return patbank.AdjustVelocity(aseq, velocity)
	case CCSelect:
		dir, turning := encoderDirection(int(ev.Data[2]))
		if !turning {
			return nil
		}
		if patbank.editingLength {
			return patbank.AdjustLength(dir)
		}
		if patbank.NoteEditActive() {
			// In note-edit mode the knob moves the palette by an octave rather than the
			// track's voice, so the pitch being chosen stays on screen while it moves.
			return patbank.ShiftPaletteOctave(dir)
		}
		return patbank.JogSelect(dir)
	case NotePlay:
		if patternClipboard != nil {
			// Copy and paste.
			if err := stopPlayback(); err != nil {
				return err
			}
			if err := patbank.SetPattern(patternClipboard); err != nil {
				return err
			}
			patternClipboard = nil
			return patbank.f.SetLed(NoteRecord, LEDOff)
		}
		if playbackStop == nil {
			playbackStop = patbank.startSequencer(aseq)
		}
	case NoteStop:
		if altOn {
			// Clear pattern. Alt stays engaged, so the track buttons keep scrolling.
			if err := stopPlayback(); err != nil {
				return err
			}
			return patbank.SetPattern(&Pattern{})
		}
		return stopPlayback()
	case NoteTap:
		return tapTempo()
	case NoteRecord:
		if patternClipboard != nil {
			patternClipboard = nil
			return patbank.f.SetLed(NoteRecord, LEDOff)
		}
		patternClipboard = patbank.CurrentPattern().Copy()
		return patbank.f.SetLed(NoteRecord, LEDGreen)
	case NotePatternSong:
		if err := switchMode(processSongEvent, LEDGreen); err != nil {
			return err
		}
		return songbank.Jump(0)
	case NoteBrowser, NoteAccent:
		return handleStateButton(int(ev.Data[1]), false)
	}
	return nil
}
