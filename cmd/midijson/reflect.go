package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/chzchzchz/midispa/sysex"
	"github.com/chzchzchz/midispa/sysex/sr16"
)

var reflectMap = map[string]interface{}{
	"sysex/sr16/DrumSet":        &sr16.DrumSet{},
	"sysex/sr16/DrumSetRequest": &sr16.DrumSetRequest{},
	"sysex/sr16/DumpRequest":    &sr16.DumpRequest{},
	"sysex/sr16/Dump":           &sr16.Dump{},
	"sysex/SysEx":               &sysex.SysEx{},
	"sysex/MasterBalance":       &sysex.MasterBalance{},
	"sysex/MasterVolume":        &sysex.MasterVolume{},
}

func readReflectedJson(ty string, r io.Reader) (interface{}, error) {
	ifaceCopy, err := copyTypeInterface(ty)
	if err != nil {
		return nil, err
	}
	// TODO: if cc, read into map and return specific CC
	dec := json.NewDecoder(r)
	if err := dec.Decode(ifaceCopy); err != nil {
		return nil, err
	}
	return ifaceCopy, nil
}

func copyTypeInterface(ty string) (interface{}, error) {
	iface, ok := reflectMap[ty]
	if !ok {
		return nil, fmt.Errorf("unknown type %s", ty)
	}
	return reflect.ValueOf(iface).Interface(), nil
}
