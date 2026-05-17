package crypto

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"context"
)

// AutoLoader generates a master key on first use and persists it to Path
// (mode 0400, atomic rename). Subsequent loads read the persisted file.
// Intended as the last-resort fallback for solo / dev runs.
type AutoLoader struct {
	Path string
}

// NewAutoLoader returns a loader that persists the auto-generated key at path.
func NewAutoLoader(path string) *AutoLoader {
	return &AutoLoader{Path: path}
}

// Load returns the persisted master key if Path exists, otherwise generates
// a fresh 32-byte key, persists it atomically with mode 0400, and emits a
// prominent slog WARN.
func (l *AutoLoader) Load(_ context.Context) (MasterKey, Source, error) {
	if l.Path == "" {
		return nil, "", fmt.Errorf("auto loader: empty path: %w", ErrLoaderUnavailable)
	}

	data, err := os.ReadFile(l.Path) //nolint:gosec // intentional: caller supplies trusted path
	switch {
	case err == nil:
		if len(data) < KeySize {
			return nil, "", fmt.Errorf("auto loader: existing %s has %d bytes; want at least %d", l.Path, len(data), KeySize)
		}
		out := make([]byte, KeySize)
		copy(out, data[:KeySize])
		return MasterKey(out), SourceAuto, nil
	case errors.Is(err, fs.ErrNotExist):
		// fall through to generation
	default:
		return nil, "", fmt.Errorf("auto loader: read %s: %w", l.Path, err)
	}

	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return nil, "", fmt.Errorf("auto loader: mkdir %s: %w", filepath.Dir(l.Path), err)
	}
	mk, err := Random(KeySize)
	if err != nil {
		return nil, "", fmt.Errorf("auto loader: generate: %w", err)
	}
	if err := writeFileAtomic(l.Path, mk, 0o400); err != nil {
		return nil, "", fmt.Errorf("auto loader: persist %s: %w", l.Path, err)
	}
	slog.Warn("master key auto-generated and persisted; this is dev-only — configure DBIL_MASTER_KEY_FILE or KMS for production",
		"source", string(SourceAuto), "path", l.Path)
	out := make([]byte, KeySize)
	copy(out, mk)
	return MasterKey(out), SourceAuto, nil
}

// writeFileAtomic writes data to path via a temp file + rename so a reader
// cannot observe a partially-written key.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dbil-mk-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	keep = true
	return nil
}
