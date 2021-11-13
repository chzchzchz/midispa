package main

import (
	"log"

	"github.com/chzchzchz/midispa/alsa"
)

type Seq struct {
	aseq    *alsa.Seq
	savef   func()
	mcs     MidiControlsMap
	assigns []Assignments
	outChan int

	pgm int
}

func (s *Seq) run() {
	for {
		if err := s.processEvent(); err != nil {
			panic(err)
		}
	}
}

func (s *Seq) processEvent() error {
	ev, err := s.aseq.Read()
	if err != nil {
		return err
	}
	cmd := ev.Data[0] & 0xf0
	if len(ev.Data) == 2 && cmd == 0xc0 {
		// Change controller.
		if v := int(ev.Data[1]); v < len(s.assigns) {
			s.pgm = v
			s.assigns[s.pgm].Enable()
			log.Printf("using controller program #%d: %q",
				s.pgm, s.assigns[s.pgm].Title)
		} else {
			log.Printf("could not find program #%d; max %d", v, len(s.assigns)-1)
		}
		return nil
	}
	if len(ev.Data) != 3 || cmd != 0xb0 {
		return nil
	}
	cc, val := int(ev.Data[1]), int(ev.Data[2])
	a := &s.assigns[s.pgm]
	inName := s.mcs[a.InDevice].Name(cc)
	if inName == "" {
		return nil
	}
	if inName == "Record" && val == 0 {
		s.savef()
		return nil
	}
	outName, outCh := a.InToOut(inName)
	writef := func() error {
		if outCh <= 0 {
			outCh = s.outChan
		}
		log.Println(inName, "->", outName, "=", val, "; ch =", outCh)
		outCC, ok := s.mcs[a.OutDevice].Set(outName, val)
		if !ok {
			log.Printf("missing cc outName=%s on device %s\n", outCC, outName, a.OutDevice)
			return nil
		}
		ch := byte(outCh - 1)
		ev = alsa.SeqEvent{
			SeqAddr: a.saOut,
			Data:    []byte{0xb0 | ch, byte(outCC), byte(val)},
		}
		return s.aseq.Write(ev)
	}
	if outName != "" {
		return writef()
	}
	if val == 0 {
		return nil
	}
	arms := a.Arm(inName)
	if len(arms) > 0 {
		log.Println("arming on", inName)
	}
	for _, arm := range arms {
		if outName, outCh = a.InToOut(arm); outName == "" {
			continue
		}
		if err := writef(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Seq) applyPatches() {
	// Apply existing patch, if any.
	log.Printf("applying old patch to %q on channel %d", s.assigns[0].OutDevice, s.outChan)
	outMcs := s.mcs[s.assigns[0].OutDevice]
	if outMcs == nil {
		panic("no out mcs" + s.assigns[0].OutDevice)
	}
	for _, msg := range outMcs.ToControlCodes() {
		msg[0] |= byte(s.outChan - 1)
		log.Println("initializing", outMcs.Name(int(msg[1])), "=", int(msg[2]))
		ev := alsa.SeqEvent{s.assigns[0].saOut, msg}
		if err := s.aseq.Write(ev); err != nil {
			panic(err)
		}
	}
}
