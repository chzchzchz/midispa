# midifilter

midi filtering with bpf and sysex configurable routing.

## BPF filters

Build BPF support using the bpf tag (needs ubpf installed):

```go
go build -tags bpf
```

midi packets will be modified using the bpf filter specified by `--bpf <path>`. See examples/ for filters.

## Routing

NB: routing turns off broadcasting

SysEx message format: `F0 00 30 33 00 CH CC CC PP F7`.

Example message to route drums from 129:0 to 16:0:
```sh
aconnect 129:0 16:0 && 
  midisend -s "F0 00 30 33 00 09 00 10 00 F7" -p midifilter
```

