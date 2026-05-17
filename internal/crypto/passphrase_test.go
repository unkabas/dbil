package crypto

import (
	"bytes"
	"testing"
)

func TestPassphrase_RoundTrip(t *testing.T) {
	salt, _ := NewSalt()
	pt := []byte("prod-db-password-1234")
	ef, err := WrapWithPassphrase("correct-horse-battery-staple", salt, "conn-prod-1", pt)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapWithPassphrase("correct-horse-battery-staple", salt, "conn-prod-1", ef)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatal("passphrase roundtrip mismatch")
	}
}

func TestPassphrase_WrongPassphraseFails(t *testing.T) {
	salt, _ := NewSalt()
	ef, err := WrapWithPassphrase("right-passphrase", salt, "c", []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnwrapWithPassphrase("wrong-passphrase", salt, "c", ef); err == nil {
		t.Fatal("expected decrypt failure with wrong passphrase")
	}
}

func TestPassphrase_DifferentConnIDFails(t *testing.T) {
	salt, _ := NewSalt()
	ef, err := WrapWithPassphrase("the-same-passphrase", salt, "conn-A", []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UnwrapWithPassphrase("the-same-passphrase", salt, "conn-B", ef); err == nil {
		t.Fatal("expected decrypt failure when connID changes")
	}
}

func TestPassphrase_RejectsShortPassphrase(t *testing.T) {
	salt, _ := NewSalt()
	if _, err := WrapWithPassphrase("short", salt, "c", []byte("d")); err == nil {
		t.Fatal("expected rejection of short passphrase")
	}
}

func TestPassphrase_RejectsShortSalt(t *testing.T) {
	if _, err := DerivePassphraseKey("long-enough-passphrase", []byte("short")); err == nil {
		t.Fatal("expected rejection of short salt")
	}
}

func TestPassphrase_RejectsBadVersion(t *testing.T) {
	salt, _ := NewSalt()
	ef, err := WrapWithPassphrase("a-passphrase", salt, "c", []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	ef.Version = 999
	if _, err := UnwrapWithPassphrase("a-passphrase", salt, "c", ef); err == nil {
		t.Fatal("expected version error")
	}
}
