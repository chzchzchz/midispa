// 1. json configuration loaded in
// 2. provided device file, opens it
// 3. virtual keyboard commands
// 4. virtual execution commands
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type DeviceConfig struct {
	Device
	Map  map[string]string
	keys map[string][]int
}

func mustLoadFile(path string) (cfg DeviceConfig) {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	cfg.Port = -1
	if err := json.Unmarshal(b, &cfg); err != nil {
		panic(err)
	}
	cfg.keys = make(map[string][]int)
	for k, v := range cfg.Map {
		mk, err := MacroKeys(v)
		if err != nil {
			panic(err)
		}
		cfg.keys[k] = mk
	}
	return cfg
}

// aseqdump -l | grep 'X6mini MIDI' | awk ' { print $1 } '
func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: midi2macro config.json [port]")
		aseq, err := OpenAlsaSeq("midi2macro")
		if err != nil {
			panic(err)
		}
		if devs, err := aseq.Devices(); err == nil {
			for _, dev := range devs {
				fmt.Printf("%+v\n", dev)
			}
		}
		os.Exit(1)
	}
	cfg := mustLoadFile(os.Args[1])
	aseq, err := OpenAlsaSeq("midi2macro")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	if cfg.PortName != "" {
		log.Println("using port name", cfg.PortName)
		if err := aseq.OpenPortName(cfg.PortName); err != nil {
			panic(err)
		}
	} else if cfg.Port > -1 {
		log.Println("using port", cfg.PortString())
		if err := aseq.OpenPort(cfg.Client, cfg.Port); err != nil {
			panic(err)
		}
	} else {
		panic("no device set in config")
	}

	kbd, err := newKeyboard()
	if err != nil {
		panic(err)
	}
	defer kbd.Close()

	for {
		b, err := aseq.Read()
		if err != nil {
			panic(err)
		}
		s := ""
		for _, b := range b {
			s += fmt.Sprintf("%02x ", b)
		}
		s = strings.ToUpper(strings.TrimSpace(s))
		if v, ok := cfg.keys[s]; ok {
			fmt.Println(s, "matched to", v)
			if err := kbd.Play(v); err != nil {
				panic(err)
			}
		} else {
			fmt.Println("got", s)
		}
	}
}
