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

Repository-wide builds require the generated trackscript grammar (`go generate ./cmd/trackscript`). The SR-16 dump integration test requires `MIDI_PORT` and a responding device; it skips when that environment variable is unset.

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
