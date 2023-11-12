package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzchzchz/midispa/midi"
)

func main() {
	strFlag := flag.String("s", "", "hex message to send (e.g., \"F0 A1 2B F7\")")
	fileFlag := flag.String("f", "", "send file using filedump")
	aportFlag := flag.String("p", "", "alsa destination port")
	jportFlag := flag.String("j", "", "jack midi destination port")

	flag.Parse()

	var aw *alsaWriter
	var w io.Writer
	var c io.Closer
	if len(*aportFlag) != 0 {
		aw = newAlsaWriter(*aportFlag)
		w, c = aw, aw
	} else if len(*jportFlag) != 0 {
		jw := newJackWriter(*jportFlag)
		w, c = jw, jw
	} else {
		panic("expected -j or -p")
	}
	defer c.Close()

	var msg []byte
	if len(*strFlag) != 0 {
		for _, hexByte := range strings.Fields(*strFlag) {
			if len(hexByte) != 2 {
				panic("malformed hex string on byte " + hexByte)
			}
			n := 0
			if _, err := fmt.Sscanf(hexByte, "%x", &n); err != nil {
				panic(err)
			}
			if n > 0xff {
				panic("value " + hexByte + " out of range")
			}
			msg = append(msg, byte(n))
		}
	} else if len(*fileFlag) != 0 {
		if aw == nil {
			panic("file dump only works with alsa midi")
		}
		f, err := os.Open(*fileFlag)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := fileDump(aw.seq, aw.sa, f); err != nil {
			panic(err)
		}
		return
	} else {
		m, err := io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		msg = m
		if len(msg) < 2 || msg[0] != midi.SysEx || msg[len(msg)-1] != midi.EndSysEx {
			panic("missing sysex start / end")
		}

	}
	if _, err := w.Write(msg); err != nil {
		panic(err)
	}
}
