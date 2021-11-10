package main

import (
	"flag"
	"log"
	"path/filepath"

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

func playCallback(s []j.AudioSample) int {
	voicesCopy := copyVoices()
	if len(voicesCopy) > 0 {
		lastWas0 = false
	}
	if lastWas0 {
		return 0
	}
	for i := 0; i < len(s); i++ {
		s[i] = 0
	}
	if len(voicesCopy) == 0 {
		lastWas0 = true
		return 0
	}
	for _, vs := range voicesCopy {
		copyLen := len(s)
		if copyLen > len(vs.remaining) {
			copyLen = len(vs.remaining)
		}
		for i := 0; i < copyLen; i++ {
			s[i] += j.AudioSample(vs.velocity * vs.remaining[i])
		}
		if len(vs.remaining) <= len(s) {
			vs.remaining = nil
		} else {
			vs.remaining = vs.remaining[len(s):]
		}
	}
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

	// Load directories for sample wav files.
	files := util.Walk(*spathFlag)
	for _, f := range files {
		path := filepath.Join(*spathFlag, f)
		if _, err := LoadSample(f, path); err != nil {
			log.Printf("error loading %q: %v", f, err)
		}
	}
	log.Printf("loaded %d of %d samples from %q", len(samples), len(files), *spathFlag)

	// Create midi sequencer for reading events from controllers.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}

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
				addVoice(sampleSlice[note], vel)
			}
		case 0xb0: /* cc */
			cc, val := int(ev.Data[1]), int(ev.Data[2])
			log.Println("got cc", cc, val)
		default:
		}
	}
}
