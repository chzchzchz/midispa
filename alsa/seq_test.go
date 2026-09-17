package alsa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
)

func testPort(t *testing.T, aseq *Seq, label string, caps PortCaps) SeqDevice {
	t.Helper()
	name := fmt.Sprintf("seqcap-%d-%d-%s", os.Getpid(), aseq.Client, label)
	if _, err := aseq.createPortAddrCaps(name, caps); err != nil {
		t.Fatal(err)
	}
	dev, err := aseq.resolvePort(name)
	if err != nil {
		t.Fatal(err)
	}
	if dev.Caps != caps {
		t.Fatalf("port caps = %#x, want %#x", dev.Caps, caps)
	}
	return dev
}

func TestParseSeqAddr(t *testing.T) {
	for _, addr := range []SeqAddr{{0, 0}, {24, 3}, {maxSeqAddress, maxSeqAddress}, SubsSeqAddr} {
		parsed, err := parseSeqAddr(addr.String())
		if err != nil || parsed != addr {
			t.Fatalf("parse %v = %v, %v", addr, parsed, err)
		}
		client, port := addr.CAddrValues()
		if int(client) != addr.Client || int(port) != addr.Port {
			t.Fatalf("address narrowed incorrectly: %v", addr)
		}
	}
	for _, bad := range []string{"", "x", "1:", ":2", "300:0", "0:-1", "-1:0", "0:256", "1:2:3", "1:2junk", "1:2 ", "99999999999999999999999:0"} {
		if _, err := parseSeqAddr(bad); err == nil {
			t.Errorf("parseSeqAddr(%q) should fail", bad)
		}
		if numericSeqName(bad) {
			aseq := &Seq{}
			if _, err := aseq.PortAddress(bad); err == nil {
				t.Errorf("PortAddress(%q) should fail", bad)
			}
			if _, err := aseq.resolvePort(bad); err == nil {
				t.Errorf("ResolvePort(%q) should fail", bad)
			}
		}
	}
}

func TestAddressValidation(t *testing.T) {
	aseq := &Seq{}
	for _, addr := range []SeqAddr{{-1, 0}, {0, -1}, {maxSeqAddress + 1, 0}, {0, maxSeqAddress + 1}, {1 << 40, 0}} {
		for name, operation := range map[string]func(SeqAddr) error{
			"validate":   SeqAddr.validate,
			"read":       aseq.OpenPortRead,
			"write":      aseq.OpenPortWrite,
			"duplex":     aseq.OpenPortDuplex,
			"closeRead":  aseq.ClosePortRead,
			"closeWrite": aseq.ClosePortWrite,
			"legacy":     func(addr SeqAddr) error { return aseq.OpenPort(addr.Client, addr.Port) },
			"event":      func(addr SeqAddr) error { return aseq.Write(SeqEvent{SeqAddr: addr, Data: []byte{0xf8}}) },
		} {
			if err := operation(addr); err == nil {
				t.Errorf("%s accepted %v", name, addr)
			}
		}
		t.Run(addr.String(), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("CAddrValues silently narrowed an invalid address")
				}
			}()
			addr.CAddrValues()
		})
	}
	if err := aseq.WritePort(MakeEvent([]byte{0xf8}), maxSeqAddress+1); err == nil {
		t.Error("invalid local port accepted")
	}
	if _, err := aseq.DevicesFiltered(PortDir(-1)); err == nil {
		t.Error("invalid discovery direction accepted")
	}
}

func TestDevicesFiltered(t *testing.T) {
	aseq := openTestSeq(t)
	source := testPort(t, aseq, "source", PortCapRead|PortCapSubsRead)
	dest := testPort(t, aseq, "dest", PortCapWrite|PortCapSubsWrite)
	duplex := testPort(t, aseq, "duplex", PortCapRead|PortCapSubsRead|PortCapWrite|PortCapSubsWrite)
	plain := testPort(t, aseq, "plain", PortCapRead|PortCapWrite|PortCapDuplex)
	for _, dir := range []PortDir{PortSource, PortDest, PortDuplex, PortAny} {
		devs, err := aseq.DevicesFiltered(dir)
		if err != nil {
			t.Fatal(err)
		}
		found := make(map[SeqAddr]bool)
		for _, dev := range devs {
			found[dev.SeqAddr] = true
		}
		for _, check := range []struct {
			dev  SeqDevice
			want bool
		}{
			{source, dir == PortSource || dir == PortAny},
			{dest, dir == PortDest || dir == PortAny},
			{duplex, true},
			{plain, dir == PortAny},
		} {
			if found[check.dev.SeqAddr] != check.want {
				t.Errorf("direction %d port %s: found = %v, want %v", dir, check.dev.PortName, found[check.dev.SeqAddr], check.want)
			}
		}
	}
	legacy, err := aseq.Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, dev := range legacy {
		if !dev.Caps.Has(PortCapRead | PortCapSubsRead) {
			t.Errorf("legacy discovery returned non-source: %+v", dev)
		}
	}
}

func TestResolvePort(t *testing.T) {
	aseq := openTestSeq(t)
	self, err := aseq.resolvePort(aseq.SeqAddr.String())
	if err != nil {
		t.Fatal(err)
	}
	byClient, err := aseq.resolvePort(self.ClientName)
	if err != nil || byClient != self {
		t.Fatalf("client name lookup: %+v, %v", byClient, err)
	}
	source := testPort(t, aseq, "shared", PortCapRead|PortCapSubsRead)
	peer := openTestSeq(t, "peer")
	if _, err := peer.createPortAddrCaps(source.PortName, PortCapWrite|PortCapSubsWrite); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{source.PortName, self.ClientName} {
		_, err := aseq.PortAddress(name)
		var ambiguous *AmbiguousPortError
		if !errors.As(err, &ambiguous) || len(ambiguous.Matches) < 2 {
			t.Fatalf("expected ambiguity for %q, got %v", name, err)
		}
		if !strings.Contains(err.Error(), source.SeqAddr.String()) {
			t.Errorf("ambiguity error lacks matching address: %v", err)
		}
	}
	resolved, err := aseq.resolvePortFiltered(source.PortName, PortSource)
	if err != nil || resolved != source {
		t.Fatalf("source resolution: %+v, %v", resolved, err)
	}
	dest, err := aseq.resolvePortFiltered(source.PortName, PortDest)
	if err != nil || dest.Client != peer.Client {
		t.Fatalf("destination resolution: %+v, %v", dest, err)
	}
	for _, name := range []string{dest.SeqAddr.String(), dest.ClientName + ":" + dest.PortName} {
		resolved, err := aseq.resolvePort(name)
		if err != nil || resolved != dest {
			t.Fatalf("lookup %q: %+v, %v", name, resolved, err)
		}
	}
	if _, err := aseq.resolvePortFiltered(dest.SeqAddr.String(), PortSource); err == nil {
		t.Error("numeric destination accepted as source")
	}
	if _, err := peer.createPortAddrCaps(source.PortName, PortCapWrite|PortCapSubsWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := aseq.resolvePort(dest.ClientName + ":" + dest.PortName); err == nil {
		t.Error("duplicate qualified name accepted")
	}
	if _, err := aseq.resolvePort(source.PortName + "-missing"); err == nil {
		t.Error("missing name accepted")
	}
}

func TestDirectionChecksAndRollback(t *testing.T) {
	aseq := openTestSeq(t)
	peer := openTestSeq(t, "peer")
	source := testPort(t, peer, "source", PortCapRead|PortCapSubsRead)
	dest := testPort(t, peer, "dest", PortCapWrite|PortCapSubsWrite)
	duplex := testPort(t, peer, "duplex", PortCapRead|PortCapSubsRead|PortCapWrite|PortCapSubsWrite)
	if err := aseq.OpenPortRead(dest.SeqAddr); err == nil {
		t.Error("destination accepted as input")
	}
	if err := aseq.OpenPortWrite(source.SeqAddr); err == nil {
		t.Error("source accepted as output")
	}
	if err := aseq.OpenPortNameRead(dest.PortName); err == nil {
		t.Error("destination name accepted as input")
	}
	if err := aseq.OpenPortNameWrite(source.PortName); err == nil {
		t.Error("source name accepted as output")
	}
	if err := aseq.OpenPortName(source.PortName); err != nil {
		t.Fatal(err)
	}
	if err := aseq.ClosePortWrite(source.SeqAddr); err == nil {
		t.Error("legacy name helper created an output subscription")
	}
	if err := aseq.ClosePortRead(source.SeqAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortNameWrite(dest.PortName); err != nil {
		t.Fatal(err)
	}
	if err := aseq.ClosePortWrite(dest.SeqAddr); err != nil {
		t.Fatal(err)
	}
	sourceAddr, err := aseq.PortAddress(source.PortName)
	if err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortDuplex(sourceAddr); err == nil {
		t.Fatal("expected duplex to source-only port to fail")
	}
	if err := aseq.ClosePortRead(source.SeqAddr); err == nil {
		t.Fatal("read subscription survived duplex rollback")
	}
	duplexAddr, err := aseq.PortAddress(duplex.PortName)
	if err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortDuplex(duplexAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.ClosePortRead(duplex.SeqAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.ClosePortWrite(duplex.SeqAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortWrite(duplex.SeqAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortDuplex(duplex.SeqAddr); err == nil {
		t.Fatal("duplicate output subscription should fail")
	}
	if err := aseq.ClosePortRead(duplex.SeqAddr); err == nil {
		t.Error("read subscription survived failed second subscription")
	}
	if err := aseq.ClosePortWrite(duplex.SeqAddr); err != nil {
		t.Fatalf("rollback removed pre-existing output subscription: %v", err)
	}
	if err := aseq.OpenPortRead(duplex.SeqAddr); err != nil {
		t.Fatal(err)
	}
	if err := aseq.OpenPortDuplex(duplex.SeqAddr); err == nil {
		t.Fatal("duplicate input subscription should fail")
	}
	if err := aseq.ClosePortRead(duplex.SeqAddr); err != nil {
		t.Fatalf("first failure removed pre-existing input subscription: %v", err)
	}
	if err := aseq.ClosePortWrite(duplex.SeqAddr); err == nil {
		t.Error("first failure created output subscription")
	}
}
const (
	alsaSongPosition       = 20
	alsaSongSelect         = 21
	alsaQuarterFrame       = 22
	alsaStart              = 30
	alsaContinue           = 31
	alsaStop               = 32
	alsaClock              = 36
	alsaTick               = 37
	alsaTuneRequest        = 40
	alsaReset              = 41
	alsaSensing            = 42
	alsaDirectQueue        = 253
	alsaControlValueOffset = 8
)

func TestExpressionRoundTrip(t *testing.T) {
	sender, receiver := openTestSeq(t), openTestSeq(t)
	source := createTestPort(t, sender, t.Name()+" expression")
	for _, channel := range []byte{0, 7, 15} {
		var messages [][]byte
		for _, note := range []byte{0, 127} {
			for _, pressure := range []byte{0, 1, 127} {
				messages = append(messages, []byte{midi.KeyAftertouch | channel, note, pressure})
			}
		}
		for _, pressure := range []byte{0, 127} {
			messages = append(messages, []byte{midi.ChannelAftertouch | channel, pressure})
		}
		for _, pitch := range [][2]byte{{0, 0}, {0, 64}, {127, 127}, {1, 0}, {0, 1}, {1, 64}} {
			messages = append(messages, []byte{midi.Pitch | channel, pitch[0], pitch[1]})
		}
		for _, data := range messages {
			t.Run(fmt.Sprintf("%x", data), func(t *testing.T) {
				if err := sender.WritePort(SeqEvent{receiver.SeqAddr, data}, source.Port); err != nil {
					t.Fatalf("write: %v", err)
				}
				marker := []byte{midi.Clock}
				if err := sender.WritePort(SeqEvent{receiver.SeqAddr, marker}, source.Port); err != nil {
					t.Fatalf("write marker: %v", err)
				}
				event := readTestEvent(t, receiver, data)
				if !bytes.Equal(event.Data, data) {
					t.Fatalf("MIDI bytes = % x, want % x", event.Data, data)
				}
				wantSource := SeqAddr{sender.Client, source.Port}
				if event.SeqAddr != wantSource {
					t.Errorf("source = %v, want %v", event.SeqAddr, wantSource)
				}
				if event := readTestEvent(t, receiver, marker); !bytes.Equal(event.Data, marker) {
					t.Fatalf("marker = % x, want % x", event.Data, marker)
				}
			})
			if t.Failed() {
				return
			}
		}
	}
}

func TestWritePortRejectsInvalidExpression(t *testing.T) {
	seq := openTestSeq(t)
	for _, status := range []byte{midi.KeyAftertouch, midi.ChannelAftertouch, midi.Pitch} {
		length := 3
		if status == midi.ChannelAftertouch {
			length = 2
		}
		for size := 1; size <= length+1; size++ {
			if size == length {
				continue
			}
			data := make([]byte, size)
			data[0] = status
			t.Run(fmt.Sprintf("length/%x", data), func(t *testing.T) {
				if err := seq.WritePort(MakeEvent(data), seq.Port); err == nil {
					t.Fatal("expected invalid length error")
				}
			})
		}
		for index := 1; index < length; index++ {
			for _, value := range []byte{128, 255} {
				data := make([]byte, length)
				data[0], data[index] = status, value
				t.Run(fmt.Sprintf("data/%x", data), func(t *testing.T) {
					if err := seq.WritePort(MakeEvent(data), seq.Port); err == nil {
						t.Fatal("expected non-seven-bit data error")
					}
				})
			}
		}
	}
}

const seqTestTimeout = time.Second

func openTestSeq(t *testing.T, suffix ...string) *Seq {
	t.Helper()
	if _, err := os.Stat("/dev/snd/seq"); os.IsNotExist(err) {
		t.Skip("ALSA sequencer is unavailable")
	}
	seq, err := OpenSeq(fmt.Sprintf("seqcap-%d-%s-%s", os.Getpid(), t.Name(), strings.Join(suffix, "-")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { seqOK(t, seq.Close()) })
	return seq
}

func seqOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func createTestPort(t *testing.T, seq *Seq, name string) SeqAddr {
	t.Helper()
	addr, err := seq.CreatePortAddr(name)
	seqOK(t, err)
	found, err := seq.PortAddress(name)
	seqOK(t, err)
	if addr != found || addr.Client != seq.Client || addr.Port < 0 {
		t.Fatalf("created address %v, ALSA address %v", addr, found)
	}
	return addr
}

func readTestEvent(t *testing.T, seq *Seq, data []byte) SeqEvent {
	t.Helper()
	deadline := time.Now().Add(seqTestTimeout)
	for time.Now().Before(deadline) {
		if !seq.MayRead() {
			time.Sleep(time.Millisecond)
			continue
		}
		event, err := seq.Read()
		seqOK(t, err)
		if slices.Equal(event.Data, data) {
			return event
		}
		t.Fatalf("unexpected event %v, want data %v", event, data)
	}
	t.Fatalf("timed out waiting for %v", data)
	return SeqEvent{}
}

func readSubscription(t *testing.T, seq *Seq, kind byte, source, destination SeqAddr) {
	t.Helper()
	readTestEvent(t, seq, []byte{kind, byte(source.Client), byte(source.Port), byte(destination.Client), byte(destination.Port)})
}

func checkSource(t *testing.T, seq *Seq, source SeqAddr, data []byte) {
	t.Helper()
	if event := readTestEvent(t, seq, data); event.SeqAddr != source {
		t.Fatalf("source %v, want %v", event.SeqAddr, source)
	}
}

func TestSeqPortLifecycle(t *testing.T) {
	sender, receiver := openTestSeq(t), openTestSeq(t)
	original := sender.SeqAddr
	first := createTestPort(t, sender, t.Name()+" first")
	second := createTestPort(t, sender, t.Name()+" second")
	if first == second || first == original || second == original || sender.SeqAddr != original {
		t.Fatal("additional ports must be distinct and preserve the default")
	}
	destination := createTestPort(t, receiver, t.Name()+" destination")
	seqOK(t, sender.OpenPortWriteAt(first, destination))
	readSubscription(t, receiver, EvPortSubscribed, first, destination)
	data := []byte{midi.Start}
	seqOK(t, sender.WritePort(MakeEvent(data), first.Port))
	checkSource(t, receiver, first, data)
	seqOK(t, sender.DeletePort(first))
	readSubscription(t, receiver, EvPortUnsubscribed, first, destination)
	if err := sender.DeletePort(first); err == nil {
		t.Fatal("deleted port accepted")
	}
	if err := sender.WritePort(MakeEvent(data), first.Port); err == nil {
		t.Fatal("write from deleted port accepted")
	}
	if _, err := sender.PortAddress(t.Name() + " first"); err == nil {
		t.Fatal("deleted port still visible")
	}
	recreated := createTestPort(t, sender, t.Name()+" recreated")
	seqOK(t, sender.OpenPortWriteAt(recreated, destination))
	readSubscription(t, receiver, EvPortSubscribed, recreated, destination)
	seqOK(t, sender.WritePort(MakeEvent(data), recreated.Port))
	checkSource(t, receiver, recreated, data)
	seqOK(t, sender.ClosePortWriteAt(recreated, destination))
	readSubscription(t, receiver, EvPortUnsubscribed, recreated, destination)
	seqOK(t, receiver.OpenPortReadAt(destination, second))
	readSubscription(t, sender, EvPortSubscribed, second, destination)
	seqOK(t, sender.WritePort(MakeEvent(data), second.Port))
	checkSource(t, receiver, second, data)
	seqOK(t, receiver.ClosePortReadAt(destination, second))
	readSubscription(t, sender, EvPortUnsubscribed, second, destination)
	seqOK(t, sender.WritePort(SeqEvent{destination, data}, recreated.Port))
	checkSource(t, receiver, recreated, data)
	seqOK(t, receiver.OpenPortReadAt(destination, recreated))
	readSubscription(t, sender, EvPortSubscribed, recreated, destination)
	seqOK(t, sender.Close())
	readSubscription(t, receiver, EvPortUnsubscribed, recreated, destination)
	devices, err := receiver.Devices()
	seqOK(t, err)
	for _, device := range devices {
		if device.Client == sender.Client {
			t.Fatalf("closed client still has port %v", device)
		}
	}
	if len(sender.ports) != 0 || sender.Port != -1 {
		t.Fatal("closed client retains local ports")
	}
}

func TestSeqDefaultPort(t *testing.T) {
	sender, receiver := openTestSeq(t), openTestSeq(t)
	original := sender.SeqAddr
	selected := createTestPort(t, sender, t.Name()+" selected")
	sender.SeqAddr = selected
	seqOK(t, sender.CreatePort(t.Name()+" compatibility"))
	if sender.SeqAddr != selected {
		t.Fatal("compatibility wrapper changed default")
	}
	seqOK(t, sender.OpenPortWrite(receiver.SeqAddr))
	readSubscription(t, receiver, EvPortSubscribed, selected, receiver.SeqAddr)
	data := []byte{midi.Stop}
	seqOK(t, sender.Write(MakeEvent(data)))
	checkSource(t, receiver, selected, data)
	_, err := sender.NewWriter(receiver.SeqAddr).Write(data)
	seqOK(t, err)
	checkSource(t, receiver, selected, data)
	seqOK(t, sender.ClosePortWrite(receiver.SeqAddr))
	readSubscription(t, receiver, EvPortUnsubscribed, selected, receiver.SeqAddr)
	for _, connect := range []func() error{
		func() error { return sender.OpenPortRead(receiver.SeqAddr) },
		func() error { return sender.OpenPort(receiver.Client, receiver.Port) },
		func() error { return sender.OpenPortName(receiver.SeqAddr.String()) },
	} {
		seqOK(t, connect())
		readSubscription(t, receiver, EvPortSubscribed, receiver.SeqAddr, selected)
		seqOK(t, receiver.Write(MakeEvent(data)))
		checkSource(t, sender, receiver.SeqAddr, data)
		seqOK(t, sender.ClosePortRead(receiver.SeqAddr))
		readSubscription(t, receiver, EvPortUnsubscribed, receiver.SeqAddr, selected)
	}
	seqOK(t, sender.DeletePort(selected))
	if sender.Port != -1 {
		t.Fatal("deleted default was not invalidated")
	}
	if err := sender.Write(MakeEvent(data)); err == nil {
		t.Fatal("write without default succeeded")
	}
	seqOK(t, sender.WritePort(SeqEvent{receiver.SeqAddr, data}, original.Port))
	checkSource(t, receiver, original, data)
	replacement := createTestPort(t, sender, t.Name()+" replacement")
	if sender.SeqAddr != replacement {
		t.Fatal("new port did not replace missing default")
	}
}

func TestSeqPortOwnership(t *testing.T) {
	owner, foreign := openTestSeq(t), openTestSeq(t)
	invalid := []SeqAddr{foreign.SeqAddr, {owner.Client, -1}, {owner.Client, 256}}
	for _, addr := range invalid {
		for _, operation := range []func() error{
			func() error { return owner.DeletePort(addr) },
			func() error { return owner.OpenPortReadAt(addr, foreign.SeqAddr) },
			func() error { return owner.OpenPortWriteAt(addr, foreign.SeqAddr) },
			func() error { return owner.ClosePortReadAt(addr, foreign.SeqAddr) },
			func() error { return owner.ClosePortWriteAt(addr, foreign.SeqAddr) },
		} {
			if err := operation(); err == nil {
				t.Fatalf("unowned port %v accepted", addr)
			}
		}
	}
	seqOK(t, owner.Close())
	if err := owner.CreatePort("closed"); err == nil {
		t.Fatal("created port on closed sequencer")
	}
	if err := owner.Write(MakeEvent([]byte{midi.Start})); err == nil {
		t.Fatal("write on closed sequencer accepted")
	}
	if err := owner.OpenPortWrite(foreign.SeqAddr); err == nil {
		t.Fatal("subscription on closed sequencer accepted")
	}
	seqOK(t, owner.Close())
}

func TestSystemMessageRoundTrip(t *testing.T) {
	cases := []struct {
		data      []byte
		eventType int
		value     uint32
	}{
		{[]byte{midi.SongPosition, 0, 0}, alsaSongPosition, 0},
		{[]byte{midi.SongPosition, 1, 0}, alsaSongPosition, 1},
		{[]byte{midi.SongPosition, 0, 64}, alsaSongPosition, 8192},
		{[]byte{midi.SongPosition, 127, 127}, alsaSongPosition, 16383},
		{[]byte{midi.SongSelect, 0}, alsaSongSelect, 0},
		{[]byte{midi.SongSelect, 127}, alsaSongSelect, 127},
	}
	for value := uint32(0); value <= midi.DataMax; value++ {
		cases = append(cases, struct {
			data      []byte
			eventType int
			value     uint32
		}{[]byte{midi.QuarterFrame, byte(value)}, alsaQuarterFrame, value})
	}
	for _, test := range cases {
		t.Run(fmt.Sprintf("%x", test.data), func(t *testing.T) {
			source, destination := SeqAddr{12, 3}, SeqAddr{45, 6}
			event, err := encodeEvent(SeqEvent{destination, test.data}, source)
			if err != nil {
				t.Fatal(err)
			}
			if int(event._type) != test.eventType {
				t.Fatalf("event type = %d, want %d", event._type, test.eventType)
			}
			if value := binary.NativeEndian.Uint32(event.data[alsaControlValueOffset:]); value != test.value {
				t.Fatalf("ALSA control value = %d, want %d", value, test.value)
			}
			if int(event.dest.client) != destination.Client || int(event.dest.port) != destination.Port || event.queue != alsaDirectQueue {
				t.Fatal("destination or direct queue changed")
			}
			decoded, err := decodeSeqEvent(event)
			if err != nil || !bytes.Equal(decoded.Data, test.data) || decoded.SeqAddr != source {
				t.Fatalf("decoded = %+v, error = %v, want %x from %+v", decoded, err, test.data, source)
			}
		})
	}
}

func TestTransportRoundTrip(t *testing.T) {
	for _, test := range []struct {
		status    byte
		eventType int
	}{
		{midi.Clock, alsaClock},
		{midi.Start, alsaStart},
		{midi.Continue, alsaContinue},
		{midi.Stop, alsaStop},
	} {
		t.Run(fmt.Sprintf("%x", test.status), func(t *testing.T) {
			event, err := encodeEvent(MakeEvent([]byte{test.status}), SeqAddr{12, 3})
			if err != nil {
				t.Fatal(err)
			}
			if int(event._type) != test.eventType || event.queue != alsaDirectQueue || event.data[0] != alsaDirectQueue {
				t.Fatalf("unexpected transport event: %+v", event)
			}
			decoded, err := decodeSeqEvent(event)
			if err != nil || !bytes.Equal(decoded.Data, []byte{test.status}) {
				t.Fatalf("decoded = %x, error = %v", decoded.Data, err)
			}
		})
	}
}

func TestUnsupportedSystemStatuses(t *testing.T) {
	for _, status := range []byte{0xf4, 0xf5, 0xf6, midi.EndSysEx, 0xf9, 0xfd, 0xfe, 0xff} {
		for _, data := range [][]byte{{status}, {status, 0}} {
			var unsupported *UnsupportedMessageError
			if err := newTestSeq().Write(MakeEvent(data)); !errors.As(err, &unsupported) {
				t.Errorf("Write(%x) = %v, want unsupported status", data, err)
			}
		}
	}
}

func TestInvalidSystemMessages(t *testing.T) {
	for _, data := range [][]byte{
		{midi.SongPosition}, {midi.SongPosition, 0}, {midi.SongPosition, 0, 0, 0},
		{midi.SongPosition, 128, 0}, {midi.SongPosition, 0, 128},
		{midi.QuarterFrame}, {midi.QuarterFrame, 0, 0}, {midi.QuarterFrame, 128},
		{midi.SongSelect}, {midi.SongSelect, 0, 0}, {midi.SongSelect, 128},
		{midi.Clock, 0}, {midi.Start, 0}, {midi.Continue, 0}, {midi.Stop, 0},
	} {
		var invalid *InvalidMessageError
		if err := newTestSeq().WritePort(MakeEvent(data), testSourcePort); !errors.As(err, &invalid) {
			t.Errorf("WritePort(%x) = %v, want invalid message", data, err)
		}
	}
	if err := newTestSeq().Write(MakeEvent(nil)); err != nil {
		t.Fatalf("empty write = %v", err)
	}
}

func TestUnsupportedSystemEvents(t *testing.T) {
	event, err := encodeEvent(MakeEvent([]byte{midi.Clock}), SeqAddr{})
	if err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []byte{alsaTick, alsaTuneRequest, alsaSensing, alsaReset} {
		switch eventType {
		case alsaTick:
			event._type = alsaTick
		case alsaTuneRequest:
			event._type = alsaTuneRequest
		case alsaSensing:
			event._type = alsaSensing
		case alsaReset:
			event._type = alsaReset
		}
		decoded, err := decodeSeqEvent(event)
		if !errors.Is(err, ErrUnsupportedEvent) || len(decoded.Data) != 0 {
			t.Errorf("event %d decoded = %x, error = %v", eventType, decoded.Data, err)
		}
	}
}

func TestInvalidSystemEventValues(t *testing.T) {
	for _, data := range [][]byte{{midi.SongPosition, 0, 0}, {midi.QuarterFrame, 0}, {midi.SongSelect, 0}} {
		event, err := encodeEvent(MakeEvent(data), SeqAddr{})
		if err != nil {
			t.Fatal(err)
		}
		maximum := uint32(midi.DataMax)
		if data[0] == midi.SongPosition {
			maximum = midi.SongPositionMax
		}
		for _, value := range []uint32{maximum + 1, ^uint32(0)} {
			binary.NativeEndian.PutUint32(event.data[alsaControlValueOffset:], value)
			decoded, err := decodeSeqEvent(event)
			if !errors.Is(err, ErrInvalidMessage) || len(decoded.Data) != 0 {
				t.Errorf("status %x value %d decoded = %x, error = %v", data[0], value, decoded.Data, err)
			}
		}
	}
}
