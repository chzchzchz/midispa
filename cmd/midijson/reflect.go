package main

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/chzchzchz/midispa/sysex"
	"github.com/chzchzchz/midispa/sysex/keystep37"
	"github.com/chzchzchz/midispa/sysex/sr16"
	"github.com/chzchzchz/midispa/sysex/stanton"
)

var reflectMap = map[string]interface{}{
	"sysex/sr16/DrumSet":                    &sr16.DrumSet{},
	"sysex/sr16/DrumSetRequest":             &sr16.DrumSetRequest{},
	"sysex/sr16/DumpRequest":                &sr16.DumpRequest{},
	"sysex/sr16/Dump":                       &sr16.Dump{},
	"sysex/SysEx":                           &sysex.SysEx{},
	"sysex/MasterBalance":                   &sysex.MasterBalance{},
	"sysex/MasterVolume":                    &sysex.MasterVolume{},
	"sysex/keystep37/PatternsDump":          &keystep37.PatternsDump{},
	"sysex/DeviceInquiryRequest":            &sysex.DeviceInquiryRequest{DeviceId: sysex.DeviceInquiryCallAll},
	"sysex/DeviceInquiryResponse":           &sysex.DeviceInquiryResponse{},
	"sysex/stanton/scs3d/SlidersMode":       &stanton.SlidersMode{},
	"sysex/stanton/scs3d/ButtonsMode":       &stanton.ButtonsMode{},
	"sysex/stanton/scs3d/ButtonsLeftMode":   &stanton.ButtonsLeftMode{},
	"sysex/stanton/scs3d/ButtonsRightMode":  &stanton.ButtonsRightMode{},
	"sysex/stanton/scs3d/CompatibilityMode": &stanton.CompatibilityMode{},
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
