package main

import (
	"log"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/sysex"
)

type Seq struct {
	aseq    *alsa.Seq
	savef   func()
	mcs     cc.MidiControlsMap
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

const sysDevId = 0x10

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
			log.Printf("could not find program #%d of %d",
				v, len(s.assigns)-1)
		}
		return nil
	}
	if len(ev.Data) != 3 {
		return nil
	}

	cc, val := int(ev.Data[1]), int(ev.Data[2])
	a := &s.assigns[s.pgm]
	cmdMask := cmd & 0xf0
	isInputButton := cmdMask&0xf0 == 0x90
	inName := s.mcs[a.InDevice].Name(cmdMask, cc)
	if inName == "" {
		return nil
	}
	outName, outCh := a.InToOut(inName)
	if inName == "Record" || outName == "Record" {
		if cmdMask == 0x90 || val == 0 {
			s.savef()
			return nil
		}
	}
	switch outName {
	case "NextChannel":
		if val > 0 {
			if s.outChan = (s.outChan + 1) % 16; s.outChan == 0 {
				s.outChan = 1
			}
			log.Println("set channel to", s.outChan)
		}
		return nil
	case "PrevChannel":
		if val > 0 {
			if s.outChan = s.outChan - 1; s.outChan < 1 {
				s.outChan = 16
			}
			log.Println("set channel to", s.outChan)
		}
		return nil
	case "MasterBalance":
		v := float32(val) * (((1 << 14) - 1) / 127.0)
		mb := sysex.MasterBalance{DeviceId: sysDevId, Balance: int(v)}
		log.Println("set master balance to", mb.Balance)
		msg, _ := mb.MarshalBinary()
		return s.aseq.Write(alsa.SeqEvent{SeqAddr: a.saOut, Data: msg})
	case "MasterVolume":
		v := float32(val) * (((1 << 14) - 1) / 127.0)
		mv := sysex.MasterVolume{DeviceId: sysDevId, Volume: int(v)}
		log.Println("set master volume to", mv.Volume)
		msg, _ := mv.MarshalBinary()
		return s.aseq.Write(alsa.SeqEvent{SeqAddr: a.saOut, Data: msg})
	case "ChorusModRate":
		cmr := sysex.ChorusModRate{
			DeviceId: 0x7f,
			ModRate:  float32(val) / 127.0 * (127.0 * 0.122),
		}
		log.Println("chorus mod rate set to", cmr.ModRate)
		msg, _ := cmr.MarshalBinary()
		return s.aseq.Write(alsa.SeqEvent{SeqAddr: a.saOut, Data: msg})
	case "ChorusSendToReverb":
		cs2r := sysex.ChorusSendToReverb{
			DeviceId:     0x7f,
			SendToReverb: float32(val) / 127.0,
		}
		log.Println("send chorus to reverb set to", cs2r.SendToReverb)
		msg, _ := cs2r.MarshalBinary()
		return s.aseq.Write(alsa.SeqEvent{SeqAddr: a.saOut, Data: msg})
	case "Dump":
		panic("stub")
	case "Load":
		panic("stub")
	}
	writef := func() error {
		if outCh <= 0 {
			outCh = s.outChan
		}
		mcc := s.mcs[a.OutDevice]
		outMC, outCC := mcc.Get(outName)
		if isInputButton && outMC != nil {
			// Flip value.
			oldVal := outMC.Get(outCC)
			if oldVal == nil || *oldVal <= 63 {
				val = 0x7f
			} else {
				val = 0
			}
		}
		log.Println(inName, "->", outName, "=", val, "; ch =", outCh)
		if outMC == nil || !outMC.Set(outCC, val) {
			log.Printf(
				"failed to set outName=%s on device %q\n",
				outName, a.OutDevice)
			return nil
		}
		ch := byte(outCh - 1)
		evOut := alsa.SeqEvent{
			SeqAddr: a.saOut,
			Data:    []byte{outMC.Cmd | ch, byte(outCC), byte(val)},
		}
		if err := s.aseq.Write(evOut); err != nil {
			return err
		}
		if isInputButton {
			// Input was a button; writeback out value to change lit value.
			ev.Data[2] = byte(val)
			if err := s.aseq.Write(ev); err != nil {
				log.Printf("failed writeback of %+v", ev)
				return err
			}
		}
		return nil
	}
	if outName != "" && cc > -1 {
		// Write input CC to output CC
		return writef()
	}
	// Try to arm if button.
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
	log.Printf("applying old patch to %q on channel %d",
		s.assigns[0].OutDevice, s.outChan)
	outMcs := s.mcs[s.assigns[0].OutDevice]
	if outMcs == nil {
		panic("no out mcs" + s.assigns[0].OutDevice)
	}
	for _, msg := range outMcs.ToControlCodes() {
		name := outMcs.Name(msg[0], int(msg[1]))
		log.Println("initializing", name, "=", int(msg[2]))
		msg[0] |= byte(s.outChan - 1)
		ev := alsa.SeqEvent{s.assigns[0].saOut, msg}
		if err := s.aseq.Write(ev); err != nil {
			panic(err)
		}
	}
}
