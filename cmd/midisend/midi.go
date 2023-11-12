package main

import (
	"io"
	"sync"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/jack"
)

type alsaWriter struct {
	seq *alsa.Seq
	sa  alsa.SeqAddr
}

func newAlsaWriter(port string) *alsaWriter {
	seq, err := alsa.OpenSeq("midisend")
	if err != nil {
		panic(err)
	}
	sa, err := seq.PortAddress(port)
	if err != nil {
		panic(err)
	}
	if err := seq.OpenPortWrite(sa); err != nil {
		panic(err)
	}
	return &alsaWriter{
		seq: seq,
		sa:  sa,
	}
}

func (aw *alsaWriter) Write(msg []byte) (int, error) {
	return len(msg), aw.seq.Write(alsa.SeqEvent{aw.sa, msg})
}

func (aw *alsaWriter) Close() error {
	return aw.seq.Close()
}

type jackWriter struct {
	p      *jack.Port
	msgs   [][]byte
	lastc  chan struct{}
	closed bool
	mu     sync.Mutex
}

func (jw *jackWriter) processMidi(w io.Writer) {
	jw.mu.Lock()
	for len(jw.msgs) > 0 {
		if _, err := w.Write(jw.msgs[0]); err != nil {
			break
		}
		jw.msgs = jw.msgs[1:]
	}
	if len(jw.msgs) == 0 {
		jw.msgs = nil
	}
	if jw.lastc != nil && !jw.closed {
		close(jw.lastc)
	}
	jw.mu.Unlock()
}

func newJackWriter(port string) *jackWriter {
	jw := &jackWriter{}
	pc := jack.PortConfig{
		ClientName:   "midispa",
		PortName:     "out",
		MatchName:    []string{port},
		MidiCallback: jw.processMidi,
	}
	p, err := jack.NewWritePort(pc)
	if err != nil {
		panic(err)
	}
	jw.p = p
	return jw
}

func (jw *jackWriter) Write(msg []byte) (int, error) {
	c := make(chan struct{})
	jw.mu.Lock()
	jw.lastc, jw.closed = c, false
	jw.msgs = append(jw.msgs, msg)
	jw.mu.Unlock()
	return 0, nil
}

func (jw *jackWriter) Close() error {
	<-jw.lastc
	jw.p.Close()
	return nil
}
