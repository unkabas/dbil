package crypto

import (
	"bytes"
	"testing"
)

func freshMK(t *testing.T) MasterKey {
	t.Helper()
	b, _ := Random(KeySize)
	mk, err := NewMasterKey(b)
	if err != nil {
		t.Fatal(err)
	}
	return mk
}

func TestDEK_WrapRoundTrip(t *testing.T) {
	mk := freshMK(t)
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	w, err := WrapDEK(mk, "conn-42", dek)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapDEK(mk, "conn-42", w)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("DEK roundtrip mismatch")
	}
}

func TestDEK_DifferentConnIDFails(t *testing.T) {
	mk := freshMK(t)
	dek, _ := GenerateDEK()
	w, _ := WrapDEK(mk, "conn-1", dek)
	if _, err := UnwrapDEK(mk, "conn-2", w); err == nil {
		t.Fatal("expected unwrap to fail when connID changes")
	}
}

func TestDEK_DifferentMKFails(t *testing.T) {
	mk1 := freshMK(t)
	mk2 := freshMK(t)
	dek, _ := GenerateDEK()
	w, _ := WrapDEK(mk1, "conn", dek)
	if _, err := UnwrapDEK(mk2, "conn", w); err == nil {
		t.Fatal("expected unwrap to fail with different master key")
	}
}

func TestDEK_RejectsBadDEKSize(t *testing.T) {
	mk := freshMK(t)
	if _, err := WrapDEK(mk, "c", []byte("short")); err == nil {
		t.Fatal("expected error for wrong DEK size")
	}
}

func TestEncryptField_RoundTrip(t *testing.T) {
	dek, _ := GenerateDEK()
	pt := []byte("super-secret-credential")
	ef, err := EncryptField(dek, "conn-1", pt)
	if err != nil {
		t.Fatal(err)
	}
	if ef.Version != EnvelopeVersion {
		t.Fatalf("version mismatch")
	}
	got, err := DecryptField(dek, "conn-1", ef)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatal("field roundtrip mismatch")
	}
}

func TestEncryptField_DifferentConnIDFails(t *testing.T) {
	dek, _ := GenerateDEK()
	ef, _ := EncryptField(dek, "conn-1", []byte("data"))
	if _, err := DecryptField(dek, "conn-2", ef); err == nil {
		t.Fatal("expected failure with different connID")
	}
}

func TestEncryptField_DifferentDEKFails(t *testing.T) {
	dek1, _ := GenerateDEK()
	dek2, _ := GenerateDEK()
	ef, _ := EncryptField(dek1, "c", []byte("data"))
	if _, err := DecryptField(dek2, "c", ef); err == nil {
		t.Fatal("expected failure with different DEK")
	}
}

func TestEncryptField_UnsupportedVersion(t *testing.T) {
	dek, _ := GenerateDEK()
	ef, _ := EncryptField(dek, "c", []byte("data"))
	ef.Version = 999
	if _, err := DecryptField(dek, "c", ef); err == nil {
		t.Fatal("expected unsupported-version error")
	}
}

func TestEncryptField_RejectsBadDEKSize(t *testing.T) {
	if _, err := EncryptField([]byte("short"), "c", []byte("x")); err == nil {
		t.Fatal("expected error for wrong DEK size")
	}
}

func TestEncryptField_FreshNonces(t *testing.T) {
	dek, _ := GenerateDEK()
	a, _ := EncryptField(dek, "c", []byte("data"))
	b, _ := EncryptField(dek, "c", []byte("data"))
	if bytes.Equal(a.Nonce, b.Nonce) {
		t.Fatal("two EncryptField calls produced identical nonces; nonce reuse")
	}
	if bytes.Equal(a.Ciphertext, b.Ciphertext) {
		t.Fatal("two EncryptField calls produced identical ciphertext; nonce reuse")
	}
}
