package sqlitedb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func pragma(t *testing.T, database *sql.DB, name string) string {
	t.Helper()
	var value string
	if err := database.QueryRowContext(context.Background(), "PRAGMA "+name).Scan(&value); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return value
}

// TestOpenStoreAppliesTheStorePragmaSet pins the one pragma set every pfm
// store runs on: its directory created, WAL, synchronous=NORMAL (1), foreign
// keys on, and the store busy timeout.
func TestOpenStoreAppliesTheStorePragmaSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "store.db")
	database, err := OpenStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for name, want := range map[string]string{
		"journal_mode": "wal", "synchronous": "1", "foreign_keys": "1", "busy_timeout": "10000",
	} {
		if got := pragma(t, database, name); got != want {
			t.Errorf("PRAGMA %s = %q, want %q", name, got, want)
		}
	}
}

// TestForeignOpenersKeepTheOwnersSettings pins the other program's store:
// read-only refuses a write, read-write can write, both carry the caller's
// busy timeout, and neither switches the owner's rollback journal to WAL.
func TestForeignOpenersKeepTheOwnersSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner.db")
	owner, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec("CREATE TABLE threads (id TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	readOnly, err := OpenReadOnly(path, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	if _, err := readOnly.Exec("INSERT INTO threads VALUES ('a')"); err == nil {
		t.Fatal("a read-only handle accepted a write")
	}
	if got := pragma(t, readOnly, "busy_timeout"); got != "2000" {
		t.Fatalf("read-only busy_timeout = %q, want 2000", got)
	}
	readWrite, err := OpenReadWrite(path, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer readWrite.Close()
	if _, err := readWrite.Exec("INSERT INTO threads VALUES ('a')"); err != nil {
		t.Fatalf("read-write insert: %v", err)
	}
	if got := pragma(t, readWrite, "busy_timeout"); got != "5000" {
		t.Fatalf("read-write busy_timeout = %q, want 5000", got)
	}
	if got := pragma(t, readWrite, "journal_mode"); got != "delete" {
		t.Fatalf("owner's journal_mode = %q, want its own rollback journal kept", got)
	}
}
