package harvestmcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	goRuntime "runtime"
	"strings"
	"testing"

	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestpy"
	"hostops/pfm/internal/paths"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStableSixToolSurfaceAndFetchPrompt(t *testing.T) {
	service, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache")})
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

// A failed search renders every backend's own error, one per line.
func TestSearchFailureRendersEachBackend(t *testing.T) {
	text := renderSearchFailure(errors.Join(errors.New("searxng http://127.0.0.1:8888: HTTP 502"), errors.New("brave: HTTP 401")))
	if !strings.Contains(text, "Web search failed") || !strings.Contains(text, "Retrieval failed") {
		t.Fatalf("search failure lost safe public message: %q", text)
	}
	for _, secret := range []string{"searxng", "127.0.0.1", "HTTP 502", "brave: HTTP 401", "harvester.config.json"} {
		if strings.Contains(text, secret) {
			t.Fatalf("search failure text exposed private backend detail %q:\n%s", secret, text)
		}
	}
	if strings.Contains(text, "unreachable or failing") {
		t.Fatalf("search failure text kept the generic message:\n%s", text)
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

// TestBrowserPathsResolveTheNormalizedPlatform pins the HIGH review finding:
// production must probe env-browser/<GOOS>-<GOARCH>/ — never the "-" sentinel
// an empty Platform{} stringifies to, which provisioning never writes.
func TestBrowserPathsResolveTheNormalizedPlatform(t *testing.T) {
	root := t.TempDir()
	converter := pythonConverter{browserRoot: root}
	interpreter, script := converter.browserPaths()
	normalized := harvestpy.Platform{GOOS: goRuntime.GOOS, GOARCH: goRuntime.GOARCH}
	wantRoot := harvestpy.BrowserRuntimeRoot(root, normalized)
	wantInterpreter := filepath.Join(wantRoot, "project", ".venv", "bin", "python")
	wantScript := filepath.Join(wantRoot, "project", "browser.py")
	if interpreter != wantInterpreter || script != wantScript {
		t.Fatalf("browser paths=%q/%q, want the normalized-platform root %q", interpreter, script, wantRoot)
	}
	if strings.Contains(interpreter, `"-/`) || strings.HasSuffix(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(interpreter)))), "-") {
		t.Fatalf("browser path uses the empty-Platform %q sentinel: %q", harvestpy.Platform{}.String(), interpreter)
	}
}
