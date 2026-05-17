package crypto

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
)

// EnvLoader reads a base64url-encoded 32-byte master key from the named
// environment variable. Emits a slog WARN noting that environment variables
// are not the recommended production source.
type EnvLoader struct {
	VarName string
}

// NewEnvLoader returns a loader that reads its key from varName. An empty
// name makes the loader return ErrLoaderUnavailable.
func NewEnvLoader(varName string) *EnvLoader {
	return &EnvLoader{VarName: varName}
}

// Load reads, decodes, and validates the env var. Decoding accepts both
// RawURL (no padding) and standard (padded) base64 to be forgiving.
func (l *EnvLoader) Load(_ context.Context) (MasterKey, Source, error) {
	if l.VarName == "" {
		return nil, "", fmt.Errorf("env loader: empty variable name: %w", ErrLoaderUnavailable)
	}
	raw := os.Getenv(l.VarName)
	if raw == "" {
		return nil, "", fmt.Errorf("env loader: %s is unset: %w", l.VarName, ErrLoaderUnavailable)
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		// Fall back to standard padded base64
		b, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "", fmt.Errorf("env loader: %s is not valid base64: %w", l.VarName, err)
		}
	}
	if len(b) != KeySize {
		return nil, "", fmt.Errorf("env loader: %s decoded to %d bytes; need %d", l.VarName, len(b), KeySize)
	}
	slog.Warn("master key loaded from environment variable; this is dev-only — use KMS or a Docker secret for production",
		"source", string(SourceEnv), "var", l.VarName)
	out := make([]byte, KeySize)
	copy(out, b)
	return MasterKey(out), SourceEnv, nil
}
