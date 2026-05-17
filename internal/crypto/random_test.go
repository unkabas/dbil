package crypto

import (
	"bytes"
	"testing"
)

func TestRandom_Zero(t *testing.T) {
	b, err := Random(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 0 {
		t.Fatalf("want empty slice, got len=%d", len(b))
	}
}

func TestRandom_Length(t *testing.T) {
	b, err := Random(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("want len=32, got %d", len(b))
	}
}

func TestRandom_Distinct(t *testing.T) {
	a, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two Random(32) calls produced identical bytes; rng broken")
	}
}
