package main

import (
	"encoding/json"
	"os"

	"github.com/chzchzchz/midispa/cc"
	"github.com/chzchzchz/midispa/util"
)

type DeviceModel struct {
	Device  string
	Name    string
	Channel int
	cc.Model
}

func mustLoadDeviceModels(path string) (m []DeviceModel) {
	m = util.MustLoadJSONFile[DeviceModel](path)
	for i := range m {
		if m[i].Channel == 0 {
			m[i].Channel = 1
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
