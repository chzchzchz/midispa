package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
)

func main() {
	strFlag := flag.String("s", "", "hex message to send (e.g., \"F0 A1 2B F7\")")
	portFlag := flag.String("p", "", "destination port")

	flag.Parse()

	aseq, err := alsa.OpenSeq("midisend")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	if len(*portFlag) == 0 {
		panic("expected -p port flag")
	}
	if len(*strFlag) == 0 {
		panic("expected -s hex flag")
	}
	sa, err := aseq.PortAddress(*portFlag)
	if err != nil {
		panic(err)
	}
	if err := aseq.OpenPortWrite(sa); err != nil {
		panic(err)
	}
	var msg []byte
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
	if err := aseq.Write(alsa.SeqEvent{sa, msg}); err != nil {
		panic(err)
	}
}
