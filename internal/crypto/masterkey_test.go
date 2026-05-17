package crypto

import (
	"bytes"
	"testing"
)

func TestNewMasterKey_RejectsWrongLen(t *testing.T) {
	if _, err := NewMasterKey([]byte("short")); err == nil {
		t.Fatal("expected error for short input")
	}
	if _, err := NewMasterKey(bytes.Repeat([]byte{0x42}, 33)); err == nil {
		t.Fatal("expected error for too-long input")
	}
}

func TestNewMasterKey_CopiesBytes(t *testing.T) {
	in, _ := Random(KeySize)
	mk, err := NewMasterKey(in)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mk, in) {
		t.Fatal("master key bytes mismatch")
	}
	// mutating the input must not mutate the wrapped key
	in[0] ^= 0xff
	if bytes.Equal(mk, in) {
		t.Fatal("NewMasterKey did not copy bytes; mutation of input leaked into MasterKey")
	}
}

func TestMasterKey_Wipe(t *testing.T) {
	in, _ := Random(KeySize)
	mk, _ := NewMasterKey(in)
	mk.Wipe()
	for i, v := range mk {
		if v != 0 {
			t.Fatalf("byte %d not zeroed after Wipe", i)
		}
	}
}
