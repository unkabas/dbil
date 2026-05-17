package crypto

import (
	"context"
	"fmt"
)

// Stub loaders for sources that are designed-in but not yet implemented.
// They return ErrLoaderUnavailable so the Chain skips them.

// KMSLoader is a placeholder for AWS / GCP / Azure / Vault integration.
type KMSLoader struct{}

// NewKMSLoader returns the stub. Replace with a real implementation in a
// later plan.
func NewKMSLoader() *KMSLoader { return &KMSLoader{} }

// Load returns ErrLoaderUnavailable.
func (l *KMSLoader) Load(_ context.Context) (MasterKey, Source, error) {
	return nil, "", fmt.Errorf("kms loader not yet implemented: %w", ErrLoaderUnavailable)
}

// KeychainLoader is a placeholder for OS keychain integration (macOS, Linux
// libsecret, Windows DPAPI). Not usable from within a container.
type KeychainLoader struct{}

// NewKeychainLoader returns the stub.
func NewKeychainLoader() *KeychainLoader { return &KeychainLoader{} }

// Load returns ErrLoaderUnavailable.
func (l *KeychainLoader) Load(_ context.Context) (MasterKey, Source, error) {
	return nil, "", fmt.Errorf("os keychain loader not yet implemented: %w", ErrLoaderUnavailable)
}

// TTYLoader is a placeholder for interactive passphrase prompt at startup.
type TTYLoader struct{}

// NewTTYLoader returns the stub.
func NewTTYLoader() *TTYLoader { return &TTYLoader{} }

// Load returns ErrLoaderUnavailable.
func (l *TTYLoader) Load(_ context.Context) (MasterKey, Source, error) {
	return nil, "", fmt.Errorf("interactive tty loader not yet implemented: %w", ErrLoaderUnavailable)
}
