package main

import (
	"flag"
	"log"
	"path/filepath"
	"unsafe"

	j "github.com/xthexder/go-jack"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
)

func main() {
	configPath := flag.String("config-path", "./", "path to configuration")
	spathFlag := flag.String("samples-path", "./dat/samples", "path to samples")
	cnFlag := flag.String("client-name", "midisampler", "midi and jack client name")
	sinkPortFlag := flag.String("sink-port", "system:playback_1", "jack sink port name")
	// NB: Set sink server via JACK_DEFAULT_SERVER
	flag.Parse()

	// Create jack instance.
	var vv *Voices
	playCallback := func(s []j.AudioSample) int {
		// TODO: have a pipeline that copies buffers into this one
		x := *(*[]float32)(unsafe.Pointer(&s))
		if vv != nil {
			vv.play(x)
		}
		return 0
	}
	wp, err := jack.NewWritePort(*cnFlag, *sinkPortFlag, playCallback)
	if err != nil {
		panic(err)
	}
	defer wp.Close()
	// Create midi sequencer for reading events from controllers.
	aseq, err := alsa.OpenSeq(*cnFlag)
	if err != nil {
		panic(err)
	}
	// Load directories for sample wav files.
	log.Printf("loading sample bank from %q", *spathFlag)
	sampBank = MustLoadSampleBank(*spathFlag)
	sampleHz := wp.Client.GetSampleRate()
	log.Println("resampling to", sampleHz, "sample rate and normalizing")
	for _, s := range sampBank.slice {
		s.Resample(int(sampleHz))
		s.Normalize()
	}
	bufferSize = int(wp.Client.GetBufferSize())
	vv = newVoices(int(sampleHz))

	log.Println("loading programs and banks")
	pm, err := LoadProgramMap(filepath.Join(*configPath, "programs.json"))
	if err != nil {
		log.Println("no programs.json defined, making one from global sample bank")
		p := ProgramFromSampleBank(sampBank)
		pm = make(ProgramMap)
		pm[p.Instrument] = p
	}
	bank, err := LoadBank(filepath.Join(*configPath, "banks.json"), pm)
	if err != nil {
		log.Println("no banks.json defined, making a bank with known programs")
		bank = BankFromProgramMap(pm)
	}
	s := NewSequencer(bank)
	log.Println("waiting on midi events")
	s.midiLoop(aseq, vv)
}
