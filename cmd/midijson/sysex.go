package main

import (
	"encoding"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

type rwSysEx struct {
	aseq *alsa.Seq
	sa   alsa.SeqAddr
	out  []byte
	in   interface{}
	path string
}

func (rws *rwSysEx) doAllSysEx() ([]interface{}, error) {
	s := sysex.SysEx{Data: rws.out}
	sp, err := s.Split()
	if err != nil {
		return nil, err
	}
	var ret []interface{}
	for _, singleSysEx := range sp {
		ev := alsa.SeqEvent{rws.sa, singleSysEx.Data}
		if err := rws.aseq.Write(ev); err != nil {
			return nil, err
		}
		log.Printf("wrote sysex for %q", rws.path)
		if rws.in != nil {
			in, err := rws.read()
			if err != nil {
				return nil, err
			}
			ret = append(ret, in...)
		}
	}
	if rws.in == nil {
		return ret, nil
	}
	// Wait and consume remaining reads, if any.
	time.Sleep(100 * time.Millisecond)
	if rws.aseq.MayRead() {
		in, err := rws.read()
		if err != nil {
			return nil, err
		}
		ret = append(ret, in...)
	}
	return ret, nil
}

func (rws *rwSysEx) read() (ins []interface{}, err error) {
	// TODO: timeout if read takes too long
	readMsg := func() error {
		ev, err := rws.aseq.ReadSysEx()
		if err != nil {
			return err
		}
		nextIn := reflect.ValueOf(rws.in).Interface()
		bu, ok := nextIn.(encoding.BinaryUnmarshaler)
		if !ok {
			return fmt.Errorf("content not binary unmarshaller")
		}
		if err := bu.UnmarshalBinary(ev.Data); err != nil {
			return err
		}
		ins = append(ins, nextIn)
		return nil
	}
	if err := readMsg(); err != nil {
		return nil, err
	}
	// Try to read more sysex replies.
	for rws.aseq.MayRead() {
		if err := readMsg(); err != nil {
			return nil, err
		}
	}
	return ins, err
}
