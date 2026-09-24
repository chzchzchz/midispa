# fireloop

`fireloop` is a hardware sequencer for the Akai Fire. It receives control events from the Fire, edits patterns and songs on the pad grid, and sends note events to the MIDI devices listed in a kit.

## Run

Build the command from the repository root:

```sh
go build -o /tmp/fireloop ./cmd/fireloop
/tmp/fireloop -kit cmd/fireloop/kits/gm_drums.json -port 'FL STUDIO FIRE Jack 1'
```

`-kit` accepts either a JSON file or a directory. When a directory is supplied, each `.json` file is loaded and devices are ordered by their `Name` field. This makes it possible to keep output-port and voice mappings in separate files.

`-port` selects the Fire MIDI port. Each device in the kit supplies its own destination port in `MidiPort`.

## Kit format

A kit file contains one or more JSON device objects. A device has a MIDI destination, a default channel, and one or more voices:

```json
{
  "Name": "GM drums",
  "MidiPort": "USB Midi 4i4o MIDI 4",
  "Channel": 10,
  "Voices": [
    { "Name": "Kick", "Note": 36 },
    { "Name": "Snare", "Note": 38 }
  ]
}
```

A voice may override the device channel with its own `Channel`. MIDI notes are in the range 0 through 127.

## Controls

### Pattern mode

- Pad grid: toggle steps in the selected pattern.
- Mute 1 through 4: select a track row.
- Encoder: change the voice assigned to the selected row.
- `Shift` plus a pad, then release `Shift`: enter a tempo.
- `Alt` plus a mute button: clear that track row.
- `Alt` plus stop: clear the current pattern.
- Record: copy the current pattern.
- Play while copied: paste the pattern.
- Stop: stop playback.
- Pattern/song: switch modes.

### Song mode

- Left 48 pads: toggle measures in the current song.
- Rightmost 16 pads: select a pattern.
- `Shift` plus a measure pad: jump playback to that measure.

Patterns and songs are held in memory for the lifetime of the process.

## Development

Run the focused checks from the repository root:

```sh
go test ./cmd/fireloop
go test -race ./cmd/fireloop
go vet ./cmd/fireloop
```
