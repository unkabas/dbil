package crypto

import "testing"

func TestZero_OverwritesAllBytes(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5, 0x42, 0xff}
	Zero(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("byte %d not zeroed: %#x", i, v)
		}
	}
}

func TestZero_HandlesEmptyAndNil(t *testing.T) {
	Zero(nil)
	Zero([]byte{})
}
