package main

import (
	"unsafe"

	"github.com/xthexder/go-jack"
)

func audioSampleToFloat32(buf []jack.AudioSample) []float32 {
	return *(*[]float32)(unsafe.Pointer(&buf))
}

func clearBuffer(buf []jack.AudioSample) {
	for i := range buf {
		buf[i] = 0
	}
}
