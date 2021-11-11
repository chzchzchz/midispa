package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	j "github.com/xthexder/go-jack"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
	"github.com/chzchzchz/midispa/util"
)

const midiNoteBias = 21 // A0

func rebiasNote(note byte) int {
	v := int(note) - midiNoteBias
	if v < 0 {
		return 0
	}
	return v
}

var lastWas0 = false
var soundOff = false

func playCallback(s []j.AudioSample) int {
	// Copy voices to avoid threading problems.
	voicesCopy := copyVoices()
	// Check if only writing out zeroes.
	if len(voicesCopy) > 0 && !soundOff {
		lastWas0 = false
	}
	if lastWas0 {
		return 0
	}
	for i := 0; i < len(s); i++ {
		s[i] = 0
	}
	if len(voicesCopy) == 0 || soundOff {
		lastWas0 = true
		return 0
	}
	// Apply all voices to sample buffer.
	for _, vs := range voicesCopy {
		copyLen := len(s)
		if copyLen > len(vs.remaining) {
			copyLen = len(vs.remaining)
		}
		if vs.adsrState == nil {
			for i := 0; i < copyLen; i++ {
				s[i] += j.AudioSample(vs.velocity * vs.remaining[i])
			}
		} else {
			for i := 0; i < copyLen; i++ {
				s[i] += j.AudioSample(vs.adsrState.Apply(vs.remaining[i]))
			}
		}

		if len(vs.remaining) <= len(s) {
			vs.remaining = nil
		} else {
			vs.remaining = vs.remaining[len(s):]
		}
	}
	return 0
}

func midiLoop(aseq *alsa.Seq, adsr *adsrCycles) {
	// midi event loop.
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
			note := rebiasNote(ev.Data[1])
			if note < len(sampleSlice) {
				stopVoice(sampleSlice[note])
			}
		case 0x90: /* note on */
			note, vel := rebiasNote(ev.Data[1]), int(ev.Data[2])
			if note < len(sampleSlice) {
				as := adsr.Press(float32(vel) / 127.0)
				addVoice(sampleSlice[note], vel, &as)
			}
		case 0xb0: /* cc */
			log.Println("got cc", cc, val)
			cc, val := int(ev.Data[1]), int(ev.Data[2])
			switch cc {
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
	files := util.Walk(*spathFlag)
	for _, f := range files {
		path := filepath.Join(*spathFlag, f)
		s, err := LoadSample(f, path)
		if err != nil {
			log.Printf("error loading %q: %v", f, err)
		} else {
			s.Resample(int(sampleHz))
		}
	}
	log.Printf("loaded %d of %d samples from %q", len(samples), len(files), *spathFlag)

	// Create midi sequencer for reading events from controllers.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}

	ad := ADSR{
		Attack:  5 * time.Millisecond,
		Decay:   100 * time.Millisecond,
		Sustain: 0.7,
		Release: 200 * time.Millisecond,
	}
	adsr := ad.Cycles(float64(sampleHz))
	midiLoop(aseq, &adsr)
}
