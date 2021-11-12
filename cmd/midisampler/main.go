package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"
	"unsafe"

	j "github.com/xthexder/go-jack"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
	"github.com/chzchzchz/midispa/util"
)

func lagrangeScale(v float64) float64 {
	return 0.25*((v*(v-127.0))/(64.0*(64.0-127.0))) +
		((v * (v - 64.0)) / (127.0 * (127.0 - 64.0)))
}

func makeADSR(c *Controls, s *Sample) *ADSR {
	// TODO: different scaling options?
	sd := float64(s.Duration)
	return &ADSR{
		Attack:  time.Duration(sd * lagrangeScale(float64(c.AttackTime))),
		Decay:   time.Duration(sd * lagrangeScale(float64(c.DecayTime))),
		Sustain: float32(c.SustainLevel) / 127.0,
		Release: time.Duration(sd * lagrangeScale(float64(c.ReleaseTime))),
	}
}

func midiLoop(aseq *alsa.Seq) {
	var controls Controls
	controlsUpdated := false
	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		// midi notes writes wavs into jack memory.
		cmd /*, ch*/ := ev.Data[0] & 0xf0 /*, (ev.Data[0] & 0xf) */
		log.Printf("midi message %+v..", ev)
		switch cmd {
		case 0x80: /* note off */
			if s := note2sample(int(ev.Data[1])); s != nil {
				stopVoice(s)
			}
		case 0x90: /* note on */
			s, vel := note2sample(int(ev.Data[1])), int(ev.Data[2])
			if s == nil {
				continue
			}
			if controlsUpdated {
				s.ADSR = makeADSR(&controls, s)
				log.Printf("adsr set to %+v on %q", s.ADSR, s.name)
				controlsUpdated = false
			}
			addVoice(s, float32(vel)/127.0)
		case 0xb0: /* cc */
			cc, val := int(ev.Data[1]), int(ev.Data[2])
			log.Println("got cc", cc, val)
			// TODO: use midicontrols
			switch cc {
			case AttackTimeCC:
				controls.AttackTime = val
				controlsUpdated = true
			case DecayTimeCC:
				controls.DecayTime = val
				controlsUpdated = true
			case SustainLevelCC:
				controls.SustainLevel = val
				controlsUpdated = true
			case ReleaseTimeCC:
				controls.ReleaseTime = val
				controlsUpdated = true
			case AllSoundOffCC:
				soundOff = val != 0
			case AllNotesOffCC:
				if val > 0 {
					for i := range sampleSlice {
						stopVoice(sampleSlice[i])
					}
				}
			}
		}
	}
}

func playCallback(s []j.AudioSample) int {
	x := *(*[]float32)(unsafe.Pointer(&s))
	playVoices(x)
	return 0
}

func main() {
	spathFlag := flag.String("samples-path", "./dat/samples", "path to samples")
	cnFlag := flag.String("client-name", "midisampler", "midi and jack client name")
	sinkPortFlag := flag.String("sink-port", "system:playback_1", "jack sink port name")
	// NB: Set sink server via JACK_DEFAULT_SERVER
	flag.Parse()

	// Create jack instance.
	wp, err := jack.NewWritePort(*cnFlag, *sinkPortFlag, playCallback)
	if err != nil {
		panic(err)
	}
	defer wp.Close()
	sampleHz := wp.Client.GetSampleRate()

	// Load directories for sample wav files.
	adsr := ADSR{
		Attack:  time.Millisecond,
		Decay:   100 * time.Millisecond,
		Sustain: 0.7,
		Release: 200 * time.Millisecond,
	}
	files := util.Walk(*spathFlag)
	for _, f := range files {
		path := filepath.Join(*spathFlag, f)
		s, err := LoadSample(f, path)
		if err != nil {
			log.Printf("error loading %q: %v", f, err)
		} else {
			s.ADSR = &adsr
			s.Resample(int(sampleHz))
			s.Normalize()
		}
	}
	log.Printf("loaded %d of %d samples from %q", len(samples), len(files), *spathFlag)

	// Create midi sequencer for reading events from controllers.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	midiLoop(aseq)
}
