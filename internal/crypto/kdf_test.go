package crypto

import (
	"bytes"
	"testing"
)

func TestHKDF_Deterministic(t *testing.T) {
	secret := []byte("master-key-material")
	a, err := HKDF(secret, "dbil:test", 32)
	if err != nil {
		t.Fatalf("HKDF: %v", err)
	}
	b, err := HKDF(secret, "dbil:test", 32)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("HKDF not deterministic for the same (secret, info, len)")
	}
}

func TestHKDF_DivergesOnInfo(t *testing.T) {
	secret := []byte("master")
	a, _ := HKDF(secret, "dbil:dek-wrap-v1", 32)
	b, _ := HKDF(secret, "dbil:audit-dek-v1", 32)
	if bytes.Equal(a, b) {
		t.Fatal("HKDF returned same bytes for different info strings; domain separation broken")
	}
}

func TestHKDF_LengthHonored(t *testing.T) {
	out, err := HKDF([]byte("k"), "x", 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 64 {
		t.Fatalf("want 64, got %d", len(out))
	}
}

func TestHKDF_RejectsZeroLen(t *testing.T) {
	if _, err := HKDF([]byte("k"), "x", 0); err == nil {
		t.Fatal("want error for outLen=0")
	}
}

func TestArgon2id_Deterministic(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, 16)
	a, err := Argon2id([]byte("password"), salt, 32)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Argon2id([]byte("password"), salt, 32)
	if !bytes.Equal(a, b) {
		t.Fatal("argon2id not deterministic for the same input")
	}
}

func TestArgon2id_DivergesOnPassword(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, 16)
	a, _ := Argon2id([]byte("password-a"), salt, 32)
	b, _ := Argon2id([]byte("password-b"), salt, 32)
	if bytes.Equal(a, b) {
		t.Fatal("argon2id collapsed two distinct passwords")
	}
}

func TestArgon2id_RejectsShortSalt(t *testing.T) {
	if _, err := Argon2id([]byte("pw"), bytes.Repeat([]byte{0}, 15), 32); err == nil {
		t.Fatal("expected error for salt < 16 bytes")
	}
}

func TestArgon2id_RejectsZeroLen(t *testing.T) {
	if _, err := Argon2id([]byte("pw"), bytes.Repeat([]byte{0}, 16), 0); err == nil {
		t.Fatal("expected error for outLen=0")
	}
}

func TestNewSalt_Length(t *testing.T) {
	s, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != SaltSize {
		t.Fatalf("want salt of %d bytes, got %d", SaltSize, len(s))
	}
}
