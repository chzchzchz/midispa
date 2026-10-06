package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The model tests work on a bare pattern, the way the pattern's own tests do,
// because the model is what the gesture and the session file both sit on.

// gridTrio places three notes on the first three cells of a beat, which is
// the state a triplet gesture needs: one note per cell the group would
// cover and nothing on the cell it draws.
func gridTrio(pattern *Pattern, voice *Voice) {
	for _, step := range []int{0, 1, 2} {
		pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(step), Velocity: 100})
	}
}

// groupedTrio places the three notes of gridTrio and sets the group on
// the beat, which is the state most of the model tests assert against.
func groupedTrio(t *testing.T, pattern *Pattern, voice *Voice, kind tripletKind) {
	t.Helper()
	gridTrio(pattern, voice)
	require.Truef(t, pattern.SetTripletGroup(0, voice, kind), "the group was refused")
}

// beatCase is one stop on a playback walk: what the playhead is set to
// and the messages the pattern is expected to write there, with no want
// for a beat nothing is due on.
type beatCase struct {
	beat float32
	want [][]byte
}

// walkBeats moves the playhead to each beat the way the playback worker
// does and asks the pattern what happens there, which is how the
// scheduler's own walk through the pattern is heard.
func walkBeats(t *testing.T, pattern *Pattern, cases []beatCase) {
	t.Helper()
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for _, tt := range cases {
		playback.setPosition(0, tt.beat)
		writer.events = nil
		_, err := playback.playBeat(writer, pattern)
		require.NoErrorf(t, err, "beat %v", tt.beat)
		events := writer.snapshot()
		require.Lenf(t, events, len(tt.want), "beat %v", tt.beat)
		for i, want := range tt.want {
			assertMidiData(t, events[i], want)
		}
	}
}

// A group re-times the three notes of a beat into the thirds of the beat, and
// the fourth cell stops being a step: a note placed there afterwards dissolves
// the group, and the repair puts every survivor back on the grid.
func TestTripletGroupRetimesTheBeatsItCovers(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletEighth)

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "the group changed how many notes the beat holds")
	for i, want := range []float32{0, 1.0 / 3, 2.0 / 3} {
		assert.InDelta(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}
	for i, want := range []int{0, 1, 2} {
		assert.Equalf(t, want, eventStep(events[i]), "note %d cell", i)
	}
	require.Equal(t, tripletEighth, events[0].Triplet, "the flag did not ride the first note")
	require.Equal(t, tripletNone, events[1].Triplet, "the flag reached the second note")
	require.Equal(t, tripletNone, events[2].Triplet, "the flag reached the third note")

	// The fourth cell is the group's, so it holds no note, and a note placed
	// there afterwards is a second way to take the group away.
	_, fourth := pattern.EventAtStep(3, voice)
	require.False(t, fourth, "the fourth cell holds a note")
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(3), Velocity: 100})
	_, covered := pattern.TripletGroupAt(0, voice)
	require.False(t, covered, "a note on the fourth cell left the group in place")
	events = pattern.EventsForVoice(voice)
	require.Len(t, events, 4, "the note placed on the fourth cell is missing")
	for i, event := range events {
		assert.Equalf(t, stepBeat(i), event.Beat, "note %d is off the grid after the group dissolved", i)
	}
}

// A group survives a save and a load: the file carries the flag on the first
// note and the off-grid beats of the other two, and the repair on load puts
// the timing back, so a group that came back as three grid notes would play
// straight and the assertion below would catch it. The eighth and the
// sixteenth group make the round trip in the one test, because the file
// format is the same for both and only the packing differs.
func TestTripletGroupSurvivesASessionSaveAndLoad(t *testing.T) {
	cases := []struct {
		name    string
		kind    tripletKind
		gesture func(t *testing.T, bank *PatternBank)
		beats   []float32
	}{
		{
			name: "eighth",
			kind: tripletEighth,
			gesture: func(t *testing.T, bank *PatternBank) {
				pressButton(t, bank, NoteAlt)
				pressPad(t, bank, 54+2*padColumns)
				pressButton(t, bank, NoteAlt)
			},
			beats: []float32{0, 1.0 / 3, 2.0 / 3},
		},
		{
			name: "sixteenth",
			kind: tripletSixteenth,
			gesture: func(t *testing.T, bank *PatternBank) {
				pressButton(t, bank, NoteAlt)
				pressButton(t, bank, NoteShift)
				pressPad(t, bank, 54+2*padColumns)
				pressButton(t, bank, NoteShift)
				pressButton(t, bank, NoteAlt)
			},
			beats: []float32{0, 1.0 / 6, 1.0 / 3},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			kit := stateKit()
			voice := kit.voices[2]
			session := stateTestBank(t, kit)
			bank, songs := session.patbank, session.songbank
			pattern := bank.CurrentPattern()
			pattern.SetChromaticNote(0, voice, 60, 100)
			pattern.SetChromaticNote(1, voice, 62, 100)
			pattern.SetChromaticNote(2, voice, 64, 100)
			// The gesture goes through the handler, on the row the
			// chromatic voice is shown on, which is the row a player
			// would press.
			tt.gesture(t, bank)
			group, covered := pattern.TripletGroupAt(0, voice)
			require.True(t, covered, "the gesture did not set the group")
			require.Equal(t, int(tt.kind), group.cells, "the gesture set the wrong size")

			index := bank.selPatIdx
			path := stateFilePath(t)
			saveTo(t, path, bank, songs, nil)
			saved, err := readStateFile(path)
			require.NoError(t, err)
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Contains(t, string(raw), fmt.Sprintf(`"triplet": %d`, int(tt.kind)),
				"the file does not carry the group")

			var savedEvents []stateEvent
			for _, statePattern := range saved.Patterns {
				if statePattern.Index == index {
					savedEvents = statePattern.Events
				}
			}
			require.Len(t, savedEvents, 3, "the saved pattern does not hold the three notes")
			require.Equal(t, tt.kind, savedEvents[0].Triplet, "the flag did not reach the file")
			require.Zero(t, savedEvents[0].Beat, "the first note is not on the beat")
			require.NotZero(t, savedEvents[1].Beat, "the second note's beat did not reach the file")
			require.NotZero(t, savedEvents[2].Beat, "the third note's beat did not reach the file")

			far := stateTestBank(t, stateKit())
			report, err := loadInto(far, path, nil)
			require.NoError(t, err)
			require.Zero(t, report.Dropped, "the load dropped events")
			loaded := far.patbank.Patterns[index]
			require.NotNil(t, loaded, "the pattern the file named is missing after the load")
			events := loaded.EventsForVoice(far.patbank.vb.voices[2])
			require.Len(t, events, 3, "the loaded pattern does not hold the three notes")
			for i, want := range tt.beats {
				assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "loaded note %d beat", i)
			}
			group, covered = loaded.TripletGroupAt(0, far.patbank.vb.voices[2])
			require.True(t, covered, "the group did not come back")
			require.Equal(t, int(tt.kind), group.cells, "the group came back at the wrong size")
		})
	}
}

// Any edit that breaks the cells a group owns dissolves the group, and
// the repair puts the survivors back on the grid: a dissolved group
// that left its notes off the grid is the failure this rule exists
// to catch, so each case asserts the survivors' beats, not only
// their cells.
func TestTripletGroupIsRepairedWhenItsNotesChange(t *testing.T) {
	cases := []struct {
		name       string
		breakGroup func(t *testing.T, pattern *Pattern, voice *Voice)
		wantBeats  []float32
	}{
		{
			name: "the first note removed",
			breakGroup: func(t *testing.T, pattern *Pattern, voice *Voice) {
				_, removed := pattern.RemoveEventAtStep(0, voice)
				require.True(t, removed, "the first note was not there to remove")
			},
			wantBeats: []float32{stepBeat(1), stepBeat(2)},
		},
		{
			name: "the middle note removed",
			breakGroup: func(t *testing.T, pattern *Pattern, voice *Voice) {
				_, removed := pattern.RemoveEventAtStep(1, voice)
				require.True(t, removed, "the middle note was not there to remove")
			},
			wantBeats: []float32{stepBeat(0), stepBeat(2)},
		},
		{
			name: "a note placed on the fourth cell",
			breakGroup: func(t *testing.T, pattern *Pattern, voice *Voice) {
				pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(3), Velocity: 100})
			},
			wantBeats: []float32{stepBeat(0), stepBeat(1), stepBeat(2), stepBeat(3)},
		},
		{
			name: "the pattern shortened past the group",
			breakGroup: func(t *testing.T, pattern *Pattern, voice *Voice) {
				pattern.SetLengthSteps(3)
			},
			wantBeats: []float32{stepBeat(0), stepBeat(1), stepBeat(2)},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			voice := trackWindowKit(8, -1).voices[0]
			pattern := &Pattern{}
			groupedTrio(t, pattern, voice, tripletEighth)
			tt.breakGroup(t, pattern, voice)
			_, covered := pattern.TripletGroupAt(0, voice)
			require.False(t, covered, "the edit left the group in place")
			events := pattern.EventsForVoice(voice)
			require.Len(t, events, len(tt.wantBeats), "the wrong number of notes survived")
			for i, event := range events {
				assert.Equalf(t, tt.wantBeats[i], event.Beat, "survivor %d is off the grid", i)
			}
		})
	}
}

// Every beat a setter writes floors back to its own cell, and a triplet's
// off-grid beats floor to the cell the note sits in rather than the one it
// rounds to, which is what the group's third note landing on its own cell
// instead of the consumed one rests on.
func TestEventStepKeepsGridBeatsOnTheirCells(t *testing.T) {
	for step := 0; step < maxPatternSteps; step++ {
		assert.Equalf(t, step, eventStep(Event{Beat: stepBeat(step)}),
			"step %d does not floor to its own cell", step)
	}
	for _, tt := range []struct {
		beat float32
		cell int
	}{
		{beat: 1.0 / 3, cell: 1},
		{beat: 2.0 / 3, cell: 2},
		{beat: 1.0 / 6, cell: 0},
	} {
		assert.Equalf(t, tt.cell, eventStep(Event{Beat: tt.beat}),
			"beat %v does not floor to cell %d", tt.beat, tt.cell)
	}
}

// Alt plus a pad toggles the group of the beat the pressed cell
// falls in, and every cell a group owns reaches the same group,
// which is what makes a press on any of them take the group away
// again.
func TestAltPadTogglesTheTripletGroupOnItsBeat(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, -1))
	voice := kit.voices[0]
	require.NoError(t, bank.SelectTrackRow(1))
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	for _, tt := range []struct {
		note    int
		covered bool
	}{
		{note: 54, covered: true},
		{note: 55, covered: false},
		{note: 56, covered: true},
		{note: 57, covered: false},
	} {
		pressPad(t, bank, tt.note)
		_, covered := bank.CurrentPattern().TripletGroupAt(0, voice)
		require.Equalf(t, tt.covered, covered,
			"a press on pad %d did not answer %v", tt.note, tt.covered)
	}
	pressButton(t, bank, NoteAlt)
}

// The gesture edits the row the press came from, not the row the
// selection is on, which is the rule that differs from a tie: a
// percussive row can be given a group with no selection at all.
func TestAltPadEditsTheRowItWasPressedOn(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, -1))
	require.NoError(t, bank.SelectTrackRow(1))
	for _, note := range []int{54, 55, 56, 70, 71, 72} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 70)
	pressButton(t, bank, NoteAlt)

	pressed, covered := bank.CurrentPattern().TripletGroupAt(0, kit.voices[1])
	require.True(t, covered, "the press on row 2 did not set its own group")
	require.Equal(t, int(tripletEighth), pressed.cells, "row 2's group is the wrong size")
	_, selected := bank.CurrentPattern().TripletGroupAt(0, kit.voices[0])
	require.False(t, selected, "the press on row 2 set the selected row's group")

	events := bank.CurrentPattern().EventsForVoice(kit.voices[1])
	require.Len(t, events, 3, "row 2's notes changed count")
	for i, want := range []float32{0, 1.0 / 3, 2.0 / 3} {
		assert.InDelta(t, float64(want), float64(events[i].Beat), 1e-6, "row 2 note %d beat", i)
	}
}

// A gesture that cannot be satisfied is refused whole, and the
// refusal is one of four named errors rather than text, so a
// caller can tell the reasons apart with errors.Is.
func TestTripletRefusalsAreNamedErrors(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	trio := func() *Pattern {
		pattern := &Pattern{}
		gridTrio(pattern, voice)
		return pattern
	}

	_, err := (&Pattern{}).ToggleTripletGroup(0, voice, tripletEighth)
	require.ErrorIs(t, err, errTripletNoNotes, "an empty beat")

	busy := trio()
	busy.ToggleEvent(Event{Voice: voice, Beat: stepBeat(3), Velocity: 100})
	_, err = busy.ToggleTripletGroup(0, voice, tripletEighth)
	require.ErrorIs(t, err, errTripletBusy, "a beat whose fourth cell holds a note")

	pastEnd := trio()
	pastEnd.SetLengthSteps(3)
	_, err = pastEnd.ToggleTripletGroup(0, voice, tripletEighth)
	require.ErrorIs(t, err, errTripletPastEnd, "a beat that runs past the length")

	overlapping := trio()
	require.True(t, overlapping.SetTripletGroup(0, voice, tripletEighth),
		"the eighth group was refused")
	_, err = overlapping.ToggleTripletGroup(0, voice, tripletSixteenth)
	require.ErrorIs(t, err, errTripletOverlap, "a sixteenth group over an eighth group's cells")
}

// A gesture that cannot be satisfied is refused whole: the readout names
// the span and the reason, and the notes are left exactly as they were,
// because a half-applied group would leave its notes off the grid with
// no flag to gather them again.
func TestAltPadRefusesWhenTheBeatCannotHoldAGroup(t *testing.T) {
	bank, kit, recorder := screenBank(t, trackWindowKit(8, -1))
	pattern := bank.CurrentPattern()
	voice := kit.voices[0]
	wantBeats := func(beats ...float32) {
		t.Helper()
		events := pattern.EventsForVoice(voice)
		require.Len(t, events, len(beats), "the refusal changed the notes")
		for i, event := range events {
			assert.InDeltaf(t, float64(beats[i]), float64(event.Beat), 1e-6,
				"note %d changed", i)
		}
	}

	// A beat with no notes in it has nothing to group.
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 58)
	require.Contains(t, recorder.row(readoutRow), "S05-08 need 3")
	wantBeats()

	// A note on the fourth cell leaves the beat too busy to draw.
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 57)
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S04 busy")
	wantBeats(stepBeat(3))

	// A beat that runs past the end of the pattern cannot hold a group.
	pattern.SetLengthSteps(3)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-04 past end")
	wantBeats()

	// A group of the other size owns the same cells.
	pattern.SetLengthSteps(16)
	pressButton(t, bank, NoteAlt)
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-04 x3")
	pressButton(t, bank, NoteShift)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-02 overlap")
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteAlt)
	wantBeats(0, 1.0/3, 2.0/3)
	_, covered := pattern.TripletGroupAt(0, voice)
	require.True(t, covered, "the refused gesture took the eighth group with it")
}

// While a length is being entered the gesture declines silently: the
// entry owns the readout row, so a group line would overwrite the
// number being typed, and the press must not fall through to the step
// grid either, or a gesture would place a note it never said anything
// about.
func TestAltPadDeclinesWhileALengthIsBeingEntered(t *testing.T) {
	bank, kit, recorder := screenBank(t, trackWindowKit(8, -1))
	pattern := bank.CurrentPattern()
	pressButton(t, bank, NoteOverview)
	require.True(t, bank.lengthEditActive(), "the length entry did not open")
	require.Contains(t, recorder.row(readoutRow), "Length")
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "Length",
		"the gesture wrote over the length being entered")
	require.Empty(t, pattern.EventsForVoice(kit.voices[0]),
		"the press placed a note")
	_, covered := pattern.TripletGroupAt(0, kit.voices[0])
	require.False(t, covered, "the press set a group")
	pressButton(t, bank, NoteAlt)
	pressButton(t, bank, NoteOverview)
}

// Every line the gesture writes names the span of the beat it acted on
// rather than the cell the cursor sits on, because the gesture snaps to
// the group's own cells: a player whose cursor parks elsewhere still
// reads which beat was answered. Pad presses leave the cursor alone, so
// it stays a witness through the whole run.
func TestTripletReadoutNamesTheBeatNotTheCursor(t *testing.T) {
	bank, kit, recorder := screenBank(t, trackWindowKit(8, -1))
	pattern := bank.CurrentPattern()
	voice := kit.voices[0]
	require.NoError(t, bank.setStepCursor(10))

	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-04 x3")
	pressPad(t, bank, 55)
	require.Contains(t, recorder.row(readoutRow), "S01-04 off")
	pressPad(t, bank, 58)
	require.Contains(t, recorder.row(readoutRow), "S05-08 need 3")

	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 57)
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S04 busy")

	pressButton(t, bank, NoteAlt)
	pattern.SetLengthSteps(3)
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-04 past end")

	pattern.SetLengthSteps(16)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-04 x3")
	pressButton(t, bank, NoteShift)
	pressPad(t, bank, 54)
	require.Contains(t, recorder.row(readoutRow), "S01-02 overlap")
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteAlt)

	require.NotContains(t, recorder.row(readoutRow), "S11")
	require.Equal(t, 10, bank.StepCursor(), "a pad press moved the step cursor")
	_, covered := pattern.TripletGroupAt(0, voice)
	require.True(t, covered, "the group did not survive the run")
}

// The gesture on a percussive row packs the three notes of the beat
// into the thirds of the beat, with the flag riding the first note. A
// percussive row needs no selection, because the press names the row
// it came from, which is what makes a group the one beat edit a
// percussive track can be given at all.
func TestAltPadOnAPercussiveRowMakesAGroup(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, -1))
	pattern := bank.CurrentPattern()
	voice := kit.voices[0]
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	pressButton(t, bank, NoteAlt)

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "the gesture changed how many notes the beat holds")
	for i, want := range []float32{0, 1.0 / 3, 2.0 / 3} {
		assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}
	group, covered := pattern.TripletGroupAt(0, voice)
	require.True(t, covered, "the gesture did not set the group")
	require.Equal(t, int(tripletEighth), group.cells, "the group is the wrong size")
	require.Equal(t, tripletEighth, events[0].Triplet, "the flag did not ride the first note")
	require.Equal(t, tripletNone, events[1].Triplet, "the flag reached the second note")
	require.Equal(t, tripletNone, events[2].Triplet, "the flag reached the third note")
}

// A group is an edit of what a beat means, not an edit of the set: the
// gesture neither stops the playback behind the pattern nor auditions a
// note of its own, the way the step grid's own press does when it places
// or removes a note. The stub session is installed but not running, so
// the writer hears nothing unless the gesture reaches for it.
func TestTripletDoesNotStopPlaybackOrAudition(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, -1))
	writer := &captureMidiWriter{}
	bank.controller.playback = stubSession(&Playback{writer: writer}, nil)
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	pressButton(t, bank, NoteAlt)

	require.NotNil(t, bank.controller.playback, "the gesture stopped the set")
	assert.Empty(t, writer.snapshot(), "the gesture auditioned a note")
}

// Alt plus a step cell in note-edit mode is the triplet gesture, not
// the second half of a tie: the gesture answers before anything is
// recorded as held for a tie, so a press cannot tie steps the player
// never asked to tie. Both steps hold a note here, which is exactly
// the pair a tie would have joined had the gesture not won the press.
func TestAltStepCellDoesNotCompleteATie(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(3, voice, 64, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())

	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, stepCellPad(0))
	require.Equal(t, noHeldStep, bank.noteEditHeldStep,
		"the gesture recorded the step as held for a tie")
	event, ok := pattern.EventAtStep(0, voice)
	require.True(t, ok, "the note on step 0 is gone")
	require.False(t, event.Tie, "the gesture tied the step")
	pressButton(t, bank, NoteAlt)

	pressPad(t, bank, stepCellPad(3))
	require.Equal(t, 3, bank.StepCursor(), "the press did not move the edit")
	require.Equal(t, 3, bank.noteEditHeldStep, "the press did not hold the step")
	event, ok = pattern.EventAtStep(0, voice)
	require.True(t, ok, "the note on step 0 is gone")
	require.False(t, event.Tie, "a single held step tied the two notes")
}

// Alt plus a palette pad keeps its note-edit meaning: the
// palette's first pad removes the note, and so does Alt on any
// pitch pad, because the gesture only claims the step cells
// beside the palette. A gesture that swallowed the removal
// would leave a note the player asked to take away.
func TestAltPalettePadStillClearsTheNote(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())

	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	pressButton(t, bank, NoteAlt)

	_, ok := pattern.EventAtStep(0, voice)
	require.False(t, ok, "Alt plus the erase pad left the note in place")
}

// The gesture only claims the press when Alt is held: a plain
// press still toggles a percussive step, and on a chromatic row
// it still moves the cursor and ties two held cells. Those are
// the gestures the triplet sits beside, so a change to one of
// them would be a change to editing itself.
func TestPlainPadGesturesAreUnchanged(t *testing.T) {
	t.Run("percussive step", func(t *testing.T) {
		bank, kit := quietBank(t, trackWindowKit(8, -1))
		voice := kit.voices[0]
		pattern := bank.CurrentPattern()
		require.NoError(t, bank.SelectTrackRow(1))
		pressPad(t, bank, 54)
		event, ok := pattern.EventAtStep(0, voice)
		require.True(t, ok, "a plain press did not place a note")
		require.Equal(t, 100, event.Velocity, "the placed note lost its dynamics")
		pressPad(t, bank, 54)
		_, ok = pattern.EventAtStep(0, voice)
		require.False(t, ok, "a second plain press did not remove the note")
	})

	t.Run("chromatic cursor and tie", func(t *testing.T) {
		bank, voice := chromaBank(t)
		pattern := bank.CurrentPattern()
		pattern.SetChromaticNote(0, voice, 60, 100)
		pattern.SetChromaticNote(1, voice, 62, 100)
		pattern.SetChromaticNote(3, voice, 64, 100)
		require.NoError(t, bank.SelectTrackRow(1))
		pressPad(t, bank, 54)
		require.Equal(t, 0, bank.StepCursor(), "the press did not move the cursor")
		_, ok := pattern.EventAtStep(0, voice)
		require.True(t, ok, "a plain press removed the chromatic note")
		pressPad(t, bank, 55)
		event, ok := pattern.EventAtStep(0, voice)
		require.True(t, ok, "the first note of the tie is gone")
		require.True(t, event.Tie, "two held cells did not tie their steps")
	})
}

// A note placed on the cell an eighth group draws takes the
// group apart, and the repair puts every survivor back on
// the grid. The press repaints the whole row rather than
// its own cell, because the repair can move notes onto
// cells the press never touched: only a full repaint can
// show the drawn cell back as a note.
func TestPlacingANoteOnAConsumedCellRepaintsTheRow(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, -1))
	recorder := usePadRecorder(t, &bank.pads)
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(8))
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressPad(t, bank, 54)
	pressButton(t, bank, NoteAlt)

	pressPad(t, bank, 57)

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 4, "the note placed on the drawn cell is missing")
	for i, event := range events {
		assert.Equalf(t, stepBeat(i), event.Beat, "note %d is off the grid", i)
	}
	for col := 0; col < 4; col++ {
		require.Equalf(t, percussionStepColor, recorder.pad(col, 0),
			"cell %d did not go back to its note colour", col)
	}
}

// A group plays its three notes inside the beat they
// share: each third of the beat sounds where the packing
// put it, and the fourth cell stays silent, which is
// what the scheduler's own walk through the pattern
// hears. The playhead is set to each beat the way the
// worker moves it, and the pattern is asked what happens
// there.
func TestTripletPlaysThreeNotesInsideOneBeat(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletEighth)
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(4), Velocity: 100})

	note, _ := voice.PercussionNote()
	hit := [][]byte{
		{midi.MakeNoteOff(0), byte(note), 100},
		{midi.MakeNoteOn(0), byte(note), 100},
	}
	// The playhead polls between the third note and the next
	// one, a sixteenth away: nothing is due at the fourth
	// quarter.
	walkBeats(t, pattern, []beatCase{
		{beat: 0, want: hit},
		{beat: 1.0 / 3, want: hit},
		{beat: 2.0 / 3, want: hit},
		{beat: 3.0 / 4},
		{beat: 1, want: hit},
	})
}

// Grid steps still play on the grid: the scheduler's
// walk answers an ordinary pattern the way it did
// before the triplet change, which is what keeps the
// two walks one loop rather than two.
func TestGridStepsStillPlayOnTheGridAfterTheSchedulerChange(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	gridTrio(pattern, voice)
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(4), Velocity: 100})

	note, _ := voice.PercussionNote()
	hit := [][]byte{
		{midi.MakeNoteOff(0), byte(note), 100},
		{midi.MakeNoteOn(0), byte(note), 100},
	}
	walkBeats(t, pattern, []beatCase{
		{beat: 0, want: hit},
		{beat: 0.25, want: hit},
		{beat: 0.5, want: hit},
		{beat: 0.75},
		{beat: 1, want: hit},
	})
}

// A triplet's second note can be tied into the third,
// which is the case the scheduler's epsilon has to
// absorb: the tie holds the second note past its own
// cell, and the third note, sharing the second's
// pitch, sounds no message at all where a pitch
// change would sound two.
func TestTripletSecondNoteCanBeTiedIntoTheThird(t *testing.T) {
	voice := trackWindowKit(8, 0).voices[0]
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 62, 100)
	pattern.SetChromaticNote(4, voice, 60, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletEighth), "the group was refused")
	require.True(t, pattern.TieEventsAtSteps(1, 2, voice), "the tie was refused")

	walkBeats(t, pattern, []beatCase{
		{beat: 0, want: [][]byte{{midi.MakeNoteOn(0), 60, 100}}},
		{beat: 1.0 / 3, want: [][]byte{
			{midi.MakeNoteOff(0), 60, 0},
			{midi.MakeNoteOn(0), 62, 100},
		}},
		{beat: 1.0 / 2},
		{beat: 2.0 / 3},
		{beat: 3.0 / 4, want: [][]byte{{midi.MakeNoteOff(0), 62, 0}}},
		{beat: 1, want: [][]byte{{midi.MakeNoteOn(0), 60, 100}}},
	})
}

// A triplet's third note can be tied past the beat
// line it ends, into a note of the next beat: the
// tie holds the note through the beat boundary and
// through the next poll, and the note that follows
// the tied pair sounds as the pair's release and
// its own start.
func TestTripletTiedThroughTheBeatLine(t *testing.T) {
	voice := trackWindowKit(8, 0).voices[0]
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	pattern.SetChromaticNote(4, voice, 64, 100)
	pattern.SetChromaticNote(5, voice, 67, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletEighth), "the group was refused")
	require.True(t, pattern.TieEventsAtSteps(2, 4, voice), "the tie was refused")

	walkBeats(t, pattern, []beatCase{
		{beat: 0, want: [][]byte{{midi.MakeNoteOn(0), 60, 100}}},
		{beat: 1.0 / 3, want: [][]byte{
			{midi.MakeNoteOff(0), 60, 0},
			{midi.MakeNoteOn(0), 62, 100},
		}},
		{beat: 2.0 / 3, want: [][]byte{
			{midi.MakeNoteOff(0), 62, 0},
			{midi.MakeNoteOn(0), 64, 100},
		}},
		{beat: 3.0 / 4},
		{beat: 1},
		{beat: 5.0 / 4, want: [][]byte{
			{midi.MakeNoteOff(0), 64, 0},
			{midi.MakeNoteOn(0), 67, 100},
		}},
	})
}

// A group lights the cell it draws with the shade of
// its first note, and its notes keep the colours of
// their pitches, which is what tells a triplet's
// note from a percussive step beside it; a tie marks
// only the step that carries it: the cells a tie runs
// across stay dark, because they hold no note of
// their own.
func TestTripletLightsFourCellsAndATieLightsThree(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletEighth), "the group was refused")
	pattern.SetChromaticNote(4, voice, 65, 100)
	pattern.SetChromaticNote(8, voice, 67, 100)
	require.True(t, pattern.TieEventsAtSteps(4, 8, voice), "the tie was refused")
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(10))
	require.NoError(t, bank.redrawTrackPads(1))

	for i, note := range []int{60, 62, 64} {
		require.Equalf(t, chromaticPaletteColor(note), sim.pads[i],
			"the group's note %d", i)
	}
	require.Equal(t, tripletShadeColor(chromaticPaletteColor(60), false), sim.pads[3],
		"the cell the group draws")
	require.Equal(t, markTieColor(chromaticPaletteColor(65), false), sim.pads[4],
		"the tied step")
	for _, cell := range []int{5, 6, 7} {
		require.Equalf(t, [3]int{}, sim.pads[cell], "the tie lights cell %d", cell)
	}
	require.Equal(t, chromaticPaletteColor(67), sim.pads[8], "the tie's end")
}

// The shade a group draws is dimmer than the note it
// comes from: the cell belongs to the group rather
// than to a step, so it reads as the note's own
// colour held at half strength, and never as a step
// that is lit or selected.
func TestTripletShadeIsDimmerThanItsNote(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, -1))
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	groupedTrio(t, pattern, voice, tripletEighth)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(8))
	require.NoError(t, bank.redrawTrackPads(1))

	require.Equal(t, tripletShadeColor(percussionStepColor, false), sim.pads[3],
		"the cell the group draws")
	dimmed := Dim(percussionStepColor, tripletShadeDivisor)
	for channel := range percussionStepColor {
		require.LessOrEqualf(t, dimmed[channel], percussionStepColor[channel],
			"channel %d is not dimmed", channel)
	}
	require.NotEqual(t, percussionStepColor, sim.pads[3],
		"the shade is the note's own colour")
	require.NotEqual(t, oledWhite, sim.pads[3],
		"the shade is the selected colour")
}

// The shade a group draws follows the playhead's own
// rule: under the playhead it is pushed down rather
// than lifted, the way a tied step is, so a group's
// cell still reads as belonging to the beat it sits in.
func TestTripletMarkFollowsThePlayhead(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletEighth), "the group was refused")
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(10))

	require.NoError(t, bank.drawPadColumnInvert(3))
	require.Equal(t, tripletShadeColor(chromaticPaletteColor(60), true), sim.pads[3],
		"the drawn cell under the playhead")
	require.NoError(t, bank.drawPadColumn(3))
	require.Equal(t, tripletShadeColor(chromaticPaletteColor(60), false), sim.pads[3],
		"the drawn cell behind the playhead")
}

// The step strip reads a group the way the grid
// does: the notes keep their pitch colours and the
// cell the group draws carries the shade, which is
// how the strip tells a group's cell from an empty
// step.
func TestTripletReadsOnTheStepStripInNoteEdit(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletEighth), "the group was refused")
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(8))
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, bank.drawNotePalette())

	for i, note := range []int{60, 62, 64} {
		require.Equalf(t, chromaticPaletteColor(note), stripCell(sim, i),
			"the group's note %d", i)
	}
	require.Equal(t, tripletShadeColor(chromaticPaletteColor(60), false), stripCell(sim, 3),
		"the cell the group draws")
}

// The step status names a triplet the way it
// names a tie: a covered step carries the
// group's x3, the cell a group draws shows the
// notes it belongs to instead of a velocity, and
// a step the group does not reach shows neither.
func TestTripletStepStatusText(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	event := &Event{Voice: voice, ChromaticNote: 60, Velocity: 100}
	tied := &Event{Voice: voice, ChromaticNote: 60, Velocity: 100, Tie: true}
	eighth := stepMarks{groupStart: 0, groupCells: int(tripletEighth)}
	sixteenth := stepMarks{groupStart: 0, groupCells: int(tripletSixteenth)}

	require.Equal(t, "S01 C4@100 x3",
		stepStatusText(voice, 0, event, eighth))
	require.Equal(t, "S02 C4@100 x3",
		stepStatusText(voice, 1, event, eighth))
	require.Equal(t, "S02 C4@100 x3->05",
		stepStatusText(voice, 1, tied, stepMarks{tieStep: 4, groupStart: 0, groupCells: int(tripletEighth)}))
	require.Equal(t, "S04 x3 01-03",
		stepStatusText(voice, 3, nil, eighth))
	require.Equal(t, "S01 C4@100 x3",
		stepStatusText(voice, 0, event, sixteenth))
	require.Equal(t, "S02 C4@100 x3",
		stepStatusText(voice, 1, event, sixteenth))
	require.Equal(t, "S05 --",
		stepStatusText(voice, 4, nil, stepMarks{groupStart: noGroup, groupCells: 0}))
}

// The gesture lights the whole span of the beat it
// answers: the three notes plus the cell the
// group draws, which is four lit pads for an
// eighth. Taking the group away again leaves the
// three notes.
func TestTraceTripletGroupLightsFourCells(t *testing.T) {
	snapshots := traceUI(t, []uiStep{
		press("select row 1", NoteMute1),
		press("note one", 54),
		press("note two", 55),
		press("note three", 56),
		press("alt", NoteAlt),
		press("gesture on the fourth cell", 57),
		press("alt off", NoteAlt),
		press("alt again", NoteAlt),
		press("gesture on the fourth cell again", 57),
		press("alt off again", NoteAlt),
	})
	require.Equal(t, 4, snapshots[5].litRow[0],
		"the group's beat did not light four cells")
	require.Equal(t, 3, snapshots[8].litRow[0],
		"the cleared beat did not light three cells")
}

// A sixteenth group packs the three notes of
// three cells into the two cells of an eighth:
// the first two notes share the first cell,
// which is what the beats alone tell apart, and
// the third cell is left free.
func TestSixteenthGroupPacksThreeCellsIntoTwo(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletSixteenth)

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "the group changed how many notes the beat holds")
	for i, want := range []float32{0, 1.0 / 6, 1.0 / 3} {
		assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}
	for i, want := range []int{0, 0, 1} {
		assert.Equalf(t, want, eventStep(events[i]), "note %d cell", i)
	}
	_, third := pattern.EventAtStep(2, voice)
	require.False(t, third, "the third cell holds a note")
	require.Equal(t, tripletSixteenth, events[0].Triplet, "the flag did not ride the first note")
	require.Equal(t, tripletNone, events[1].Triplet, "the flag reached the second note")
	require.Equal(t, tripletNone, events[2].Triplet, "the flag reached the third note")
}

// Clearing a sixteenth group puts its notes
// back on the cells they came from: the
// packing is undone whole, so a note left
// behind at a packed beat would play off the
// grid with no group to gather it.
func TestSixteenthGroupUnpacksWhenCleared(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletSixteenth)

	set, err := pattern.ToggleTripletGroup(0, voice, tripletSixteenth)
	require.NoError(t, err, "clearing the group was refused")
	require.False(t, set, "the group did not clear")

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "clearing changed how many notes the beat holds")
	for i, want := range []int{0, 1, 2} {
		assert.Equalf(t, stepBeat(want), events[i].Beat, "note %d beat", i)
		assert.Equalf(t, want, eventStep(events[i]), "note %d cell", i)
	}
	_, covered := pattern.TripletGroupAt(0, voice)
	require.False(t, covered, "the group survived being cleared")
}

// Removing one note of a sixteenth group breaks
// the packing, and the survivors spread onto
// the cells the walk finds free: the first keeps
// its own cell and the next takes the one after
// it, so a voice never holds two notes on one
// cell once the group is gone.
func TestSixteenthGroupSurvivorsSpreadWhenANoteIsRemoved(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletSixteenth)

	// The group's third note is the one on its own cell; taking it
	// away leaves the first two sharing a cell, which is the walk
	// the repair runs.
	_, removed := pattern.RemoveEventAtStep(1, voice)
	require.True(t, removed, "the group's third note was not there to remove")

	_, covered := pattern.TripletGroupAt(0, voice)
	require.False(t, covered, "the group survived losing a note")
	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 2, "the wrong number of notes survived")
	for i, want := range []int{0, 1} {
		assert.Equalf(t, want, eventStep(events[i]), "survivor %d cell", i)
		assert.InDeltaf(t, float64(stepBeat(want)), float64(events[i].Beat), 1e-6,
			"survivor %d beat", i)
	}
}

// Groups of the two sizes refuse to share
// cells, because the repair could not tell
// which group a survivor belonged to. Two
// sixteenth groups do coexist, one per half
// beat: the packing frees the cell between
// them, which is what lets a beat hold six
// notes.
func TestGroupsRefuseToOverlap(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	trio := func() *Pattern {
		pattern := &Pattern{}
		gridTrio(pattern, voice)
		return pattern
	}

	eighth := trio()
	require.True(t, eighth.SetTripletGroup(0, voice, tripletEighth), "the eighth group was refused")
	_, err := eighth.ToggleTripletGroup(0, voice, tripletSixteenth)
	require.ErrorIs(t, err, errTripletOverlap, "a sixteenth group over an eighth group")
	_, err = eighth.ToggleTripletGroup(2, voice, tripletSixteenth)
	require.ErrorIs(t, err, errTripletOverlap, "a sixteenth group over an eighth group's cell")

	sixteenth := trio()
	require.True(t, sixteenth.SetTripletGroup(0, voice, tripletSixteenth), "the sixteenth group was refused")
	_, err = sixteenth.ToggleTripletGroup(0, voice, tripletEighth)
	require.ErrorIs(t, err, errTripletOverlap, "an eighth group over a sixteenth group")

	pattern := &Pattern{}
	for _, step := range []int{0, 1, 2, 3, 4} {
		pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(step), Velocity: 100})
	}
	require.True(t, pattern.SetTripletGroup(0, voice, tripletSixteenth), "the first group was refused")
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(2), Velocity: 100})
	require.True(t, pattern.SetTripletGroup(2, voice, tripletSixteenth), "the second group was refused")

	groups := pattern.tripletGroups(voice)
	require.Equal(t, []tripletGroup{{start: 0, cells: 2}, {start: 2, cells: 2}}, groups,
		"the two groups did not both survive")
	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 6, "the beat does not hold six notes")
	for i, event := range events {
		assert.InDeltaf(t, float64(i)/6, float64(event.Beat), 1e-6, "note %d beat", i)
	}
}

// Alt plus Shift plus a pad toggles the
// sixteenth group of the eighth the pressed
// cell sits in: every cell of the group
// reaches the same group, which is what
// makes a press on any of them take the
// group away again.
func TestAltShiftPadTogglesTheSixteenthGroupOnItsEighth(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(8, -1))
	pattern := bank.CurrentPattern()
	voice := kit.voices[0]
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressButton(t, bank, NoteShift)

	pressPad(t, bank, 54)
	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "the gesture changed how many notes the beat holds")
	for i, want := range []float32{0, 1.0 / 6, 1.0 / 3} {
		assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}
	group, covered := pattern.TripletGroupAt(0, voice)
	require.True(t, covered, "the gesture did not set the group")
	require.Equal(t, int(tripletSixteenth), group.cells, "the gesture set the wrong size")

	// The second cell reaches the same group, so pressing it takes
	// the group away again.
	pressPad(t, bank, 55)
	_, covered = pattern.TripletGroupAt(0, voice)
	require.False(t, covered, "a press on the second cell left the group in place")
	events = pattern.EventsForVoice(voice)
	require.Len(t, events, 3, "clearing changed how many notes the beat holds")
	for i, want := range []int{0, 1, 2} {
		assert.InDeltaf(t, float64(stepBeat(want)), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}

	pressPad(t, bank, 54)
	_, covered = pattern.TripletGroupAt(0, voice)
	require.True(t, covered, "the gesture did not set the group again")
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteAlt)
}

// With Alt off, Shift plus a pad still types a
// tempo digit, which is the binding the
// sixteenth gesture costs: the pair has to
// mean the group when both modifiers are held
// and a tempo digit when only Shift is.
func TestAltShiftPadStillTypesTempoWhenAltIsOff(t *testing.T) {
	bank, _, recorder := screenBank(t, trackWindowKit(8, -1))
	pressButton(t, bank, NoteShift)
	pressPad(t, bank, padTyping(6))
	require.Equal(t, 6, bank.controller.pending, "the pad did not type a digit")
	require.Contains(t, recorder.row(readoutRow), "Tempo",
		"the typed digit is not on the readout")
	pressButton(t, bank, NoteShift)
	require.Zero(t, bank.controller.pending, "the release left the digit standing")
}

// The sixteenth gesture edits the row the
// press came from, not the row the selection
// is on, which is the same rule the eighth
// gesture holds. In note-edit mode the
// gesture answers on the selected voice, the
// voice the step cells edit.
func TestAltShiftPadEditsTheRowItWasPressedOn(t *testing.T) {
	t.Run("percussive rows", func(t *testing.T) {
		bank, kit := quietBank(t, trackWindowKit(8, -1))
		require.NoError(t, bank.SelectTrackRow(1))
		for _, note := range []int{54, 55, 56, 70, 71, 72} {
			pressPad(t, bank, note)
		}
		pressButton(t, bank, NoteAlt)
		pressButton(t, bank, NoteShift)
		pressPad(t, bank, 70)
		pressButton(t, bank, NoteShift)
		pressButton(t, bank, NoteAlt)

		pattern := bank.CurrentPattern()
		group, covered := pattern.TripletGroupAt(0, kit.voices[1])
		require.True(t, covered, "the press on row 2 did not set its own group")
		require.Equal(t, int(tripletSixteenth), group.cells, "row 2's group is the wrong size")
		_, selected := pattern.TripletGroupAt(0, kit.voices[0])
		require.False(t, selected, "the press on row 2 set the selected row's group")

		events := pattern.EventsForVoice(kit.voices[1])
		require.Len(t, events, 3, "row 2's notes changed count")
		for i, want := range []float32{0, 1.0 / 6, 1.0 / 3} {
			assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "row 2 note %d beat", i)
		}
	})

	t.Run("note edit", func(t *testing.T) {
		bank, voice := chromaBank(t)
		pattern := bank.CurrentPattern()
		pattern.SetChromaticNote(0, voice, 60, 100)
		pattern.SetChromaticNote(1, voice, 62, 100)
		pattern.SetChromaticNote(2, voice, 64, 100)
		require.NoError(t, bank.SelectTrackRow(1))
		require.NoError(t, bank.ToggleNoteMode())
		pressButton(t, bank, NoteAlt)
		pressButton(t, bank, NoteShift)
		pressPad(t, bank, stepCellPad(0))
		pressButton(t, bank, NoteShift)
		pressButton(t, bank, NoteAlt)

		group, covered := pattern.TripletGroupAt(0, voice)
		require.True(t, covered, "the gesture did not set the group on the selected voice")
		require.Equal(t, int(tripletSixteenth), group.cells, "the group is the wrong size")
		events := pattern.EventsForVoice(voice)
		require.Len(t, events, 3, "the gesture changed how many notes the beat holds")
		for i, want := range []float32{0, 1.0 / 6, 1.0 / 3} {
			assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
		}
	})
}

// Two sixteenth groups fill one beat: the
// flag rides the first note of each group,
// which is how the repair tells a group's
// own notes from a survivor sharing a cell.
func TestTwoSixteenthGroupsFillOneBeat(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	gridTrio(pattern, voice)
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(3), Velocity: 100})
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(4), Velocity: 100})
	require.True(t, pattern.SetTripletGroup(0, voice, tripletSixteenth), "the first group was refused")
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(2), Velocity: 100})
	require.True(t, pattern.SetTripletGroup(2, voice, tripletSixteenth), "the second group was refused")

	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 6, "the beat does not hold six notes")
	for i, want := range []float32{0, 1.0 / 6, 1.0 / 3, 1.0 / 2, 2.0 / 3, 5.0 / 6} {
		assert.InDeltaf(t, float64(want), float64(events[i].Beat), 1e-6, "note %d beat", i)
	}
	require.Equal(t, tripletSixteenth, events[0].Triplet,
		"the first group's flag did not ride its first note")
	require.Equal(t, tripletNone, events[1].Triplet,
		"the flag reached the first group's middle note")
	require.Equal(t, tripletNone, events[2].Triplet,
		"the flag reached the first group's third note")
	require.Equal(t, tripletSixteenth, events[3].Triplet,
		"the second group's flag did not ride its first note")
	require.Equal(t, tripletNone, events[4].Triplet,
		"the flag reached the second group's middle note")
	require.Equal(t, tripletNone, events[5].Triplet,
		"the flag reached the second group's third note")
}

// A sixteenth group plays its three notes
// inside the half beat it owns: each third
// of the half beat sounds where the packing
// put it, and the note after the group
// sounds on the grid it was placed on.
func TestSixteenthGroupPlaysThreeNotesInsideHalfABeat(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	groupedTrio(t, pattern, voice, tripletSixteenth)
	// The packing freed the third cell, so a note can sit after
	// the group on the grid.
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(2), Velocity: 100})

	note, _ := voice.PercussionNote()
	hit := [][]byte{
		{midi.MakeNoteOff(0), byte(note), 100},
		{midi.MakeNoteOn(0), byte(note), 100},
	}
	walkBeats(t, pattern, []beatCase{
		{beat: 0, want: hit},
		{beat: 1.0 / 6, want: hit},
		{beat: 1.0 / 3, want: hit},
		{beat: 1.0 / 4},
		{beat: 1.0 / 2, want: hit},
	})
}

// A sixteenth group draws no cell, so the
// pads show only the notes: the first two
// share the first cell's pad, and the third
// cell the packing freed stays dark.
func TestSixteenthGroupHasNoPadMark(t *testing.T) {
	bank, _, sim := recordedBank(t, trackWindowKit(8, -1))
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(8))
	for _, note := range []int{54, 55, 56} {
		pressPad(t, bank, note)
	}
	pressButton(t, bank, NoteAlt)
	pressButton(t, bank, NoteShift)
	pressPad(t, bank, 54)
	pressButton(t, bank, NoteShift)
	pressButton(t, bank, NoteAlt)
	require.NoError(t, bank.redrawTrackPads(1))

	require.Equal(t, percussionStepColor, sim.pads[0], "the group's first cell")
	require.Equal(t, percussionStepColor, sim.pads[1], "the group's second cell")
	require.Equal(t, [3]int{}, sim.pads[2], "the freed cell is lit")
}

// The step status reads the note the cell
// shows, which for a sixteenth group's
// first cell is its first note: the middle
// note shares that cell, and the status
// reports the note a player placed there.
func TestSixteenthGroupReadsOnTheStepStatus(t *testing.T) {
	bank, kit, recorder := screenBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	require.True(t, pattern.SetTripletGroup(0, voice, tripletSixteenth), "the group was refused")
	require.NoError(t, bank.SelectTrackRow(1))

	require.NoError(t, bank.setStepCursor(0))
	status := recorder.row(readoutRow)
	require.Contains(t, status, "S01")
	require.Contains(t, status, "C4@100")
	require.Contains(t, status, "x3")

	require.NoError(t, bank.setStepCursor(1))
	status = recorder.row(readoutRow)
	require.Contains(t, status, "S02")
	require.Contains(t, status, "x3")
}

// The sixteenth gesture lights the pair of
// cells it owns: the three notes sit on
// two pads, which is what tells the
// packing from a note placed by hand.
// Taking the group away lights the three
// cells the notes came from.
func TestTraceSixteenthGroupPacksThreeCells(t *testing.T) {
	snapshots := traceUI(t, []uiStep{
		press("select row 1", NoteMute1),
		press("note one", 54),
		press("note two", 55),
		press("note three", 56),
		press("alt", NoteAlt),
		press("shift", NoteShift),
		press("gesture on the first cell", 54),
		press("shift off", NoteShift),
		press("alt off", NoteAlt),
		press("alt again", NoteAlt),
		press("shift again", NoteShift),
		press("gesture on the first cell again", 54),
		press("shift off again", NoteShift),
		press("alt off again", NoteAlt),
	})
	require.Equal(t, 2, snapshots[6].litRow[0],
		"the group's half beat did not light two cells")
	require.Equal(t, 3, snapshots[11].litRow[0],
		"the cleared half beat did not light three cells")
}

// Clearing a sixteenth group with a full
// tail leaves nowhere for the third
// survivor to walk to, so it shares the
// last cell with the note already there:
// the repair would rather two notes
// share a cell than invent a cell past
// the end of the pattern.
func TestSixteenthGroupClearedWithAFullTailSharesTheLastCell(t *testing.T) {
	voice := trackWindowKit(8, -1).voices[0]
	pattern := &Pattern{}
	for step := 0; step < maxPatternSteps; step++ {
		pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(step), Velocity: 100})
	}
	require.True(t, pattern.SetTripletGroup(0, voice, tripletSixteenth), "the group was refused")
	// The packing freed the third cell; filling it again is the
	// state the clearing below has to repair.
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(2), Velocity: 100})
	events := pattern.EventsForVoice(voice)
	require.Len(t, events, 17, "the pattern did not fill back up")

	set, err := pattern.ToggleTripletGroup(0, voice, tripletSixteenth)
	require.NoError(t, err, "clearing the group was refused")
	require.False(t, set, "the group did not clear")

	events = pattern.EventsForVoice(voice)
	require.Len(t, events, 17, "clearing changed how many notes the pattern holds")
	atLastCell := 0
	for _, event := range events {
		assert.InDeltaf(t, float64(stepBeat(eventStep(event))), float64(event.Beat), 1e-6,
			"a note is off the grid after the repair")
		if eventStep(event) == maxPatternSteps-1 {
			atLastCell++
		}
	}
	require.Equal(t, 2, atLastCell, "the third survivor did not share the last cell")
}
