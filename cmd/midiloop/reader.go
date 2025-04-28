package main

import (
	"io"

	"github.com/chzchzchz/midispa/track"
)

func NewPatternReader(p *track.Pattern) *PatternReader {
	if p == nil {
		panic("nil pattern")
	}
	return &PatternReader{p, 0, 0}
}

type Reader interface {
	Read() ([]track.TickMessage, error)
}

type EmptyReader struct {
	ticks int
	tick  int
}

func NewEmptyReader(ticks int) Reader { return &EmptyReader{ticks, 0} }

func (r *EmptyReader) Read() ([]track.TickMessage, error) {
	if r.ticks == r.tick {
		return nil, io.EOF
	}
	r.tick++
	return nil, nil
}

type PatternReader struct {
	p    *track.Pattern
	idx  int
	tick int
}

func (pr *PatternReader) Read() ([]track.TickMessage, error) {
	if pr.tick >= int(pr.p.LastTick) {
		return nil, io.EOF
	}
	pr.tick++
	start := pr.idx
	for pr.p.Msgs[pr.idx].Tick == pr.tick-1 && pr.idx < len(pr.p.Msgs) {
		pr.idx++
	}
	return pr.p.Msgs[start:pr.idx], nil
}

type ExactTickReader struct {
	r      Reader
	target int
	tick   int
}

func NewExactTickReader(r Reader, target int) *ExactTickReader {
	return &ExactTickReader{r, target, 0}
}

func (r *ExactTickReader) Read() ([]track.TickMessage, error) {
	if r.tick == r.target {
		// Spill out all remaining events.
		var out []track.TickMessage
		for {
			t, err := r.r.Read()
			if err == nil {
				out = append(out, t...)
			} else if err == io.EOF {
				break
			} else {
				return out, err
			}
		}
		return out, io.EOF
	}
	r.tick++
	t, err := r.r.Read()
	if err == nil {
		return t, nil
	} else if err == io.EOF {
		// Padding
		return nil, nil
	}
	return nil, err
}
