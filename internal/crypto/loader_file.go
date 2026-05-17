package crypto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// FileLoader reads the master key from a file at Path.
type FileLoader struct {
	Path string
}

// NewFileLoader returns a loader that reads its key from path. An empty path
// makes the loader return ErrLoaderUnavailable so it can be conditionally
// included in a Chain without branching at the call site.
func NewFileLoader(path string) *FileLoader {
	return &FileLoader{Path: path}
}

// Load reads up to 64 bytes from Path and returns the first KeySize bytes as
// the master key. Returns ErrLoaderUnavailable when the path is empty or the
// file does not exist; any other read or length problem is a hard error.
func (l *FileLoader) Load(_ context.Context) (MasterKey, Source, error) {
	if l.Path == "" {
		return nil, "", fmt.Errorf("file loader: empty path: %w", ErrLoaderUnavailable)
	}
	f, err := os.Open(l.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("file loader: %s missing: %w", l.Path, ErrLoaderUnavailable)
		}
		return nil, "", fmt.Errorf("file loader: open %s: %w", l.Path, err)
	}
	defer f.Close()

	buf := make([]byte, 64)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, "", fmt.Errorf("file loader: read %s: %w", l.Path, err)
	}
	if n < KeySize {
		return nil, "", fmt.Errorf("file loader: %s shorter than %d bytes", l.Path, KeySize)
	}
	out := make([]byte, KeySize)
	copy(out, buf[:KeySize])
	return MasterKey(out), SourceFile, nil
}
