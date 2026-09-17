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

Repository-wide builds currently require the missing generated `Grammar` definition. The SR-16 dump test also requires `MIDI_PORT` and a responding device. Fireloop has pre-existing unkeyed event-literal vet warnings.
