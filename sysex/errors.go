package sysex

import (
	"errors"
)

var ErrBadRange = errors.New("bad value range")
var ErrBadHeader = errors.New("bad header")
var ErrNoEox = errors.New("no EOX")
