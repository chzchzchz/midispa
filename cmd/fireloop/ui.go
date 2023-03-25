package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

type eventProcessFunc func(*alsa.Seq, alsa.SeqEvent) error

var processEvent eventProcessFunc
var shiftOn = false
var altOn = false
var pendingNumber = 0
var bpm = 139
var cancelPlayback context.CancelFunc
var patternClipboard *Pattern
var songbank *SongBank
var patbank *PatternBank

var tapTempoTimes []time.Time

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
	bpm = int(60.0 / dur.Seconds())
	s := fmt.Sprintf("Tempo: %03d", bpm)
	return patbank.f.Print(4, 3, s)
}

func handleSongGrid(aseq *alsa.Seq, x, y, vel int) error {
	if x >= 12 {
		return songbank.SelectPattern((x - 12 + 1) + (y * 4))
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
	if midi.IsNoteOff(status) || !(midi.IsCC(status) || midi.IsNoteOn(status)) {
		return nil
	}
	if x, y, ok := Note2Grid(int(ev.Data[1])); ok {
		return handleSongGrid(aseq, x, y, int(ev.Data[2]))
	}
	switch int(ev.Data[1]) {
	case NotePlay:
		if cancelPlayback == nil {
			cancelPlayback = songbank.startSequencer(aseq)
		}
	case NoteStop:
		if cancelPlayback != nil {
			cancelPlayback()
			cancelPlayback = nil
		}
	case NoteShift:
		shiftOn = !shiftOn
		if shiftOn {
			return songbank.f.SetLed(NoteShift, LEDRed)
		} else {
			return songbank.f.SetLed(NoteShift, 0)
		}
	case NotePatternSong:
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
	if altOn {
		patbank.ClearTrackRow(n)
		toggleAlt()
		return patbank.Jump(0)
	}
	return patbank.SelectTrackRow(n)
}

func handlePatternGrid(aseq *alsa.Seq, x, y, vel int) error {
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
	patEv, err := patbank.ToggleEvent(y, x, vel)
	if err != nil {
		return err
	}
	if cancelPlayback != nil || patEv.Velocity == 0 {
		return nil
	}
	return writeMidiMsgs(aseq, patEv.device.SeqAddr, patEv.ToMidi())
}

func processPatternEvent(aseq *alsa.Seq, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	if midi.IsNoteOff(status) || !(midi.IsCC(status) || midi.IsNoteOn(status)) {
		return nil
	}
	if x, y, ok := Note2Grid(int(ev.Data[1])); ok {
		return handlePatternGrid(aseq, x, y, int(ev.Data[2]))
	}
	switch int(ev.Data[1]) {
	case NoteShift:
		shiftOn = !shiftOn
		if !shiftOn {
			if err := patbank.f.SetLed(NoteShift, 0); err != nil {
				return err
			}
			if pendingNumber > 20 && pendingNumber < 300 {
				bpm = pendingNumber
				pendingNumber = 0
				return patbank.Jump(0)
			}
		} else {
			return patbank.f.SetLed(NoteShift, LEDRed)
		}
	case NotePatternUp:
		return patbank.Jump(1)
	case NotePatternDown:
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
		return patbank.JogSelect(dir)
	case NotePlay:
		if patternClipboard != nil {
			// Copy and paste.
			if err := patbank.SetPattern(patternClipboard); err != nil {
				return err
			}
			patternClipboard = nil
			return patbank.f.SetLed(NoteRecord, LEDOff)
		}
		if cancelPlayback == nil {
			cancelPlayback = patbank.startSequencer(aseq)
		}
	case NoteStop:
		if altOn {
			// Clear pattern.
			patbank.SetPattern(&Pattern{})
			return toggleAlt()
		}
		if cancelPlayback != nil {
			cancelPlayback()
			cancelPlayback = nil
		}
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
		processEvent = processSongEvent
		if err := patbank.f.SetLed(NotePatternSong, LEDGreen); err != nil {
			return err
		}
		return songbank.Jump(0)
	}
	return nil
}
