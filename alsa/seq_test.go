package alsa

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
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

func openTestSeq(t *testing.T) *Seq {
	t.Helper()
	if _, err := os.Stat("/dev/snd/seq"); os.IsNotExist(err) {
		t.Skip("ALSA sequencer is unavailable")
	}
	seq, err := OpenSeq(t.Name())
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
