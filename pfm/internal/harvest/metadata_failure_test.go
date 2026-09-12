package harvest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestMetadataOutageSurvivesMissingLibraryFallback(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	oa := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("metadata connection refused")
	})}
	var libraryLookups int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "libgen.test" || r.URL.Path != "/json.php" {
			t.Errorf("unexpected library request: %s", r.URL)
		}
		libraryLookups++
		return response(r, http.StatusOK, "application/json", "[]"), nil
	})}
	h := mustNew(t, Options{CacheDir: t.TempDir(), OA: oa, Client: client, Chrome: &http.Client{Transport: client.Transport}, LibGenURL: "https://libgen.test"})
	got := h.FetchPublic(context.Background(), providerFixtureDOI, FetchOptions{})
	if libraryLookups != 1 {
		t.Fatalf("library lookups = %d; want the configured fallback attempted once", libraryLookups)
	}
	if got.Error == "" || got.ErrorKind != "connect" || got.Path != "" || strings.Contains(strings.ToLower(got.Error), "not found") {
		t.Fatalf("metadata outage became an absence after a missing fallback: %#v", got)
	}
}
