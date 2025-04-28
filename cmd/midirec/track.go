package main

import (
	"bytes"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/chzchzchz/midispa/midi"

	gomidi "gitlab.com/gomidi/midi"

	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/realtime"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"
)

type Track struct {
	events [][]Event // indexed by second offset
	bpm    int
}

type Event struct {
	clock time.Duration // time since start of song
	data  []byte
}

func (t *Track) Channels() []int {
	used := make(map[int]struct{})
	for _, evs := range t.events {
		for _, ev := range evs {
			if midi.IsNoteOn(ev.data[0]) {
				ch := midi.Channel(ev.data[0])
				used[ch] = struct{}{}
			}
		}
	}
	out := make([]int, 0, 16)
	for a := range used {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

func (t *Track) First() Event {
	for _, evs := range t.events {
		if len(evs) > 0 {
			return evs[0]
		}
	}
	panic("no events")
}

func (t *Track) Last() Event {
	for i := len(t.events) - 1; i >= 0; i-- {
		if last := t.events[i]; len(last) > 0 {
			return last[len(last)-1]
		}
	}
	panic("no events")
}

func (t *Track) Events() (ret []Event) {
	for _, evs := range t.events {
		for _, ev := range evs {
			ret = append(ret, ev)
		}
	}
	return ret
}

func (t *Track) Empty() bool { return len(t.events) == 0 }

func (t *Track) Add(ev Event) {
	idx := int(ev.clock.Seconds())
	if needed := idx - (len(t.events) - 1); needed > 0 {
		t.events = append(t.events, make([][]Event, needed)...)
	}
	evs := t.events[idx]
	cmp := func(v int) bool { return evs[v].clock >= ev.clock }
	i := sort.Search(len(evs), cmp)
	evs = append(evs[:i], append([]Event{ev}, evs[i:]...)...)
	t.events[idx] = evs
}

func (t *Track) ShiftTime(d time.Duration) {
	evs := t.Events()
	t.events = nil
	for _, ev := range evs {
		ev.clock += d
		t.Add(ev)
	}
}

func (t *Track) Erase(start time.Duration, end time.Duration) {
	if int(start.Seconds()) >= len(t.events) {
		return
	}
	if int(end.Seconds()) >= len(t.events) {
		end = time.Second * time.Duration(len(t.events)-1)
	}
	if start < 0 || start > end {
		return
	}
	ssecs, esecs := start.Seconds(), end.Seconds()
	// Partial beginning second.
	startEvs := t.events[int(ssecs)]
	cmpStartLo := func(v int) bool { return startEvs[v].clock >= start }
	cmpStartHi := func(v int) bool { return startEvs[v].clock >= end }
	startLo := sort.Search(len(startEvs), cmpStartLo)
	startHi := sort.Search(len(startEvs), cmpStartHi)
	t.events[int(ssecs)] = startEvs[startLo:startHi]
	// Full seconds that can be dropped completely.
	for i := int(ssecs) + 1; i < int(esecs); i++ {
		t.events[i] = nil
	}
	// Partial end second.
	endEvs := t.events[int(esecs)]
	cmpEndHi := func(v int) bool { return endEvs[v].clock >= end }
	endHi := sort.Search(len(endEvs), cmpEndHi)
	t.events[int(esecs)] = endEvs[:endHi]
}

func (t *Track) save(midipath string) error {
	tpq := smf.MetricTicks(960)
	msg2midi := func(data []byte) (gomidi.Message, error) {
		rd := midireader.New(bytes.NewBuffer(data), func(m realtime.Message) {})
		return rd.Read()
	}
	writeMIDI := func(wr smf.Writer) {
		// Microseconds per quarter note.
		sig := meta.TimeSig{Numerator: 4, Denominator: 3, ClocksPerClick: 24, DemiSemiQuaverPerQuarter: 8}
		must(wr.Write(sig))
		must(wr.Write(meta.Tempo(uint32((60.0 / float64(t.bpm)) * 1e6))))
		lastClock := time.Duration(0)
		evs := t.Events()
		for _, ev := range evs {
			wr.SetDelta(tpq.Ticks(uint32(t.bpm), ev.clock-lastClock))
			mm, err := msg2midi(ev.data)
			must(err)
			must(wr.Write(mm))
			lastClock = ev.clock
		}
		log.Printf("wrote %d events; last clock %v", len(evs), lastClock)
		wr.Write(meta.EndOfTrack)
	}
	err := smfwriter.WriteFile(
		midipath, writeMIDI, smfwriter.NumTracks(1), smfwriter.TimeFormat(tpq))
	if err != smf.ErrFinished {
		return err
	}
	return nil
}
