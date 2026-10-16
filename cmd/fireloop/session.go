package main

import (
	"context"
	"log"
	"time"
)

// PlaybackSession is one running set: the worker sounding it, the handle that ends it,
// and what it ended with.
//
// It exists because "is anything playing" had three answers in this package, a stop handle
// on the controller and a *Playback on each bank, and nothing forced them to agree. The
// guards read one of them and the cleanup walked the other two, so a set that had already
// ended could still be believed running, and a note could be left sounding by whichever
// record the caller did not think to look at. There is one session now and the controller
// owns it, so the question has one place to ask and one place to answer.
//
// One goroutine starts a session and stops it, and the worker only writes to it, so it
// needs no lock of its own. Done and Err are for whoever watches rather than calls, and
// the close of Done is what orders the worker's write before such a reader sees it.
type PlaybackSession struct {
	playback *Playback
	stop     playbackStopFunc
	done     chan struct{}
	err      error
}

// newPlaybackSession wraps a playback that has not started. Starting and stopping are
// separate from building it, because a bank builds what to play and the controller decides
// that something is.
func newPlaybackSession(playback *Playback) *PlaybackSession {
	return &PlaybackSession{playback: playback, done: make(chan struct{})}
}

// startWithSettle launches a session whose kit has already been sent, with
// the settle that one send asked for. A missing writer still makes a
// session, the way the test seam's Start does, and nothing is sent again.
func (s *PlaybackSession) startWithSettle(aseq sequencerWriter, settle time.Duration) {
	s.launch(aseq, settle)
}

// launch runs the worker. A missing writer still makes a session: reset and
// closed, with nothing running behind it but something there to answer
// Stop. A dump is not a channel message, so the worker waits out the settle
// the kit's send asked for before it plays anything.
func (s *PlaybackSession) launch(aseq sequencerWriter, settle time.Duration) {
	playback := s.playback
	playback.reset()
	playback.writer = aseq
	playback.settle = settle
	if isNilMidiWriter(aseq) {
		close(s.done)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	// The clock is handed to the loop rather than left on the playback, so where the time
	// comes from is an argument at every call rather than a field a test has to remember
	// to set and a nil check has to cover when it forgets.
	clock := &realStepClock{}
	s.stop = func() error {
		cancel()
		<-s.done
		return s.err
	}
	go func() {
		s.err = playback.run(ctx, aseq, clock)
		if s.err != nil && ctx.Err() == nil {
			log.Printf("fireloop playback stopped: %v", s.err)
		}
		close(s.done)
	}()
}

// Stop ends the set and waits for the worker to finish, reporting what it ended with. It
// is safe to call more than once and gives the same answer each time: cancelling twice is
// free and the worker has ended either way. There is nothing extra to silence here, because
// the worker releases its own notes on the way out and its exit carries whatever that cost.
func (s *PlaybackSession) Stop() error {
	if s.stop == nil {
		return nil
	}
	return s.stop()
}

// Running reports whether the worker is still going. A set that ended on its own is not
// running even though the session is still installed, and that is the difference the
// guards need: an installed session no longer passes for a playing one.
func (s *PlaybackSession) Running() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// Done is closed once the worker has ended and written what it ended with.
func (s *PlaybackSession) Done() <-chan struct{} {
	return s.done
}

// Err is what the set ended with. It is only settled once Done is closed or Stop has
// returned, which is the only time anything writes it.
func (s *PlaybackSession) Err() error {
	return s.err
}

// SeekSongBeat schedules a seek to the next pattern boundary and reports the beat it is
// seeking away from. It is the one thing a bank needs the running set for that is not
// stopping it, and it is why a bank can reach its owner for a seek without holding one.
func (s *PlaybackSession) SeekSongBeat(beat float32) float32 {
	return s.playback.JumpSongBeat(beat)
}
