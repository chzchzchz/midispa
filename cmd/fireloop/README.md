# fireloop

`fireloop` is a hardware sequencer for the Akai Fire. It receives control events from the Fire, edits patterns and songs on the pad grid, and sends note events to the MIDI devices listed in a kit.

## Run

Build the command from the repository root:

```sh
go build -o /tmp/fireloop ./cmd/fireloop
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -port 'FL STUDIO FIRE Jack 1'
```

`-kit` accepts a JSON file containing a top-level device array or a directory of JSON files. When a directory is supplied, each `.json` file is loaded and devices are ordered by their `Name` field. This makes it possible to keep output-port and voice mappings in separate files.

`-kit` can be repeated, and the devices of every kit are merged in the order the flags are given:

```sh
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -kit my_leads.json -port 'FL STUDIO FIRE Jack 1'
```

A file and a directory can be mixed in one command line. A path that fails to load stops startup, because a half-loaded kit would silently drop voices. Track numbers follow the merged order, so reordering the flags renumbers the kit's voices.

`-port` selects the Fire MIDI port. Each device in the kit supplies its own destination port in `MidiPort`.

`-shared-midi-destination` sends instrument MIDI through Fireloop's main ALSA port to every connected destination, matching the legacy broadcast behavior. The default remains per-device routing; shared mode intentionally sends every note to all connected outputs.

`-log-level` sets log verbosity: `debug`, `info` (the default), `warn` or `error`. At `debug` the sequencer logs every MIDI message it writes, every change to the Fire's display, and the steps its editors act on, which is what to capture when a note or a light is not behaving:

```sh
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -port 'FL STUDIO FIRE Jack 1' -log-level debug 2>/tmp/fireloop.log
```

`-log-format` chooses `text` (the default) or `json`. Both carry the same fields; text is easier to read by eye and JSON is easier to feed to a tool. An unrecognised value falls back to text with a warning rather than leaving the sequencer silent:

```sh
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -log-level debug -log-format json 2>/tmp/fireloop.log
```

## Kit format

A kit file is a top-level array of device objects, so each device can use a different MIDI port. A single device object is still accepted for compatibility. Omitting a voice's `Note` selects a chromatic instrument; an explicit `Note`, including `0`, selects a percussive instrument. A voice `Channel` overrides its device's channel:

```json
[
  {
    "Name": "GM drums",
    "MidiPort": "USB Midi 4i4o MIDI 4",
    "Channel": 10,
    "Voices": [
      { "Name": "Kick", "Note": 36 },
      { "Name": "Snare", "Note": 38 }
    ]
  },
  {
    "Name": "Lead",
    "MidiPort": "MIDI4x4 MIDI 4",
    "Channel": 1,
    "Voices": [
      { "Name": "Lead", "Channel": 2 }
    ]
  }
]
```

Percussion MIDI notes are in the range 0 through 127. Chromatic events store their own pitch in the pattern editor.

## Controls

### Pattern mode

- Pad grid: toggle steps for a percussive voice. Empty chromatic steps are ignored here.
- In pattern/step mode, a pad on the selected chromatic track moves the editing step cursor. Two held pads still create a tie.
- Pattern up/down: change the selected pattern from 1 to 999 and stop playback.
- Mute 1 through 4: select a track row and its voice. The selected row lights green, and pressing it again deselects. A running pattern keeps playing, so a row can be followed while the pattern loops.
- `Alt`, then pattern up/down: scroll the track window. `Alt` lights up while it is engaged and stays engaged until it is pressed again, so press it once more before using a mute button or stop.
- `Shift` plus `Alt`: blackout. See [Blackout](#blackout).
- The header shows the window as `<FIRST>/<TOTAL>`, for example `Pattern 001  3/8` means the pad rows show tracks 3, 4, 5, and 6 of 8.
- Encoder: change the voice assigned to the selected row. In note-edit mode it moves the palette by an octave instead, because changing the voice there would take the editor to a different instrument.
- Grid left/right: move the current step cursor.
- Overview: enter or leave length-edit mode.
- Encoder in length mode: change the pattern from 1 to 16 sixteenth-note steps; shortening removes later events and clears affected ties.
- Mode on a selected chromatic voice: enter or leave note-edit mode. The pad grid becomes an editor: the first twelve columns are a pitch palette and the rightmost four are a strip of step indicators. Toggling Mode does not stop playback. Note editing follows the selection, so a mute button moves the palette to another voice rather than closing it; press Mode to leave.
- Volume knob with a selected chromatic voice: each detent moves the knob's own value by one, clamped to 0..127, and writes that value to the selected step. The knob is not re-read from the step you select, so its value carries over to the next step.
- Encoder in note-edit mode: transpose the palette by an octave. Left turns it down, right turns it up, and every palette pad's colour shifts with it, so the octave on screen is visible without reading anything. Only the pitches a pad press can choose change; notes already written into a pattern keep their pitch and their colour. The palette stops at the ends of the MIDI range, two octaves below A1 and three above.
- Pad in note-edit mode: assign and audition the selected pitch, at the velocity the pad was pressed with. `Alt` plus a pad clears the event at the current step and leaves `Alt` engaged.
- Two held pads in the selected chromatic row: tie two existing adjacent events when exactly two pads are held. A tie holds the note past its step. Cross-row and three-or-more-pad gestures are ignored.
- Two held step cells while choosing notes: the same tie, on the step strip where the steps live while the palette owns the grid. Both steps need a note, and nothing may sit between them; if the tie cannot be made the edit stays where it was rather than jumping to a step whose note has nothing to hold.
- `Shift` plus a pad, then release `Shift`: enter a tempo. The entry shows on the bottom row only, so the rest of the display keeps showing the pattern, and a value from 21 to 299 is applied on release; anything else is discarded.
- Tap: tap out a tempo. It takes at least two taps, ignores taps more than three seconds apart, averages the last five, and shows `Tempo: NNN` on the bottom row.
- `Alt` plus a mute button: clear that track row. `Alt` stays engaged, so several rows can be cleared in a row. Clearing changes what is being played, so it stops playback, unlike selecting a row.
- `Alt` plus stop: clear the current pattern. `Alt` stays engaged.
- Record: copy the current pattern, including pitches and ties. Record lights green while a copy is armed, and pressing Record again discards it. There is one copy slot; a new copy replaces the old one.
- Play while copied: paste the pattern.
- Stop: stop playback and release active chromatic notes.
- Pattern/song: switch between pattern editing and [song mode](#song-mode). Switching releases `Alt` and `Shift`, so neither carries into the other mode.
- The four lights at the top left, labelled Channel, Mixer, User 1 and User 2, are indicators with no button behind them. Fireloop does not report state on them yet, and clears them whenever it clears the indicators, so they stay dark.

#### Choosing voices

A track is a voice from the kit, and any voice can go on any track.

1. The kit is the list of voices loaded from `-kit`, in the order the kit paths are given. Percussive voices have a `Note`; chromatic voices omit it.
2. Press Mute 1 through 4 to select a track row. Its name appears on the display as `NAME [DRM]` or `NAME [CHR]`, inverted while selected, so the two kinds are easy to tell apart at a glance.
3. Turn the Encoder to move that track through the kit's voices. It wraps around at either end, so every voice is reachable from every row. In note-edit mode the Encoder turns the palette instead, so press Mode again before changing the voice.
4. Nothing stops two rows from holding the same voice. Clearing one of them clears the notes on both, and editing one edits the other, because the notes belong to the voice rather than the row.
5. The track window decides which tracks you can reach this way. Scroll it with `Alt` plus pattern up/down when you want one voice per track rather than reaching voices a few at a time with the Encoder.

#### Display

- Row 0: `Pattern 003  2/6`, the pattern number and the track window.
- Row 1: a separator.
- Rows 2 to 5: the voice on each visible track row, inverted for the selected row.
- Row 6 is the readout row and carries one of: `Length NN steps` in length mode, `S<STEP> <NOTE>@<VELOCITY>` for a selected chromatic track, `S<STEP> --` for a selected chromatic track on a step with no note, or `Tempo: NNN` while a tempo is being entered. It is blank otherwise.

The pad grid shows the notes of the four visible tracks and the editing step is lit slightly brighter. A chromatic step is coloured by pitch, using the same colour as that pitch's palette pad, and a percussive step is dark green.

During playback the playing column is inverted while the others are redrawn dimmer. Inversion is per channel, so a chromatic step keeps its pitch colour while being played and returns to it exactly when the playhead moves on. A tied step is marked by pushing its colour away from the playhead, lifted normally and lowered when inverted.

While choosing notes the palette owns the grid, so the playhead lights the matching cell of the step strip instead and the palette is never overwritten.

#### Track window

A pattern starts with four tracks, one per pad row, and the four pad rows show four of them at a time.

1. The header reads `1/4` with the first track on the top pad row.
2. Press `Alt`; the button lights up and pattern up/down move the window instead of changing pattern. The header follows, so `3/6` shows tracks 3 through 6 on the pads.
3. Scrolling down past the last track adds another track, so a kit of any size only takes on the tracks you scroll to. The count never drops back for the rest of the session, and a kit of 100 voices still opens on four tracks.
4. Growth stops at one track per kit voice, and the window stops when the last track reaches the bottom row, so the pads never end on rows without a track behind them.
5. Everything that follows the visible tracks: pad presses, Mute 1 through 4, the Encoder, and `Alt` plus a mute button. The selected mute row keeps its selection and now points at a different track.
6. Scrolling leaves note-edit mode and releases held pads, so a gesture started in the previous window cannot carry over.
7. A kit with fewer than four voices keeps all four rows usable by wrapping its voices.
8. Tracks belong to the pattern bank, not to a single pattern, so the track count and the track-to-voice mapping are shared by every pattern. Changing a track's voice with the Encoder changes it everywhere.

#### Chromatic editing flow

1. Select a chromatic voice with Mute 1 through 4 and the Encoder if needed.
2. In pattern/step mode, press a pad on the selected chromatic track to move the editing step cursor. This does not create or remove a note; two held pads still create a tie.
3. Move the cursor with Grid left/right or by pressing another step pad. Steps beyond the current pattern length are ignored.
4. Press Mode to enter note-edit mode. The pad grid splits in two: the left twelve columns are the pitch palette and the rightmost four show the sixteen steps.
5. The palette runs four rows from A1, each starting on A and spanning the twelve semitones to the G# above it, so the rows cover A1–G#2, A2–G#3, A3–G#4 and A4–G#5. Press a palette pad to create or update the note at the current step and audition it.
6. Turn the Encoder to move the whole palette by an octave: left takes it down, right takes it up, and the pad colours shift with it so the octave is visible. It changes nothing already written into the pattern; only the pitches a pad press can choose. The position holds for the session, so it is still there when you come back to this voice.
7. The step strip reads four steps per row, in the same order as the step grid. Each cell is dark when the step holds no note and otherwise shows that note's colour, with the step being edited brightened. Press a cell to move the edit there. While a pattern plays, the cell the playhead is on goes white.
8. Hold one cell and press another to tie those two steps, which is how a note is held past its step. Both steps need a note and no other note may sit between them; otherwise nothing is tied and the edit stays put.
9. The palette's first pad means "no note", so pressing it clears the current step. It is A1 until the palette is turned, and moves with the palette afterwards. `Alt` plus any palette pad does the same.
10. How hard a palette pad is pressed sets the step's velocity, whether the note is new or its pitch is being changed, so what is heard and what is shown agree. The Volume encoder then adjusts it by one detent at a time, clamped to 0..127. The encoder carries its own value rather than re-reading the selected step, so a step picked after you set it takes the encoder's value on the next detent.
11. The status line shows `S<STEP> <NOTE>@<VELOCITY>`, or `S<STEP> --` with no velocity when the step holds no note. A tied step shows the step it is tied to as well, for example `S01 C4@100->04`.
12. Press Mode again to return to step mode.

#### Chromatic note length

A chromatic note lasts one sixteenth-note step. It stops as soon as the playhead moves to the next step, so a lone note is a short hit rather than a sound that rings for the rest of the pattern.

- To hold a note longer, tie it. A tie keeps the note sounding past its step until the tied event arrives, and a chain of ties holds it further still.
- A tie between two notes of the same pitch reuses the sounding note, so there is no retrigger. A tie to a different pitch is legato: the new pitch starts before the old one stops.
- Untied notes retrigger, so a note followed by another on the same voice stops exactly where the next one begins.
- Stopping playback, switching pattern, or reaching the end of a pattern releases anything still sounding.

#### Blackout

`Shift` plus `Alt` turns every light and the screen off, which is useful on a dark stage. It is a display state rather than a control state: nothing is cancelled or forgotten.

- Nothing is lit while the display is dark, and display output is suppressed entirely, so a running playhead cannot light the pads behind the blackout.
- `Alt` is not released by the blackout. Its light goes dark with everything else and comes back to the state it was left in.
- The next press wakes the display and then does what it says, so the screen comes back showing the result of that press.
- While `Shift` is held, `Alt` is always the blackout, so release `Shift` before using `Alt` to release it.

The unit also blanks its own display after a stretch with no display traffic from Fireloop. That is not a blackout: it can happen minutes after startup with nothing pressed, and any control that redraws brings the screen back. Pressing Shift plus Alt is the deliberate way to blank the display.

### Song mode

Song mode arranges the patterns you have written into a song: each measure holds one pattern, and the song plays those patterns back to back using their own lengths.

#### Entering song mode

- Press the Pattern/Song button. Its LED turns green in song mode and dark in pattern mode, so the current mode is always visible.
- The button works both ways: press it in song mode to go back to pattern editing.
- Switching modes stops playback and leaves any pattern edit mode, so step, note, and length editing do not follow you across.
- The display switches from the pattern track window to the arrangement view described below.

#### Arrangement view

| Pads | Contents |
| --- | --- |
| Left 48 pads, three 4x4 banks | 48 measures of the song, filled left to right and top to bottom inside each bank |
| Rightmost 16 pads, one 4x4 bank | 16 pattern slots from the visible pattern window, filled down each column |

Each pattern number has a colour from the display's colour table. The selected pattern is lit at full brightness, other patterns are dimmed, and empty measures are dark. A measure that is playing is lit brighter while the song runs.

#### Scrolling a long song

A song holds up to 1000 measures, so the 48 measure pads only ever show part of it.

- Grid left/right: move the window by a full 4x4 bank, 16 measures at a time.
- `Shift` plus grid left/right: move by four measures, one row of pads.
- Row 3 always reports the measures in view, for example `M 001-048` at the start of the song and `M 953-1000` at the end, so it is clear which measures the pads hold.
- Scrolling only moves the window. It never places or clears a measure, so browsing a long arrangement cannot change it.
- The window stops at the end of the song, which keeps the final measures fully reachable instead of cutting them off.
- Pattern slots are paged the same way, 16 at a time out of 999.

#### Display

- Row 0: the song slot, for example `Song 001`.
- Row 1: the selected pattern and its length in steps, for example `Pattern 003 L16`.
- Row 2: the current tempo.
- Row 3: the visible windows, for example `P 001-016 M 001-048`.
- Rows 4 and 5: during playback the playing measure and its pattern, for example `Measure 005` and `Pat-bar 003`.

#### Controls

- Measure pad: place the selected pattern in that measure. A measure that already holds that pattern is cleared instead, so empty a measure by selecting its pattern first and pressing the pad again. A measure holding any other pattern is replaced.
- Pattern pad: select the pattern in that slot. The selection is shared with pattern mode, so the pattern you select here is the one you edit there.
- Pattern up/down: scroll the pattern window by 16 slots.
- `Shift` plus pattern up/down: move the selected pattern by one slot.
- Grid left/right: scroll measures by 16, one 4x4 bank.
- `Shift` plus grid left/right: scroll measures by 4.
- `Shift` plus a measure pad: jump playback to that measure. The jump only applies while the song is playing, takes effect at the next pattern boundary, and the display reports the move.
- Play: play the song from the top and loop it. The playing measure lights up and rows 4 and 5 follow it.
- Stop: stop playback and release active chromatic notes.
- `Alt` is a pattern-mode modifier and does nothing here, so there is no blackout in song mode.

Only song slot 1 is reachable from the controls today; the display names the slot and there is no button bound to changing it.

#### Arranging workflow

1. Write and check your patterns in pattern mode. The Play button loops the selected pattern on its own.
2. Press Pattern/Song to enter song mode. The pattern window opens on the pattern that was selected, and its pattern pad is already lit.
3. Choose the pattern for a measure with the pattern pads, then place it with measure pads. To repeat a pattern, leave the selection alone and fill the following measures.
4. Use `Shift` plus grid left/right to move in steps of four measures when filling in a dense arrangement.
5. Press Play to hear the arrangement, and `Shift` plus a measure pad to move around while it runs.
6. Press Pattern/Song to go back and edit any pattern in the song; every measure holding that pattern changes with it, including its length.

Measures reference the pattern itself rather than a copy, so edits are shared. Percussive and chromatic patterns can be mixed freely in one song. Patterns and songs are held in memory for the lifetime of the process, so nothing is written to disk.

## Development

Run the focused checks from the repository root:

```sh
go test ./cmd/fireloop
go test -race ./cmd/fireloop
go vet ./cmd/fireloop
```

The tracer replays a scripted button sequence through the real handler and reports what the
Fire would be showing after each step, so a UI problem can be reproduced without hardware:

```sh
go test -run Trace -v ./cmd/fireloop
```

Each line is one input and what the display held afterwards, for example:

```
alt: blackout              pads  0 lit (rows [0 0 0 0])  LEDs: none
stop                       pads  4 lit (rows [1 1 1 1])  LEDs: Mute1=2 Shift=1
```

Display assertions go through a text-only stand-in for the Fire rather than decoding
pixels, so a test reads the string a row shows. The note lifecycle tests read back the MIDI
that would reach a device, which is how the note-on and note-off order stays pinned.
