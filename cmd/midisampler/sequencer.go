package main

import (
	"log"
	"time"

	"github.com/chzchzchz/midispa/alsa"
)

var sampBank *SampleBank
var channels [16]Channel
var currentBank *Bank

func lagrangeScale(v float64) float64 {
	return 0.25*((v*(v-127.0))/(64.0*(64.0-127.0))) +
		((v * (v - 64.0)) / (127.0 * (127.0 - 64.0)))
}

func makeADSR(c *Controls, td time.Duration) *ADSR {
	// TODO: different scaling options?
	d := float64(td)
	return &ADSR{
		Attack:  time.Duration(d * lagrangeScale(float64(c.AttackTime))),
		Decay:   time.Duration(d * lagrangeScale(float64(c.DecayTime))),
		Sustain: float32(c.SustainLevel) / 127.0,
		Release: time.Duration(d * lagrangeScale(float64(c.ReleaseTime))),
	}
}

func midiLoop(aseq *alsa.Seq) {
	lastDuration := time.Second
	for i := range channels {
		ch := &channels[i]
		if ch.Program = currentBank.programs[i]; ch.Program == nil {
			continue
		}
		log.Printf("setting channel %d to program %q", i, ch.Program.Instrument)
		ch.Volume = 1.0
		// TODO: load controls from programs
		ch.Controls.Volume = 127
		ch.Controls.SustainLevel = 127
	}
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		cmd, ch := ev.Data[0]&0xf0, int(ev.Data[0]&0xf)
		channel := &channels[ch]
		pgm := channel.Program
		if pgm == nil {
			log.Printf("no pgm set for ch=%d, ignoring midi %+v", ch, ev)
			continue
		}
		controls := &channel.Controls
		switch cmd {
		case 0x80:
			/* note off */
			if s := pgm.Note2Sample(int(ev.Data[1])); s != nil {
				stopVoice(s)
			}
		case 0x90: /* note on */
			note, vel := int(ev.Data[1]), int(ev.Data[2])
			s := pgm.Note2Sample(note)
			log.Println("got note on", ev.Data)
			if s == nil {
				log.Println("could not find note", note, "in program", pgm.Instrument)
				continue
			}
			lastDuration = s.Duration
			if controls.updated {
				s.ADSR = makeADSR(controls, s.Duration)
				channel.Volume = float32(controls.Volume) / 127.0
				log.Printf("adsr set to %+v on %q", *s.ADSR, s.Name)
				controls.updated = false
			}
			addVoice(s, channel.Volume*float32(vel)/127.0)
		case 0xb0: /* cc */
			cc, val := int(ev.Data[1]), int(ev.Data[2])
			// TODO use controls code from midicc
			if controls.Set(cc, val) {
				log.Printf("pending adsr %+v", *makeADSR(controls, lastDuration))
				continue
			}
			switch cc {
			case AllSoundOffCC:
				soundOff = val != 0
			case AllNotesOffCC:
				if val > 0 {
					stopVoices()
				}
			default:
				log.Printf("unrecognized control message %+v..", ev)
			}
		default:
			log.Printf("unrecognized midi message %+v..", ev)
		}
	}
}
