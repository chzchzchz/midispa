package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/chzchzchz/midispa/sysex"
	"github.com/chzchzchz/midispa/sysex/sr16"
)

var reflectMap = map[string]interface{}{
	"sysex/sr16/DrumSet":        &sr16.DrumSet{},
	"sysex/sr16/DrumSetRequest": &sr16.DrumSetRequest{},
	"sysex/MasterBalance":       &sysex.MasterBalance{},
	"sysex/MasterVolume":        &sysex.MasterVolume{},
}

func loadReflectedJson(typePath []string, r io.Reader) (interface{}, error) {
	ty := strings.Join(typePath, "/")
	iface, ok := reflectMap[ty]
	if !ok {
		return nil, fmt.Errorf("unknown type %s", ty)
	}
	ifaceCopy := reflect.ValueOf(iface).Interface()

	// TODO: if cc, read into map and return specific CC
	dec := json.NewDecoder(r)
	if err := dec.Decode(ifaceCopy); err != nil {
		return nil, err
	}
	return ifaceCopy, nil
}
