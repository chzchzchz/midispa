package alsa

import (
	"errors"
	"fmt"
	"testing"
)

func TestBoundedMessageDump(t *testing.T) {
	const largeMessageSize = 1 << 20
	for _, size := range []int{messageDumpLimit, messageDumpLimit + 1, largeMessageSize} {
		for _, status := range []byte{0xf0, 0xf4} {
			data := make([]byte, size)
			data[0] = status
			data[len(data)-2] = 0xff
			data[len(data)-1] = 0xf7
			err := newTestSeq().Write(MakeEvent(data))
			var invalid *InvalidMessageError
			var unsupported *UnsupportedMessageError
			var base error
			if status == 0xf0 {
				if !errors.As(err, &invalid) || invalid.Reason != fmt.Sprintf("data byte %d is not seven-bit", size-2) {
					t.Fatalf("lost offending offset: %v", err)
				}
				base = invalid
			} else {
				if !errors.As(err, &unsupported) {
					t.Fatalf("lost unsupported type: %v", err)
				}
				base = unsupported
			}
			want := fmt.Sprintf("%v; bytes: [% X]", base, data[:min(size, messageDumpLimit)])
			if size > messageDumpLimit {
				want = fmt.Sprintf("%v; bytes: [% X ...] (%d bytes total)", base, data[:messageDumpLimit], size)
			}
			if err.Error() != want {
				t.Fatalf("unexpected dump: %v", err)
			}
			clear(data)
			if err.Error() != want {
				t.Fatal("dump changed after buffer reuse")
			}
		}
	}
}

func TestMessageErrorDetails(t *testing.T) {
	cases := []struct {
		data        []byte
		want        string
		unsupported bool
	}{
		{[]byte{0x93, 0x40}, "invalid MIDI Note On (0x93): expected 3 bytes, got 2; bytes: [93 40]", false},
		{[]byte{0xb5, 0x07, 0xff}, "invalid MIDI Control Change (0xb5): data byte 2 is not seven-bit; bytes: [B5 07 FF]", false},
		{[]byte{0xf0, 0x01}, "invalid MIDI SysEx (0xf0): SysEx must be framed by F0 and F7; bytes: [F0 01]", false},
		{[]byte{0xf1}, "invalid MIDI Quarter Frame (0xf1): expected 2 bytes, got 1; bytes: [F1]", false},
		{[]byte{0xf1, 0x80}, "invalid MIDI Quarter Frame (0xf1): data byte 1 is not seven-bit; bytes: [F1 80]", false},
		{[]byte{0xf4}, "unsupported MIDI message (0xf4); bytes: [F4]", true},
		{[]byte{0x01, 0x02}, "invalid MIDI message (0x01): missing status byte; bytes: [01 02]", false},
	}
	for _, test := range cases {
		t.Run(test.want, func(t *testing.T) {
			seq := newTestSeq()
			seq.output = func(*outputEvent) error {
				t.Fatal("invalid message reached output")
				return nil
			}
			for _, write := range []func() error{
				func() error { return seq.Write(MakeEvent(test.data)) },
				func() error { return seq.WritePort(MakeEvent(test.data), seq.Port) },
				func() error {
					count, err := seq.NewWriter(SubsSeqAddr).Write(test.data)
					if count != 0 {
						t.Fatalf("accepted %d bytes", count)
					}
					return err
				},
			} {
				err := write()
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				var invalid *InvalidMessageError
				var unsupported *UnsupportedMessageError
				if test.unsupported {
					if !errors.As(err, &unsupported) {
						t.Fatal("lost unsupported error type")
					}
				} else if !errors.As(err, &invalid) {
					t.Fatal("lost invalid error type")
				}
			}
			err := seq.Write(MakeEvent(test.data))
			for index := range test.data {
				test.data[index] = 0
			}
			if err.Error() != test.want {
				t.Fatal("error dump changed after input buffer reuse")
			}
		})
	}
}
