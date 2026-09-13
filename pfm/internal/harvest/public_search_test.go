package harvest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchCachePublicReportsUnreadablePrivateEntries(t *testing.T) {
	setHarvestTestJail(t)
	cacheDir := t.TempDir()
	h := mustNew(t, Options{CacheDir: cacheDir})
	privatePath := filepath.Join(cacheDir, "html", "000-broken.md")
	if err := os.MkdirAll(filepath.Dir(privatePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cacheDir, "html", "never-created.md"), privatePath); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SearchCachePublic("needle", 1, false); err == nil {
		t.Fatal("unreadable private cache entry was reported as an empty search")
	}
}
