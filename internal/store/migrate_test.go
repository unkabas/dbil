package store

import (
	"path/filepath"
	"testing"
)

func TestApply_CreatesTables(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)

	if err := Apply(db); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name IN ('users','audit_log')
		ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if len(names) != 2 || names[0] != "audit_log" || names[1] != "users" {
		t.Fatalf("expected [audit_log users], got %v", names)
	}
}

func TestApply_Idempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	if err := Apply(db); err != nil {
		t.Fatalf("second Apply failed: %v", err)
	}
}
