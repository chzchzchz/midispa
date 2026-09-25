# fireloop

`fireloop` is a hardware sequencer for the Akai Fire. It receives control events from the Fire, edits patterns and songs on the pad grid, and sends note events to the MIDI devices listed in a kit.

## Run

Build the command from the repository root:

```sh
go build -o /tmp/fireloop ./cmd/fireloop
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -port 'FL STUDIO FIRE Jack 1'
```

`-kit` accepts a JSON file containing a top-level device array or a directory of JSON files. When a directory is supplied, each `.json` file is loaded and devices are ordered by their `Name` field. This makes it possible to keep output-port and voice mappings in separate files.

`-port` selects the Fire MIDI port. Each device in the kit supplies its own destination port in `MidiPort`.

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
- Mute 1 through 4: select a track row and its voice.
- Encoder: change the voice assigned to the selected row.
- Grid left/right: move the current step cursor.
- Overview: enter or leave length-edit mode.
- Encoder in length mode: change the pattern from 1 to 16 sixteenth-note steps; shortening removes later events and clears affected ties.
- Mode on a selected chromatic voice: enter or leave note-edit mode. The pad grid then selects pitches starting at MIDI 21 (A0), with four rows of sixteen semitones.
- Pad in note-edit mode: assign and audition the selected pitch. `Alt` plus a pad clears the event at the current step.
- Two held pads in the selected chromatic row: tie two existing adjacent events when exactly two pads are held. Cross-row and three-or-more-pad gestures are ignored.
- `Shift` plus a pad, then release `Shift`: enter a tempo.
- `Alt` plus a mute button: clear that track row.
- `Alt` plus stop: clear the current pattern.
- Record: copy the current pattern, including pitches and ties.
- Play while copied: paste the pattern.
- Stop: stop playback and release active chromatic notes.
- Pattern/song: switch modes.

### Song mode

- Left 48 pads: toggle measures in the visible measure window.
- Rightmost 16 pads: select a pattern from the visible pattern window.
- Pattern up/down: scroll the pattern window by 16 slots.
- `Shift` plus pattern up/down: move the selected pattern by one slot.
- Grid left/right: scroll measures by 16, one 4x4 bank.
- `Shift` plus grid left/right: scroll measures by 4.
- `Shift` plus a measure pad: jump playback to that measure.
- OLED row 3 shows the visible pattern and measure ranges.

Patterns and songs are held in memory for the lifetime of the process. Each song slot uses its pattern's 1–16 step duration, and percussive and chromatic patterns can be mixed in one song. Ties never cross a pattern or song-slot boundary.

## Development

Run the focused checks from the repository root:

```sh
go test ./cmd/fireloop
go test -race ./cmd/fireloop
go vet ./cmd/fireloop
```
