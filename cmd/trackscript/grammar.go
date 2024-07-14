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

func (g *Grammar) addPlay() {
	pl := &PlayLine{}
	for _, id := range g.idList {
		p := g.script.findPattern(id)
		pl.patterns = append(pl.patterns, p)
	}
	if g.curPhrase == nil {
		g.script.song = append(g.script.song, pl.ToPattern())
	} else {
		g.curPhrase.lines = append(g.curPhrase.lines, pl)
	}
	g.idList = nil
}
