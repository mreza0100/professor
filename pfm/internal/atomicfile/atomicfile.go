// Package atomicfile is the one atomic file writer: every file pfm replaces
// whole — config, caches, crumbs, receipts, staged assets — goes through Write,
// so a reader sees the old content or the new, never a torn write, and a
// failure leaves the old file and no scratch behind.
package atomicfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Write replaces path with content in one rename: a scratch file beside path
// (".<name>.tmp-*", so the rename never crosses a filesystem) is written, given
// mode's permission bits, synced and closed, then renamed over path. A missing
// parent directory is created 0o700; a caller that wants a wider directory
// creates it first.
func Write(path string, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("write %s: create directory: %w", path, err)
	}
	scratch, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("write %s: create scratch: %w", path, err)
	}
	scratchPath := scratch.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(scratchPath)
		}
	}()
	if err := scratch.Chmod(mode.Perm()); err != nil {
		_ = scratch.Close()
		return fmt.Errorf("write %s: set mode: %w", path, err)
	}
	if _, err := scratch.Write(content); err != nil {
		_ = scratch.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := scratch.Sync(); err != nil {
		_ = scratch.Close()
		return fmt.Errorf("write %s: sync: %w", path, err)
	}
	if err := scratch.Close(); err != nil {
		return fmt.Errorf("write %s: close scratch: %w", path, err)
	}
	if err := os.Rename(scratchPath, path); err != nil {
		return fmt.Errorf("write %s: replace: %w", path, err)
	}
	published = true
	return nil
}
