package main

import (
	"encoding/json"
	"os"

	"github.com/chzchzchz/midispa/cc"
)

type DeviceModel struct {
	Device  string
	Name    string
	Channel int
	cc.Model
}

func mustLoadDeviceModels(path string) (m []DeviceModel) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		m = append(m, DeviceModel{})
		mm := &m[len(m)-1]
		if err := dec.Decode(mm); err != nil {
			panic(err)
		}
		if mm.Channel == 0 {
			mm.Channel = 1
		}
	}
	return m
}

func mustSaveDeviceModels(dms []DeviceModel, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	for _, dm := range dms {
		if err := enc.Encode(dm); err != nil {
			panic(err)
		}
	}
}
