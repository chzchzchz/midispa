package sysex

import (
	"errors"
)

var ErrBadRange = errors.New("bad value range")
var ErrBadHeader = errors.New("bad header")
var ErrNoEox = errors.New("no EOX")
var ErrBadSubId = errors.New("bad sub id")
var ErrBadType = errors.New("bad type")
var ErrDataTooLarge = errors.New("too much data to send")
