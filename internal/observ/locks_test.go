package observ

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/unkabas/dbil/internal/postgres"
)

func TestTerminateBackend_Success(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT CASE": {Rows: [][]any{{true}}},
	}}
	ok, err := TerminateBackend(context.Background(), pool, 1234)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
	if !strings.Contains(pool.executed[0], "pg_terminate_backend(1234)") {
		t.Fatalf("sql shape: %s", pool.executed[0])
	}
	if !strings.Contains(pool.executed[0], "pg_backend_pid()") {
		t.Fatalf("self-protect missing: %s", pool.executed[0])
	}
}

func TestTerminateBackend_AlreadyGone(t *testing.T) {
	pool := &fakePool{results: map[string]*postgres.Result{
		"SELECT CASE": {Rows: [][]any{{false}}},
	}}
	ok, err := TerminateBackend(context.Background(), pool, 1234)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false")
	}
}

func TestTerminateBackend_RejectsBadPID(t *testing.T) {
	pool := &fakePool{}
	for _, pid := range []int{0, -1, -100} {
		_, err := TerminateBackend(context.Background(), pool, pid)
		if !errors.Is(err, ErrSelfTerminate) {
			t.Fatalf("pid=%d: want ErrSelfTerminate, got %v", pid, err)
		}
	}
}
