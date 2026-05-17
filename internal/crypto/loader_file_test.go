package crypto

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileLoader_Missing(t *testing.T) {
	l := NewFileLoader(filepath.Join(t.TempDir(), "absent"))
	_, _, err := l.Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestFileLoader_EmptyPath(t *testing.T) {
	l := NewFileLoader("")
	_, _, err := l.Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestFileLoader_Exact32Bytes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mk")
	want := bytes.Repeat([]byte{0x55}, KeySize)
	if err := os.WriteFile(p, want, 0o400); err != nil {
		t.Fatal(err)
	}
	mk, src, err := NewFileLoader(p).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceFile {
		t.Fatalf("want source %s, got %s", SourceFile, src)
	}
	if !bytes.Equal(mk, want) {
		t.Fatal("bytes mismatch")
	}
}

func TestFileLoader_LongerOK(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mk")
	want := bytes.Repeat([]byte{0x66}, KeySize)
	if err := os.WriteFile(p, append(want, []byte("\n")...), 0o400); err != nil {
		t.Fatal(err)
	}
	mk, _, err := NewFileLoader(p).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mk, want) {
		t.Fatal("only the first KeySize bytes should be returned")
	}
}

func TestFileLoader_ShortIsHardError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mk")
	if err := os.WriteFile(p, []byte("short"), 0o400); err != nil {
		t.Fatal(err)
	}
	_, _, err := NewFileLoader(p).Load(context.Background())
	if err == nil || errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want hard error (not ErrLoaderUnavailable), got %v", err)
	}
}
