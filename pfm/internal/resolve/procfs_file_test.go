package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileProcFSStatRejectsTruncatedRecordWithoutPanicking(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "4242")
	if err := os.MkdirAll(procDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "stat"), []byte("4242 (fixture) S\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := (fileProcFS{root: root}).Stat(4242)
	if err == nil {
		t.Fatal("truncated proc stat unexpectedly parsed successfully")
	}
	if !strings.Contains(err.Error(), "expected parent field") {
		t.Fatalf("error=%q, want named malformed-stat error", err)
	}
}

func TestFileProcFSStatReadsParentAfterCommandName(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "4242")
	if err := os.MkdirAll(procDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The command name may contain a closing parenthesis; the final ") " is
	// the kernel delimiter and the fields after it begin with state and ppid.
	stat := "4242 (worker ) helper) S 41 4242 4242 0 -1 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
	if err := os.WriteFile(filepath.Join(procDir, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := (fileProcFS{root: root}).Stat(4242)
	if err != nil {
		t.Fatalf("Stat() error=%v", err)
	}
	if got.ParentPID != 41 {
		t.Fatalf("ParentPID=%d, want 41", got.ParentPID)
	}
}
