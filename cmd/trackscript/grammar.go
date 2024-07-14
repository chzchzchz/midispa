//go:generate peg grammar.peg
package main

import (
	"fmt"
	"strings"

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
	_, ok := g.script.patterns[g.id]
	if ok {
		panic("already defined pattern: " + g.id)
	}
	midiPath := g.str
	if strings.HasSuffix(g.str, ".abc") {
		midiPath = g.str[:len(g.str)-3] + "mid"
		if err := abc2midi(g.str, midiPath); err != nil {
			panic("could not generate " + g.str + ": " + err.Error())
		}
	}
	p, err := track.NewPattern(midiPath)
	if err != nil {
		panic("pattern error: \"" + err.Error() + "\" on " + g.id)
	}
	g.script.patterns[g.id] = p
}

func (g *Grammar) addPlay() {
	pl := &PlayLine{}
	for _, id := range g.idList {
		p := g.script.patterns[id]
		if p == nil {
			panic("could not find pattern " + id)
		}
		pl.patterns = append(pl.patterns, p)
	}
	if g.curPhrase == nil {
		g.script.song = append(g.script.song, pl.ToPattern())
	} else {
		g.curPhrase.lines = append(g.curPhrase.lines, pl)
	}
	g.idList = nil
}
