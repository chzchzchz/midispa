package sr16

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex"
)

var midiPort = os.Getenv("MIDI_PORT")

func noErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncodeDataBytes(t *testing.T) {
	plain := []byte{0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff}
	plain = append(plain, plain...)
	enc := sysex.LoHiEncodeDataBytes(plain)
	vals := []byte{0x7f, 0x40, 0x1f, 0x70, 0x07, 0x7c, 0x01, 0x7f}
	vals = append(vals, vals...)
	for i := range vals {
		if enc[i] != vals[i] {
			t.Errorf("0x%x = enc[%d] != expected[%d] = 0x%x", enc[i], i, i, vals[i])
		}
	}
	dec := sysex.LoHiDecodeDataBytes(vals)
	for i := range plain {
		if dec[i] != plain[i] {
			t.Errorf("0x%x = dec[%d] != expected[%d] = 0x%x", dec[i], i, i, plain[i])
		}
	}
}

func TestDumpUnmarshalShortData(t *testing.T) {
	// Data shorter than 7 bytes should not panic
	d := &Dump{}
	err := d.UnmarshalBinary([]byte{0xf0, 0, 0, 0xe, 5, 0})
	if err == nil {
		t.Error("expected error for short data")
	}
	// Data of exactly 7 bytes should also fail (too short for payload)
	err = d.UnmarshalBinary([]byte{0xf0, 0, 0, 0xe, 5, 0, 0x01})
	if err == nil {
		t.Error("expected error for too-short payload")
	}
}

func TestDumpUnmarshalBadHeader(t *testing.T) {
	d := &Dump{}
	// Bad header should return error
	err := d.UnmarshalBinary([]byte{0xf0, 0, 0, 0xe, 5, 0, 0x01, 0xf7})
	if err == nil {
		t.Error("expected error for bad header")
	}
}

func TestDump(t *testing.T) {
	if midiPort == "" {
		t.Skip("set MIDI_PORT to an SR16 MIDI port to run this hardware test")
	}
	aseq, err := alsa.OpenSeq("testdump")
	noErr(t, err)
	defer aseq.Close()

	sa, err := aseq.PortAddress(midiPort)
	noErr(t, err)
	noErr(t, aseq.OpenPortWrite(sa))
	defer aseq.ClosePortWrite(sa)
	noErr(t, aseq.OpenPortRead(sa))
	defer aseq.ClosePortRead(sa)

	outMsg, _ := (&InquiryRequest{}).MarshalBinary()
	ev := alsa.SeqEvent{sa, outMsg}
	noErr(t, aseq.Write(ev))

	ev, err = aseq.Read()
	noErr(t, err)

	outMsg, _ = (&DumpRequest{}).MarshalBinary()
	ev = alsa.SeqEvent{sa, outMsg}
	noErr(t, aseq.Write(ev))

	ev, err = aseq.ReadSysEx()
	noErr(t, err)

	d := &Dump{}
	noErr(t, d.UnmarshalBinary(ev.Data))

	fmt.Printf("%q\n", d.Memory)
	for _, tt := range []struct {
		off int
		v   byte
	}{
		// {0xe0, 0}, always 1?
		{0xe2, 0},
		{0xe6, 1},
		{0xf4, 0},
		{0xfe, 0x27},
		{0xff, 0xb5},
	} {
		if tt.off >= len(d.Memory) {
			t.Errorf("0x%x out of range for data length %d", tt.off, len(d.Memory))
		}
		if v := d.Memory[tt.off]; v != tt.v {
			t.Errorf("data[0x%x] = 0x%x but want 0x%x", tt.off, v, tt.v)
		}
	}
	enc, _ := d.MarshalBinary()
	if bytes.Compare(enc, ev.Data) != 0 {
		for i := 0; i < len(ev.Data); i++ {
			if enc[i] != ev.Data[i] {
				t.Errorf("0x%x = enc[0x%x] != ev.Data[0x%x] = 0x%x", enc[i], i, i, ev.Data[i])
				break
			}
		}
		t.Fatalf("encoded sysex did not match sysex from sr-16")
	}
}
