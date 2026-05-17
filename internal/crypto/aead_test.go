package crypto

import (
	"bytes"
	"testing"
)

func freshKey(t *testing.T) []byte {
	t.Helper()
	k, err := Random(KeySize)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestAEAD_RoundTrip(t *testing.T) {
	key := freshKey(t)
	plaintext := []byte("hello, dbil")
	aad := []byte("conn:42")
	nonce, ct, err := Encrypt(key, plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}
	if len(nonce) != NonceSize {
		t.Fatalf("nonce wrong length: %d", len(nonce))
	}
	got, err := Decrypt(key, nonce, ct, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("roundtrip mismatch: got %q want %q", got, plaintext)
	}
}

func TestAEAD_AADMismatch(t *testing.T) {
	key := freshKey(t)
	nonce, ct, err := Encrypt(key, []byte("data"), []byte("conn:a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(key, nonce, ct, []byte("conn:b")); err == nil {
		t.Fatal("expected authentication failure on AAD mismatch")
	}
}

func TestAEAD_WrongKey(t *testing.T) {
	k1 := freshKey(t)
	k2 := freshKey(t)
	nonce, ct, _ := Encrypt(k1, []byte("data"), nil)
	if _, err := Decrypt(k2, nonce, ct, nil); err == nil {
		t.Fatal("expected authentication failure with wrong key")
	}
}

func TestAEAD_BadNonceLength(t *testing.T) {
	key := freshKey(t)
	if _, err := Decrypt(key, []byte{1, 2}, []byte("x"), nil); err == nil {
		t.Fatal("expected error on short nonce")
	}
}

func TestAEAD_BadKeyLength(t *testing.T) {
	if _, _, err := Encrypt([]byte("short"), nil, nil); err == nil {
		t.Fatal("expected error on short key in Encrypt")
	}
	if _, err := Decrypt([]byte("short"), make([]byte, NonceSize), []byte("x"), nil); err == nil {
		t.Fatal("expected error on short key in Decrypt")
	}
}

func TestAEAD_FreshNonce(t *testing.T) {
	key := freshKey(t)
	n1, _, _ := Encrypt(key, []byte("a"), nil)
	n2, _, _ := Encrypt(key, []byte("a"), nil)
	if bytes.Equal(n1, n2) {
		t.Fatal("two Encrypt calls produced identical nonces; rng broken or nonce reused")
	}
}

func TestAEAD_TamperedCiphertext(t *testing.T) {
	key := freshKey(t)
	nonce, ct, _ := Encrypt(key, []byte("important"), []byte("aad"))
	if len(ct) == 0 {
		t.Fatal("ciphertext empty")
	}
	ct[0] ^= 0x01
	if _, err := Decrypt(key, nonce, ct, []byte("aad")); err == nil {
		t.Fatal("expected authentication failure on flipped ciphertext bit")
	}
}
