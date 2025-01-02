package main

import (
	"bytes"
	"log"
	"sort"
	"time"

	gomidi "gitlab.com/gomidi/midi"

	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midimessage/realtime"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfwriter"
)

type Track struct {
	events  [][]Event // indexed by second offset
	bpm     int
	channel int
}

type Event struct {
	clock time.Duration // time since start of song
	data  []byte
}

func (t *Track) Events() (ret []Event) {
	for _, evs := range t.events {
		for _, ev := range evs {
			ret = append(ret, ev)
		}
	}
	return ret
}

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

func (t *Track) Erase(start time.Duration, end time.Duration) {
	if int(start.Seconds()) >= len(t.events) {
		return
	}
	if int(end.Seconds()) >= len(t.events) {
		end = time.Second * time.Duration(len(t.events)-1)
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
	t.channel = -1
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
			if t.channel < 0 {
				noteOn, ok := mm.(channel.NoteOn)
				if ok {
					t.channel = int(noteOn.Channel())
				}
			}
			must(wr.Write(mm))
			lastClock = ev.clock
		}
		log.Printf("wrote %d events", len(evs))
		wr.Write(meta.EndOfTrack)
	}
	err := smfwriter.WriteFile(
		midipath, writeMIDI, smfwriter.NumTracks(1), smfwriter.TimeFormat(tpq))
	if err != smf.ErrFinished {
		return err
	}
	return nil
}
