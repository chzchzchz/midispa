package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

type sequencer struct {
	channels           [16]*Channel
	lastSampleDuration time.Duration

	storage *Storage
	voices  *Voices
	sampler *Sampler

	lastPlayStart    time.Time
	lastPlayDuration time.Duration
	storeArmed       bool
}

// Interpolated polynomial for (0, 0), (64, 0.25), (127, 1.0)
// Note: (0,0) => x_0 term is 0
func lagrangeScale(v float64) float64 {
	return 0.25*((v*(v-127.0))/(64.0*(64.0-127.0))) +
		((v * (v - 64.0)) / (127.0 * (127.0 - 64.0)))
}

func makeADSR(c *Controls, td time.Duration) *ADSR {
	// TODO: different scaling options? Might be better to have 0.75.
	// TODO: option to choose between sample length and plain ms
	d := float64(td)
	return &ADSR{
		Attack:  time.Duration(d * lagrangeScale(float64(c.AttackTime))),
		Decay:   time.Duration(d * lagrangeScale(float64(c.DecayTime))),
		Sustain: scale(c.SustainLevel),
		Release: time.Duration(d * lagrangeScale(float64(c.ReleaseTime))),
	}
}

func NewSequencer(s *Storage, voices *Voices, sampler *Sampler) *sequencer {
	seq := &sequencer{
		lastSampleDuration: time.Second,
		storage:            s,
		voices:             voices,
		sampler:            sampler,
	}
	bank := s.ProgramBank()
	for i := range seq.channels {
		ch := NewChannel()
		seq.channels[i] = ch
		if ch.Program = bank.programs[i]; ch.Program != nil {
			log.Printf(
				"setting channel %d to program %q",
				i,
				ch.Program.Instrument)
		}
	}
	return seq
}

func (s *sequencer) midiLoop(aseq *alsa.Seq) {
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		s.handleEvent(ev)
	}
}

func (seq *sequencer) handleEvent(ev alsa.SeqEvent) {
	cmd, ch := ev.Data[0]&0xf0, int(ev.Data[0]&0xf)
	channel := seq.channels[ch]
	pgm := channel.Program
	if pgm == nil {
		log.Printf("no pgm set for ch=%d, ignoring midi %+v", ch, ev)
		return
	}
	controls := &channel.Controls
	switch cmd {
	case 0x80:
		/* note off */
		if s := pgm.Note2Sample(int(ev.Data[1])); s != nil {
			seq.voices.stop(s)
		}
	case 0x90: /* note on */
		note, vel := int(ev.Data[1]), int(ev.Data[2])
		if seq.storeArmed {
			s := seq.sampler.currentSample()
			seq.storeArmed = false
			seq.sampler.reset()
			pgm.StoreSample(note, s)
			sp := filepath.Join(seq.storage.SamplePath, s.Name)
			if err := s.Save(sp); err != nil {
				log.Printf("failed saving sample %q", sp)
				return
			}
			pgms := seq.storage.Programs()
			if err := pgms.Save(seq.storage.ProgramsPath()); err != nil {
				log.Printf(
					"failed saving note to %q: %v",
					pgm.Instrument, err)
				return
			}
			log.Printf(
				"saved sample %q to note %d@%d in %q",
				sp, note, ch, pgm.Instrument)
			return
		}
		s := pgm.Note2Sample(note)
		if s == nil {
			log.Printf("could not find note %d@%d in %q", note, ch, pgm.Instrument)
			return
		}
		log.Printf("got note %d/%d@%d from %s/%q", note, vel, ch, pgm.Instrument, s.Name)
		seq.lastSampleDuration = s.Duration
		if channel.UpdateControls() {
			s.ADSR = makeADSR(controls, s.Duration)
			log.Printf("adsr set to %+v on %q", *s.ADSR, s.Name)
		}
		seq.voices.add(s, channel.Volume*scale(vel), &channel.FxLevel)
	case 0xb0: /* cc */
		cc, val := int(ev.Data[1]), int(ev.Data[2])
		// TODO use controls code from midicc
		if controls.Set(cc, val) {
			log.Printf("cc ch%d: %+v", ch, *controls)
			log.Printf("pending adsr %+v",
				*makeADSR(controls, seq.lastSampleDuration))
			return
		}
		// TODO: associate sampler's samples with channels.
		switch cc {
		case RecordCC:
			if val > 0 {
				seq.sampler.startRecord()
			} else {
				seq.sampler.stopRecord()
			}
		case PlayCC:
			ss := seq.sampler.currentSample()
			if ss == nil {
				return
			}
			if val == 0 {
				seq.lastPlayDuration = time.Since(seq.lastPlayStart)
				seq.voices.stop(ss)
			} else {
				seq.lastPlayStart = time.Now()
				seq.voices.add(ss, 1.0, &channel.FxLevel)
			}
		case StopCC:
			seq.storeArmed = false
			seq.sampler.reset()
		case SeekForwardCC:
			if val > 0 {
				seq.sampler.chop(0, seq.lastPlayDuration)
				seq.lastPlayDuration = 0
			}
		case SeekBackCC:
			if val > 0 {
				seq.sampler.chop(seq.lastPlayDuration, 0)
				seq.lastPlayDuration = 0
			}
		case RepeatCC:
			seq.storeArmed = seq.sampler.currentSample() != nil
		case AllSoundOffCC:
			seq.voices.soundOff = val != 0
		case AllNotesOffCC:
			if val > 0 {
				seq.voices.stopAll()
			}
		default:
			log.Printf("unrecognized control message %+v", ev)
		}
	case 0xf0: /* sysex */
		s := sysex.Decode(ev.Data)
		if s == nil {
			log.Printf("? sysex %+v", ev)
			return
		}
		switch ss := s.(type) {
		case *sysex.MasterVolume:
			masterVolume = ss.Float32()
			log.Println("setting master volume to", masterVolume)
		case *sysex.ChorusSendToReverb:
			seq.voices.fx.SendToReverb = ss.SendToReverb
			log.Println("setting send to reverb to", ss.SendToReverb)
		case *sysex.ChorusModRate:
			*seq.voices.fx.ChorusModRate = ss.ModRate
			log.Println("setting chorus mod rate to ", ss.ModRate)
		default:
			log.Printf("? sysex %+v", s)
		}
	default:
		log.Printf("? midi %+v..", ev)
	}
}
