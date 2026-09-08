package deps

import (
	"os"
	"path/filepath"
	"testing"
)

// resetResolveCache clears the memoized resolution table before and after a
// test so PATH fixtures from other tests in this package (or a previous run
// of this one) cannot leak a cached path across t.Setenv("PATH", ...) calls.
func resetResolveCache(t *testing.T) {
	t.Helper()
	resolveCacheMu.Lock()
	previous := resolveCache
	resolveCache = map[string]string{}
	resolveCacheMu.Unlock()
	t.Cleanup(func() {
		resolveCacheMu.Lock()
		resolveCache = previous
		resolveCacheMu.Unlock()
	})
}

func writeExecutable(t *testing.T, directory, name string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fixture executable %s: %v", name, err)
	}
	return path
}

// TestResolveMemoizesASuccessfulLookupPerName pins the cache population this
// change adds: a resolved path is remembered under (name, $PATH) so a second
// call needs neither a fresh Registry() build nor a fresh PATH walk. Every
// tmux capture-pane and every spawned `codex app-server` crosses
// deps.Executable, and paying for both on EVERY exec is what a parked
// Limits/Codex-idle poll was doing every 2s (2026-09-08 measurement,
// devbox).
func TestResolveMemoizesASuccessfulLookupPerName(t *testing.T) {
	resetResolveCache(t)
	directory := t.TempDir()
	want := writeExecutable(t, directory, "probe-fixture")
	t.Setenv("PATH", directory)

	got, err := Resolve("probe-fixture")
	if err != nil || got != want {
		t.Fatalf("Resolve() = %q, %v; want %q, nil", got, err, want)
	}

	resolveCacheMu.Lock()
	cached, ok := resolveCache[resolveCacheKey("probe-fixture")]
	resolveCacheMu.Unlock()
	if !ok || cached != want {
		t.Fatalf("resolveCache after Resolve() = %q, %v; want %q cached", cached, ok, want)
	}

	// A second call under the identical PATH must return the exact same
	// path without needing the fixture to still be the only thing on PATH
	// — it hits the memoized entry via the cache, not a fresh walk.
	got, err = Resolve("probe-fixture")
	if err != nil || got != want {
		t.Fatalf("second Resolve() = %q, %v; want %q, nil", got, err, want)
	}

	if got := Executable("probe-fixture"); got != want {
		t.Fatalf("Executable() = %q, want %q", got, want)
	}
}

// TestResolveCacheInvalidatesWhenTheBinaryDisappears pins the safety half of
// the memoization: a cached path must never survive its target vanishing —
// the fixture in the parked-poll comment (2026-09-08) is a Codex binary
// disappearing mid-session, and a stale hit there would silently misreport
// dependency health.
func TestResolveCacheInvalidatesWhenTheBinaryDisappears(t *testing.T) {
	resetResolveCache(t)
	directory := t.TempDir()
	path := writeExecutable(t, directory, "probe-vanishing")
	t.Setenv("PATH", directory)

	if got, err := Resolve("probe-vanishing"); err != nil || got != path {
		t.Fatalf("Resolve() = %q, %v; want %q, nil", got, err, path)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove fixture executable: %v", err)
	}

	if _, err := Resolve("probe-vanishing"); err == nil {
		t.Fatalf("Resolve() after the binary disappeared = nil error, want a lookup failure")
	}

	resolveCacheMu.Lock()
	_, stillCached := resolveCache[resolveCacheKey("probe-vanishing")]
	resolveCacheMu.Unlock()
	if stillCached {
		t.Fatalf("resolveCache still holds an entry for a binary that no longer exists")
	}
}

// TestResolveCacheIsScopedByPATH proves the cache key includes $PATH, not
// just the binary name: a name that resolves to a different file once PATH
// changes must never return the earlier PATH's answer.
func TestResolveCacheIsScopedByPATH(t *testing.T) {
	resetResolveCache(t)
	first := t.TempDir()
	second := t.TempDir()
	wantFirst := writeExecutable(t, first, "probe-scoped")
	wantSecond := writeExecutable(t, second, "probe-scoped")

	t.Setenv("PATH", first)
	if got, err := Resolve("probe-scoped"); err != nil || got != wantFirst {
		t.Fatalf("Resolve() under first PATH = %q, %v; want %q, nil", got, err, wantFirst)
	}

	t.Setenv("PATH", second)
	if got, err := Resolve("probe-scoped"); err != nil || got != wantSecond {
		t.Fatalf("Resolve() under second PATH = %q, %v; want %q, nil", got, err, wantSecond)
	}
}
