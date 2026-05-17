package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// NonceSize is the AES-GCM nonce size in bytes used by every DBil envelope
// and field encryption.
const NonceSize = 12

// KeySize is the AES key size used by DBil (AES-256).
const KeySize = 32

// Encrypt seals plaintext with AES-256-GCM under key, generating a fresh
// random NonceSize-byte nonce. The aad parameter is bound into the GCM tag.
//
// The nonce is returned separately from the ciphertext so that callers can
// store them in distinct columns or fields. Ciphertext already contains the
// GCM authentication tag suffix.
func Encrypt(key, plaintext, aad []byte) (nonce, ciphertext []byte, err error) {
	if len(key) != KeySize {
		return nil, nil, errors.New("crypto: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = Random(NonceSize)
	if err != nil {
		return nil, nil, err
	}
	ciphertext = aead.Seal(nil, nonce, plaintext, aad)
	return nonce, ciphertext, nil
}

// Decrypt verifies and decrypts ciphertext under key, using nonce and aad
// that must match exactly the values used at encryption time.
func Decrypt(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, errors.New("crypto: key must be 32 bytes")
	}
	if len(nonce) != NonceSize {
		return nil, errors.New("crypto: nonce must be 12 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}
