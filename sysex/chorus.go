package sysex

import (
	"time"
)

const (
	ChorusType1        = 0
	ChorusType2        = 1
	ChorusType3        = 2
	ChorusType4        = 3
	ChorusTypeFBChorus = 4
	ChorusTypeFlanger  = 8
)

const (
	ChorusParameterType         = 0
	ChorusParameterModRate      = 1
	ChorusParameterModDepth     = 2
	ChorusParameterFeedback     = 3
	ChorusParameterSendToReverb = 4
)

type ChorusParameters struct {
	DeviceId  int
	Parameter int
	Value     int
}

func (r *ChorusParameters) Encode() []byte {
	return []byte{
		0xf0, IdRealTime, byte(r.DeviceId),
		SubIdDeviceControl, DeviceControlIdGlobalParameterControl,
		1, 1, 1, /* slot, param id, val wid */
		1, 2, /* Effect 0102: chorus */
		byte(r.Parameter),
		byte(r.Value),
		0xf7,
	}
}

type ChorusType struct {
	DeviceId int
	Type     int
}

func (rt *ChorusType) Encode() []byte {
	rp := ChorusParameters{
		DeviceId:  rt.DeviceId,
		Parameter: ChorusParameterType,
		Value:     rt.Type,
	}
	return rp.Encode()
}

type ChorusModRate struct {
	DeviceId int
	ModRate  float32 // hz; * 0.122
}

func (cm *ChorusModRate) Encode() []byte {
	cp := ChorusParameters{
		DeviceId:  cm.DeviceId,
		Parameter: ChorusParameterModRate,
		Value:     int(cm.ModRate / 0.122),
	}
	return cp.Encode()
}

type ChorusModDepth struct {
	DeviceId int
	ModDepth time.Duration // val + 1 / 3.2; peak-to-peak swing time in ms
}

func (cm *ChorusModDepth) Encode() []byte {
	cp := ChorusParameters{
		DeviceId:  cm.DeviceId,
		Parameter: ChorusParameterModDepth,
		Value:     int(float32(cm.ModDepth/time.Millisecond) / 3.2),
	}
	return cp.Encode()
}

type ChorusFeedback struct {
	DeviceId int
	Feedback float32 // val * 0.763; pct
}

func (rt *ChorusFeedback) Encode() []byte {
	rp := ChorusParameters{
		DeviceId:  rt.DeviceId,
		Parameter: ChorusParameterFeedback,
	}
	return rp.Encode()
}

type ChorusSendToReverb struct {
	DeviceId     int
	SendToReverb float32 // val * 0.787; send level in pct
}

func (rt *ChorusSendToReverb) Encode() []byte {
	rp := ChorusParameters{
		DeviceId:  rt.DeviceId,
		Parameter: ChorusParameterSendToReverb,
		Value:     int((rt.SendToReverb * 100.0) / 0.787),
	}
	return rp.Encode()
}
