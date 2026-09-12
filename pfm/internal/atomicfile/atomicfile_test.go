package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteReplacesWholeWithTheModeAndLeavesNoScratch pins the writer's
// contract: the new content lands whole with exactly the asked permission bits
// (not the process umask, not the old file's), a missing parent is created, and
// nothing but the target remains in the directory.
func TestWriteReplacesWholeWithTheModeAndLeavesNoScratch(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "nested")
	path := filepath.Join(directory, "config.json")
	if err := Write(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("second\n"), 0o600|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "second\n" {
		t.Fatalf("content = %q, %v; want the second write whole", content, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode() != 0o600 {
		t.Fatalf("mode = %v, %v; want exactly 0600 — only the permission bits", info.Mode(), err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory holds %v, %v; want the target alone", entries, err)
	}
}

// TestWriteFailureLeavesTheOldFileAndNoScratch pins the failure half: a write
// that cannot publish — here the target is a directory the rename cannot
// replace — reports which path failed, keeps what was there, and removes its
// scratch file.
func TestWriteFailureLeavesTheOldFileAndNoScratch(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "occupied")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep"), []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Write(target, []byte("new"), 0o600)
	if err == nil || !strings.Contains(err.Error(), target) {
		t.Fatalf("Write over a directory = %v, want an error naming %s", err, target)
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil || len(entries) != 1 || entries[0].Name() != "occupied" {
		t.Fatalf("directory holds %v, %v; want only the untouched target", entries, readErr)
	}
	if kept, _ := os.ReadFile(filepath.Join(target, "keep")); string(kept) != "kept" {
		t.Fatalf("the old content was disturbed: %q", kept)
	}
}
