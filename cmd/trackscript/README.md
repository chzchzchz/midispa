# trackscript

## Building

Build basic trackscript:
```sh
go install https://github.com/pointlander/peg
go generate
go build
```

Use `-tags bpf` to enable ubpf filtering:
```sh
go build -tags bpf
```

## Running

Generate a midi file from `whatever.score` to `out.mid`:
```sh
trackscript whatever.score
```

## Examples

### Basic patterns

Play a, then a and b together:
```
bpm 120
pat aaa a.abc
pat bbb b.mid
aaa
aaa | bbb
```

Anonymous patterns based on filename, where `a.abc` and `b.mid` are inferred:
```
a
a | b
```

### Phrases

Create phrases from patterns:
```
phrase ac { a a c c }
phrase ba { b b a a }
ac ac ac ac
ba ba ba ba
```

Replicate a pattern or phrase:
```
phrase ac { a*2 c*2}
ac*4
(b;b;a;a)*4
```

### Filters

Filters are BPF programs that mutate midi events. Filters are applied to patterns and phrases by using the `|` operator. For more examples, see `midifilter`.

To build with ubpf filters enabled, build `trackscript` with the following command:
```sh
go build -tags bpf
```

Create a pattern `aaa` filtered by `f`:
```
filter f f.c
pat aaa a.mid | f
```

Define a filter `f2` with compile-time `#define` arguments (`-D` is automatically prefixed):
```
filter f2 f.c { ABC=2 DEF=3 }
```

Anonymous filtering of a phrase using `f.c` and `g.c` with compile-time arguments:
```
phrase some_phrase { ... } | f { ABC=2 } | g { DEF=3 }
```