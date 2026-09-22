package sysex

import (
	"testing"
)

func TestEncodeDecode7bitInt(t *testing.T) {
	for _, v := range []int{0, 1, 127, 128, 255, 16383, 16384} {
		encoded, err := encode7bitInt(v, 3)
		if err != nil {
			t.Fatalf("encode7bitInt(%d): %v", v, err)
		}
		decoded, err := decode7bitInt(encoded)
		if err != nil {
			t.Fatalf("decode7bitInt(%v): %v", encoded, err)
		}
		if decoded != v {
			t.Errorf("round-trip failed: encode(%d) = %v, decode = %d", v, encoded, decoded)
		}
	}
}

func TestEncode7bitIntOverflow(t *testing.T) {
	_, err := encode7bitInt(1<<21, 3)
	if err == nil {
		t.Error("expected error for overflow")
	}
}
