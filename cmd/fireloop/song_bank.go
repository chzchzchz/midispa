package main

import (
	"fmt"
	"math"

	"github.com/chzchzchz/midispa/sysex/akai"
)

type SongBank struct {
	Songs      map[int]*Song
	selSongIdx int // [1,999]
	pb         *PatternBank
	f          *Fire
	playback   *Playback
}

func NewSongBank(f *Fire, pb *PatternBank) *SongBank {
	sb := &SongBank{
		Songs:      make(map[int]*Song),
		selSongIdx: 1,
		pb:         pb,
		f:          f,
	}
	sb.Songs[sb.selSongIdx] = &Song{}
	return sb
}

func (sb *SongBank) CurrentSong() *Song {
	return sb.Songs[sb.selSongIdx]
}

func (s *SongBank) Jump(n int) error {
	newIdx := s.selSongIdx + n
	if newIdx <= 0 || newIdx > 999 {
		return nil
	} else if _, ok := s.Songs[newIdx]; !ok {
		s.Songs[newIdx] = &Song{}
	}
	s.selSongIdx = newIdx

	must(s.PrintSong())
	must(s.PrintPattern())
	must(s.PrintTempo())
	must(s.printRow(3, " "))
	must(s.printRow(4, " "))
	must(s.printRow(5, " "))

	must(s.DrawPadMeasures())
	must(s.DrawPadPatterns())
	return nil
}

func (s *SongBank) JumpMeasure(x, y int) error {
	if s.playback == nil {
		return nil
	}
	// Determine beat from grid position.
	song := s.Songs[s.selSongIdx]
	idx := (y * 4) + (x % 4) + (16 * (x / 4))
	b := song.IndexToBeat(idx)
	lastSongBeat := s.playback.JumpSongBeat(b)
	return s.printRow(4, fmt.Sprintf(
		"Measure %03d->%03d",
		int(math.Floor(float64((lastSongBeat/4.0)))),
		int(math.Floor(float64(b/4.0)))))
}

func (sb *SongBank) ToggleMeasure(x, y int) error {
	song := sb.Songs[sb.selSongIdx]
	p, _ := sb.pb.Patterns[sb.pb.selPatIdx]
	idx := (y * 4) + (x % 4) + (16 * (x / 4))
	if sp := song.GetPattern(idx); sp == p {
		song.SetPattern(nil, idx)
		p = nil
	} else {
		song.SetPattern(p, idx)
	}
	p2c := sb.patternsToColors()
	return sb.f.LightPadSlice([]akai.Pad{makePad(x, y, Dim(p2c[p], 16))})
}

func (sb *SongBank) ToggleMeasureBrightness(lastMeasure, nextMeasure int) error {
	s := sb.CurrentSong()
	pi, i := s.BeatToPattern(float32(lastMeasure * 4))
	pj, j := s.BeatToPattern(float32(nextMeasure * 4))

	xi, yi := (i%4)+4*(i/16), (i%16)/4
	xj, yj := (j%4)+4*(j/16), (j%16)/4

	p2c := sb.patternsToColors()
	pads := []akai.Pad{
		makePad(xi, yi, Dim(p2c[pi], 16)),
		makePad(xj, yj, p2c[pj]),
	}
	return sb.f.LightPadSlice(pads)
}

func (s *SongBank) SelectPattern(n int) error {
	if n <= 0 || n > len(s.pb.Patterns) {
		return nil
	}
	s.pb.selPatIdx = n
	if err := s.DrawPadPatterns(); err != nil {
		return err
	}
	return s.PrintPattern()
}

func (s *SongBank) PrintSong() error {
	return s.printRow(0, fmt.Sprintf("Song %03d", s.selSongIdx))
}

func (s *SongBank) PrintPattern() error {
	return s.printRow(1, fmt.Sprintf("Pattern %03d", s.pb.selPatIdx))
}

func (s *SongBank) PrintTempo() error {
	return s.printRow(2, fmt.Sprintf("Tempo %03d", bpm))
}

func (s *SongBank) DrawPadMeasures() error {
	p2c := s.patternsToColors()
	song := s.Songs[s.selSongIdx]
	// left three banks of 16 pads
	pads := make([]akai.Pad, 16*3)
	for i := 0; i < len(pads); i++ {
		col := i%4 + 4*(i/16)
		row := (i / 4) % 4
		color := oledBlack
		if i < len(song.Patterns) {
			color = Dim(p2c[song.Patterns[i]], 16)
		}
		pads[i] = makePad(col, row, color)
	}
	return s.f.LightPadSlice(pads)
}

func (s *SongBank) DrawPadPatterns() error {
	padslice := make([]akai.Pad, 16)
	for i := 0; i < len(padslice); i++ {
		color := oledBlack
		if i < len(s.pb.Patterns) {
			color = oledColorTable[(3*i)%len(oledColorTable)]
		}
		if i+1 != s.pb.selPatIdx {
			color = Dim(color, 16)
		}
		// rightmost bank of 16 pads
		x, y := (i%4)+4*3, i/4
		padslice[i] = makePad(x, y, color)
	}
	return s.f.LightPadSlice(padslice)
}

func (s *SongBank) patternsToColors() map[*Pattern][3]int {
	ret := make(map[*Pattern][3]int)
	for i := 1; i <= len(s.pb.Patterns); i++ {
		ret[s.pb.Patterns[i]] = oledColorTable[(3*(i-1))%len(oledColorTable)]
	}
	return ret
}

func (sb *SongBank) printRow(row int, s string) error {
	if err := sb.f.ClearOLEDRows(row, 1); err != nil {
		return err
	}
	return sb.f.Print(0, row, s)
}
