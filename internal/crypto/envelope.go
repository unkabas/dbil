package crypto

import (
	"fmt"
	"strconv"
)

// EnvelopeVersion is the on-disk format version for encrypted fields. It is
// stored alongside each ciphertext so future format evolution remains
// decryptable.
const EnvelopeVersion uint32 = 1

// infoDEKWrap is the HKDF info string that derives the DEK-wrap key from the
// master key. Changing it would break every wrapped DEK, so treat it as a
// protocol constant.
const infoDEKWrap = "dbil:dek-wrap-v1"

// WrappedDEK is a data encryption key encrypted under a master-key-derived
// wrap key.
type WrappedDEK struct {
	Nonce      []byte
	Ciphertext []byte
}

// EncryptedField is a piece of ciphertext encrypted under a DEK or
// passphrase-derived key, with an explicit format version.
type EncryptedField struct {
	Nonce      []byte
	Ciphertext []byte
	Version    uint32
}

// GenerateDEK returns a fresh 32-byte data encryption key.
func GenerateDEK() ([]byte, error) {
	return Random(KeySize)
}

// WrapDEK encrypts dek under a key derived from mk, binding the ciphertext
// to connID via AAD. Returns a WrappedDEK that can be persisted.
func WrapDEK(mk MasterKey, connID string, dek []byte) (WrappedDEK, error) {
	if len(dek) != KeySize {
		return WrappedDEK{}, fmt.Errorf("crypto: dek must be %d bytes", KeySize)
	}
	wk, err := HKDF(mk, infoDEKWrap, KeySize)
	if err != nil {
		return WrappedDEK{}, err
	}
	defer Zero(wk)
	nonce, ct, err := Encrypt(wk, dek, []byte("conn:"+connID))
	if err != nil {
		return WrappedDEK{}, err
	}
	return WrappedDEK{Nonce: nonce, Ciphertext: ct}, nil
}

// UnwrapDEK reverses WrapDEK. Fails if the master key, connID, or stored
// nonce / ciphertext have changed.
func UnwrapDEK(mk MasterKey, connID string, w WrappedDEK) ([]byte, error) {
	wk, err := HKDF(mk, infoDEKWrap, KeySize)
	if err != nil {
		return nil, err
	}
	defer Zero(wk)
	return Decrypt(wk, w.Nonce, w.Ciphertext, []byte("conn:"+connID))
}

// EncryptField encrypts plaintext under dek with AAD that binds the
// ciphertext to connID and the current EnvelopeVersion.
func EncryptField(dek []byte, connID string, plaintext []byte) (EncryptedField, error) {
	if len(dek) != KeySize {
		return EncryptedField{}, fmt.Errorf("crypto: dek must be %d bytes", KeySize)
	}
	aad := fieldAAD(connID, EnvelopeVersion)
	nonce, ct, err := Encrypt(dek, plaintext, aad)
	if err != nil {
		return EncryptedField{}, err
	}
	return EncryptedField{Nonce: nonce, Ciphertext: ct, Version: EnvelopeVersion}, nil
}

// DecryptField reverses EncryptField. It rejects ciphertexts whose version
// is not supported by this build so a future operator notices a downgrade
// attempt instead of getting random plaintext.
func DecryptField(dek []byte, connID string, ef EncryptedField) ([]byte, error) {
	if ef.Version != EnvelopeVersion {
		return nil, fmt.Errorf("crypto: unsupported envelope version %d", ef.Version)
	}
	return Decrypt(dek, ef.Nonce, ef.Ciphertext, fieldAAD(connID, ef.Version))
}

func fieldAAD(connID string, version uint32) []byte {
	return []byte("creds:" + connID + ":" + strconv.FormatUint(uint64(version), 10))
}
