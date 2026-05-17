package crypto

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoLoader_CreateThenReuse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "master.key")
	l := NewAutoLoader(p)

	mk1, src, err := l.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceAuto {
		t.Fatalf("want auto source, got %s", src)
	}
	if len(mk1) != KeySize {
		t.Fatalf("want %d bytes, got %d", KeySize, len(mk1))
	}

	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o400 {
		t.Fatalf("want mode 0400, got %#o", info.Mode().Perm())
	}

	// Second call must return the same bytes.
	mk2, _, err := l.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mk1, mk2) {
		t.Fatal("second call produced different bytes; expected reuse from disk")
	}
}

func TestAutoLoader_CreatesParentDir(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "nested", "deep", "master.key")
	if _, _, err := NewAutoLoader(p).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file at %s: %v", p, err)
	}
}

func TestAutoLoader_EmptyPath(t *testing.T) {
	_, _, err := NewAutoLoader("").Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestAutoLoader_ShortFileIsHardError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(p, []byte("short"), 0o400); err != nil {
		t.Fatal(err)
	}
	_, _, err := NewAutoLoader(p).Load(context.Background())
	if err == nil || errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want hard error, got %v", err)
	}
}
