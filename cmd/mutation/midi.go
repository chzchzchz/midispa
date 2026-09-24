package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
)

const (
	alsaClientName    = "mutation"
	sustainController = 64
)

func openMIDIOutput(portName string) (io.Writer, io.Closer, error) {
	if strings.TrimSpace(portName) == "" {
		return nil, nil, fmt.Errorf("MIDI port is empty")
	}
	sequence, err := alsa.OpenSeq(alsaClientName)
	if err != nil {
		return nil, nil, fmt.Errorf("open ALSA sequencer: %w", err)
	}
	address, err := sequence.PortAddress(portName)
	if err != nil {
		sequence.Close()
		return nil, nil, fmt.Errorf("find MIDI port %q: %w", portName, err)
	}
	if err := sequence.OpenPortWrite(address); err != nil {
		sequence.Close()
		return nil, nil, fmt.Errorf("open MIDI port %q: %w", portName, err)
	}
	return sequence.NewWriter(address), sequence, nil
}

type midiPlayer struct {
	output  io.Writer
	channel int
	now     func() time.Time
	waitFor func(context.Context, time.Duration) error
}

func newMIDIPlayerForChannel(output io.Writer, channelNumber int) *midiPlayer {
	return &midiPlayer{
		output:  output,
		channel: channelNumber - 1,
		now:     time.Now,
		waitFor: waitForMIDI,
	}
}

func (player *midiPlayer) auditionChannels(playback *track.Pattern) []int {
	channels := []int{player.channel}
	if playback == nil {
		return channels
	}
	seen := map[int]bool{player.channel: true}
	for _, message := range playback.Msgs {
		if len(message.Raw) == 0 || (!midi.IsNoteOn(message.Raw[0]) && !midi.IsNoteOff(message.Raw[0])) {
			continue
		}
		channel := midi.Channel(message.Raw[0])
		if !seen[channel] {
			seen[channel] = true
			channels = append(channels, channel)
		}
	}
	return channels
}

func (player *midiPlayer) resetChannels(channels []int) error {
	cleanupErrors := make([]error, 0, len(channels)*2)
	for _, channel := range channels {
		sustainOff := []byte{midi.MakeCC(channel), sustainController, 0}
		if err := player.send(sustainOff); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("channel %d sustain off: %w", channel+1, err))
		}
		allNotesOff := []byte{midi.MakeCC(channel), midi.AllNotesOff, 0}
		if err := player.send(allNotesOff); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("channel %d all notes off: %w", channel+1, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

// Reapply the whole patch before every audition so hardware state from the
// previous candidate cannot bias the next comparison. Resetting before and
// after playback also releases sustain and recovers from interrupted notes.
func (player *midiPlayer) audition(ctx context.Context, patch *Patch, playback *track.Pattern) (err error) {
	channels := player.auditionChannels(playback)
	defer func() {
		err = errors.Join(err, player.resetChannels(channels))
	}()
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err = player.resetChannels(channels); err != nil {
		return err
	}
	messages, err := patch.controlChanges(player.channel)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		if err := player.send(message); err != nil {
			return fmt.Errorf("send patch CC: %w", err)
		}
	}
	if playback == nil {
		return player.playProbeNotes(ctx)
	}
	return player.playPattern(ctx, playback)
}

func (player *midiPlayer) send(message []byte) error {
	written, err := player.output.Write(message)
	if err != nil {
		return err
	}
	if written != len(message) {
		return io.ErrShortWrite
	}
	return nil
}
