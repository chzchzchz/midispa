# cccli

`cmd/cccli` edits a CC model by hand. It shows one cell per control change field of the model, sends every edit to an ALSA output port as a control change the moment it is made, and saves the model back to a `.mid` file.

## Build

```sh
mkdir -p out
go build -o out/cccli ./cmd/cccli
```

The command uses the repository's ALSA output support and expects an ALSA sequencer destination name.

## Run

```sh
out/cccli \
  --model "Volca Bass" \
  --port "MIDI Out" \
  --output-midi-channel 1 \
  --filter-midi-channel 1 \
  --input seed.mid \
  --field-rules rules.json \
  --output edited.mid
```

Model names are the canonical names accepted by `cc.NewModelParams`, such as `Craft Synth 2`, `Volca Bass`, `Uno Synth`, or `Sound Controller`.

The editor opens with every field at the value the model declares, zero for a field the model leaves unset, or at the value a seed file set. The header names the model, the channel, the port, and how many control changes have been sent so far.

`--output-midi-channel` is the channel every edit is sent on and saved with. `--filter-midi-channel` limits a seed file to the control changes stamped with that channel; 0, the default, accepts every channel.

## Editing

The cells pack into as many columns as the terminal is wide, column by column, with field 0 at the top left, so tab order follows the reading order. The focused cell is bold and bright, and the mouse wheel scrolls the grid when it does not fit.

| Key | Effect |
| --- | --- |
| `up` / `down` | move the focused field's value by one |
| `pgup` / `pgdn` | move it by ten |
| `tab` / `shift-tab` | move focus by one field, wrapping at the ends |
| `left` / `right` | move focus by one visual row, stopping at the first and last field |
| `ctrl+s` | save the model to `--output` |
| `esc` / `ctrl+c` | quit |

`up` and `pgup` increase the value, `down` and `pgdn` decrease it, and the value clamps at 0 and 127 without wrapping, so a field at its end stops instead of jumping to the other end. Every press, and every repeat a held key sends, writes the value to the model and sends the control change at once, so holding a key sweeps a parameter the way a knob turns. A field at its limit shows no change and sends nothing further.

A cell can also be typed into. Each digit appends to the buffer the cell holds, and every keystroke that parses as a value in range writes the model and sends the control change immediately, so typing `1`, `2`, `7` into an empty cell sends 1, 12, and 127 in turn. A buffer that is empty or outside 0-127 is not a value: the model keeps what it had, the status line says `invalid`, and the cell is restored to the model's value when focus leaves it.

Two fields that share a controller number are separate cells; editing either sends that controller's control change, and a seed sets both.

The footer lists the keys and the last action's result, colored green when a control change went out or a save landed, and red when a send or a save failed or a buffer did not parse. There is no unsaved-changes prompt: `esc` quits and takes the unsaved model with it.

## Field rules

`--field-rules` accepts the same JSON array `cmd/mutation` reads with `--gene-semantics`, so one rule file serves both commands:

```json
[
  {"Select1": {"policy": "exclude"}}
]
```

A field named by an `exclude` rule is not shown, not seeded, and not saved. A `fixed` rule is ignored with a warning on stderr, because an editor has no way to honor a frozen value, so a mutation rule file with mixed policies loads with its excludes intact. A rule naming a field the model does not declare is a misspelling and is rejected, as is a file that would exclude every field.

## Seed and save

`--input` reads a seed file, trying SMF framing first and a raw MIDI stream second, and applies its control changes to the model. The last message for a controller wins, controllers the model does not know are ignored, and the channel a message was stamped with is ignored unless `--filter-midi-channel` is set, in which case control changes on other channels are dropped. A seed that sets no field of the model is rejected, because a model left at zero is indistinguishable from an editor that failed to load; the same check applies after filtering, so a seed whose channels are all filtered out is refused.

`ctrl+s` writes every field in declaration order to `--output` as a Standard MIDI File. Saving with no `--output` is refused with a message on the status line rather than an error.

## Out of scope

Fields tagged `nrpn`, which need three messages each, and note-tagged fields are not rendered. A model that has them says so in the initial status line, for example `3 nrpn fields are not shown`, so the missing parameters are explained rather than silent.

## Tests

```sh
go test ./cmd/cccli
go test -race ./cmd/cccli
```

Every test drives the editor with synthetic key, mouse, and window-size messages and injects a writer, so no test needs a terminal or an ALSA port.
