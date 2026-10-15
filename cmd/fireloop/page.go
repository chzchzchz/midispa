package main

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/akai"
)

// maxPages is the number of pages the grid can show: one per pad.
const maxPages = padColumns * padRows

// pageDoubleTapWindow is how far apart two presses of the same
// page pad may be and still count as a double tap, which jumps
// to that page. It is a variable so a test can drive the
// window deterministically.
var pageDoubleTapWindow = 300 * time.Millisecond

// pageNow is the clock the double-tap window is measured
// against, a variable so a test can drive it the way knobNow
// does for the Volume knob.
var pageNow = time.Now

// pageHintText names the pad gestures on the grid's second row.
const pageHintText = "Pad:on/off 2x:jump"

// Page is one page: the state a jump saves and restores. The
// banks stay single instances owned by the controller, so a
// page is a memento of the bank fields a page owns rather
// than a bank of its own; the maps move by reference, which
// is safe because the bank only ever mutates what it
// currently points at.
type Page struct {
	pattern   pagePatternState
	song      pageSongState
	view      viewMode
	clipboard *Pattern
	// on is whether the page plays in song mode. A disabled
	// page keeps its state and its pad; it just stays out
	// of the mix.
	on bool
}

// newPage is a page as a run starts: one empty pattern and
// one empty song at index 1, the default track window, the
// pattern view, and enabled, so a new page plays the moment
// it exists.
func newPage(vb *VoiceBank) *Page {
	trackVoices := make([]int, padRows)
	for i := range trackVoices {
		trackVoices[i] = i % len(vb.voices)
	}
	return &Page{
		pattern: pagePatternState{
			Patterns:          map[int]*Pattern{1: {}},
			SelPatIdx:         1,
			TrackVoices:       trackVoices,
			ChromaticVelocity: defaultStepVelocity,
		},
		song: pageSongState{
			Songs:      map[int]*Song{1: {}},
			SelSongIdx: 1,
		},
		view: patternView,
		on:   true,
	}
}

// currentPage is the page the controller is sitting on. The
// invariant "the current page has an entry" holds from boot,
// so this never returns nil while a controller exists.
func (c *Controller) currentPage() *Page {
	return c.pages[c.pageIdx]
}

// firstPageIndex is the lowest index in the table, which is
// where a jump with no target of its own lands. Both callers
// have just made sure the table holds a page, so the minimum
// exists; an empty table would panic rather than answer with
// an index no pad can show.
func (c *Controller) firstPageIndex() int {
	return slices.Min(slices.Collect(maps.Keys(c.pages)))
}

// updatePageIndicator tells the song bank which page it is
// showing and whether that page is enabled, for the
// arrangement's row 0. The controller owns the page table, so
// it is the only writer of the pair.
func (c *Controller) updatePageIndicator() {
	c.songbank.setPageIndicator(c.pageIdx, c.currentPage().on)
}

// saveCurrentPage commits the live banks, the view and the
// clipboard into the current page's memento. It is a no-op
// when the current page has no entry, which is the guard
// that keeps a deleted page from being resurrected by its
// own save: a deletion removes the entry before the jump it
// runs. The view recorded is prevMode while the controller
// sits in page mode, because every jump runs from page mode
// and page mode is a mode of the controller rather than a
// view of a page, so a page is always left on the pattern
// or song view and landing on it never re-enters page mode.
func (c *Controller) saveCurrentPage() {
	page := c.currentPage()
	if page == nil {
		return
	}
	view := c.mode
	if view == pageView {
		view = c.prevMode
	}
	page.pattern = c.patbank.savePageState()
	page.song = c.songbank.savePageState()
	page.view = view
	page.clipboard = c.clipboard
}

// loadPage restores a page's memento into the live banks and
// the clipboard into the controller. The memento is assigned
// verbatim, UI fields and all, because a jump must put back
// exactly the state the page was left with.
func (c *Controller) loadPage(page *Page) {
	c.patbank.loadPageState(page.pattern)
	c.songbank.loadPageState(page.song)
	c.clipboard = page.clipboard
}

// jumpToPage moves the UI to a page: the state the controller
// holds live is committed into the page being left, and the
// target page's memento is restored into the same live
// objects. Jumping to the page already current is the same
// path with a self-referential save and load, which is
// idempotent.
//
// The set keeps playing across a jump: every session goes on playing the
// page it was built for, and the page the jump lands on was already
// sounding if it was enabled when the set started. What moves is the view
// — the playhead hands over to the landed page's session, because each
// session draws only while its own page is shown — and the banks. The
// beat lock waits out whatever beat is in flight and keeps the next ones
// out while the screen and the banks move together, so no session reads a
// bank half-moved or draws for a screen that has already moved. A page
// the set has no session for — one that was disabled when Play was
// pressed — stays silent until the next set includes it.
// The beat lock waits out whatever beat is in flight and keeps the next
// ones out while the screen and the banks move together, so no session
// reads a bank half-moved or draws for a screen that has already moved. A
// page the set has no session for — one that was disabled when Play was
// pressed — stays silent until the next set includes it.
func (c *Controller) jumpToPage(idx int) error {
	return c.holdBeatsWhile(func() error {
		c.saveCurrentPage()
		// Modifiers, held pads and the note-edit playhead belong
		// to the view being left. The edit modes are not exited
		// here: their fields are overwritten by the load below,
		// and the indicators and redraw that follow put every
		// mode light back from the restored bank, which is the
		// same answer an exit would have computed.
		if err := c.releaseModifiers(); err != nil {
			return err
		}
		c.patbank.clearPadState()
		if err := c.patbank.clearStepPlayhead(); err != nil {
			return err
		}
		page := c.pages[idx]
		// The page you land on always plays.
		page.on = true
		// The screen and the banks move as one step: a beat that starts
		// after this reads the landed page's banks and draws for the landed
		// page's screen. A jump lands on the page's own view, never on page
		// mode: page mode is a mode of the controller, and every jump runs
		// from it, so the view restored above is where the UI goes.
		c.setScreen(idx, page.view)
		c.loadPage(page)
		c.updatePageIndicator()
		return c.redrawMode()
	})
}

// enterPageMode moves the controls onto the page grid. The set keeps
// playing: the grid borrows the pads from the view, so the playhead yields
// to it rather than the set ending for a view change, and the modifiers are
// released and the held pads cleared. The edit mode is left set: it is page
// state, and page mode takes over the grid the mode owned, so leaving page
// mode redraws the view with the mode intact.
func (c *Controller) enterPageMode() error {
	if err := c.releaseModifiers(); err != nil {
		return err
	}
	c.patbank.clearPadState()
	c.prevMode = c.mode
	c.setView(pageView)
	c.pagePad = -1
	// The grid repaint holds the beats, so the playhead it takes the pads
	// from cannot be mid-draw underneath it.
	return c.holdBeatsWhile(c.drawPageMode)
}

// leavePageMode returns to the view page mode was entered
// from. The double-tap tracking is cleared with the mode, so
// a press just before leaving cannot jump a page on the way
// back in.
func (c *Controller) leavePageMode() error {
	c.setView(c.prevMode)
	c.pagePad = -1
	return c.holdBeatsWhile(c.redrawMode)
}

// togglePageMode is the Pattern/Song button while page mode
// is showing: the button means "switch views" in every mode,
// so it leaves page mode and toggles the underlying view.
// The set keeps playing, for the same reason entering page
// mode leaves it playing. The edit mode survives the toggle
// the way it survives a jump, because the page did not change.
func (c *Controller) togglePageMode() error {
	if err := c.releaseModifiers(); err != nil {
		return err
	}
	c.patbank.clearPadState()
	view := patternView
	if c.prevMode == patternView {
		view = songView
	}
	c.prevMode = view
	c.setView(view)
	c.pagePad = -1
	return c.holdBeatsWhile(c.redrawMode)
}

// createPage adds an enabled page at the pad's index. The
// pad held no page a moment ago, so nothing can be
// displaced.
func (c *Controller) createPage(idx int) *Page {
	page := newPage(c.patbank.vb)
	c.pages[idx] = page
	return page
}

// togglePage flips a page's enabled flag. A disabled page
// stays in the table with its state; it just does not play
// in song mode.
func (c *Controller) togglePage(idx int) {
	page := c.pages[idx]
	page.on = !page.on
	c.updatePageIndicator()
}

// deletePage removes a page from the table. A page other
// than the current one simply goes dark on the grid; the
// current page is removed first and the jump that follows
// lands on the first remaining page, or a fresh page 1 when
// none is left, so the controller always sits on a page.
// Removing the entry before the jump is what keeps the
// deleted page from being resurrected by its own save: the
// save that opens the jump has no entry to write.
func (c *Controller) deletePage(idx int) error {
	delete(c.pages, idx)
	// The pad the page sat on no longer holds it, so a later press
	// of that pad cannot be the second half of a double tap on the
	// page that just left.
	c.pagePad = -1
	if len(c.pages) == 0 {
		c.pages[1] = newPage(c.patbank.vb)
	}
	if idx != c.pageIdx {
		return c.drawPageMode()
	}
	return c.jumpToPage(c.firstPageIndex())
}

// handlePageGrid answers a pad press in page mode. A Shift
// press deletes the page on the pad. An empty pad creates a
// page. A page's pad toggles its enabled flag. And a second
// press of the same pad within the double-tap window jumps
// to that page: the first press's toggle stands, but a jump
// always enables the page, so the net effect of a double tap
// is a jump to an enabled page, and a double tap on an
// empty pad creates the page and jumps to it.
func (c *Controller) handlePageGrid(x, y int) error {
	idx := y*padColumns + x + 1
	if c.shift {
		if c.pages[idx] == nil {
			return nil
		}
		if err := c.deletePage(idx); err != nil {
			return err
		}
		return c.pageFeedback(fmt.Sprintf("Page %03d gone", idx))
	}
	page := c.pages[idx]
	created := false
	if page == nil {
		page = c.createPage(idx)
		created = true
	} else {
		c.togglePage(idx)
	}
	now := pageNow()
	if idx == c.pagePad && now.Sub(c.pagePadTime) <= pageDoubleTapWindow {
		c.pagePad = -1
		if err := c.jumpToPage(idx); err != nil {
			return err
		}
		return c.pageFeedback(fmt.Sprintf("Page %03d", idx))
	}
	c.pagePad = idx
	c.pagePadTime = now
	if err := c.drawPageMode(); err != nil {
		return err
	}
	if created {
		return c.pageFeedback(fmt.Sprintf("Page %03d created", idx))
	}
	if page.on {
		return c.pageFeedback(fmt.Sprintf("Page %03d on", idx))
	}
	return c.pageFeedback(fmt.Sprintf("Page %03d off", idx))
}

// pageFeedback writes a gesture's result on the readout row,
// which the next grid event or redraw replaces.
func (c *Controller) pageFeedback(text string) error {
	return c.patbank.printReadout(0, fitOLEDText(text))
}

// pageHeaderText is the grid's header: the current page, how
// many pages are in use and how many pads there are.
func (c *Controller) pageHeaderText() string {
	return fmt.Sprintf("Page %03d %2d/%d", c.pageIdx, len(c.pages), maxPages)
}

// drawPageMode paints the page grid: every pad is a page,
// with the header on row 0, the gesture hint on row 1 and
// rows 2-5 cleared.
func (c *Controller) drawPageMode() error {
	if err := c.drawPagePads(); err != nil {
		return err
	}
	if err := c.patbank.printText(0, 0, c.pageHeaderText(), false); err != nil {
		return err
	}
	if err := c.patbank.printText(1, 0, pageHintText, false); err != nil {
		return err
	}
	return c.patbank.clearTextRows(2, 4)
}

// drawPagePads lights every pad with its page's colour. The
// brightness rules apply in order, so the current page keeps
// its full-brightness reading even when its flag is off; a
// disabled current page is short-lived, because the next
// jump or Play press turns it back on.
func (c *Controller) drawPagePads() error {
	pads := make([]akai.Pad, 0, maxPages)
	for idx := 1; idx <= maxPages; idx++ {
		var color [3]int
		switch page := c.pages[idx]; {
		case page == nil:
			color = oledBlack
		case idx == c.pageIdx:
			color = patternColor(idx)
		case page.on:
			color = Dim(patternColor(idx), chromaticPaletteDim)
		default:
			color = dimColor(patternColor(idx))
		}
		pads = append(pads, makePad((idx-1)%padColumns, (idx-1)/padColumns, color))
	}
	return c.patbank.pads.LightPadSlice(pads)
}

// processPageEvent answers a Fire event while the page grid
// is showing. The shape mirrors the other two handlers: a
// control change wakes the blackout and routes through the
// shared knob guard, a pad release is nothing because the
// held pads were cleared on entry, and a pad press is a
// page gesture. Browser and Accent route through the same
// state-button handler the other views use, so a plain
// Browser leaves page mode and the Shift pair keeps its
// session meaning.
func (c *Controller) processPageEvent(aseq sequencerWriter, ev alsa.SeqEvent) error {
	if len(ev.Data) != 3 {
		return nil
	}
	status := ev.Data[0]
	velocity := int(ev.Data[2])
	logIncoming(int(ev.Data[1]), int(status), velocity)
	if midi.IsCC(status) {
		if err := c.wakeBlackout(); err != nil {
			return err
		}
		return c.handleControlChange(aseq, ev)
	}
	x, y, onGrid := Note2Grid(int(ev.Data[1]))
	if onGrid && isPadRelease(status, velocity) {
		return nil
	}
	if !midi.IsNoteOn(status) || velocity == 0 {
		return nil
	}
	// A release must not end a blackout, so the wake waits for a real press.
	if err := c.wakeBlackout(); err != nil {
		return err
	}
	if onGrid {
		return c.handlePageGrid(x, y)
	}
	switch int(ev.Data[1]) {
	case NoteStop:
		return c.stopPlayback()
	case NoteShift:
		c.shift = !c.shift
		if c.shift {
			return c.patbank.pads.SetLed(NoteShift, LEDRed)
		}
		return c.patbank.pads.SetLed(NoteShift, 0)
	case NotePatternSong:
		return c.togglePageMode()
	case NoteBrowser, NoteAccent:
		return c.handleStateButton(int(ev.Data[1]))
	}
	// Every button the unit has is bound above, so reaching
	// this means a number nobody bound arrived.
	logger.Debug("unbound control", "note", int(ev.Data[1]))
	return nil
}

// startSetPlayback starts a set over every enabled page, each playing
// what its own view points at. The Play press first enables the current
// page, because it is a claim that the page on screen belongs in the
// mix. The commit that opens it points the current page's memento at
// the live banks and the live view, so the shown page's session reads
// the live state through the memento as the same objects; the other
// enabled pages play from their own mementos. All the workers launch
// together, so every page starts on the same beat.
func (c *Controller) startSetPlayback(aseq sequencerWriter) error {
	page := c.currentPage()
	page.on = true
	c.updatePageIndicator()
	c.saveCurrentPage()
	c.running = newRunningSet(c, aseq)
	// The flag flip shows at once, but only on the arrangement: row 0
	// belongs to the pattern header on the pattern view.
	if c.mode == songView {
		return c.songbank.PrintSong()
	}
	return nil
}
