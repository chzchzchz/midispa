package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// The renderer is the playhead's only writer once the worker stops drawing,
// so its boundary behaviour is pinned directly, no worker and no timing:
// the first render lights column 0, a wrap heals the column the playhead
// left, and a pattern shorter than sixteen steps leaves nothing stuck
// across its boundary — the behaviour the pattern source's old lastColumn
// reset used to break.
func TestRendererDrawsTheShownPlayhead(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	controller := bank.controller
	noteColor := chromaticPaletteColor(40)
	bank.CurrentPattern().SetChromaticNote(3, kit.voices[0], 40, 100)
	require.NoError(t, controller.startSetPlayback(nil))

	// One row per render over a sixteen-step pattern, and what the two
	// columns the playhead touches read after it.
	for _, tt := range []struct {
		name       string
		beat       float32
		col0, col3 [3]int
	}{
		{
			// The first render lights column 0 the way the worker's first beat did.
			name: "the first render lights column 0",
			beat: 0,
			col0: emptyStepColor(true),
			// The note was placed after the startup redraw, so the playhead
			// has not touched its column yet.
			col3: [3]int{},
		},
		{
			name: "the playhead lights the note and restores column 0",
			beat: stepBeat(3),
			col0: emptyStepColor(false),
			col3: invertColor(noteColor),
		},
		{
			// Beat 4 is column 0 of the next measure, so the wrap heals
			// column 3 with no boundary signal at all.
			name: "the wrap heals the column the playhead left",
			beat: 4,
			col0: emptyStepColor(true),
			col3: noteColor,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			playheadBeat(t, controller, tt.beat)
			require.Equal(t, tt.col0, sim.pads[0], "column 0")
			require.Equal(t, tt.col3, sim.pads[3], "column 3")
		})
	}

	// A four-step pattern crosses its boundary after column 3. The restore
	// the next render makes lands on the column actually drawn, which is
	// what the deleted sixteen-step reset could not do.
	bank.CurrentPattern().SetLengthSteps(4)
	playheadBeat(t, controller, stepBeat(3))
	playheadBeat(t, controller, 4)
	require.Equal(t, noteColor, sim.pads[3], "a short pattern's last column healed")
	require.Equal(t, emptyStepColor(true), sim.pads[0], "column 0 lit after the short pattern's boundary")
}

// The shown-page-and-view rule is the renderer's, applied at render time: a
// session plays whether or not its page shows, but its playhead is drawn
// only while the screen shows its page and the view it was built for. A
// beat published before the screen moved is discarded here.
func TestRendererSkipsWhatTheScreenDoesNotShow(t *testing.T) {
	t.Run("a page that is not shown", func(t *testing.T) {
		bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
		controller := bank.controller

		// Page 1 stays on the pattern view; page 2 is left on the pattern
		// view with a note of its own, so its playhead would paint a column
		// the shown page never lights.
		bank.CurrentPattern().SetChromaticNote(0, kit.voices[0], 40, 100)
		doubleTapPage(t, controller, 2) // create page 2 and jump to it
		bank.CurrentPattern().SetChromaticNote(5, kit.voices[0], 62, 100)
		doubleTapPage(t, controller, 1) // back to page 1

		require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))
		require.Len(t, controller.running.sessions, 2, "both pages should play")

		playheadBeat(t, controller, 0)
		// Page 1's note sits on step 0, so the playhead shows it inverted.
		require.Equal(t, invertColor(chromaticPaletteColor(40)), sim.pads[0], "the shown page's playhead")

		// The hidden page's session publishes a beat; the render must skip it.
		controller.running.sessions[2].playback.latestBeat.Store(math.Float32bits(stepBeat(5)))
		controller.renderPlayheads()
		require.Equal(t, [3]int{}, sim.pads[5], "a hidden page's playhead painted the shown grid")
	})

	t.Run("a view the session was not built for", func(t *testing.T) {
		bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
		controller := bank.controller
		bank.CurrentPattern().SetChromaticNote(3, kit.voices[0], 40, 100)

		require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))
		playheadBeat(t, controller, 0)
		playheadBeat(t, controller, stepBeat(3))
		require.Equal(t, invertColor(chromaticPaletteColor(40)), sim.pads[3], "the playhead draws its own view")

		// Page mode's view toggle keeps the set and moves the screen to the
		// arrangement. The beat published before the move is rendered after
		// it, and the render must discard itself.
		pressButton(t, bank, NoteBrowser)
		pressButton(t, bank, NotePatternSong)
		require.Equal(t, songView, controller.mode)
		controller.running.sessions[1].playback.latestBeat.Store(math.Float32bits(4))
		controller.renderPlayheads()
		require.Equal(t, [3]int{}, sim.pads[0], "a pattern playhead painted over the arrangement")
		require.Equal(t, [3]int{}, sim.pads[3], "the arrangement's redraw was undone by a stale playhead")
	})
}

// The arrangement's playhead moves the lit measure and names the measure and
// pattern it reached, and its sentinel makes the first render a no-op: the
// current measure compares equal to itself, so nothing lights until the
// second measure.
func TestRendererMovesTheMeasurePlayhead(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	controller := bank.controller
	songs := controller.songbank

	pattern := bank.CurrentPattern()
	pattern.SetLengthSteps(4)
	songs.CurrentSong().SetPattern(pattern, 0)
	songs.CurrentSong().SetPattern(pattern, 1)
	recorder := useScreenRecorder(t, &songs.screen)
	measurePads := usePadRecorder(t, &songs.pads)

	// The page is left on the arrangement, so the session is built for it.
	pressButton(t, bank, NotePatternSong)
	require.Equal(t, songView, controller.mode)
	require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))

	playheadBeat(t, controller, 0)
	require.NotContains(t, recorder.row(4), "Measure", "the first render drew a measure")
	require.NotContains(t, recorder.row(5), "Pat-bar", "the first render drew a pattern readout")

	playheadBeat(t, controller, 1)
	require.Equal(t, "Measure 002", recorder.row(4), "the measure readout")
	require.Equal(t, "Pat-bar 001", recorder.row(5), "the pattern readout")
	require.Equal(t, patternColor(1), measurePads.pad(1, 0), "the measure reached is lit")
	require.Equal(t, dimColor(patternColor(1)), measurePads.pad(0, 0), "the measure left is dimmed")
}

// A failing display write is logged and costs the set nothing: the renderer
// has no error path into the running set, so a wedged display cannot end a
// performance the way a failed worker write used to. The pads seam points at
// a Fire whose every write fails, the same stand-in a handler-failure test
// uses.
func TestRendererFailureCostsTheSetNothing(t *testing.T) {
	bank, kit, _ := recordedBank(t, trackWindowKit(8, 0))
	controller := bank.controller
	bank.CurrentPattern().SetChromaticNote(3, kit.voices[0], 40, 100)
	require.NoError(t, controller.startSetPlayback(nil))
	capture := useCaptureLog(t)

	bank.pads = NewFire(func([]byte) error { return errTestWrite })

	playheadBeat(t, controller, stepBeat(3))

	require.NotNil(t, controller.running, "a failed render ended the set")
	require.Len(t, controller.running.sessions, 1, "a failed render dropped the session")
	require.Contains(t, capture.messages(), "playhead restore", "the failed write was not reported")
}

// Stopping discards what the wake queued: the drain runs after the workers
// are joined and before the grid restore, so a wake that queued behind a
// stopping worker cannot render over the cleared pads.
func TestStopDrainsAPendingWake(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	controller := bank.controller
	bank.CurrentPattern().SetChromaticNote(3, kit.voices[0], 40, 100)
	require.NoError(t, controller.startSetPlayback(nil))

	installedPlayback(t, controller).latestBeat.Store(math.Float32bits(stepBeat(3)))
	controller.wake <- struct{}{}

	require.NoError(t, controller.stopPlayback())
	select {
	case <-controller.wake:
		require.FailNow(t, "the wake queued behind the stop survived it")
	default:
	}
	// The pass the event loop would run next finds no set and no playhead.
	// The grid holds whatever the startup redraw painted — the note was
	// placed after it, so column 3 is still dark — and a stale wake must
	// not have lit it.
	controller.renderPlayheads()
	require.Equal(t, [3]int{}, sim.pads[3], "a stale wake painted over the stopped grid")
}
