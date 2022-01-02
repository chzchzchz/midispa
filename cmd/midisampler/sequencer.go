package main

import (
	"log"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

var sampBank *SampleBank

type sequencer struct {
	channels    [16]*Channel
	currentBank *Bank
}

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
		Sustain: scale(c.SustainLevel),
		Release: time.Duration(d * lagrangeScale(float64(c.ReleaseTime))),
	}
}

func NewSequencer(b *Bank) *sequencer {
	s := &sequencer{currentBank: b}
	for i := range s.channels {
		ch := NewChannel()
		s.channels[i] = ch
		if ch.Program = s.currentBank.programs[i]; ch.Program != nil {
			log.Printf("setting channel %d to program %q", i, ch.Program.Instrument)
		}
	}
	return s
}

func (s *sequencer) midiLoop(aseq *alsa.Seq, vv *Voices) {
	lastDuration := time.Second
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		cmd, ch := ev.Data[0]&0xf0, int(ev.Data[0]&0xf)
		channel := s.channels[ch]
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
				vv.stop(s)
			}
		case 0x90: /* note on */
			note, vel := int(ev.Data[1]), int(ev.Data[2])
			s := pgm.Note2Sample(note)
			if s == nil {
				log.Printf("could not find note %d@%d in %q", note, ch, pgm.Instrument)
				continue
			}
			log.Printf("got note %d/%d@%d from %s/%q", note, vel, ch, pgm.Instrument, s.Name)
			lastDuration = s.Duration
			if channel.UpdateControls() {
				s.ADSR = makeADSR(controls, s.Duration)
				log.Printf("adsr set to %+v on %q", *s.ADSR, s.Name)
			}
			vv.add(s, channel.Volume*scale(vel), &channel.FxLevel)
		case 0xb0: /* cc */
			cc, val := int(ev.Data[1]), int(ev.Data[2])
			// TODO use controls code from midicc
			if controls.Set(cc, val) {
				log.Printf("cc ch%d: %+v", ch, *controls)
				log.Printf("pending adsr %+v", *makeADSR(controls, lastDuration))
				continue
			}
			switch cc {
			case AllSoundOffCC:
				vv.soundOff = val != 0
			case AllNotesOffCC:
				if val > 0 {
					vv.stopAll()
				}
			default:
				log.Printf("unrecognized control message %+v", ev)
			}
		case 0xf0: /* sysex */
			s := sysex.Decode(ev.Data)
			if s == nil {
				log.Printf("? sysex %+v", ev)
				continue
			}
			switch ss := s.(type) {
			case *sysex.MasterVolume:
				masterVolume = ss.Float32()
				log.Println("setting master volume to", masterVolume)
			case *sysex.ChorusSendToReverb:
				vv.fx.SendToReverb = ss.SendToReverb
				log.Println("setting send to reverb to", ss.SendToReverb)
			case *sysex.ChorusModRate:
				*vv.fx.ChorusModRate = ss.ModRate
				log.Println("setting chorus mod rate to ", ss.ModRate)
			default:
				log.Printf("? sysex %+v", s)
			}
		default:
			log.Printf("? midi %+v..", ev)
		}
	}
}
