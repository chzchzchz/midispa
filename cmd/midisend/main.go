package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzchzchz/midispa/midi"
)

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	panic("dead")
}

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
		var err error
		aw, err = newAlsaWriter(*aportFlag)
		if err != nil {
			die("alsa writer: %v", err)
		}
		w, c = aw, aw
	} else if len(*jportFlag) != 0 {
		var err error
		var jw *jackWriter
		jw, err = newJackWriter(*jportFlag)
		if err != nil {
			die("jack writer: %v", err)
		}
		w, c = jw, jw
	} else {
		die("expected -j or -p")
	}
	defer c.Close()

	var msg []byte
	if len(*strFlag) != 0 {
		for _, hexByte := range strings.Fields(*strFlag) {
			if len(hexByte) != 2 {
				die("malformed hex string on byte %s", hexByte)
			}
			n := 0
			if _, err := fmt.Sscanf(hexByte, "%x", &n); err != nil {
				die("bad hex byte %s: %v", hexByte, err)
			}
			if n > 0xff {
				die("value %s out of range", hexByte)
			}
			msg = append(msg, byte(n))
		}
	} else if len(*fileFlag) != 0 {
		if aw == nil {
			die("file dump only works with alsa midi")
		}
		f, err := os.Open(*fileFlag)
		if err != nil {
			die("open file: %v", err)
		}
		defer f.Close()
		if err := fileDump(aw.seq, aw.sa, f); err != nil {
			die("file dump: %v", err)
		}
		return
	} else {
		m, err := io.ReadAll(os.Stdin)
		if err != nil {
			die("read stdin: %v", err)
		}
		msg = m
		if len(msg) < 2 || msg[0] != midi.SysEx || msg[len(msg)-1] != midi.EndSysEx {
			die("missing sysex start / end")
		}
	}
	if _, err := w.Write(msg); err != nil {
		die("write: %v", err)
	}
}
