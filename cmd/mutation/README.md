# Interactive MIDI Patch Mutation

`cmd/mutation` evolves MIDI patches by generating a population, auditioning every candidate on an ALSA output, and asking for a rank from 0 to 9.

A `Patch` is one complete MIDI patch in a selected format. `Mutation` owns the genetic population, the current champion, generation history, and interactive audition loop.

Two formats are supported:

- `cc` evolves CC controller values for a model from `cc/model.go`, and reads and writes SMF files. This is the default.
- `dx7-single` evolves one Yamaha DX7 voice, and reads and writes raw SysEx files (`.syx`).
- `pro800` evolves one Behringer Pro 800 patch, and reads and writes raw SysEx files (`.syx`).

Each format decides for itself which flags apply to it and what its output files
are called, so the rules below follow the selected format rather than its name.

The genetic engine only ever sees a list of bounded gene values, so the two formats share every breeding, selection, and reporting rule.

## Build

```sh
mkdir -p out
go build -o out/mutation ./cmd/mutation
```

The command uses the repository's ALSA output support and expects an ALSA sequencer destination name.

## Run

```sh
out/mutation \
  --model "Volca Bass" \
  --port "MIDI Out" \
  --seed seed.mid \
  --playback phrase.mid \
  --gene-semantics genes.json \
  --output best.mid \
  --mutation-rate 0.9 \
  --mutation-sigma 16 \
  --mutated-genes 2 \
  --crossover-rate 0.7 \
  --round-size 6 \
  --midi-channel 1
```

Model names are the canonical names accepted by `cc.NewModelParams`, such as `Craft Synth 2`, `Volca Bass`, `Uno Synth`, or `Sound Controller`.

## DX7 Voices

`--format dx7-single` evolves the voice stored in the DX7 edit buffer. It needs a raw `.syx` dump holding exactly one 163-byte one-voice message as its seed, and it writes raw SysEx output:

```sh
out/mutation \
  --format dx7-single \
  --port "MIDI Out" \
  --seed voice.syx \
  --output best.syx \
  --playback phrase.mid \
  --midi-channel 1
```

Gene names and value bounds are discovered by reflecting over the DX7 program
struct and its `range` and `oneof` tags, so every constrained field of a voice
becomes a gene and nothing else does. For example, `Osc[0].EgRate[1]` and
`Algorithm`.

Fields that are not sound parameters declare that themselves with a
`mutate:"skip"` tag and are left out of the catalog entirely, so a search can
never touch them and a changed-gene report never has to explain them. On the
DX7 that is the channel and the voice name. Because a skipped field is not a
gene, a rule cannot name it either: a voice name comes from the seed you chose
and is not editable from here.

The seed must decode cleanly. A wrong length, a non-Yamaha manufacturer byte, a
bad checksum, trailing bytes, or a second message are all rejected rather than
partially loaded.

Each candidate is sent as one complete message, and the command waits
`--sysex-settle` before playing the probe or the playback so the instrument has
loaded the voice. The output file is written only after the whole message
encodes, so a rejected value never leaves a truncated dump.

## Pro 800 Patches

`--format pro800` evolves one patch of the Pro 800 and needs a raw `.syx` dump
of one patch as its seed:

```sh
out/mutation \
  --format pro800 \
  --port "MIDI Out" \
  --seed patch.syx \
  --output best.syx \
  --midi-channel 1
```

Gene names follow the decoded record, so they are nested and carry the wrapper
the dump puts around the patch: `Patch.OscA.Volume`, `Patch.Filter.Cutoff`,
`Patch.Tuning[0]`, `Patch.Lfo.Shape`. The DX7's own fields are flat because
the dump embeds the voice rather than nesting it.

Each candidate is sent as a patch dump response, the same message the
instrument sends when it is asked for a patch.

**The fields are wide.** Most are 14-bit, where a controller value is 7-bit,
so `--mutation-sigma 16` is a very fine step. The tuning table is a signed
32-bit value per note in the instrument's own units, and the notes publish a
table from -40 to +100 cents without giving the curve, so a drawn value is
not a temperament. Both are evolvable, but a run at the default settings
searches a much larger space than a DX7 run.

**Some fields depend on the dump layout.** Four settings exist only in the
newer layout and are left out of the catalog for both, so what a run can
evolve does not depend on the layout its seed carried. The fine tuning and
the sync switch exist only on the second oscillator. Where a mutation still
lands somewhere the seed's layout cannot carry, the command prints `skipped:`
and carries on with the rest of the round.

## Judging

Each candidate is announced with its generation and position in the round:

```text
generation 0000, candidate 2/4
  OscMix: 40 -> 68 (delta +28)
  Cutoff: 72 -> 81 (delta +9)
  rank 0-9, r to replay, [ ] round size, - + genes, < > rate, pattern:
```

Input is whitespace-delimited, so press Enter after:

- `r` to replay the current candidate.
- `0` through `9` to rank it, where 0 is worst and 9 is best.
- `[` and `]` to shrink or grow the next generation, from 3 to 32 candidates.
- `-` and `+` to mutate fewer or more genes, from the automatic 1-3 range up to every mutable gene.
- `<` and `>` to lower or raise the mutation rate in steps of 0.1.
- a Go regexp of two or more characters, such as `Pitch` or `^Osc`, to mutate and crossover only matching genes in the next generation.

A value entered while the melody plays stops it, and the
value counts as the answer for that candidate, so a judge
who has heard enough does not wait out the probe or the
playback pattern.

The tuning keys echo their new value and apply from the next
generation; the candidates already being judged keep their round.
Every generation opens with a line naming its active settings.

A gene pattern matches gene names without anchoring, so `Pitch`
finds every gene containing it; `.*Pitch.*` is the explicit
form. A pattern that is not a valid regexp, or that matches no
mutable gene, is rejected with a message. The pattern applies to
the generation bred after it is entered, restricts both mutation
and crossover, and clears automatically, so every change that
generation reports is in a matching gene.

The highest-ranked patch remains the champion. A later round cannot replace it unless it receives a strictly higher rank.

## Genetic Search

- The initial population starts from the seeded or randomized parent; additional candidates apply the configured mutation policy, which uses uniform random values by default for one to three mutable genes.
- The current champion and one other elite survive each generation.
- The champion's effective rank decays by 0.5 per generation, allowing an aged champion to be replaced; `--parent-decay` controls the rate.
- Rank-weighted roulette selection chooses parents for new candidates; the weight is `rank + 1`, so higher ranks dominate while rank 0 remains possible.
- Uniform crossover combines genes from the selected parents.
- Descendant mutations use bounded Gaussian deltas around inherited values.
- Ten percent of descendant mutations use uniform values to preserve exploration.
- The best patch seen during the session is written after every completed generation.

`--round-size` controls both the number of candidates judged per generation and the population size. It must be at least 3 so at least one descendant can be produced after preserving the champion and one other elite.

## Flags

| Flag | Required | Default | Description |
| --- | --- | --- | --- |
| `--format` | no | `cc` | Patch format to evolve: `cc`, `dx7-single` or `pro800`. |
| `--model` | for `cc` | | Model name from `cc/model.go`. |
| `--port` | yes | | ALSA MIDI output destination. |
| `--output` | yes | | Latest patch path and base for numbered generation files. The extension must match the format: `.mid` for `cc`, `.syx` for `dx7-single`. |
| `--seed` | for a SysEx format | | SMF file for `cc`, raw SysEx dump for `dx7-single` and `pro800`. |
| `--playback` | no | | SMF file played after each candidate patch is applied. |
| `--gene-semantics` | no | | JSON file containing exclusion and fixed-value rules. |
| `--dump-excludes` | no | | Write every parameter of the model to this JSON file as an exclude rule, then exit without evolving anything. |
| `--mutation-rate` | no | `1` | Probability that a candidate is mutated, from 0 to 1. |
| `--mutation-sigma` | no | `16` | Standard deviation for bounded Gaussian descendant mutations. |
| `--mutated-genes` | no | `0` | Exact number of mutable genes changed; 0 chooses 1 through 3. |
| `--crossover-rate` | no | `0.7` | Probability that a child combines genes from two selected patches. |
| `--round-size` | no | `4` | Number of candidates in each generation and the population; must be at least 3. |
| `--parent-decay` | no | `0.5` | Rank points by which the champion decays each generation; set 0 to disable. |
| `--midi-channel` | no | `1` | Output channel for probe notes, and for the message of a format that carries one, from 1 to 16. |
| `--sysex-settle` | no | `100ms` | Delay after a SysEx patch message before playing a note. |
| `--rng-seed` | no | time-based | Optional random seed for reproducible mutation runs. |
| `-json` | for `cc` only | `false` | Write a JSON dump of the model struct alongside each SMF output. |

Each flag combination is checked against the format that would carry it out, so
the two formats each reject what does not apply to them:

| Flag | `cc` | a SysEx format |
| --- | --- | --- |
| `--model` | required | rejected |
| `--seed` | optional SMF | required raw SysEx dump |
| `--output` | must end in `.mid` | must end in `.syx` |
| `--json` | optional | rejected |

A SysEx format applies `--midi-channel` to the probe notes, and to the message
itself only if the message carries a channel byte. The DX7 edit-buffer dump
does; the Pro 800 patch dump does not, and its channel stays the one the seed
was captured on.

Each format declares its own output extension, so the requirement follows the
format rather than its name. Without `--playback`, the program probes the patch
with middle C at 10 ms, 100 ms, and 1000 ms durations, played at each
of the velocities 127, 99, and 64, with 500 ms between probes.

## Gene Semantics

`--gene-semantics` accepts a JSON array with one gene rule per object. Gene names are the field names declared by the selected model in `cc/model.go`, or the reflected field names of the selected SysEx program.

```json
[
  {"Glide": {"policy": "exclude"}},
  {"ExpressionPedal": {"policy": "fixed", "value": 40}},
  {"SustainPedal": {"policy": "fixed"}}
]
```

For `dx7-single` the same rules address reflected field names:

```json
[
  {"Osc[0].EgRate[0]": {"policy": "exclude"}},
  {"Osc[0].EgRate[1]": {"policy": "fixed", "value": 42}},
  {"Osc[0].OutLevel": {"policy": "fixed", "value": 99}}
]
```

### `exclude`

An excluded gene is omitted from the stored patch and is never:

- randomized or mutated,
- included in crossover,
- sent to the controller, or
- written to the generated output file.

### `fixed`

A fixed gene remains in the patch and is always part of the emitted patch or SMF output, but mutation and crossover cannot change it.

- With `"value"`, that explicit value is used and takes precedence over a seed value.
- Without `"value"`, the matching value from `--seed` is used.
- A fixed rule without a value or matching seed value is an error.

Unknown gene names, duplicate rules, unsupported policies, excluded rules with values, and fixed values outside the gene's own range are rejected. For a CC model every value is a 0-127 MIDI data byte; for a SysEx program each field carries its own bounds, so a fixed value of 50 on a `0..31` field is rejected by name.

### Starting From a Dump

Gene names are not something to guess: a SysEx name is a reflected field path
like `Patch.Lfo.Shape`, and a wrong spelling is rejected rather than ignored.
`--dump-excludes` writes them out instead:

```sh
out/mutation --model "Volca Bass" --dump-excludes volca.json
out/mutation --format dx7-single --dump-excludes dx7.json
```

Every parameter the model exposes gets one rule, all of them `exclude`, in the
order the model declares them. Delete the entries worth evolving and pass the
file to `--gene-semantics`; what is left is the search space.

A dump is the whole run replaced by one question about the model, so it needs
nothing else: no `--port`, no `--output`, and no `--seed`, because a SysEx
catalog comes from the program type rather than from a particular dump. It also
means the file must be edited before use — a file that excludes everything
evolves nothing — and it cannot be combined with `--gene-semantics`, which it
would have to ignore. Any existing file at the given path is overwritten.

## Output

For `--output best.mid`, each completed generation writes:

```text
best.mid.0000
best.mid.0001
...
```

The latest completed champion is also copied to:

```text
best.mid
```

If a later generation does not improve the champion, its numbered file repeats the current best patch.

With `-json`, the command also writes `best.mid.0000.json` and `best.mid.json` for `--output best.mid`. The JSON is the selected model struct marshaled directly, with the current patch gene values applied.

For `--output best.syx`, each completed generation writes the raw one-voice
message to `best.syx.0000`, `best.syx.0001`, and so on, and copies the latest
champion to `best.syx`.

## Playback Safety

Before and after every audition, the command:

- releases sustain (CC 64),
- sends All Notes Off (CC 123),
- cleans both the configured patch channel and channels used by the playback file,
- waits `--sysex-settle` after a SysEx patch message, but never after CC messages,
- repeats cleanup after MIDI write failures, and
- performs best-effort cleanup when interrupted.

## Tests

```sh
go test ./cmd/mutation
go test -race ./cmd/mutation
```
