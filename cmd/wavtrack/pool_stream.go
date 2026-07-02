package main

import (
	"context"

	"github.com/xthexder/go-jack"
)

type PoolStream interface {
	Chan() chan<- []jack.AudioSample
	Buffer(ctx context.Context) []jack.AudioSample
}
