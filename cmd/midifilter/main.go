package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// Filters midi clocks and more

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

func makeWriter(aseq alsa.EventWriter, dst alsa.SeqAddr) *EvWriter {
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

// FilterSeq reads events from one half of the ALSA client and writes them back out of the
// other. The two halves are held as two fields rather than as one composed type, so each
// says which direction it is for, and a stand-in only has to stand in for the half under
// test.
type FilterSeq struct {
	reader alsa.EventReader
	writer alsa.EventWriter
	// out is the default output, used when no route claims the message.
	out    *EvWriter
	routes [16]*EvWriter
	// routing counts the armed routes. It is what turns broadcast off: once any channel
	// is routed somewhere, an unrouted one is dropped rather than leaked to subscribers
	// that were not expecting it.
	routing int
	policy  Policy
}

func newFilterSeq(reader alsa.EventReader, writer alsa.EventWriter, out alsa.SeqAddr, p Policy) *FilterSeq {
	return &FilterSeq{
		reader: reader,
		writer: writer,
		out:    makeWriter(writer, out),
		policy: p,
	}
}

func (f *FilterSeq) handleRoute(ev alsa.SeqEvent) {
	r := decodeRouteSysEx(ev.Data)
	if r == nil {
		log.Println("rejecting bad route:", r)
		return
	}
	fmt.Println("arming route:", r)
	if oldr := f.routes[r.midiChannel]; oldr != nil {
		log.Println("kicking out old route on", r.midiChannel)
		f.routing--
		go func() { oldr.Close() }()
	}
	f.routes[r.midiChannel] = makeWriter(f.writer, r.dst)
	f.routing++
	return
}

func (f *FilterSeq) handleEvent() error {
	ev, err := f.reader.Read()
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
	} else if isRouteSysEx(ev.Data) {
		f.handleRoute(ev)
		return nil
	} else if f.policy.handle(ev.Data) {
		return nil
	}
	outc := f.out.outc
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
	f.out.Close()
	for _, r := range f.routes {
		if r != nil {
			r.Close()
		}
	}
}

func main() {
	cnFlag := flag.String("name", "midifilter", "midi client name")
	policyFlag := flag.String("bpf", defaultPolicyPath, "bpf elf path")
	inputFlag := flag.String("i", "", "input midi port (optional)")
	outputFlag := flag.String("o", "", "output midi port (optional), connected for writing on startup")
	broadcastFlag := flag.Bool("broadcast", true, "broadcast output to every subscriber; false sends only to -o")

	flag.Parse()
	// Turning broadcast off is only meaningful with a port to send to
	// instead, so a lone -broadcast=false is refused before anything is
	// opened.
	if !*broadcastFlag && *outputFlag == "" {
		panic("-broadcast=false needs -o")
	}
	// Create midi sequencer for reading/writing events.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}

	if *inputFlag != "" {
		sa, err := aseq.PortAddress(*inputFlag)
		must(err)
		must(aseq.OpenPortRead(sa))
	}

	// out is the default output destination: the subscribers address
	// while broadcast is on, or the -o port once -broadcast is false, so
	// an unrouted message reaches only that port. The port is connected
	// either way, so broadcast mode still reaches it as a subscriber.
	out := alsa.SubsSeqAddr
	if *outputFlag != "" {
		sa, err := aseq.PortAddress(*outputFlag)
		must(err)
		must(aseq.OpenPortWrite(sa))
		if !*broadcastFlag {
			out = sa
		}
	}

	log.Printf("%q: %+v", *cnFlag, aseq.SeqAddr)
	policy := initPolicy(*policyFlag, aseq.NewWriter(out))
	f := newFilterSeq(aseq, aseq, out, policy)
	defer f.Close()
	for {
		if err := f.handleEvent(); err != nil {
			panic(err)
		}
	}
}
