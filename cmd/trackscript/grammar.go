//go:generate peg grammar.peg
package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/track"
)

// PlayLine is a list of patterns to be played in parallel.
type PlayLine struct {
	patterns []*track.Pattern
}

func (pl *PlayLine) ToPattern() *track.Pattern {
	newPattern := track.EmptyPattern()
	for _, p := range pl.patterns {
		newPattern.Merge(p)
	}
	return newPattern
}

// Phrase is a list of playlines to play sequentially.
type Phrase struct {
	name  string
	lines []*PlayLine
}

func NewGrammar(in string) *Grammar {
	return &Grammar{
		Buffer: in,
		script: NewScript(),
	}
}

func (g *Grammar) startPhrase() {
	if _, ok := g.script.patterns[g.id]; ok {
		panic("already declared pattern/phrase " + g.id)
	}
	g.lastPatternId = g.id
	g.curPhrase = &Phrase{name: g.id}
}

func (g *Grammar) endPhrase() {
	p := track.EmptyPattern()
	for _, pl := range g.curPhrase.lines {
		p.Append(pl.ToPattern())
	}
	g.script.patterns[g.curPhrase.name] = p
	g.curPhrase = nil
}

func (g *Grammar) setBPM() {
	if g.script.bpm != 0 {
		panic("set bpm twice")
	}
	g.script.bpm = g.num
}
func (g *Grammar) addPosition() { fmt.Println("stub: add position") }

func (g *Grammar) addPattern() {
	g.script.AddPattern(g.id, g.str)
	g.lastPatternId = g.id
}

func (g *Grammar) addToIdList(id string) {
	p := g.script.findPattern(id)
	g.curPlayLine.patterns = append(g.curPlayLine.patterns, p)
}

func (g *Grammar) playOpRepeat(n int) {
	p := g.curPlayLine.ToPattern()
	p2 := track.EmptyPattern()
	for i := 0; i < n; i++ {
		p2.Append(p)
	}
	g.curPlayLine.patterns = []*track.Pattern{p2}
}

func (g *Grammar) playOpConcat() {
	top := g.playStack[len(g.playStack)-1]
	g.playStack = g.playStack[:len(g.playStack)-1]

	p1, p2 := top.ToPattern(), g.curPlayLine.ToPattern()
	p1.Append(p2)
	g.curPlayLine.patterns = []*track.Pattern{p1}
}

func (g *Grammar) playOpParallel() {
	top := g.playStack[len(g.playStack)-1]
	g.playStack = g.playStack[:len(g.playStack)-1]
	g.curPlayLine.patterns = append(g.curPlayLine.patterns, top.patterns...)
}

func (g *Grammar) pushPlay() {
	g.playStack = append(g.playStack, g.curPlayLine)
	g.curPlayLine.patterns = nil
}

func (g *Grammar) addPlay() {
	pl := g.curPlayLine
	if g.curPhrase == nil {
		p := pl.ToPattern()
		p.Name = g.str
		fmt.Printf("%s [%d bars; %d beats]\n", g.str, int(p.Bars()), int(p.Beats()))
		g.script.song = append(g.script.song, p)
	} else {
		g.curPhrase.lines = append(g.curPhrase.lines, &pl)
	}
	g.curPlayLine.patterns = nil
}

func (g *Grammar) addFilter() {
	g.script.filters[g.id] = &Filter{path: g.str}
}

func (g *Grammar) addFilterArg() {
	arg := g.str
	f, ok := g.script.filters[g.id]
	if !ok {
		// Anonymous filter.
		g.str = g.id + ".c"
		g.id = fmt.Sprintf("%s_tmp_%d", g.id, len(g.script.filters))
		g.addFilter()
		f = g.script.filters[g.id]
	}
	if f.f != nil {
		panic("adding args to filter " + g.id + " but already compiled")
	}
	if arg[0] == '-' {
		panic("filter arguments will automatically prefix -D")
	}
	f.args = append(f.args, "-D"+arg)
}

func (g *Grammar) applyFilter() {
	f, ok := g.script.filters[g.id]
	if !ok {
		panic("filter " + g.id + " not found when applying to " + g.lastPatternId)
	}
	pat, ok := g.script.patterns[g.lastPatternId]
	if !ok {
		panic("pattern " + g.lastPatternId + " not found when applying filter " + g.id)
	}
	// Rebuild pattern using BPF filtered messages.
	newPat := track.EmptyPattern()
	newPat.MidiTimeSig, newPat.Name = pat.MidiTimeSig, pat.Name
	for _, oldMsg := range pat.Msgs {
		rawMsgs := f.Apply(oldMsg.Raw)
		for _, raw := range rawMsgs {
			msg := track.TickMessage{Tick: oldMsg.Tick, Raw: raw}
			newPat.AppendMessage(msg)
		}
	}
	// Keep pattern length the same.
	if newPat.LastTick > pat.LastTick {
		panic("filtered pattern " + pat.Name + " tick greater than original pattern")
	}
	newPat.LastTick = pat.LastTick

	g.script.patterns[g.lastPatternId] = newPat
}
