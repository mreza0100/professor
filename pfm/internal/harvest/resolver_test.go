package harvest

import (
	"context"
	"net/http"
	"testing"
)

func TestLegacyUnknownOASourceKeepsWorstDefaultPriority(t *testing.T) {
	if got := candidatePriority("future-provider", "", "", "pdf"); got != 50 {
		t.Fatalf("unknown provider priority=%d, want legacy default 50", got)
	}
}

// TestProviderContactComesFromOptionsNotEnvironment pins the config-only
// contract: the scholarly identity is Options.ContactEmail (fed from
// harvester.config.json), and the retired HARVESTER_CONTACT_EMAIL is ignored.
func TestProviderContactComesFromOptionsNotEnvironment(t *testing.T) {
	t.Setenv("HARVESTER_CONTACT_EMAIL", "env@example.test")
	var seen *http.Request
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Clone(r.Context())
		return jsonResponse(r, `{"records":[{"pmcid":"PMC1234567"}]}`), nil
	})}
	h := mustNew(t, Options{CacheDir: t.TempDir(), OA: client, ContactEmail: "config@example.test"})
	_, err := h.resolver().ResolvePMID(context.Background(), "1234567")
	if err != nil {
		t.Fatal(err)
	}
	if seen == nil {
		t.Fatal("resolver made no idconv request")
	}
	if got := seen.URL.Query().Get("email"); got != "config@example.test" {
		t.Fatalf("contact query = %q, want config@example.test", got)
	}
	if got := seen.Header.Get("User-Agent"); got != "harvester-mcp/1.0 (mailto:config@example.test)" {
		t.Fatalf("scholarly UA = %q", got)
	}
}
