package crypto

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
)

type stubLoader struct {
	mk  MasterKey
	src Source
	err error
}

func (s stubLoader) Load(_ context.Context) (MasterKey, Source, error) {
	return s.mk, s.src, s.err
}

func unavailable(name string) stubLoader {
	return stubLoader{err: fmt.Errorf("%s: %w", name, ErrLoaderUnavailable)}
}

func TestChain_FirstSuccessWins(t *testing.T) {
	want := bytes.Repeat([]byte{0xaa}, KeySize)
	c := NewChain(
		unavailable("kms"),
		stubLoader{mk: want, src: SourceFile},
		unavailable("env"),
	)
	mk, src, err := c.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceFile {
		t.Fatalf("want %s, got %s", SourceFile, src)
	}
	if !bytes.Equal(mk, want) {
		t.Fatal("wrong key bytes")
	}
}

func TestChain_AllUnavailable(t *testing.T) {
	c := NewChain(unavailable("a"), unavailable("b"))
	_, _, err := c.Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestChain_FailFastOnRealError(t *testing.T) {
	boom := errors.New("boom")
	never := bytes.Repeat([]byte{0xee}, KeySize)
	c := NewChain(
		unavailable("a"),
		stubLoader{err: boom},
		stubLoader{mk: never, src: SourceFile}, // must not be reached
	)
	_, _, err := c.Load(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}

func TestChain_Empty(t *testing.T) {
	c := NewChain()
	_, _, err := c.Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}
