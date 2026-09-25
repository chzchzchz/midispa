package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

type eventProcessFunc func(*alsa.Seq, alsa.SeqEvent) error

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

func tapTempo() error {
	// TODO: have this use the pads instead
	if len(tapTempoTimes) > 0 {
		// Reset if below minimum of 20 bpm.
		last := tapTempoTimes[len(tapTempoTimes)-1]
		if time.Since(last) > time.Minute/20 {
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
	if altOn = !altOn; altOn {
		return patbank.f.SetLed(NoteAlt, LEDYellow)
	}
	return patbank.f.SetLed(NoteAlt, 0)
}

func processSongEvent(aseq *alsa.Seq, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	if !(midi.IsCC(status) || midi.IsNoteOn(status)) {
		return nil
	}
	if x, y, ok := Note2Grid(int(ev.Data[1])); ok {
		if isPadRelease(status, velocity) {
			return nil
		}
		return handleSongGrid(x, y)
	}
	if midi.IsNoteOn(status) && velocity == 0 {
		return nil
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
		if err := stopPlayback(); err != nil {
			return err
		}
		if err := exitPatternEditModes(); err != nil {
			return err
		}
		processEvent = processPatternEvent
		if err := patbank.f.SetLed(NotePatternSong, LEDOff); err != nil {
			return err
		}
		return patbank.Jump(0)
	case NoteAlt:
		return toggleAlt()
	}
	return nil
}

func handlePatternMute(n int) error {
	if err := stopPlayback(); err != nil {
		return err
	}
	if altOn {
		if err := patbank.ClearTrackRow(n); err != nil {
			return err
		}
		if err := toggleAlt(); err != nil {
			return err
		}
		return patbank.Jump(0)
	}
	return patbank.SelectTrackRow(n)
}

func handlePatternGrid(aseq *alsa.Seq, x, y, vel int) error {
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
		if err := patbank.f.ClearOLED(); err != nil {
			return err
		}
		s := fmt.Sprintf("Tempo: %03d", pendingNumber)
		return patbank.f.Print(4, 3, s)
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

func processPatternEvent(aseq *alsa.Seq, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	if x, y, ok := Note2Grid(int(ev.Data[1])); ok {
		if isPadRelease(status, velocity) {
			patbank.releasePad(y, x)
			return nil
		}
		if midi.IsNoteOn(status) {
			return handlePatternGrid(aseq, x, y, velocity)
		}
		return nil
	}
	if !(midi.IsCC(status) || midi.IsNoteOn(status)) || (midi.IsNoteOn(status) && velocity == 0) {
		return nil
	}
	switch int(ev.Data[1]) {
	case NoteShift:
		shiftOn = !shiftOn
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
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.ToggleNoteMode()
	case NoteGridLeft:
		return patbank.MoveStepCursor(-1)
	case NoteGridRight:
		return patbank.MoveStepCursor(1)
	case NotePatternUp:
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.Jump(1)
	case NotePatternDown:
		if err := stopPlayback(); err != nil {
			return err
		}
		return patbank.Jump(-1)
	case NoteAlt:
		if !altOn && shiftOn {
			// Turn off lights but don't activate alt.
			return patbank.f.Off()
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
	case CCSelect:
		dir := 1
		if int(ev.Data[2]) == EncoderLeft {
			dir = -1
		}
		if patbank.editingLength {
			return patbank.AdjustLength(dir)
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
			// Clear pattern.
			if err := stopPlayback(); err != nil {
				return err
			}
			if err := patbank.SetPattern(&Pattern{}); err != nil {
				return err
			}
			return toggleAlt()
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
		if err := stopPlayback(); err != nil {
			return err
		}
		if err := exitPatternEditModes(); err != nil {
			return err
		}
		processEvent = processSongEvent
		if err := patbank.f.SetLed(NotePatternSong, LEDGreen); err != nil {
			return err
		}
		return songbank.Jump(0)
	}
	return nil
}
