package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHarvesterWriteAtomicKeepsItsContract pins the harvester writer that stays
// outside atomicfile: the file lands whole, private (0600), with its missing
// parent created, and no ".config.json.tmp-*" scratch left beside it.
func TestHarvesterWriteAtomicKeepsItsContract(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "harvester")
	path := filepath.Join(directory, "config.json")
	if err := writeAtomic(path, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "{}\n" {
		t.Fatalf("content = %q, %v; want the write whole", content, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v; want 0600", info.Mode(), err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory holds %v, %v; want the target alone", entries, err)
	}
}
