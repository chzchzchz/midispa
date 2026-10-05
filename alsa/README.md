# Sequencer ports

`OpenSeq` creates a client and a default duplex port. `Seq.SeqAddr` holds the actual ALSA address of that default, not an assumed port zero. `Write`, `NewWriter`, `OpenPort`, `OpenPortName`, and the original read/write subscription helpers use this address. To select another default, assign an address returned by the same sequencer to `Seq.SeqAddr` without changing its client ID.

`CreatePortAddr(name)` returns the address assigned by ALSA. Additional ports leave the default unchanged. `CreatePort(name)` remains an error-only compatibility wrapper; its ports have the same ownership and cleanup rules.

For explicit routing:

- `WritePort(event, local.Port)` uses the given local source port; `event.SeqAddr` remains the destination. `SubsSeqAddr` broadcasts to that source port's subscribers.
- `OpenPortReadAt(local, remote)` subscribes remote output to local input.
- `OpenPortWriteAt(local, remote)` subscribes local output to remote input.
- `ClosePortReadAt` and `ClosePortWriteAt` disconnect the corresponding subscription.

The sequencer owns all ports it creates. `DeletePort(address)` accepts only an existing local address and removes that port and its subscriptions. Deleting the default invalidates it (`Port == -1`); default routing fails until a newly created port becomes the default or an existing owned address is selected. Other ports remain usable.

`Close` releases all ports and subscriptions, including those created through the compatibility wrapper, and is safe to repeat. Port creation, deletion, writes, and subscription operations reject a closed sequencer. Serialize lifecycle changes with other operations on the same sequencer. Addresses are numeric identities, not generation-aware handles: ALSA may reuse a deleted port ID, so discard saved addresses when deleting ports or closing clients.

## Discovery and resolution

`Seq.Devices()` retains its source-only behavior. Each `SeqDevice` includes `Caps`, the raw ALSA capability flags. `DevicesFiltered` supports:

| Filter | Required capabilities |
| --- | --- |
| `PortSource` | `PortCapRead \| PortCapSubsRead` |
| `PortDest` | `PortCapWrite \| PortCapSubsWrite` |
| `PortDuplex` | Both source and destination capabilities |
| `PortAny` | None |

Directions describe the remote port. A destination is a synth or other receiver that this client writes to. `PortCapDuplex` alone does not establish that both subscription directions are supported.

`PortAddress` resolves exact port names, client names, or `client-name:port-name` pairs across all ports, including output-only destinations. Multiple matching ports return `*AmbiguousPortError`, containing the matching devices and numeric addresses. Use a qualified name or numeric address to disambiguate. If even the qualified name is duplicated, use its numeric address.

Numeric `PortAddress` accepts decimal `client:port` addresses without checking whether the port exists. Each field must be in 0 through 255; malformed, trailing, overflowing, and negative values are rejected. A numeric client prefix followed by a colon is treated as an address rather than a name. Subscription and event-writing operations validate before narrowing fields. The legacy `CAddrValues` signature is unchanged; invalid input panics instead of silently wrapping. ALSA special addresses such as `SubsSeqAddr` remain representable, but are not ordinary ports for discovery or subscription.

## Connecting

| Helper | Subscription |
| --- | --- |
| `OpenPortRead`, `OpenPortNameRead` | Remote source to the default port |
| `OpenPortWrite`, `OpenPortNameWrite` | Default port to remote destination |
| `OpenPortDuplex` | Both directions |

`OpenPort` and `OpenPortName` remain input-only. Name-based read/write helpers resolve against the corresponding direction. For a duplex connection to a named port, resolve it with `PortAddress` and pass the address to `OpenPortDuplex`.

All subscription helpers check the remote capabilities. Duplex connects the input direction first. If output connection fails, it removes only the input subscription it just created. Rollback errors are joined with the original failure. Existing subscriptions are not silently adopted or removed when the first connection fails. Disconnect using `ClosePortRead` and `ClosePortWrite`.

## Verification

```sh
go test ./alsa ./cmd/fireloop
go test -race ./alsa -count=1 -timeout=60s
go build ./alsa ./cmd/fireloop
go vet ./alsa
```

ALSA tests create isolated software clients without using physical MIDI devices. They skip when `/dev/snd/seq` is absent and fail on other open errors. Tests cover actual returned IDs, multiple ports, deletion/recreation, explicit and default routing, subscription endpoints, ownership rejection, and client cleanup.

Integration tests also create source-only, destination-only, duplex, unsubscribable, and duplicate-name ports. Parsing and range-validation tests do not require a sequencer. The rollback tests cover both a direction failure and an ALSA failure caused by an existing output subscription, plus preservation of existing input subscriptions.

# Test seams

Commands do their MIDI I/O through the sequencer, and it is the only part of that plumbing that needs hardware. A consumer holding one of five narrow views instead of `*Seq` can therefore be driven with no `/dev/snd/seq` at all:

| Interface | Method | Reached by |
| --- | --- | --- |
| `EventReader` | `Read() (SeqEvent, error)` | A blocking read loop |
| `EventWriter` | `Write(SeqEvent) error` | A write on the default port |
| `PortWriter` | `WritePort(SeqEvent, int) error` | A write from a chosen local port |
| `Closer` | `Close() error` | Shutdown |
| `Scheduler` | `Start`, `ScheduleIn`, `SchedulePort`, `Reset`, `Close` | A queue that delivers later |

Take the narrowest one that fits: a wider one would force every stand-in to implement methods its caller never reaches. Where something reads and writes, a field per direction says which is which, and reaching for the wrong one does not compile. Where one thing genuinely needs both — notes out of the default port and transport out of a second — a named type carrying the two methods says so once and reads better than two parameters passed side by side; write those methods out rather than embedding the two, so the calls it makes stay visible where it is declared. `Scheduler` is the exception to the one-method rule, because its lifecycle is not a second job: a stand-in that could accept a scheduled event and had no way to be started would be recording something the real queue could never do. `*Seq` satisfies the first four and `*SeqQueue` the fifth, and `interfaces.go` asserts both at build time, so a signature that stops matching is a compile error rather than something a test has to notice.

`alsa/fake` is the stand-in, in its own package so that a consumer's test can import it while no production binary links it. `fake.Seq` records writes with the port each went out on, hands back queued events, and injects a read, write or close failure. `fake.Queue` is the same idea for scheduled output: it records each submission with the delay and the port it was given, and refuses what the real queue refuses — a delay in the past, a queue that is not running, and a released queue — so a consumer's branch for each is reachable with no hardware. A full queue is the exception: the real ceiling is a number `alsa` chose rather than one it can report, so `fake.Queue` takes it as a field and counts nothing until a test sets it.

Three things about it are worth knowing before relying on it. `Read` does not block the way the real one does: it returns `fake.ErrNoEvents` once the queue is empty, so a read loop driven by a stand-in has to treat an error as "nothing right now" rather than "broken". And it has no lock, for the same reason `Seq` has none: the contract to serialize lifecycle changes against other operations still holds, so a test that races has a sequencing bug and the fix belongs in the test. It also accepts writes on any port and validates nothing, since port ownership and message validation are `Seq`'s own policy and are covered by this package's tests. The same applies to `fake.Queue` and the queue's event ceiling, which is a policy about how much lookahead this package will hold rather than something a test can be shown.

This makes the package testable without hardware, not without cgo. Every `C.` call lives in `seq_cgo.go` and the policy it serves in `seq.go` and `queue.go`, but cgo is a property of the package rather than of a file and `Seq` holds an `*snd_seq_t` either way.

## Verification

```sh
go test ./alsa/fake
go vet ./alsa ./alsa/fake
```

`alsa/fake`'s own tests need neither a sequencer nor libasound at runtime, but building it still does, because it imports `alsa`.

# ALSA sequencer system-message support

`Seq.Read`, `Seq.Write`, and `Seq.WritePort` use the following MIDI 1.0 status mappings. This table concerns sequencer translation, not raw-device I/O or stream framing.

| Status | Message | Input | Output |
| --- | --- | --- | --- |
| F0 | System Exclusive | SYSEX payload copied | SYSEX variable-length payload |
| F1 | MIDI Time Code quarter frame | QFRAME control value | QFRAME control value |
| F2 | Song Position Pointer | SONGPOS control value | SONGPOS control value |
| F3 | Song Select | SONGSEL control value | SONGSEL control value |
| F4, F5 | Undefined | No standard ALSA mapping | Unsupported status error |
| F6 | Tune Request | Unsupported event error | Unsupported status error |
| F7 | End of Exclusive | Preserved within SYSEX payload | Preserved within SYSEX payload; unsupported standalone |
| F8 | Timing Clock | CLOCK | CLOCK |
| F9 | Undefined | ALSA TICK returns unsupported event error | Unsupported status error |
| FA | Start | START | START |
| FB | Continue | CONTINUE | CONTINUE |
| FC | Stop | STOP | STOP |
| FD | Undefined | No standard ALSA mapping | Unsupported status error |
| FE | Active Sensing | Unsupported event error | Unsupported status error |
| FF | System Reset | Unsupported event error | Unsupported status error |

Song Position is `LSB | (MSB << 7)`, in the range 0 through 16383. Both SONGPOS and QFRAME use `snd_seq_ev_ctrl_t.value`, not queue-control parameters. This matches the installed `/usr/include/alsa/seq_event.h` event declarations and controller structure. Quarter-frame data preserves every 7-bit value, including the message-type nibble; it does not assemble or interpret complete time codes.

Clock, Start, Continue, and Stop retain direct event delivery and direct queue-control selection. ALSA TICK is not standard MIDI Timing Clock and is not emitted as F8 or the undefined F9 status. The legacy `midi.Tick` constant does not imply sequencer support.

Output validation uses the shared `validateMessage` path: unsupported statuses wrap `*UnsupportedMessageError`, while malformed messages wrap `*InvalidMessageError`. Use `errors.As` to inspect their status and reason; error text includes a bounded byte dump. Unsupported input events (tick, tune request, active sensing, system reset) wrap `ErrUnsupportedEvent`; out-of-range incoming SONGPOS/QFRAME/SONGSEL values wrap `ErrInvalidMessage`, identifiable with `errors.Is`. Empty writes remain no-ops on an owned, open port; closed sequencers and unowned ports still return lifecycle errors. Channel messages, including aftertouch and pitch bend, and skipping of unrelated ALSA notifications are unchanged.

SysEx input payloads are copied unchanged. Output requires F0/F7 framing and seven-bit payload bytes, preserving the shared validator's checks; embedded realtime messages must be sent separately.

## Verification

Requires CGO, installed ALSA headers, and libasound, but no MIDI hardware or sequencer port:

```sh
go test ./alsa ./midi
go test -race ./alsa
go vet ./alsa ./midi
go build ./alsa ./midi
```

Tests call the same event encoder and decoder used by `WritePort` and `Read`. They check ALSA event types, the native controller-value field, Song Position values 0/1/8192/16383, all 128 quarter-frame data bytes, transport round trips, and invalid or unsupported inputs.
