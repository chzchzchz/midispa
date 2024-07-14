package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// Filters midi clocks and more

// F0 00 30 33 00 CH CC CC PP F7
func isRouteSysEx(data []byte) bool {
	if len(data) != 10 {
		return false
	}
	if data[0] != midi.SysEx {
		return false
	}
	if data[1] != 0 || data[2] != 0x30 || data[3] != 0x33 {
		// Wrong vendor
		return false
	}
	if data[4] != 0 {
		// command byte: 0 = route channel
		return false
	}
	if data[9] != midi.EndSysEx {
		return false
	}
	return true
}

type Route struct {
	dst         alsa.SeqAddr
	midiChannel int
}

func decodeRouteSysEx(data []byte) *Route {
	channel := int(data[5])
	if channel > 15 || channel < 0 {
		return nil
	}
	client := (int(data[6]) << 7) | int(data[7])
	port := int(data[8])
	return &Route{alsa.SeqAddr{client, port}, channel}
}

func evPortToSeqAddrs(data []byte) (alsa.SeqAddr, alsa.SeqAddr) {
	sender := alsa.SeqAddr{int(data[1]), int(data[2])}
	rxer := alsa.SeqAddr{int(data[3]), int(data[4])}
	return sender, rxer
}

type ChannelWriter[T any] struct {
	outc  chan<- T
	donec <-chan struct{}
}

func (cw *ChannelWriter[T]) Close() {
	close(cw.outc)
	<-cw.donec
}

type EvWriter ChannelWriter[alsa.SeqEvent]

func (ew *EvWriter) Close() { ((*ChannelWriter[alsa.SeqEvent])(ew)).Close() }

func makeWriter(aseq *alsa.Seq, dst alsa.SeqAddr) *EvWriter {
	outc, donec := make(chan alsa.SeqEvent, 16), make(chan struct{})
	go func() {
		defer close(donec)
		for ev := range outc {
			ev.SeqAddr = dst
			//log.Printf("event out: %+v", ev)
			if err := aseq.Write(ev); err != nil {
				log.Printf("write %v failed: %v", ev, err)
				panic(err)
			}
		}
	}()
	return &EvWriter{outc, donec}
}

type FilterSeq struct {
	aseq    *alsa.Seq
	bcast   *EvWriter
	routes  [16]*EvWriter
	routing int
}

func newFilterSeq(aseq *alsa.Seq) *FilterSeq {
	return &FilterSeq{
		aseq:  aseq,
		bcast: makeWriter(aseq, alsa.SubsSeqAddr),
	}
}

func (f *FilterSeq) handleEvent() error {
	ev, err := f.aseq.Read()
	if err != nil {
		return err
	}
	cmd := ev.Data[0]
	if !midi.IsMessage(cmd) {
		// internal message
		switch cmd {
		case alsa.EvPortSubscribed:
			sender, rxer := evPortToSeqAddrs(ev.Data)
			log.Println("subscribed", sender, "->", rxer)
		case alsa.EvPortUnsubscribed:
			sender, rxer := evPortToSeqAddrs(ev.Data)
			log.Println("unsubscribed", sender, "->", rxer)
		}
		return nil
	} else if midi.IsClock(cmd) {
		return nil
	} else if isRouteSysEx(ev.Data) {
		r := decodeRouteSysEx(ev.Data)
		if r == nil {
			log.Println("rejecting bad route:", r)
			return nil
		}
		fmt.Println("arming route:", r)
		if oldr := f.routes[r.midiChannel]; oldr != nil {
			log.Println("kicking out old route on", r.midiChannel)
			f.routing--
			go func() { oldr.Close() }()
		}
		f.routes[r.midiChannel] = makeWriter(f.aseq, r.dst)
		f.routing++
		return nil
	}
	outc := f.bcast.outc
	if midi.IsChannelMessage(ev.Data[0]) {
		ch := midi.Channel(ev.Data[0])
		if r := f.routes[ch]; r != nil {
			outc = r.outc
		} else if f.routing > 0 {
			log.Println("routing: dropping", ev)
			return nil
		}
	}
	outc <- ev
	return nil
}

func (f *FilterSeq) Close() {
	f.bcast.Close()
	for _, r := range f.routes {
		if r != nil {
			r.Close()
		}
	}
}

func main() {
	cnFlag := flag.String("name", "midifilter", "midi client name")
	flag.Parse()
	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%q: %+v\n", *cnFlag, aseq.SeqAddr)
	f := newFilterSeq(aseq)
	defer f.Close()
	for {
		if err := f.handleEvent(); err != nil {
			panic(err)
		}
	}
}
