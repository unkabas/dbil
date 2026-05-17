package store

import (
	"path/filepath"
	"testing"
)

func TestOpen_SimpleQuery(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)

	var n int
	if err := db.QueryRow("SELECT 1").Scan(&n); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1, got %d", n)
	}
}

func TestOpen_BadPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/that/should/not/exist/dbil.db")
	if err == nil {
		t.Fatal("expected open to fail when parent dir is missing")
	}
}
