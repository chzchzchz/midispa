// 1. json configuration loaded in
// 2. provided device file, opens it
// 3. virtual keyboard commands
// 4. virtual execution commands
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/util"
)

type DeviceConfig struct {
	alsa.SeqDevice
	Map  map[string]string
	keys map[string][]int
}

// aseqdump -l | grep 'X6mini MIDI' | awk ' { print $1 } '
func main() {
	fmt.Println("usage: midi2macro config.json [port]")
	aseq, err := alsa.OpenSeq("midi2macro")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()
	if len(os.Args) < 2 {
		if devs, err := aseq.Devices(); err == nil {
			for _, dev := range devs {
				fmt.Printf("%+v\n", dev)
			}
		}
		os.Exit(1)
	}

	cfgs := util.MustLoadJSONFile[DeviceConfig](os.Args[1])
	for i := range cfgs {
		cfgs[i].keys = make(map[string][]int)
		for k, v := range cfgs[i].Map {
			mk, err := MacroKeys(v)
			if err != nil {
				panic(err)
			}
			cfgs[i].keys[k] = mk
		}
	}
	for i, cfg := range cfgs {
		if cfg.PortName != "" {
			log.Println("using port name", cfg.PortName)
			cfgs[i].SeqAddr, err = aseq.PortAddress(cfg.PortName)
			if err != nil {
				panic(err)
			}
		}
		if cfgs[i].Client != 0 {
			log.Println("using port", cfgs[i].String())
			if err := aseq.OpenPort(cfgs[i].Client, cfgs[i].Port); err != nil {
				panic(err)
			}
		} else {
			panic("no port found")
		}
	}

	kbd, err := newKeyboard()
	if err != nil {
		panic(err)
	}
	defer kbd.Close()

	for {
		ev, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		s := ""
		for _, b := range ev.Data {
			s += fmt.Sprintf("%02x ", b)
		}
		s = strings.ToUpper(strings.TrimSpace(s))
		for _, cfg := range cfgs {
			if ev.Client != cfg.Client || ev.Port != cfg.Port {
				continue
			}
			if v, ok := cfg.keys[s]; ok {
				fmt.Println(s, "matched to", v)
				if err := kbd.Play(v); err != nil {
					panic(err)
				}
			} else {
				fmt.Println("got", s)
			}
			break
		}
	}
}
