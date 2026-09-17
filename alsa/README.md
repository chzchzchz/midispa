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

## Verification

```sh
go test ./alsa ./cmd/fireloop
go test -race ./alsa -count=10 -timeout=30s
go build ./alsa ./cmd/fireloop
go vet ./alsa
```

ALSA tests create isolated software clients without using physical MIDI devices. They skip when `/dev/snd/seq` is absent and fail on other open errors. Tests cover actual returned IDs, multiple ports, deletion/recreation, explicit and default routing, subscription endpoints, ownership rejection, and client cleanup.

Repository-wide builds currently require the missing generated `Grammar` and `defaultPolicy` definitions. The SR-16 dump test also requires `MIDI_PORT` and a responding device. Fireloop has pre-existing unkeyed event-literal vet warnings.
