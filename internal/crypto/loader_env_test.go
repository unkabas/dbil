package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
)

func TestEnvLoader_Unset(t *testing.T) {
	t.Setenv("DBIL_MK_TEST_UNSET", "")
	_, _, err := NewEnvLoader("DBIL_MK_TEST_UNSET").Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestEnvLoader_EmptyName(t *testing.T) {
	_, _, err := NewEnvLoader("").Load(context.Background())
	if !errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want ErrLoaderUnavailable, got %v", err)
	}
}

func TestEnvLoader_RawURL(t *testing.T) {
	want := bytes.Repeat([]byte{0x77}, KeySize)
	t.Setenv("DBIL_MK_TEST_OK", base64.RawURLEncoding.EncodeToString(want))
	mk, src, err := NewEnvLoader("DBIL_MK_TEST_OK").Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if src != SourceEnv {
		t.Fatalf("want source env, got %s", src)
	}
	if !bytes.Equal(mk, want) {
		t.Fatal("mismatch")
	}
}

func TestEnvLoader_PaddedBase64(t *testing.T) {
	want := bytes.Repeat([]byte{0x88}, KeySize)
	t.Setenv("DBIL_MK_TEST_PAD", base64.StdEncoding.EncodeToString(want))
	mk, _, err := NewEnvLoader("DBIL_MK_TEST_PAD").Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mk, want) {
		t.Fatal("mismatch")
	}
}

func TestEnvLoader_BadBase64(t *testing.T) {
	t.Setenv("DBIL_MK_TEST_BAD", "!!! not base64 !!!")
	_, _, err := NewEnvLoader("DBIL_MK_TEST_BAD").Load(context.Background())
	if err == nil || errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want hard error, got %v", err)
	}
}

func TestEnvLoader_WrongDecodedLength(t *testing.T) {
	t.Setenv("DBIL_MK_TEST_LEN", base64.RawURLEncoding.EncodeToString([]byte("not-thirty-two-bytes")))
	_, _, err := NewEnvLoader("DBIL_MK_TEST_LEN").Load(context.Background())
	if err == nil || errors.Is(err, ErrLoaderUnavailable) {
		t.Fatalf("want hard error, got %v", err)
	}
}
