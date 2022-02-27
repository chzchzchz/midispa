package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

type Playback struct {
	songBeat float32
	patBeat  float32

	// < 0 if no skip, >= 0 in case of a skip.
	nextSongBeat float32

	updatePads  func(curBeat float32) error
	nextPattern func(curBeat float32) *Pattern
}

func (p *Playback) Start(aseq *alsa.Seq) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go p.run(ctx, aseq)
	return cancel
}

func (p *Playback) JumpSongBeat(beat float32) (oldSongBeat float32) {
	oldSongBeat = p.songBeat
	p.nextSongBeat = beat
	return oldSongBeat
}

func (p *Playback) playBeat(aseq *alsa.Seq, pat *Pattern) (float32, error) {
	evs := pat.FindBeat(p.patBeat)
	nextBeat := float32(0)
	i := 0
	for i < len(evs) {
		if evs[i].Beat > p.patBeat {
			// No more events to send.
			nextBeat = evs[i].Beat
			break
		}
		msgs := evs[i].ToMidi()
		if err := writeMidiMsgs(aseq, evs[i].device.SeqAddr, msgs); err != nil {
			return 0, err
		}
		i++
	}
	return nextBeat, p.updatePads(p.songBeat)
}

func (p *Playback) run(ctx context.Context, aseq *alsa.Seq) error {
	p.nextSongBeat = -1
	curBpm, curPattern := bpm, p.nextPattern(0)
	// Compute measures w/r/t this start time + now() to avoid drift.
	start := time.Now()
	for {
		if p.songBeat == 0 {
			ev := alsa.SeqEvent{alsa.SubsSeqAddr, []byte{0xfa}}
			if err := aseq.WritePort(ev, 1); err != nil {
				return err
			}
		}
		if curPattern == nil {
			curPattern = &emptyPattern
		}
		nextBeat, err := p.playBeat(aseq, curPattern)
		if err != nil {
			return err
		}
		// Find next event time, if any.
		next16th := float32(math.Floor(float64(p.patBeat*4.0))+1.0) / 4.0
		if (nextBeat == 0 || nextBeat > next16th) && p.patBeat < 4.0 {
			// TODO: this should be PPQ for midi clock mastering.
			nextBeat = next16th
		}
		if nextBeat >= curPattern.Beats() {
			// Past measure; reset.
			nextBeat = 0
		}
		var waitUntil time.Duration
		if nextBeat != 0 {
			scale := float32(time.Minute / time.Duration(curBpm))
			waitTime := (nextBeat - p.patBeat) * scale
			p.songBeat += nextBeat - p.patBeat
			p.patBeat = nextBeat
			waitUntil = time.Duration(waitTime)
		} else {
			// Reset to next measure.
			measureLength := 4 * (time.Minute / time.Duration(curBpm))
			start = start.Add(measureLength)
			waitUntil = time.Until(start)
			curBpm = bpm
			if p.nextSongBeat < 0 {
				p.songBeat += curPattern.Beats() - p.patBeat
			} else {
				p.songBeat = p.nextSongBeat
				p.nextSongBeat = -1
			}
			curPattern, p.patBeat = p.nextPattern(p.songBeat), 0
			if curPattern == nil {
				// Loop.
				curPattern, p.songBeat = p.nextPattern(0), 0
			}
		}
		select {
		case <-time.After(waitUntil):
		case <-ctx.Done():
			ev := alsa.SeqEvent{alsa.SubsSeqAddr, []byte{0xfc}}
			return aseq.WritePort(ev, 1)
		}
	}
}

func (pb *PatternBank) startSequencer(aseq *alsa.Seq) context.CancelFunc {
	var lastColumn int
	// Reset to start of pattern.
	next := func(beat float32) *Pattern {
		lastColumn = 15
		if beat == 0 {
			return pb.CurrentPattern()
		}
		return nil
	}
	// Light up column if new position.
	update := func(beat float32) error {
		thisColumn := int(math.Floor(float64(beat*4))) % 16
		if thisColumn == lastColumn {
			// No update.
			return nil
		}
		// Reset last column.
		if err := pb.drawPadColumn(lastColumn); err != nil {
			return err
		}
		// Set new column.
		lastColumn = thisColumn
		return pb.drawPadColumnInvert(thisColumn)
	}
	p := Playback{updatePads: update, nextPattern: next}
	return p.Start(aseq)
}

func (sb *SongBank) startSequencer(aseq *alsa.Seq) context.CancelFunc {
	p := &Playback{}
	sb.playback = p
	// Move to next song pattern.
	p.nextPattern = func(beat float32) *Pattern {
		pat, _ := sb.CurrentSong().BeatToPattern(beat)
		return pat
	}
	// Light playing measure.
	lastSongBeat := float32(-99999)
	p.updatePads = func(beat float32) error {
		lastMeasure := int(math.Floor(float64(lastSongBeat)) / 4)
		givenMeasure := int(math.Floor(float64(beat)) / 4)
		lastSongBeat = beat
		if lastMeasure == givenMeasure {
			return nil
		}
		must(sb.ToggleMeasureBrightness(lastMeasure, givenMeasure))
		must(sb.printRow(4, fmt.Sprintf("Measure %03d", givenMeasure+1)))

		s := sb.CurrentSong()
		pat, _ := s.BeatToPattern(beat)
		pidx := sb.pb.PatternIdxMap()[pat]
		return sb.printRow(5, fmt.Sprintf("Pat-bar %03d", pidx))
	}
	return p.Start(aseq)
}
