package main

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"time"
)

// RunningSet is one running performance: a session per enabled page, the
// verbs the event loop drives it with, and the rule that keeps every
// session honest about the screen. It exists so the question "what does
// the running set do when the UI moves" has one home. A session sounds the
// page it was built for whether or not that page is still shown; its
// playhead is drawn only by the set's renderer, which draws only the page
// the screen shows; and its pattern source follows the live selection only
// while its page is shown, replaying the pattern it was playing once the
// view moves elsewhere. A page jump moves the screen and the banks between
// two of the workers' beats, so no session ever sees one without the other.
type RunningSet struct {
	controller *Controller
	// sessions is one entry per page the set plays. The set is built once
	// at Play and torn down at Stop, both on the event loop, and each
	// worker touches only its own session's playback, so the map needs no
	// lock of its own.
	sessions map[int]*PlaybackSession
	// playheads is the renderer's bookkeeping, one entry per session,
	// created with the set so a new Play starts with clean playhead state.
	// The renderer runs on the event loop, the same goroutine that builds
	// and tears the set down, so this map is touched from nowhere else.
	playheads map[int]*playheadState
}

// playheadState is one session's rendered position: the view the session was
// built for, and where the renderer last drew. The initial values reproduce
// the first render the drawing closures made: the pattern playhead opens by
// restoring column 15 and lighting column 0, and the arrangement's sentinel
// makes its first render a no-op, the current measure comparing equal to
// itself.
type playheadState struct {
	view         viewMode
	lastColumn   int
	lastSongBeat float32
}

// noRenderedSongBeat stands for "the arrangement playhead has drawn
// nothing", a beat no song position can hold.
const noRenderedSongBeat float32 = -99999

func newPlayheadState(view viewMode) *playheadState {
	return &playheadState{view: view, lastColumn: padColumns - 1, lastSongBeat: noRenderedSongBeat}
}

// newRunningSet starts a set over every enabled page, in page order, all
// launched together so every page starts on the same beat. The page on
// screen plays the live banks; the rest play what their own mementos point
// at until their page is shown again. The kit's patches go out once for the
// whole set and every session shares the settle that send asked for, so a
// Play press costs one kit's worth of wire time however many pages play,
// and the first note still waits for the dump.
func newRunningSet(c *Controller, aseq sequencerWriter) *RunningSet {
	r := &RunningSet{
		controller: c,
		sessions:   make(map[int]*PlaybackSession, len(c.pages)),
		playheads:  make(map[int]*playheadState, len(c.pages)),
	}
	// Every session plays the same kit — the banks all point at it — so the
	// send belongs to the set rather than to any one session. A session
	// started with nowhere to write sends nothing, and neither does the set.
	var settle time.Duration
	if !isNilMidiWriter(aseq) {
		var err error
		settle, err = sendKitPatches(aseq, c.patbank.vb)
		if err != nil {
			logger.Error("kit patches", "error", err)
		}
	}
	for _, idx := range slices.Sorted(maps.Keys(c.pages)) {
		page := c.pages[idx]
		if !page.on {
			continue
		}
		session := newPlaybackSession(r.playbackFor(idx, page))
		session.startWithSettle(aseq, settle)
		r.sessions[idx] = session
		r.playheads[idx] = newPlayheadState(page.view)
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
// gesture that moves the screen and the banks together takes: the in-flight
// beat finishes first and no new one starts until fn is done, so the
// workers never see a half-moved state. With nothing running it is fn
// itself. The sessions are captured before fn runs, so what gets locked is
// exactly what gets unlocked however fn leaves the set.
func (c *Controller) holdBeatsWhile(fn func() error) error {
	if c.running == nil {
		return fn()
	}
	running := c.running
	for _, session := range running.sessions {
		session.playback.beatMu.Lock()
	}
	defer func() {
		for _, session := range running.sessions {
			session.playback.beatMu.Unlock()
		}
	}()
	return fn()
}

// playbackFor builds the playback a page's session sounds: the pattern its
// view points at on the pattern view, the song its view points at on the
// arrangement. The pattern source goes through patternSource. The playback
// publishes its beats to the controller's wake; whether a playhead is
// wanted is answered at render time, by the set's renderer.
func (r *RunningSet) playbackFor(idx int, page *Page) *Playback {
	var p *Playback
	if page.view == songView {
		p = r.controller.songbank.newPlaybackOf(page.song.Songs, page.song.SelSongIdx)
	} else {
		p = r.controller.patbank.newPlaybackOf(r.patternSource(idx, page))
	}
	p.wake = r.controller.wake
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

// renderPlayheads draws every session's latest beat for the page it plays.
// It runs on the event loop, which makes it the pads' and OLED rows' only
// writer while a set plays, and it applies the shown-page-and-view rule
// here rather than at publish time, so a playhead published before a jump
// discards itself: the screen it is checked against has already moved. A
// wake that outlives its set finds the live sessions here or nothing at
// all, and a session whose worker has ended re-renders the position it
// already shows, which every playhead answers with a no-op.
func (r *RunningSet) renderPlayheads() {
	shown, shownView := r.controller.showing()
	for idx, session := range r.sessions {
		state, ok := r.playheads[idx]
		if !ok {
			// Hand-built sets in tests carry no playhead bookkeeping; a set
			// from newRunningSet pairs the two maps key for key.
			continue
		}
		if shown != idx || shownView != state.view {
			continue
		}
		beat := math.Float32frombits(session.playback.latestBeat.Load())
		if state.view == songView {
			r.renderSongPlayhead(state, beat)
			continue
		}
		r.renderPatternPlayhead(state, beat)
	}
}

// renderPatternPlayhead is the pattern view's playhead, lifted from the
// drawing closure the worker used to call: in note-edit mode it moves the
// step strip, and otherwise it restores the column it last lit before
// inverting the newest one. Either way it restores exactly the cell it last
// lit, so a beat the renderer skipped strands nothing. The bookkeeping only
// moves once a render's writes have all landed, so a failed write is
// retried by the next beat's render.
func (r *RunningSet) renderPatternPlayhead(state *playheadState, beat float32) {
	pb := r.controller.patbank
	if pb.noteEditActive() {
		// Note editing owns the grid with the pitch palette, so the playhead moves
		// along the step strip rather than over the palette.
		if err := pb.drawStepPlayhead(beatStep(beat)); err != nil {
			logger.Error("playhead strip", "error", err)
		}
		return
	}
	thisColumn := beatStep(beat) % padColumns
	if thisColumn == state.lastColumn {
		// The display already shows this position.
		return
	}
	if err := pb.drawPadColumn(state.lastColumn); err != nil {
		logger.Error("playhead restore", "error", err)
		return
	}
	if err := pb.drawPadColumnInvert(thisColumn); err != nil {
		logger.Error("playhead column", "error", err)
		return
	}
	state.lastColumn = thisColumn
}

// renderSongPlayhead is the arrangement's playhead, lifted from the drawing
// closure the worker used to call: it moves the lit measure and names the
// measure and the pattern it reached on the rows below the header. A failed
// write is logged and left behind, the way the pattern playhead's is; the
// bookkeeping has already moved, so the next measure picks the drawing up.
func (r *RunningSet) renderSongPlayhead(state *playheadState, beat float32) {
	sb := r.controller.songbank
	s := sb.CurrentSong()
	if s == nil {
		return
	}
	_, lastMeasure := s.BeatToPattern(state.lastSongBeat)
	pat, givenMeasure := s.BeatToPattern(beat)
	if givenMeasure < 0 {
		return
	}
	state.lastSongBeat = beat
	if lastMeasure == givenMeasure {
		return
	}
	if err := sb.ToggleMeasureBrightness(lastMeasure, givenMeasure); err != nil {
		logger.Error("measure playhead", "error", err)
		return
	}
	if err := sb.printRow(4, fmt.Sprintf("Measure %03d", givenMeasure+1)); err != nil {
		logger.Error("measure readout", "error", err)
		return
	}
	pidx := sb.pb.PatternIdxMap()[pat]
	if err := sb.printRow(5, fmt.Sprintf("Pat-bar %03d", pidx)); err != nil {
		logger.Error("pattern readout", "error", err)
	}
}
