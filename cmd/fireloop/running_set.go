package main

import (
	"maps"
	"slices"
)

// RunningSet is one running performance: a session per enabled page, the
// verbs the event loop drives it with, and the rule that keeps every
// session honest about the screen. It exists so the question "what does
// the running set do when the UI moves" has one home. A session sounds the
// page it was built for whether or not that page is still shown; it draws
// its playhead only while its page is shown; and its pattern source
// follows the live selection only while its page is shown, replaying the
// pattern it was playing once the view moves elsewhere. A page jump moves
// the screen and the banks between two of the workers' beats, so no
// session ever sees one without the other.
type RunningSet struct {
	controller *Controller
	// sessions is one entry per page the set plays. The set is built once
	// at Play and torn down at Stop, both on the event loop, and each
	// worker touches only its own session, so the map needs no lock of its
	// own.
	sessions map[int]*PlaybackSession
}

// newRunningSet starts a set over every enabled page, in page order, all
// launched together so every page starts on the same beat. The page on
// screen plays the live banks; the rest play what their own mementos point
// at until their page is shown again.
func newRunningSet(c *Controller, aseq sequencerWriter) *RunningSet {
	r := &RunningSet{controller: c, sessions: make(map[int]*PlaybackSession, len(c.pages))}
	for _, idx := range slices.Sorted(maps.Keys(c.pages)) {
		page := c.pages[idx]
		if !page.on {
			continue
		}
		session := newPlaybackSession(r.playbackFor(idx, page))
		session.Start(aseq)
		r.sessions[idx] = session
	}
	return r
}

// playing reports whether any session is still going. A worker only ends
// when it is stopped or its write fails, and a failed write can end one
// session while another keeps running, so the guards ask about all of
// them.
func (r *RunningSet) playing() bool {
	for _, session := range r.sessions {
		if session.Running() {
			return true
		}
	}
	return false
}

// stop ends every session and reports the first failure, the way stopping
// the set one session at a time would.
func (r *RunningSet) stop() error {
	var firstErr error
	for _, session := range r.sessions {
		if err := session.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// seek schedules a seek on every session, so the pages a set plays stay in
// sync, and reports the beat the shown page's session seeks away from for
// the arrangement's readout. Nothing is scheduled for a page with no
// session, and a shown page with none reports -1, since a live playhead is
// never behind zero.
func (r *RunningSet) seek(beat float32) float32 {
	shown, _ := r.controller.showing()
	old := float32(-1)
	for idx, session := range r.sessions {
		if idx == shown {
			old = session.SeekSongBeat(beat)
			continue
		}
		session.SeekSongBeat(beat)
	}
	return old
}

// holdBeatsWhile runs fn with every session's beat held, which is what a
// gesture that moves the screen and the banks together, or repaints the
// pads a running set also draws, takes: the in-flight beat finishes first
// and no new one starts until fn is done, so the workers never see a
// half-moved state and the pads never see two writers at once. With
// nothing running it is fn itself.
func (c *Controller) holdBeatsWhile(fn func() error) error {
	if c.running == nil {
		return fn()
	}
	for _, session := range c.running.sessions {
		session.playback.beatMu.Lock()
	}
	defer func() {
		for _, session := range c.running.sessions {
			session.playback.beatMu.Unlock()
		}
	}()
	return fn()
}

// playbackFor builds the playback a page's session sounds: the pattern its
// view points at on the pattern view, the song its view points at on the
// arrangement. The pattern source goes through patternSource and the
// playhead through drawnWhileShown, which is where both rules live.
func (r *RunningSet) playbackFor(idx int, page *Page) *Playback {
	if page.view == songView {
		p := r.controller.songbank.newPlaybackOf(page.song.Songs, page.song.SelSongIdx)
		p.updatePads = r.drawnWhileShown(idx, songView, p.updatePads)
		return p
	}
	p := r.controller.patbank.newPlaybackOf(r.patternSource(idx, page))
	p.updatePads = r.drawnWhileShown(idx, patternView, p.updatePads)
	return p
}

// patternSource answers the pattern a page's session plays: the bank's own
// selection while the page is the one on screen, so a pattern chosen while
// the set runs takes over at the next boundary, and the pattern it last
// answered once the view moves elsewhere — the one still sounding —
// because the selection then belongs to the page the user jumped to. The
// cache is touched only by the worker that calls it, so it needs no lock.
func (r *RunningSet) patternSource(idx int, page *Page) func() *Pattern {
	patterns := page.pattern.Patterns
	last := patterns[page.pattern.SelPatIdx]
	return func() *Pattern {
		if shown, _ := r.controller.showing(); shown == idx {
			last = r.controller.patbank.CurrentPattern()
		}
		return last
	}
}

// drawnWhileShown wraps a session's playhead so it draws only while the
// screen shows the page and the view the session plays for. Page mode and
// the other view own the pads otherwise, which is what lets a set survive
// a walk onto the page grid, a jump, and a view toggle without ending.
func (r *RunningSet) drawnWhileShown(idx int, view viewMode, draw func(float32) error) func(float32) error {
	return func(beat float32) error {
		shown, shownView := r.controller.showing()
		if shown != idx || shownView != view {
			return nil
		}
		return draw(beat)
	}
}
