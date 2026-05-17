// Package crypto provides cryptographic primitives for DBil:
//   - secure random byte generation
//   - byte-slice zeroing
//   - HKDF-SHA256 and Argon2id key derivation
//   - AES-256-GCM authenticated encryption with associated data
//   - master key types and loader chain (file/env/auto/stubs)
//   - envelope encryption (DEK wrap, field encrypt) and passphrase wrapping
//
// All primitives are kept free of higher-level domain types so they can be
// audited in isolation.
package crypto

import "crypto/rand"

// Random returns n cryptographically random bytes from crypto/rand.
// For n == 0 it returns an empty (non-nil) slice with a nil error.
func Random(n int) ([]byte, error) {
	if n == 0 {
		return []byte{}, nil
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
