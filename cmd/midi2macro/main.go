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

func mustLoadFile(path string) (cfgs []DeviceConfig) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	for dec.More() {
		cfg := DeviceConfig{Device: Device{Port: -1}}
		if err := dec.Decode(&cfg); err != nil {
			panic(err)
		}
		cfgs = append(cfgs, cfg)
	}
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
	return cfgs
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
	aseq, err := OpenAlsaSeq("midi2macro")
	if err != nil {
		panic(err)
	}
	defer aseq.Close()

	cfgs := mustLoadFile(os.Args[1])
	for i, cfg := range cfgs {
		if cfg.PortName != "" {
			log.Println("using port name", cfg.PortName)
			cfgs[i].Client, cfgs[i].Port, err = aseq.PortAddress(cfg.PortName)
			if err != nil {
				panic(err)
			}
		}
		if cfgs[i].Port > -1 {
			log.Println("using port", cfgs[i].PortString())
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
