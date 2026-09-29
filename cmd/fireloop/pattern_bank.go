package main

import (
	"fmt"
)

type PatternBank struct {
	Patterns          map[int]*Pattern
	selPatIdx         int
	selTrackRow       int // valid rows [1,4]
	editingLength     bool
	editingNote       bool
	stepCursor        int
	chromaticVelocity int
	pressedPads       uint64
	rowPadMasks       [4]uint16
	trackVoices       [4]int
	f                 *Fire
	vb                *VoiceBank
	playback          *Playback
}

func NewPatternBank(f *Fire, vb *VoiceBank) *PatternBank {
	if len(vb.voices) == 0 {
		panic("no voices")
	}
	ret := &PatternBank{
		Patterns:          make(map[int]*Pattern),
		chromaticVelocity: defaultChromaticVelocity,
		f:                 f,
		vb:                vb,
	}
	for i := 0; i < 4; i++ {
		ret.trackVoices[i] = i % len(vb.voices)
	}
	return ret
}

func (p *PatternBank) CurrentPattern() *Pattern {
	return p.Patterns[p.selPatIdx]
}

func (pb *PatternBank) PatternIdxMap() map[*Pattern]int {
	ret := make(map[*Pattern]int)
	for i, p := range pb.Patterns {
		ret[p] = i
	}
	return ret
}

func (p *PatternBank) SetPattern(pat *Pattern) error {
	// Don't swap out pointer since song may already be using it.
	if pat == nil {
		return nil
	}
	if err := stopPlayback(); err != nil {
		return err
	}
	oldPat, ok := p.Patterns[p.selPatIdx]
	if !ok || oldPat == nil {
		oldPat = &Pattern{}
		p.Patterns[p.selPatIdx] = oldPat
	}
	pat.mu.RLock()
	events := append([]Event(nil), pat.Events...)
	lengthSteps := pat.lengthSteps
	pat.mu.RUnlock()
	if lengthSteps == 0 {
		lengthSteps = defaultPatternSteps
	}
	oldPat.mu.Lock()
	oldPat.Events = events
	oldPat.lengthSteps = lengthSteps
	oldPat.normalizeLocked()
	oldPat.mu.Unlock()
	p.editingNote = false
	p.stepCursor = 0
	p.clearPadState()
	if p.f != nil {
		if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
			return err
		}
	}
	return p.Jump(0)
}

func (p *PatternBank) Jump(n int) error {
	newIdx := p.selPatIdx + n
	if newIdx <= 0 || newIdx > 999 {
		return nil
	}
	changed := newIdx != p.selPatIdx
	if changed {
		if err := stopPlayback(); err != nil {
			return err
		}
	}
	p.selPatIdx = newIdx
	if _, ok := p.Patterns[p.selPatIdx]; !ok {
		p.Patterns[p.selPatIdx] = &Pattern{}
	}
	if changed {
		p.editingNote = false
		p.stepCursor = 0
		p.clearPadState()
		if p.f != nil {
			if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
				return err
			}
		}
	}
	p.clampStepCursor()
	if p.f == nil {
		return nil
	}
	if err := p.f.Print(0, 0, fmt.Sprintf("Pattern %03d", p.selPatIdx)); err != nil {
		return err
	}
	if err := p.f.Print(0, 1, "-----------"); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		if err := p.redrawTrackPads(i + 1); err != nil {
			return err
		}
		if err := p.printTrackRow(i+1, i+1 == p.selTrackRow); err != nil {
			return err
		}
	}
	if p.editingLength {
		return p.printLength()
	}
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) ClearTrackRow(n int) error {
	if n < 1 || n > 4 {
		return nil
	}
	if err := stopPlayback(); err != nil {
		return err
	}
	v := p.vb.voices[p.trackVoices[n-1]]
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	pattern.ClearVoice(v)
	p.editingNote = false
	p.clearPadState()
	if p.f != nil {
		if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
			return err
		}
	}
	for i := 0; i < 4; i++ {
		if p.trackVoices[n-1] != p.trackVoices[i] {
			continue
		}
		if err := p.redrawTrackPads(i + 1); err != nil {
			return err
		}
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) SelectTrackRow(n int) error {
	if n < 0 || n > 4 {
		return nil
	}
	if err := stopPlayback(); err != nil {
		return err
	}
	// Deselect currently selected row, if any.
	if p.selTrackRow > 0 {
		if p.f != nil {
			if err := p.f.SetLed(CCMuteLED1+(p.selTrackRow-1), 0); err != nil {
				return err
			}
		}
		if err := p.printTrackRow(p.selTrackRow, false); err != nil {
			return err
		}
	}
	p.editingNote = false
	p.clearPadState()
	if p.f != nil {
		if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
			return err
		}
	}
	if p.selTrackRow == n {
		p.selTrackRow = 0
		return p.printChromaticStatus()
	}
	// Select new row.
	p.selTrackRow = n
	if err := p.printTrackRow(n, true); err != nil {
		return err
	}
	if p.f != nil {
		if err := p.f.SetLed(CCMuteLED1+(n-1), LEDGreen); err != nil {
			return err
		}
	}
	if err := p.redrawTrackPads(n); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) printTrackRow(n int, inv bool) error {
	if p.f == nil {
		return nil
	}
	if err := p.f.ClearOLEDRows(n+1, 1); err != nil {
		return err
	}
	v := p.vb.voices[p.trackVoices[n-1]]
	name := voiceDisplayName(v)
	if inv {
		return p.f.PrintInvert(0, n+1, name)
	}
	return p.f.Print(0, n+1, name)
}

func (p *PatternBank) JogSelect(n int) error {
	if p.selTrackRow == 0 || len(p.vb.voices) == 0 {
		return nil
	}
	if err := stopPlayback(); err != nil {
		return err
	}
	p.editingNote = false
	p.clearPadState()
	if p.f != nil {
		if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
			return err
		}
	}
	tv := &p.trackVoices[p.selTrackRow-1]
	*tv = *tv + n
	if *tv >= len(p.vb.voices) {
		*tv = 0
	} else if *tv < 0 {
		*tv = len(p.vb.voices) - 1
	}
	if err := p.printTrackRow(p.selTrackRow, true); err != nil {
		return err
	}
	if err := p.redrawTrackPads(p.selTrackRow); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) redrawTrackPads(track int) error {
	if p == nil || p.f == nil || track < 1 || track > 4 {
		return nil
	}
	pat := p.CurrentPattern()
	if pat == nil {
		return nil
	}
	tv := p.vb.voices[p.trackVoices[track-1]]
	evs := pat.FindBeat(0)
	var rgb [16][3]int
	for _, ev := range evs {
		if ev.Voice != tv {
			continue
		}
		idx := eventStep(ev)
		if idx < 0 || idx >= len(rgb) {
			continue
		}
		rgb[idx] = chromaticEventColor(ev)
		if ev.Tie {
			rgb[idx] = markTieColor(rgb[idx])
		}
	}
	if !p.editingNote {
		if p.stepCursor >= 0 && p.stepCursor < len(rgb) {
			rgb[p.stepCursor] = markCursorColor(rgb[p.stepCursor])
		}
	}
	return p.f.LightPadRow(track-1, rgb)
}

func (p *PatternBank) drawPadColumn(col int) error {
	f := func(ev *Event) [3]int {
		if ev == nil {
			return [3]int{0, 0, 0}
		}
		return [3]int{0, 50, 0}
	}
	return p.drawPadColumnColor(col, f)
}

func (p *PatternBank) drawPadColumnInvert(col int) error {
	f := func(ev *Event) [3]int {
		if ev == nil {
			return [3]int{50, 50, 50}
		}
		return [3]int{50, 0, 50}
	}
	return p.drawPadColumnColor(col, f)
}

type evColorFunc func(*Event) [3]int

func (p *PatternBank) drawPadColumnColor(col int, f evColorFunc) error {
	if col < 0 || col > 15 {
		return errOutOfRange
	}
	if p == nil || p.f == nil {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	var rgb [4][3]int
	for row := 0; row < 4; row++ {
		rgb[row] = f(nil)
	}
	evs := pattern.FindBeat(stepBeat(col))
	for _, ev := range evs {
		idx := eventStep(ev)
		if idx < col {
			continue
		}
		if idx >= col+1 {
			break
		}
		for row, v := range p.trackVoices {
			if ev.Voice == p.vb.voices[v] {
				rgb[row] = f(&ev)
				if ev.Tie {
					rgb[row] = markTieColor(rgb[row])
				}
			}
		}
	}
	return p.f.LightPadColumn(col, rgb)
}

func (p *PatternBank) ToggleEvent(row, col, v int) (Event, error) {
	if p == nil || p.vb == nil || row < 0 || row >= len(p.trackVoices) {
		return Event{}, nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil || col < 0 || col >= pattern.LengthSteps() {
		return Event{}, nil
	}
	voice := p.vb.voices[p.trackVoices[row]]
	if voice.IsChromatic() {
		if event, ok := pattern.EventAtStep(col, voice); ok {
			pattern.RemoveEventAtStep(col, voice)
			event.Velocity = 0
			return event, nil
		}
		return Event{}, nil
	}
	if v < 0 {
		v = 0
	}
	if v > midiNoteMax {
		v = midiNoteMax
	}
	ev := Event{
		Voice:    voice,
		Beat:     stepBeat(col),
		Velocity: v,
	}
	added := pattern.ToggleEvent(ev)
	if !added {
		ev.Velocity = 0
	}
	if p.f != nil {
		g := 50
		if !added {
			g = 0
		}
		for i := 0; i < 4; i++ {
			if p.trackVoices[i] == p.trackVoices[row] {
				if err := p.f.LightPad(col, i, 0, g, 0); err != nil {
					return ev, err
				}
			}
		}
	}
	return ev, nil
}
