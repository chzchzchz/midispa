package sysex

import (
	"math"
	"time"
)

const (
	ReverbTypeSmallRoom  = 0
	ReverbTypeMediumRoom = 1
	ReverbTypeLargeRoom  = 2
	ReverbTypeMediumHall = 3
	ReverbTypeLargeHall  = 4
	ReverbTypePlate      = 8
)

const (
	ReverbParameterType = 0
	ReverbParameterTime = 1
)

type ReverbParameters struct {
	DeviceId  int
	Parameter int
	Value     int
}

func (r *ReverbParameters) Encode() []byte {
	return []byte{
		0xf0, IdRealTime, byte(r.DeviceId),
		SubIdDeviceControl, DeviceControlIdGlobalParameterControl,
		1, 1, 1, /* slot, param id, val wid */
		1, 1, /* Effect 0101: reverb */
		byte(r.Parameter),
		byte(r.Value),
		0xf7,
	}
}

type ReverbType struct {
	DeviceId int
	Type     int
}

func (rt *ReverbType) Encode() []byte {
	rp := ReverbParameters{
		DeviceId:  rt.DeviceId,
		Parameter: ReverbParameterType,
		Value:     rt.Type,
	}
	return rp.Encode()
}

type ReverbTime struct {
	DeviceId int
	Time     time.Duration
}

func (rt *ReverbTime) Encode() []byte {
	rp := ReverbParameters{
		DeviceId:  rt.DeviceId,
		Parameter: ReverbParameterTime,
		Value:     int(math.Log(rt.Time.Seconds())/0.025 + 40),
	}
	return rp.Encode()
}
