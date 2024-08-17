package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	j "github.com/xthexder/go-jack"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
)

func main() {
	configPath := flag.String("config-path", "./", "path to configuration")
	spathFlag := flag.String("samples-path", "./dat/samples", "path to samples")
	cnFlag := flag.String("client-name", "midisampler", "midi and jack client name")
	samplingPortFlag := flag.String("sampling-port", "", "default jack source port for sampling")
	sinkPortFlag := flag.String("sink-port", "system:playback", "jack sink port names; comma delimited")
	midiInputPortFlag := flag.String("midi-in-port", "", "subscribe to given existing ports; comma separated")

	// NB: Set sink server via JACK_DEFAULT_SERVER
	flag.Parse()

	// Create jack port for playback.
	var vv *Voices
	playCallback := func(s []j.AudioSample) int {
		// TODO: have a pipeline that copies buffers into this one
		x := *(*[]float32)(unsafe.Pointer(&s))
		if vv != nil {
			vv.play(x)
		}
		return 0
	}
	pcOut := jack.PortConfig{
		ClientName:    *cnFlag,
		PortName:      "out",
		MatchName:     strings.Split(*sinkPortFlag, ","),
		AudioCallback: playCallback,
	}
	wp, err := jack.NewWritePort(pcOut)
	if err != nil {
		panic(err)
	}
	defer wp.Close()

	// Create jack port for recording.
	var sampler *Sampler
	recCallback := func(s []j.AudioSample) int {
		x := *(*[]float32)(unsafe.Pointer(&s))
		if sampler != nil {
			sampler.record(x)
		}
		return 0
	}
	pcIn := jack.PortConfig{
		ClientName:    *cnFlag + "-record",
		PortName:      "in",
		MatchName:     strings.Split(*samplingPortFlag, ","),
		AudioCallback: recCallback,
	}
	rp, err := jack.NewReadPort(pcIn)
	if err != nil {
		panic(err)
	}
	defer rp.Close()

	log.Printf("sampling at rate %d", rp.Client.GetSampleRate())
	sampler = NewSampler(int(rp.Client.GetSampleRate()))
	capturePath := filepath.Join(*spathFlag, "capture")
	if err := os.MkdirAll(capturePath, 0755); err != nil {
		panic("could not create \"" + capturePath + "\": " + err.Error())
	}

	// Create midi sequencer for reading events from controllers.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	midiInputs := strings.Split(*midiInputPortFlag, ",")
	for _, input := range midiInputs {
		if len(input) == 0 {
			continue
		}
		log.Printf("looking up input midi address %q", input)
		sa, err := aseq.PortAddress(input)
		if err != nil {
			panic(err)
		}
		log.Printf("listening on input midi port %v", sa)
		if err := aseq.OpenPort(sa.Client, sa.Port); err != nil {
			panic(err)
		}
	}

	storage := NewStorage(*configPath, *spathFlag)
	sampleHz := wp.Client.GetSampleRate()
	log.Println("resampling to", sampleHz, "sample rate and normalizing")
	for _, s := range storage.SampleBank().slice {
		s.Resample(int(sampleHz))
		s.Normalize()
	}
	bufferSize = int(wp.Client.GetBufferSize())
	vv = newVoices(int(sampleHz))

	s := NewSequencer(storage, vv, sampler)

	log.Println("waiting on midi events")
	s.midiLoop(aseq)
}
