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
		script: Script{
			bpm:      0,
			patterns: make(map[string]*track.Pattern),
		},
	}
}

func (g *Grammar) startPhrase() {
	if _, ok := g.script.patterns[g.id]; ok {
		panic("already declared pattern/phrase " + g.id)
	}
	g.curPhrase = &Phrase{name: g.id}
}

func (g *Grammar) endPhrase() {
	fmt.Println("ending phrase", g.curPhrase.name)
	p := track.EmptyPattern()
	for _, pl := range g.curPhrase.lines {
		p.Append(pl.ToPattern())
	}
	g.script.patterns[g.curPhrase.name] = p
	g.curPhrase = nil
}

func (g *Grammar) addDevice() { panic("stub") }

func (g *Grammar) setBPM() {
	if g.script.bpm != 0 {
		panic("set bpm twice")
	}
	g.script.bpm = g.num
}
func (g *Grammar) addPosition() { fmt.Println("stub: add position") }

func (g *Grammar) addPattern() {
	g.script.AddPattern(g.id, g.str)
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
		g.script.song = append(g.script.song, pl.ToPattern())
	} else {
		g.curPhrase.lines = append(g.curPhrase.lines, &pl)
	}
	g.curPlayLine.patterns = nil
}
