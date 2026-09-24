package main

import (
	"context"
	"io"

	"github.com/chzchzchz/midispa/track"
)

// The runner depends on these boundaries rather than ALSA or SMF implementations.
type auditioner interface {
	audition(context.Context, *Patch) error
}

type patchStore interface {
	path() string
	save(*Patch, int) error
}

// midiOutputFactory defers ALSA setup until the command has validated its configuration.
type midiOutputFactory func(string) (io.Writer, io.Closer, error)

type midiAuditioner struct {
	player   *midiPlayer
	playback *track.Pattern
}

func (audition midiAuditioner) audition(ctx context.Context, patch *Patch) error {
	return audition.player.audition(ctx, patch, audition.playback)
}
