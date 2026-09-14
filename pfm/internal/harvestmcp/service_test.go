package harvestmcp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/paths"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStableSixToolSurfaceAndFetchPrompt(t *testing.T) {
	service, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache"), SearXNGURL: "http://searxng.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := service.Server().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	want := []string{"archive", "fetch", "fetchImage", "findWorks", "search", "searchCache"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %#v, want %#v", got, want)
	}
	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "fetch" {
		t.Fatalf("prompts = %#v, want one fetch prompt", prompts.Prompts)
	}
}

// listToolNames connects an in-process client to the given service and
// returns the tool names it advertises — the one place both search-gating
// tests below read the registered surface, rather than poking register()
// internals.
func listToolNames(t *testing.T, service *Service) []string {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := service.Server().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestSearchToolHiddenWithoutABackend is the regression for a `search` tool
// advertised with nowhere to search: register() used to gate only on
// !DisableSearch, so a Service with neither SearXNGURL nor BraveAPIKey set
// still listed `search`, and calling it always failed with a configuration
// error the caller had no way to see in advance.
func TestSearchToolHiddenWithoutABackend(t *testing.T) {
	service, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	names := listToolNames(t, service)
	for _, name := range names {
		if name == "search" {
			t.Fatalf("tool list %v advertises `search` with no backend configured", names)
		}
	}
}

// TestSearchToolListedWithSearXNGConfigured is TestSearchToolHiddenWithoutABackend's
// positive twin: a configured backend must still register the tool.
func TestSearchToolListedWithSearXNGConfigured(t *testing.T) {
	service, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache"), SearXNGURL: "http://searxng.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	names := listToolNames(t, service)
	found := false
	for _, name := range names {
		if name == "search" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tool list %v does not advertise `search` with SearXNGURL configured", names)
	}
}

func TestOracleReceiptRenderers(t *testing.T) {
	listing := renderArchiveListing("/tmp/sample.zip", []harvest.Member{{Name: "a|b.txt", UncompressedSize: 7}})
	if want := `archive(source="/tmp/sample.zip", member="<name>")`; !contains(listing, want) {
		t.Fatalf("archive listing does not teach archive member call: %q", listing)
	}
	if !contains(listing, `| a\|b.txt | 7 | file |`) {
		t.Fatalf("archive listing does not escape table member: %q", listing)
	}
}

func TestDescribeLegacyFailureKindsNameTheSameRecovery(t *testing.T) {
	tests := []struct {
		name   string
		result harvest.Result
		want   []string
	}{
		{
			name:   "invalid URL",
			result: harvest.Result{ErrorKind: "invalid"},
			want:   []string{"input is invalid", "findWorks"},
		},
		{
			name:   "timeout",
			result: harvest.Result{ErrorKind: "timeout"},
			want:   []string{"timed out", "Retry later"},
		},
		{
			name:   "challenge",
			result: harvest.Result{Challenge: true, HTTPStatus: 200},
			want:   []string{"access challenge", "another copy"},
		},
		{
			name:   "HTTP 404",
			result: harvest.Result{Content: "tiny", ContentChars: 4, HTTPStatus: 404},
			want:   []string{"not found", "findWorks"},
		},
		{
			name:   "thin extraction",
			result: harvest.Result{HTTPStatus: 200},
			want:   []string{"no readable content", "`search`", "`findWorks`"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (*Service)(nil).describeFetch("https://fixture.example/source", test.result, false)
			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Fatalf("describe receipt missing %q: %q", want, got)
				}
			}
		})
	}
}

// TestServiceCacheIsTheOneRootNotTheWorkingDirectory pins the split-cache
// defect: NewConfigured resolved a cwd-relative ".cache", so the daemon
// (systemd cwd = $HOME) cached into ~/.cache while the CLI used
// ~/.professor/.cache and the two never shared a hit.
func TestServiceCacheIsTheOneRootNotTheWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv(paths.EnvHome, home)
	t.Setenv("WEBFETCH_DIR", filepath.Join(t.TempDir(), "legacy"))
	service, err := NewConfigured("test", Runtime{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	if want := filepath.Join(home, ".professor", ".cache"); service.runtime.CacheDir != want {
		t.Fatalf("service cache root = %q, want the one default %q", service.runtime.CacheDir, want)
	}
}

func TestConfiguredServiceCarriesScholarlyProviderRuntime(t *testing.T) {
	home := t.TempDir()
	service, err := NewConfigured("test", Runtime{
		Home:             home,
		CacheDir:         filepath.Join(home, "cache"),
		SciHubURL:        "https://mirror.example/scihub",
		AnnasURL:         "https://annas.example",
		SciDBURL:         "https://scidb.example",
		LibGenURL:        "https://libgen.example",
		GoogleScholarURL: "https://scholar.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = service.Close() }()
	for _, tc := range []struct{ name, got, want string }{
		{"SciHubURL", service.runtime.SciHubURL, "https://mirror.example/scihub"},
		{"AnnasURL", service.runtime.AnnasURL, "https://annas.example"},
		{"SciDBURL", service.runtime.SciDBURL, "https://scidb.example"},
		{"LibGenURL", service.runtime.LibGenURL, "https://libgen.example"},
		{"GoogleScholarURL", service.runtime.GoogleScholarURL, "https://scholar.example"},
	} {
		if tc.got != tc.want {
			t.Errorf("service runtime %s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestSearchCacheMissHintsSearchOnlyWhenAvailable pins the searchCache
// empty-match hint, the one harvestmcp-side message in the closed
// `use `search`` list: it must not point at a `search` tool the server does
// not advertise.
func TestSearchCacheMissHintsSearchOnlyWhenAvailable(t *testing.T) {
	off, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = off.Close() }()
	result, _, err := off.searchCache(context.Background(), nil, CacheInput{Pattern: "no-such-needle"})
	if err != nil {
		t.Fatalf("searchCache(no backend) error: %v", err)
	}
	offText := result.Content[0].(*mcp.TextContent).Text
	if strings.Contains(offText, "`search`") {
		t.Fatalf("searchCache miss text %q names `search` with no backend configured", offText)
	}

	on, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache"), SearXNGURL: "http://searxng.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = on.Close() }()
	result, _, err = on.searchCache(context.Background(), nil, CacheInput{Pattern: "no-such-needle"})
	if err != nil {
		t.Fatalf("searchCache(with backend) error: %v", err)
	}
	onText := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(onText, "`search`") {
		t.Fatalf("searchCache miss text %q dropped `search` with a backend configured", onText)
	}
}

func mustWorkingDir(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return working
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}
