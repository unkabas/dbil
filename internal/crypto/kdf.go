package crypto

import (
	"crypto/hkdf"
	"crypto/sha256"
	"errors"

	"golang.org/x/crypto/argon2"
)

// SaltSize is the salt length required for Argon2id throughout DBil.
const SaltSize = 16

// Argon2id parameters (NIST-aligned, 2024 guidance).
const (
	argon2Time      uint32 = 3
	argon2MemoryKiB uint32 = 64 * 1024
	argon2Threads   uint8  = 4
)

// HKDF derives outLen bytes from secret using HKDF-SHA256 with the supplied
// info string. The salt is intentionally nil; callers manage per-purpose
// salts at higher layers (passphrase derivation uses a stored per-user salt).
func HKDF(secret []byte, info string, outLen int) ([]byte, error) {
	if outLen <= 0 {
		return nil, errors.New("crypto: HKDF outLen must be > 0")
	}
	return hkdf.Key(sha256.New, secret, nil, info, outLen)
}

// Argon2id derives a key of outLen bytes from password and salt using the
// standard DBil Argon2id parameters (time=3, memory=64MiB, threads=4).
// Salt must be at least SaltSize (16) bytes; longer salts are accepted.
func Argon2id(password, salt []byte, outLen uint32) ([]byte, error) {
	if len(salt) < SaltSize {
		return nil, errors.New("crypto: argon2id salt must be >= 16 bytes")
	}
	if outLen == 0 {
		return nil, errors.New("crypto: argon2id outLen must be > 0")
	}
	return argon2.IDKey(password, salt, argon2Time, argon2MemoryKiB, argon2Threads, outLen), nil
}

// NewSalt returns a fresh SaltSize-byte salt suitable for Argon2id.
func NewSalt() ([]byte, error) {
	return Random(SaltSize)
}
