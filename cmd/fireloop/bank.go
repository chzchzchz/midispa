package main

import (
	"fmt"
)

type PatternBank struct {
	Patterns    map[int]*Pattern
	selPatIdx   int
	selTrackRow int // valid rows [1,4]
	trackVoices [4]int
	f           *Fire
	vb          *VoiceBank
}

var patbank *PatternBank

func NewPatternBank(f *Fire, vb *VoiceBank) *PatternBank {
	if len(vb.voices) == 0 {
		panic("no voices")
	}
	ret := &PatternBank{Patterns: make(map[int]*Pattern), f: f, vb: vb}
	for i := 0; i < 4; i++ {
		ret.trackVoices[i] = i % len(vb.voices)
	}
	return ret
}

func (p *PatternBank) CurrentPattern() *Pattern {
	return p.Patterns[p.selPatIdx]
}

func (p *PatternBank) Jump(n int) error {
	newIdx := p.selPatIdx + n
	if newIdx <= 0 || newIdx > 999 {
		return nil
	}
	if pp, ok := p.Patterns[p.selPatIdx]; ok && len(pp.Events) == 0 {
		delete(p.Patterns, p.selPatIdx)
	}
	p.selPatIdx = newIdx
	if _, ok := p.Patterns[p.selPatIdx]; !ok {
		p.Patterns[p.selPatIdx] = &Pattern{}
	}
	if err := p.f.Print(0, 0, fmt.Sprintf("Pattern %03d", p.selPatIdx)); err != nil {
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
	return nil
}

func (p *PatternBank) SelectTrackRow(n int) error {
	if p.selTrackRow > 0 {
		if err := p.f.SetLed(CCMuteLED1+(p.selTrackRow-1), 0); err != nil {
			return err
		}
		if err := p.printTrackRow(p.selTrackRow, false); err != nil {
			return err
		}
	}
	if p.selTrackRow == n {
		p.selTrackRow = 0
		return nil
	}
	p.selTrackRow = n
	if err := p.printTrackRow(n, true); err != nil {
		return err
	}
	return p.f.SetLed(CCMuteLED1+(n-1), LEDGreen)
}

func (p *PatternBank) printTrackRow(n int, inv bool) error {
	if err := p.f.ClearOLEDRows(n+1, 1); err != nil {
		return err
	}
	v := p.vb.voices[p.trackVoices[n-1]]
	if inv {
		return p.f.PrintInvert(0, n+1, v.Name)
	}
	return p.f.Print(0, n+1, v.Name)
}

func (p *PatternBank) JogSelect(n int) error {
	if p.selTrackRow == 0 {
		return nil
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
	return p.redrawTrackPads(p.selTrackRow)
}

func (p *PatternBank) redrawTrackPads(track int) error {
	pat := p.Patterns[p.selPatIdx]
	tv := p.vb.voices[p.trackVoices[track-1]]
	var rgb [16][3]int
	for i := 0; i < 16; i++ {
		for _, ev := range pat.Events {
			if ev.Voice == tv {
				idx := int(ev.Beat * 4)
				rgb[idx][1] = 50
			}
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
		panic("bad column")
	}
	var rgb [4][3]int
	for row := 0; row < 4; row++ {
		rgb[row] = f(nil)
	}
	thisBeat := float32(col) / 4.0
	nextBeat := thisBeat + float32(1.0/4.0)
	evs := p.Patterns[p.selPatIdx].FindBeat(thisBeat)
	for _, ev := range evs {
		if ev.Beat < thisBeat {
			fmt.Printf("%+v\n\n%+v vs %v\n", evs, ev, nextBeat)
			panic("oops")
		}
		if ev.Beat >= nextBeat {
			break
		}
		for row, v := range p.trackVoices {
			if ev.Voice == p.vb.voices[v] {
				rgb[row] = f(&ev)
			}
		}
	}
	return p.f.LightPadColumn(col, rgb)
}

func (p *PatternBank) ToggleEvent(row, col, v int) error {
	ev := Event{
		Voice:    p.vb.voices[p.trackVoices[row]],
		Beat:     float32(col) / 4.0,
		Velocity: v,
	}
	g := 0
	if p.Patterns[p.selPatIdx].ToggleEvent(ev) {
		g = 50
	}
	for i := 0; i < 4; i++ {
		if p.trackVoices[i] == p.trackVoices[row] {
			if err := p.f.LightPad(col, i, 0, g, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

type VoiceBank struct {
	voices []*Voice
}

func NewVoiceBank(devs []Device) *VoiceBank {
	vb := &VoiceBank{}
	for _, d := range devs {
		for i := range d.Voices {
			vv := &d.Voices[i]
			vv.device = &d
			vb.voices = append(vb.voices, vv)
		}
	}
	return vb
}
