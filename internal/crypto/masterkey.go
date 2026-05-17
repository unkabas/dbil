package crypto

import (
	"context"
	"errors"
)

// MasterKey is the 32-byte root key from which every other key in the system
// is derived (DEK wrap, audit DEK, SQLite encryption if added later, etc.).
type MasterKey []byte

// ErrLoaderUnavailable signals that a loader is not configured or is not
// available in this environment (file missing, env var unset, KMS not yet
// implemented, etc.). The loader Chain treats this as "skip and try the next
// loader" rather than as a hard failure.
var ErrLoaderUnavailable = errors.New("loader unavailable")

// Source identifies which loader produced the master key. It is logged at
// startup so an operator can verify the key came from where they expect.
type Source string

const (
	SourceKMS      Source = "kms"
	SourceKeychain Source = "keychain"
	SourceFile     Source = "file"
	SourceEnv      Source = "env"
	SourceTTY      Source = "tty"
	SourceAuto     Source = "auto"
)

// NewMasterKey wraps b after validating its length (must equal KeySize).
func NewMasterKey(b []byte) (MasterKey, error) {
	if len(b) != KeySize {
		return nil, errors.New("crypto: master key must be exactly 32 bytes")
	}
	out := make([]byte, KeySize)
	copy(out, b)
	return MasterKey(out), nil
}

// Wipe overwrites the master key bytes with zeros (best effort).
func (m MasterKey) Wipe() {
	Zero(m)
}

// Loader produces a master key. The Chain runs Loaders in order; returning
// ErrLoaderUnavailable causes the Chain to continue to the next Loader, while
// any other error is treated as a hard, fail-fast failure.
type Loader interface {
	Load(ctx context.Context) (MasterKey, Source, error)
}
