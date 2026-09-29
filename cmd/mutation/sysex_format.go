package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/sysex/behringer/pro800"
	dx7 "github.com/chzchzchz/midispa/sysex/yamaha/dx7"
)

// sysexFormat is the vendor boundary for one SysEx message kind. It owns the
// root type, the wire codec, and the file the dumps are written to. The
// engine only ever sees reflected gene values, so a new vendor is a new
// implementation of this interface rather than a new patch type.
type sysexFormat interface {
	ID() string
	NewRoot() any
	Decode(message []byte) (any, error)
	Encode(root any) ([][]byte, error)
	// OutputExtension is the suffix a raw dump of this message kind uses.
	OutputExtension() string
}

// programFormat is the shape every format in this package happens to have: one
// decoded program carried in one SysEx message, marshalled by the vendor
// package's own codec. Stating that once means a format is a table of what
// actually differs, which for these two is the program type, the codec, and
// whether the message carries a channel to apply.
//
// A format that needs more than this — several messages, or a program that is
// not a single struct — implements sysexFormat directly instead.
type programFormat[T any] struct {
	formatID string
	// channel is the zero-based channel to apply, and is only read when
	// applyChannel is set.
	channel int
	// decode and encode are the vendor codec's own methods, taken as method
	// expressions so that a format does not restate what they validate.
	decode func(program *T, message []byte) error
	encode func(program *T) ([]byte, error)
	// applyChannel stamps the configured channel onto a program. A message
	// with no channel byte leaves this nil and the program as decoded.
	applyChannel func(program *T, channel int)
}

func (format programFormat[T]) ID() string { return format.formatID }

func (format programFormat[T]) OutputExtension() string { return sysexOutputExtension }

func (format programFormat[T]) NewRoot() any { return new(T) }

func (format programFormat[T]) Decode(message []byte) (any, error) {
	program := new(T)
	if err := format.decode(program, message); err != nil {
		return nil, err
	}
	if format.applyChannel != nil {
		format.applyChannel(program, format.channel)
	}
	return program, nil
}

// Encode copies before stamping the channel, so a patch a seed supplied keeps
// the address or channel it was read with and only the caller decides a
// different one.
func (format programFormat[T]) Encode(root any) ([][]byte, error) {
	program, ok := root.(*T)
	if !ok {
		return nil, fmt.Errorf("format %s expects a %T, got %T", format.formatID, program, root)
	}
	stamped := *program
	if format.applyChannel != nil {
		format.applyChannel(&stamped, format.channel)
	}
	message, err := format.encode(&stamped)
	if err != nil {
		return nil, err
	}
	return [][]byte{message}, nil
}

const sysexOutputExtension = ".syx"

// newDX7Format speaks the 155-byte one-voice edit-buffer message. A bulk dump
// would overwrite all 32 internal voices, so this format cannot emit one. The
// message's channel byte is the routing, and the record marks it as not a
// sound, so stamping it is the format's job rather than a gene's.
func newDX7Format(channel int) sysexFormat {
	return programFormat[dx7.SingleVoice]{
		formatID: dx7SingleFormatName,
		channel:  channel,
		decode:   (*dx7.SingleVoice).UnmarshalBinary,
		encode:   (*dx7.SingleVoice).MarshalBinary,
		applyChannel: func(voice *dx7.SingleVoice, channel int) {
			voice.Channel = channel
		},
	}
}

// newPro800Format speaks the patch dump the instrument sends and accepts. The
// dump carries no channel byte: the address says which slot the panel shows,
// and the record marks it as not a sound.
//
// The instrument's message set offers one patch message in each direction, a
// request and a response, and a candidate goes out as the response, which is
// the whole of what can be written to it. That the instrument acts on a
// response it did not ask for is the one thing here a test cannot show and a
// person with the hardware has to confirm.
func newPro800Format() sysexFormat {
	return programFormat[pro800.PatchData]{
		formatID: pro800FormatName,
		decode:   (*pro800.PatchData).UnmarshalBinary,
		encode:   (*pro800.PatchData).MarshalBinary,
	}
}
