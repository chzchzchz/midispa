# Interactive MIDI Patch Mutation

`cmd/mutation` evolves CC-based MIDI patches by generating a population, auditioning every candidate on an ALSA output, and asking for a rank from 0 to 9.

A `Patch` is one complete MIDI patch for a selected model. `Mutation` owns the genetic population, the current champion, generation history, and interactive audition loop.

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

## Judging

Each candidate is announced with its generation and position in the round:

```text
generation 0000, candidate 2/4
  OscMix: 40 -> 68 (delta +28)
  Cutoff: 72 -> 81 (delta +9)
  rank 0-9, or r to replay:
```

Input is whitespace-delimited, so press Enter after:

- `r` to replay the current candidate.
- `0` through `9` to rank it, where 0 is worst and 9 is best.

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
| `--model` | yes | | Model name from `cc/model.go`. |
| `--port` | yes | | ALSA MIDI output destination. |
| `--output` | yes | | Latest patch path and base for numbered generation files. |
| `--seed` | no | | SMF file used to initialize patch values. |
| `--playback` | no | | SMF file played after each candidate patch is applied. |
| `--gene-semantics` | no | | JSON file containing exclusion and fixed-value rules. |
| `--mutation-rate` | no | `1` | Probability that a candidate is mutated, from 0 to 1. |
| `--mutation-sigma` | no | `16` | Standard deviation for bounded Gaussian descendant mutations. |
| `--mutated-genes` | no | `0` | Exact number of mutable genes changed; 0 chooses 1 through 3. |
| `--crossover-rate` | no | `0.7` | Probability that a child combines genes from two selected patches. |
| `--round-size` | no | `4` | Number of candidates in each generation and the population; must be at least 3. |
| `--parent-decay` | no | `0.5` | Rank points by which the champion decays each generation; set 0 to disable. |
| `--midi-channel` | no | `1` | Output channel for generated CCs, probe notes, and patch files, from 1 to 16. |
| `--rng-seed` | no | time-based | Optional random seed for reproducible mutation runs. |
| `-json` | no | `false` | Write a JSON dump of the model struct alongside each SMF output. |

Without `--playback`, the program probes the patch with middle C at 10 ms, 100 ms, and 1000 ms durations, with 500 ms between probes.

## Gene Semantics

`--gene-semantics` accepts a JSON array with one gene rule per object. Gene names are the field names declared by the selected model in `cc/model.go`.

```json
[
  {"Glide": {"policy": "exclude"}},
  {"ExpressionPedal": {"policy": "fixed", "value": 40}},
  {"SustainPedal": {"policy": "fixed"}}
]
```

### `exclude`

An excluded gene is omitted from the stored patch and is never:

- randomized or mutated,
- included in crossover,
- sent to the controller, or
- written to generated SMF output.

### `fixed`

A fixed gene remains in the patch and is always emitted to the controller and SMF output, but mutation and crossover cannot change it.

- With `"value"`, that explicit value is used and takes precedence over a seed value.
- Without `"value"`, the matching value from `--seed` is used.
- A fixed rule without a value or matching seed value is an error.

Unknown gene names, duplicate rules, unsupported policies, excluded rules with values, and fixed values outside 0 through 127 are rejected.

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

## Playback Safety

Before and after every audition, the command:

- releases sustain (CC 64),
- sends All Notes Off (CC 123),
- cleans both the configured patch channel and channels used by the playback file,
- repeats cleanup after MIDI write failures, and
- performs best-effort cleanup when interrupted.

## Tests

```sh
go test ./cmd/mutation
go test -race ./cmd/mutation
```
