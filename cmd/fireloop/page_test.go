package main

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pagePadNote is the Fire note of the pad page idx sits on. Pad n, counting
// from zero, is note 54+n, so page idx is note 53+idx: page 1 is the pad at
// the top left of the grid.
func pagePadNote(idx int) int {
	return 53 + idx
}

// The flow the page grid is for: a song is playing and the grid is wanted.
// Browser enters page mode without ending the set, the measure playhead
// yields to the grid rather than painting over it, the grid does its work
// while the set plays, and Browser again returns to the arrangement still
// playing. The sequencer is started for real here, so a regression that
// ends the set, or one that lets the playhead paint the page grid, is
// heard or seen rather than inferred.
func TestPageModeKeepsASongSetPlaying(t *testing.T) {
	controller, recorder, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	songPads := usePadRecorder(t, &songs.pads)
	setBPM(300)

	// A two-measure song, so the measure playhead crosses a boundary while
	// the page grid is being watched.
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, bank.vb.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(pattern, 0)
	songs.CurrentSong().SetPattern(pattern, 1)

	instrument := newSoundingInstrument()
	capture := &captureMidiWriter{}
	fanout := &fanoutWriter{writers: []sequencerWriter{capture, instrument}}

	// The song plays from the arrangement, which is the view the flow
	// this test guards starts from.
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())

	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)
	require.True(t, controller.playing(), "entering page mode stopped the set")

	// The page grid owns the pads while it shows, so give the playhead the
	// time to cross its boundary and try: not one pad may move, and the
	// measure readout may not come back over the grid's rows.
	grid := songPads.pads
	waitFor(t, "the set to cross a measure boundary", func() bool {
		ons := 0
		for _, event := range capture.snapshot() {
			if len(event.Data) == 3 && midi.IsNoteOn(event.Data[0]) &&
				event.Data[2] > 0 && int(event.Data[1]) == 60 {
				ons++
			}
		}
		return ons >= 2
	})
	require.Equal(t, grid, songPads.pads, "the playhead painted over the page grid")
	require.NotContains(t, recorder.row(4), "Measure", "the playhead wrote over the page grid's rows")

	// The grid does its work while the set plays: a press creates a page.
	pressButton(t, bank, pagePadNote(2))
	require.NotNil(t, controller.pages[2], "the grid press created no page")
	require.True(t, controller.playing(), "a page gesture stopped the set")

	// And leaving returns to the arrangement, still playing.
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, songView, controller.mode)
	require.True(t, controller.playing(), "leaving page mode stopped the set")
	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")
}

// The flow the page table is for: a song is playing and another page is
// wanted. A double tap on the page's pad jumps to it without ending the
// set, every session goes on sounding its own page across the jump, and
// the jump back leaves the same set running. The sequencer is started for
// real here, so a regression that ends the set is heard rather than
// inferred.
func TestPageJumpKeepsTheSongSetPlaying(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	setBPM(300)

	// Two enabled pages, each left on the arrangement with a song of its
	// own, so a set over both sounds two notes the wire tells apart.
	first := bank.CurrentPattern()
	first.SetChromaticNote(0, bank.vb.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(first, 0)

	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	songs.CurrentSong().SetPattern(second, 0)
	pressButton(t, bank, NotePatternSong) // page 2 onto the arrangement
	doubleTapPage(t, controller, 1)       // back to page 1
	pressButton(t, bank, NotePatternSong) // arrangement

	instrument := newSoundingInstrument()
	capture := &captureMidiWriter{}
	fanout := &fanoutWriter{writers: []sequencerWriter{capture, instrument}}
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 2, "both pages should play")

	ons := func(note int) int { return countNoteOns(capture.snapshot(), note) }
	waitFor(t, "both pages to sound", func() bool {
		return ons(60) > 0 && ons(62) > 0
	})

	// The jump runs from page mode: Browser onto the grid, then a double
	// tap on page 2's pad. Neither press may end the set, and both pages
	// keep sounding across it.
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)
	first60, first62 := ons(60), ons(62)
	pressButton(t, bank, pagePadNote(2))
	pressButton(t, bank, pagePadNote(2))
	require.Equal(t, 2, controller.pageIdx, "the double tap should jump")
	require.True(t, controller.playing(), "the jump stopped the set")
	waitFor(t, "both pages to sound after the jump", func() bool {
		return ons(60) > first60 && ons(62) > first62
	})

	// And back: the same two sessions are still what is playing.
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(1))
	pressButton(t, bank, pagePadNote(1))
	require.Equal(t, 1, controller.pageIdx)
	require.True(t, controller.playing(), "the jump back stopped the set")
	waitFor(t, "both pages to sound after the jump back", func() bool {
		return ons(60) > first60+1 && ons(62) > first62+1
	})

	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")
}

// A jump moves the view, not the sound: the pattern set a jump leaves
// running keeps playing the pattern it was playing, and the page it lands
// on keeps sounding through its own session. The wire tells the two apart,
// so a primary that followed the jump — or a set that stopped — is heard.
func TestPageJumpKeepsThePatternSetPlaying(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	setBPM(300)

	// Page 1 is left on the pattern view with one note; page 2 is left on
	// the pattern view with another.
	firstPattern := bank.CurrentPattern()
	firstPattern.ToggleEvent(Event{Voice: bank.vb.voices[0], Beat: 0, Velocity: 100})

	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	secondPattern := bank.CurrentPattern()
	secondPattern.ToggleEvent(Event{Voice: bank.vb.voices[1], Beat: 0, Velocity: 100})
	doubleTapPage(t, controller, 1) // back to page 1

	capture := &captureMidiWriter{}
	require.NoError(t, controller.Handle(capture, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 2, "both pages should play")

	ons := func(note int) int { return countNoteOns(capture.snapshot(), note) }
	waitFor(t, "both pages to sound", func() bool {
		return ons(36) > 0 && ons(37) > 0
	})

	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(2))
	pressButton(t, bank, pagePadNote(2))
	require.Equal(t, 2, controller.pageIdx)
	require.True(t, controller.playing(), "the jump stopped the set")

	// Page 1's pattern is still what the set it left behind plays, and
	// page 2's own session still sounds its note.
	first36, first37 := ons(36), ons(37)
	waitFor(t, "both pages to sound after the jump", func() bool {
		return ons(36) > first36 && ons(37) > first37
	})

	require.NoError(t, controller.stopPlayback())
	require.Nil(t, controller.running, "stopping left the sequencer armed")
}

// usePageController is the setup the page tests start from: a controller on a
// kit with one chromatic voice, its text and its pads recorded, sitting on
// page 1. Both banks draw through the same recorder, so a test reads a row
// whichever view is showing.
func usePageController(t *testing.T) (*Controller, *screenRecorder, *padRecorder) {
	t.Helper()
	kit := trackWindowKit(6, 2)
	controller := useController(t, NewFire(func([]byte) error { return nil }), kit)
	recorder := useScreenRecorder(t, &controller.patbank.screen)
	controller.songbank.screen = recorder
	pads := usePadRecorder(t, &controller.patbank.pads)
	return controller, recorder, pads
}

// pageClock is a clock the tests drive by hand, so a press can be placed inside
// or outside the double-tap window at will rather than relying on how fast the
// test runs.
type pageClock struct {
	now time.Time
}

func (c *pageClock) time() time.Time { return c.now }

func (c *pageClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// usePageClock points pageNow at a controllable clock for the duration of a test.
func usePageClock(t *testing.T) *pageClock {
	t.Helper()
	clock := &pageClock{now: time.Now()}
	previous := pageNow
	pageNow = clock.time
	t.Cleanup(func() { pageNow = previous })
	return clock
}

// fanoutWriter writes to two writers at once, so one playback can be watched on
// the wire and at the instrument behind it with a single set.
type fanoutWriter struct {
	writers []sequencerWriter
}

func (w *fanoutWriter) Write(event alsa.SeqEvent) error {
	for _, writer := range w.writers {
		if err := writer.Write(event); err != nil {
			return err
		}
	}
	return nil
}

func (w *fanoutWriter) WritePort(event alsa.SeqEvent, port int) error {
	for _, writer := range w.writers {
		if err := writer.WritePort(event, port); err != nil {
			return err
		}
	}
	return nil
}

// carriesNoteOn reports whether the events carry a note-on for a pitch, which
// is how a test tells a note that sounded from one that did not.
func carriesNoteOn(events []alsa.SeqEvent, note int) bool {
	return countNoteOns(events, note) > 0
}

// countNoteOns counts the note-ons for one pitch, which is how a test tells
// "still sounding" from "sounded once".
func countNoteOns(events []alsa.SeqEvent, note int) int {
	n := 0
	for _, event := range events {
		if len(event.Data) != 3 {
			continue
		}
		if midi.IsNoteOn(event.Data[0]) && event.Data[2] > 0 && int(event.Data[1]) == note {
			n++
		}
	}
	return n
}

// doubleTapPage enters page mode and double-taps the pad of page idx, which
// creates the page when it does not exist and jumps to it either way. A jump
// lands on the page's own view, so pad presses after a jump are pattern
// edits unless page mode is entered again.
func doubleTapPage(t *testing.T, controller *Controller, idx int) {
	t.Helper()
	pressButton(t, controller.patbank, NoteBrowser)
	pressButton(t, controller.patbank, pagePadNote(idx))
	pressButton(t, controller.patbank, pagePadNote(idx))
}

// shiftPad presses a page pad with Shift held, which deletes the page on the
// pad. Shift is a toggle rather than a momentary modifier, so the press pairs
// with one of its own to leave the modifier released for the gesture that
// follows.
func shiftPad(t *testing.T, bank *PatternBank, idx int) {
	t.Helper()
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, pagePadNote(idx))
	pressButton(t, bank, NoteShift)
}

// Browser alone enters page mode from either view without ending the set,
// and leaves the edit mode alone; Browser again returns to the view it was
// entered from. Pattern/Song while page mode shows leaves it and toggles
// the underlying view, with the mode's light reporting the view landed on.
func TestPageModeEntryFromBothViews(t *testing.T) {
	kit := trackWindowKit(8, 0) // voice 0 is chromatic
	controller := useController(t, NewFire(func([]byte) error { return nil }), kit)
	bank := controller.patbank
	require.NoError(t, bank.SelectTrackRow(1))

	// An edit mode set before entry is page state, so it survives the round trip.
	require.NoError(t, bank.ToggleNoteMode())
	require.True(t, bank.noteEditActive())

	// The round trip is the same from either view, so it runs once per
	// view: Browser enters page mode, Browser again returns to the view
	// it was entered from, and the set keeps playing through the round
	// trip. The edit mode is page state that survives the round trip; the
	// song view exits it on the way to its own round trip, so only the
	// pattern view's survives to be checked.
	for _, from := range []viewMode{patternView, songView} {
		if from == songView {
			// The view switch itself is one of the two switches that still
			// end a set, so each leg's stub is installed after it.
			pressButton(t, bank, NotePatternSong)
		}
		stopped := 0
		stubRunning(controller, nil, func() error {
			stopped++
			return nil
		})
		pressButton(t, bank, NoteBrowser)
		require.Equal(t, pageView, controller.mode, "Browser should enter page mode")
		require.Zero(t, stopped, "entering page mode stopped the set")
		pressButton(t, bank, NoteBrowser)
		require.Equal(t, from, controller.mode, "Browser should return to the view it entered from")
		require.Zero(t, stopped, "leaving page mode stopped the set")
		if from == patternView {
			require.True(t, bank.noteEditActive(), "the round trip kept the edit mode")
		}
	}

	// Back in pattern view, an edit mode survives the Pattern/Song toggle out of
	// page mode the way it survives a jump: the page did not change.
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, patternView, controller.mode)
	require.NoError(t, bank.ToggleNoteMode())
	require.True(t, bank.noteEditActive())
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)
	pads := usePadRecorder(t, &bank.pads)
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode, "Pattern/Song should leave page mode onto the toggled view")
	require.Equal(t, LEDGreen, pads.led(NotePatternSong), "the light should report song view")
	require.True(t, bank.noteEditActive(), "the toggle out of page mode exited the edit mode")
}

// A press on a dark pad creates an enabled page, lights its pad at the dim
// brightness of an enabled page, and moves the header's count.
func TestPagePadCreatesPage(t *testing.T) {
	controller, recorder, pads := usePageController(t)
	bank := controller.patbank
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)

	pressButton(t, bank, pagePadNote(2))
	page := controller.pages[2]
	require.NotNil(t, page, "pressing a dark pad created no page")
	require.True(t, page.on, "a new page should be enabled")
	require.Equal(t, Dim(patternColor(2), chromaticPaletteDim), pads.pad(1, 0), "the new page's pad")
	require.Equal(t, "Page 001  2/64", recorder.row(0), "the header should count the new page")
}

// A page's pad toggles its enabled flag: dimmed to the arrangement's dim when
// off, back to the palette's dim when on.
func TestPagePadTogglesEnabled(t *testing.T) {
	controller, _, pads := usePageController(t)
	bank := controller.patbank
	clock := usePageClock(t)
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(2)) // create page 2

	// Each press is spaced beyond the double-tap window, so it toggles the
	// page rather than completing a jump to it.
	for _, tt := range []struct {
		name string
		on   bool
		pad  [3]int
	}{
		{name: "off", on: false, pad: dimColor(patternColor(2))},
		{name: "on", on: true, pad: Dim(patternColor(2), chromaticPaletteDim)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clock.advance(pageDoubleTapWindow + time.Millisecond)
			pressButton(t, bank, pagePadNote(2))
			require.Equal(t, tt.on, controller.pages[2].on)
			require.Equal(t, tt.pad, pads.pad(1, 0))
		})
	}
}

// Two presses of the same pad inside the window jump to that page: the UI leaves
// page mode onto the page's own view, and the page's state is what the banks
// show. The first press's toggle stands, but the jump enables the page, so the
// net effect is a jump to an enabled page.
func TestPageDoubleTapJumps(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	usePageClock(t)

	// Edit page 1: pattern index, track window, track voice, song
	// index, view, edit mode and clipboard are all page state.
	require.NoError(t, bank.Jump(1))
	require.NoError(t, bank.ScrollTracks(1))
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.JogSelect(1))
	require.NoError(t, songs.Jump(1))
	require.NoError(t, bank.ToggleNoteMode())
	pressButton(t, bank, NoteRecord)
	require.NotNil(t, controller.clipboard)
	controller.setView(songView) // page 1 is left on the arrangement

	// Double-tap page 2's pad: the first press creates it, the second jumps.
	doubleTapPage(t, controller, 2)
	require.Equal(t, patternView, controller.mode, "a double tap should leave page mode")
	require.Equal(t, 2, controller.pageIdx)
	require.True(t, controller.pages[2].on, "a jump should enable the page")

	// Page 2 opens on the state a run starts on.
	require.Equal(t, 1, bank.selPatIdx)
	require.Equal(t, 0, bank.TrackOffset())
	require.Equal(t, 0, bank.selTrackRow)
	require.Equal(t, 1, songs.selSongIdx, "page 2's song selection")
	require.False(t, bank.noteEditActive())
	require.Nil(t, controller.clipboard, "page 2 has no clipboard of its own")

	// Edit page 2, then double-tap back to page 1.
	require.NoError(t, bank.Jump(1))
	doubleTapPage(t, controller, 1)
	require.Equal(t, songView, controller.mode, "page 1 was left on the song view")
	require.Equal(t, 2, bank.selPatIdx, "page 1's pattern selection")
	require.Equal(t, 1, bank.TrackOffset(), "page 1's track window")
	require.Equal(t, 2, bank.trackVoices[1], "page 1's track voice")
	require.Equal(t, 1, bank.selTrackRow, "page 1's selected row")
	require.Equal(t, 2, songs.selSongIdx, "page 1's song selection")
	require.True(t, bank.noteEditActive(), "page 1's edit mode")
	require.NotNil(t, controller.clipboard, "page 1's clipboard")
}

// Presses further apart than the window are two toggles and no jump; a press
// at the same instant after that is a double tap and jumps.
func TestPageDoubleTapWindowExpires(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	clock := usePageClock(t)

	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(2)) // create page 2

	// Presses spaced beyond the window are two toggles and no jump.
	for _, on := range []bool{false, true} {
		clock.advance(pageDoubleTapWindow + time.Millisecond)
		pressButton(t, bank, pagePadNote(2))
		require.Equal(t, pageView, controller.mode, "an expired press should stay in page mode")
		require.Equal(t, on, controller.pages[2].on)
	}

	// The same instant is inside the window, so this press is a double tap.
	pressButton(t, bank, pagePadNote(2))
	pressButton(t, bank, pagePadNote(2))
	require.Equal(t, patternView, controller.mode, "a double tap should jump")
	require.Equal(t, 2, controller.pageIdx)
}

// Shift plus a pad deletes its page and darkens its pad. Deleting the current
// page jumps to the first remaining page; deleting the last page leaves a
// fresh, enabled page 1.
func TestShiftPadDeletesPage(t *testing.T) {
	controller, _, pads := usePageController(t)
	bank := controller.patbank
	usePageClock(t)

	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(2)) // create page 2
	pressButton(t, bank, pagePadNote(3)) // create page 3

	// Shift plus a pad deletes the page on it.
	shiftPad(t, bank, 2)
	require.Nil(t, controller.pages[2], "the page should be gone")
	require.Equal(t, oledBlack, pads.pad(1, 0), "a deleted page's pad")

	// Deleting the current page jumps to the first remaining page.
	pressButton(t, bank, pagePadNote(3))
	pressButton(t, bank, pagePadNote(3)) // double-tap: jump to page 3
	require.Equal(t, 3, controller.pageIdx)
	pressButton(t, bank, NoteBrowser)
	shiftPad(t, bank, 3)
	require.Nil(t, controller.pages[3])
	require.Equal(t, 1, controller.pageIdx, "deleting the current page should jump to the first remaining")
	require.Equal(t, patternView, controller.mode, "the jump should land on the first page's view")

	// Deleting the last page leaves a fresh enabled page 1. The deletion
	// before this one jumped to the pattern view, so page mode is entered
	// again for the pad press to be a page gesture.
	pressButton(t, bank, NoteBrowser)
	shiftPad(t, bank, 1)
	require.Len(t, controller.pages, 1)
	require.NotNil(t, controller.pages[1])
	require.True(t, controller.pages[1].on, "a fresh page should be enabled")
	require.NotNil(t, controller.pages[1].pattern.Patterns[1], "a fresh page holds its first pattern")
	require.Equal(t, 1, controller.pageIdx)
}

// A seek lands every installed session where its own source sends it: the
// song-mode pages of a set stay in sync after a jump within the song, while
// a page left on the pattern view is carried back to its own start, its
// source answering only beat zero.
func TestSeekAppliesToAllPages(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	// A fast tempo keeps the measure the seek waits for well inside the wait
	// the test allows itself.
	setBPM(300)

	// Page 1, on the arrangement: a two-measure song holding one note.
	first := bank.CurrentPattern()
	first.SetChromaticNote(0, bank.vb.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(first, 0)
	songs.CurrentSong().SetPattern(first, 1)

	// Page 2 is left on the pattern view: its session plays its pattern,
	// whose source answers only beat zero.
	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	doubleTapPage(t, controller, 1) // back to page 1, on the arrangement

	// Page 3, on the arrangement: another two-measure song.
	doubleTapPage(t, controller, 3) // create page 3 and jump to it
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	third := bank.CurrentPattern()
	third.SetChromaticNote(0, bank.vb.voices[2], 64, 100)
	songs.CurrentSong().SetPattern(third, 0)
	songs.CurrentSong().SetPattern(third, 1)
	doubleTapPage(t, controller, 1) // back to page 1, on the arrangement
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)

	writer := &captureMidiWriter{}
	require.NoError(t, controller.Handle(writer, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 3, "pages 1, 2 and 3 should play")

	// Shift plus a measure pad seeks to that measure: measure 1 starts at beat 4.
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, pagePadNote(1)) // the measure pad at (1, 0)
	pressButton(t, bank, NoteShift)

	// The seek lands every session where its own page's source sends it:
	// the shown arrangement to measure 1, the pattern-view page back to
	// its own start, the other arrangement to its measure 1.
	wantBeats := map[int]float32{1: 4, 2: 0, 3: 4}
	waitFor(t, "every session to seek", func() bool {
		for page, want := range wantBeats {
			if beat, _, _ := controller.running.sessions[page].playback.position(); beat != want {
				return false
			}
		}
		return true
	})
	require.NoError(t, controller.stopPlayback())
}

// In song mode Play plays every enabled page's own selection,
// all starting on the same beat: a page plays the song its
// arrangement has selected, not the current page's. A page with
// no song at its own selected index plays silence, a disabled
// page plays nothing, and a Play press enables the current page
// before building the set.
func TestSongModePlaysEnabledPages(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	usePageClock(t)

	// Page 1: a song that places pattern 1, holding one chromatic note.
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, bank.vb.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(pattern, 0)

	// Page 2: its own selection, song 2, holding another note. The
	// page is left on the arrangement, which is what makes the song
	// its playback's source, and the pattern bank jumps with the
	// song so song 2's measure is a pattern of its own.
	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	require.Equal(t, 2, controller.pageIdx)
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	require.NoError(t, songs.Jump(1))
	require.NoError(t, bank.Jump(1))
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	songs.CurrentSong().SetPattern(second, 0)

	// Page 3: enabled and left on the arrangement, but with no song
	// at its own selected index, so it plays silence.
	doubleTapPage(t, controller, 1) // back to page 1
	doubleTapPage(t, controller, 3) // create page 3 and jump to it
	require.Equal(t, 3, controller.pageIdx)
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	delete(songs.Songs, songs.selSongIdx)

	// Page 4: its own note, disabled so it must not sound.
	doubleTapPage(t, controller, 1) // back to page 1
	doubleTapPage(t, controller, 4) // create page 4 and jump to it
	fourth := bank.CurrentPattern()
	fourth.SetChromaticNote(0, bank.vb.voices[2], 64, 100)
	songs.CurrentSong().SetPattern(fourth, 0)
	doubleTapPage(t, controller, 1) // back to page 1
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(4)) // disable page 4
	require.False(t, controller.pages[4].on)

	// Press Play in song mode: every enabled page's song plays its own pattern.
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	instrument := newSoundingInstrument()
	capture := &captureMidiWriter{}
	fanout := &fanoutWriter{writers: []sequencerWriter{capture, instrument}}
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 3, "pages 1, 2 and 3 should play")

	events := capture.waitForEvents(t, 5)
	require.True(t, carriesNoteOn(events, 60), "page 1's note should sound")
	require.True(t, carriesNoteOn(events, 62), "page 2's own song should sound")
	require.False(t, carriesNoteOn(events, 64), "a disabled page's note should not sound")
	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")

	// A page disabled before Play is enabled by the Play press and plays.
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(1)) // disable the current page
	require.False(t, controller.pages[1].on)
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, songView, controller.mode)
	before := len(capture.snapshot())
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.pages[1].on, "Play should enable the current page")
	events = capture.waitForEvents(t, before+5)[before:]
	require.True(t, carriesNoteOn(events, 60), "the enabled page's note should sound")
	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")
}

// Play starts a set over every enabled page, whatever view that
// page was left on: the current page's session plays the view on
// screen and draws it, every other enabled page plays what its own
// view points at, and a disabled page plays nothing. A Play press
// while a set runs changes nothing.
func TestSetPlaybackPlaysEveryPage(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	usePageClock(t)

	// Page 1 stays on the pattern view: its session plays its
	// selected pattern and draws the visible grid.
	first := bank.CurrentPattern()
	first.SetChromaticNote(0, bank.vb.voices[2], 60, 100)

	// Page 2 is left on the arrangement, with song 2 selected and
	// its measure 0 a pattern of its own.
	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	require.NoError(t, songs.Jump(1))
	require.NoError(t, bank.Jump(1))
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	songs.CurrentSong().SetPattern(second, 0)

	// Page 3 is disabled before the set, so it plays nothing.
	doubleTapPage(t, controller, 1) // back to page 1
	doubleTapPage(t, controller, 3) // create page 3 and jump to it
	doubleTapPage(t, controller, 1) // back to page 1
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(3)) // disable page 3
	require.False(t, controller.pages[3].on)
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, patternView, controller.mode)

	instrument := newSoundingInstrument()
	capture := &captureMidiWriter{}
	fanout := &fanoutWriter{writers: []sequencerWriter{capture, instrument}}
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 2, "pages 1 and 2 should play")

	// Two sessions, each writing a MIDI Start before its note.
	events := capture.waitForEvents(t, 4)
	require.True(t, carriesNoteOn(events, 60), "page 1's pattern should sound")
	require.True(t, carriesNoteOn(events, 62), "page 2's song should sound")

	// A Play press while a set runs is a no-op: the same sessions
	// stay installed and the wire stays quiet.
	sessions := maps.Clone(controller.running.sessions)
	before := len(capture.snapshot())
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.Equal(t, sessions, controller.running.sessions)
	require.Len(t, capture.snapshot(), before)

	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")
}

// Each page's session plays the page's own selection, not the
// current page's: page 2 plays the song its arrangement has
// selected even while page 1 shows another, and the song page 2
// left behind stays silent.
func TestSetPlaybackUsesEachPagesOwnSelection(t *testing.T) {
	controller, _, _ := usePageController(t)
	bank := controller.patbank
	songs := controller.songbank
	usePageClock(t)

	// Page 1, on the arrangement: song 1, its measure 0 holding
	// one note.
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	first := bank.CurrentPattern()
	first.SetChromaticNote(0, bank.vb.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(first, 0)

	// Page 2, also on the arrangement: its song 1 holds a decoy
	// note, then its selection moves to song 2, whose measure 0
	// holds the note that should sound.
	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	decoy := bank.CurrentPattern()
	decoy.SetChromaticNote(0, bank.vb.voices[2], 61, 100)
	songs.CurrentSong().SetPattern(decoy, 0)
	require.NoError(t, songs.Jump(1))
	require.NoError(t, bank.Jump(1))
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	songs.CurrentSong().SetPattern(second, 0)

	doubleTapPage(t, controller, 1) // back to page 1, on the arrangement
	instrument := newSoundingInstrument()
	capture := &captureMidiWriter{}
	fanout := &fanoutWriter{writers: []sequencerWriter{capture, instrument}}
	require.NoError(t, controller.Handle(fanout, padMessage(NotePlay, 100)))
	require.True(t, controller.playing())
	require.Len(t, controller.running.sessions, 2, "pages 1 and 2 should play")

	events := capture.waitForEvents(t, 4)
	require.True(t, carriesNoteOn(events, 60), "page 1's song should sound")
	require.True(t, carriesNoteOn(events, 62), "page 2's own selection should sound")
	require.False(t, carriesNoteOn(events, 61), "page 2's unselected song should stay silent")

	require.NoError(t, controller.stopPlayback())
	require.Empty(t, instrument.stillSounding(), "stopping should release every note")
}

// The arrangement's row 0 names the page it is showing and whether that page
// is enabled.
func TestSongViewShowsPageIndicator(t *testing.T) {
	controller, recorder, _ := usePageController(t)
	bank := controller.patbank
	usePageClock(t)

	// Each gesture ends on the arrangement, whose row 0 names the page it is
	// showing and whether that page is enabled.
	for _, tt := range []struct {
		name string
		act  func()
		want string
	}{
		{
			name: "the page a run starts on",
			act:  func() { pressButton(t, bank, NotePatternSong) },
			want: "Song 001 Pg 001 on",
		},
		{
			name: "a page jumped to",
			act: func() {
				doubleTapPage(t, controller, 3) // create page 3 and jump to it
				pressButton(t, bank, NotePatternSong)
			},
			want: "Song 001 Pg 003 on",
		},
		{
			name: "a disabled page",
			act: func() {
				pressButton(t, bank, NoteBrowser)
				pressButton(t, bank, pagePadNote(3)) // toggle page 3 off
				pressButton(t, bank, NoteBrowser)
			},
			want: "Song 001 Pg 003 off",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.act()
			require.Equal(t, tt.want, recorder.row(0))
		})
	}
}

// A round trip through the session file brings every page's content and
// enabled flag back. The landed page shows the defaults a run starts on,
// because where the user was looking is not saved.
func TestSessionRoundTripKeepsPages(t *testing.T) {
	kit := stateKit()
	controller := useStateController(t, stateFilePath(t), NewFire(func([]byte) error { return nil }), kit, nil)
	bank := controller.patbank
	songs := controller.songbank
	usePageClock(t)

	// Page 1: a pattern, a song and a track voice of its own.
	first := bank.CurrentPattern()
	first.SetChromaticNote(0, kit.voices[2], 60, 100)
	songs.CurrentSong().SetPattern(first, 0)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.JogSelect(2))

	// Page 2: its own pattern, song and track voice, disabled before the save.
	doubleTapPage(t, controller, 2) // create page 2 and jump to it
	second := bank.CurrentPattern()
	second.SetChromaticNote(0, bank.vb.voices[2], 62, 100)
	songs.CurrentSong().SetPattern(second, 0)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.JogSelect(1))
	doubleTapPage(t, controller, 1) // back to page 1
	pressButton(t, bank, NoteBrowser)
	pressButton(t, bank, pagePadNote(2)) // disable page 2
	require.False(t, controller.pages[2].on)

	saveTo(t, controller.sessionPath, controller, nil)

	far := stateTestBank(t, stateKit())
	report, err := loadInto(far, controller.sessionPath, nil)
	require.NoError(t, err)
	require.Zero(t, report.Dropped)
	require.Len(t, far.pages, 2)
	require.True(t, far.pages[1].on)
	require.False(t, far.pages[2].on)

	// Each page's content came back.
	event, ok := far.pages[1].pattern.Patterns[1].EventAtStep(0, far.patbank.vb.voices[2])
	require.True(t, ok, "page 1's note should survive the round trip")
	require.Equal(t, 60, event.ChromaticNote)
	require.NotNil(t, far.pages[1].song.Songs[1].GetPattern(0), "page 1's arrangement")
	require.Equal(t, []int{2, 1, 2, 3}, far.pages[1].pattern.TrackVoices, "page 1's track voices")

	event, ok = far.pages[2].pattern.Patterns[1].EventAtStep(0, far.patbank.vb.voices[2])
	require.True(t, ok, "page 2's note should survive the round trip")
	require.Equal(t, 62, event.ChromaticNote)
	require.NotNil(t, far.pages[2].song.Songs[1].GetPattern(0), "page 2's arrangement")
	require.Equal(t, []int{1, 1, 2, 3}, far.pages[2].pattern.TrackVoices, "page 2's track voices")

	// The landed page shows the state a run starts on.
	require.Equal(t, 1, far.pageIdx)
	require.Equal(t, 1, far.patbank.selPatIdx)
	require.Equal(t, 1, far.songbank.selSongIdx)
	require.Equal(t, patternView, far.mode)
}

// The Fire's own knobs mean nothing in page mode, while a controller's tempo and
// swing controls keep working everywhere.
func TestPageModeIgnoresDeviceKnobs(t *testing.T) {
	controller, recorder, _ := usePageController(t)
	bank := controller.patbank
	require.NoError(t, bank.SelectTrackRow(1))
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)

	require.NoError(t, sendCC(bank, deviceChannel, CCSelect, EncoderRight))
	require.NoError(t, sendCC(bank, deviceChannel, CCVolume, 64))
	require.Equal(t, 0, bank.trackVoices[0], "the encoder jogged a track in page mode")
	require.Equal(t, defaultStepVelocity, bank.chromaticVelocity, "the volume knob moved the palette in page mode")

	require.NoError(t, sendCC(bank, softwareChannel, CcBpmMsb, 60))
	assert.Contains(t, recorder.row(readoutRow), "Tempo", "the tempo control should report")
	require.NoError(t, sendCC(bank, softwareChannel, CcSwing, 64))
	assert.Contains(t, recorder.row(readoutRow), "Swing", "the swing control should report")
}

// Shift plus Browser saves and Shift plus Accent loads from page mode without
// leaving it, and the load repaints the page grid over the new table.
func TestPageModeKeepsSessionGestures(t *testing.T) {
	kit := stateKit()
	controller := useStateController(t, stateFilePath(t), NewFire(func([]byte) error { return nil }), kit, nil)
	bank := controller.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	controller.songbank.screen = recorder

	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 60, 100)

	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)

	// Shift plus Browser saves from page mode without leaving it.
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteBrowser)
	waitFor(t, "the save to report", func() bool {
		return strings.HasPrefix(recorder.row(readoutRow), "Saved")
	})
	pressButton(t, bank, NoteShift)
	require.Equal(t, pageView, controller.mode, "the save left page mode")

	// An edit made after the save is what the load replaces.
	pressButton(t, bank, NoteBrowser)
	require.Equal(t, patternView, controller.mode)
	bank.CurrentPattern().SetChromaticNote(0, kit.voices[2], 72, 100)

	pressButton(t, bank, NoteBrowser)
	require.Equal(t, pageView, controller.mode)
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteAccent)
	waitFor(t, "the load to report", func() bool {
		return strings.HasPrefix(recorder.row(readoutRow), "Loaded")
	})
	pressButton(t, bank, NoteShift)
	require.Equal(t, pageView, controller.mode, "the load left page mode")
	require.Contains(t, recorder.row(0), "Page 001", "the load repainted the page grid")
	event, ok := bank.CurrentPattern().EventAtStep(0, kit.voices[2])
	require.True(t, ok)
	require.Equal(t, 60, event.ChromaticNote, "the load restored the saved note")
}

// The tracer replays a page session through the real handler: entry, creation,
// a toggle, a double-tap jump and a Shift+pad deletion, with the grid after
// each step.
func TestTracePageGrid(t *testing.T) {
	steps := []uiStep{
		press("browser: page mode", NoteBrowser),
		press("pad: create page 2", 55),
		press("pad: create page 3", 56),
		press("pad: toggle page 2", 55),
		press("pad: double-tap page 2", 55),
		press("browser: page mode", NoteBrowser),
		press("shift", NoteShift),
		press("shift+pad: delete page 3", 56),
		press("shift", NoteShift),
	}
	snapshots := traceUI(t, steps)

	require.Equal(t, 1, snapshots[0].litPads, "page mode should light one page")
	require.Equal(t, 1, snapshots[0].litRow[0])
	require.Equal(t, 2, snapshots[1].litPads, "a new page lights its pad")
	require.Equal(t, 3, snapshots[2].litPads, "a second new page lights its pad")
	require.Equal(t, 3, snapshots[3].litPads, "a disabled page's pad stays lit, dimmed")
	// The double tap lands on the pattern view, where the step cursor marks
	// its cell on every row: a mark lifts black to grey, which reads as lit.
	require.Equal(t, 4, snapshots[4].litPads, "a double tap jumps to a pattern view")
	require.Equal(t, [4]int{1, 1, 1, 1}, snapshots[4].litRow, "the cursor marks column zero of every row")
	require.Equal(t, 3, snapshots[5].litPads, "page mode lights every page again")
	require.Equal(t, 2, snapshots[7].litPads, "a deleted page's pad goes dark")
	// Shift stays lit from its press until the press that releases it, so
	// only the steps before it can expect no lit button.
	for _, snapshot := range snapshots[:6] {
		require.Empty(t, snapshot.litLEDs, "no button should be lit in page mode")
	}
}
