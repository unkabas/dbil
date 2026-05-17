package crypto

import (
	"errors"
	"fmt"
)

// MinPassphraseLen is the smallest accepted passphrase length. Production
// connections require a passphrase; the threshold here is the minimum at
// the crypto layer (the UI may enforce stricter rules).
const MinPassphraseLen = 8

// DerivePassphraseKey derives a 32-byte symmetric key from passphrase and
// a stored per-connection salt using Argon2id with the standard DBil params.
func DerivePassphraseKey(passphrase string, salt []byte) ([]byte, error) {
	if len(passphrase) < MinPassphraseLen {
		return nil, fmt.Errorf("crypto: passphrase must be at least %d characters", MinPassphraseLen)
	}
	if len(salt) < SaltSize {
		return nil, errors.New("crypto: passphrase salt must be at least 16 bytes")
	}
	return Argon2id([]byte(passphrase), salt, KeySize)
}

// WrapWithPassphrase encrypts plaintext under a key derived from passphrase
// and the per-connection salt, binding the ciphertext to connID via AAD.
// The passphrase itself is never persisted; only the salt and the returned
// EncryptedField are stored.
func WrapWithPassphrase(passphrase string, salt []byte, connID string, plaintext []byte) (EncryptedField, error) {
	passKey, err := DerivePassphraseKey(passphrase, salt)
	if err != nil {
		return EncryptedField{}, err
	}
	defer Zero(passKey)
	nonce, ct, err := Encrypt(passKey, plaintext, []byte("pass:"+connID))
	if err != nil {
		return EncryptedField{}, err
	}
	return EncryptedField{Nonce: nonce, Ciphertext: ct, Version: EnvelopeVersion}, nil
}

// UnwrapWithPassphrase reverses WrapWithPassphrase.
func UnwrapWithPassphrase(passphrase string, salt []byte, connID string, ef EncryptedField) ([]byte, error) {
	if ef.Version != EnvelopeVersion {
		return nil, fmt.Errorf("crypto: unsupported envelope version %d", ef.Version)
	}
	passKey, err := DerivePassphraseKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	defer Zero(passKey)
	return Decrypt(passKey, ef.Nonce, ef.Ciphertext, []byte("pass:"+connID))
}
