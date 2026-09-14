package installer

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledThemeInstallsFromSourceRepoThenReleaseAndReportsAMissingFile(t *testing.T) {
	themeBody := []byte(`{"name":"Sonar Gold","base":"dark","overrides":{"claude":"#ffd60a"}}` + "\n")
	manifest := `{"bundled":{"sonar-gold":{"file":"sonar-gold.json","target":"~/.claude/themes/sonar-gold.json","activate":"/theme","requires":"fixture"}}}`
	run := func(home, sourceRepo, manifestURL string) (Report, string, error) {
		var output bytes.Buffer
		report, err := Run(context.Background(), Options{
			Mode: ModeApply, Home: home, SourceRepo: sourceRepo, ThemeManifestURL: manifestURL, Stdout: &output,
			Runner: &fakeRunner{nameSyncIdle: true}, CodexHomes: []string{}, InstallThemes: true,
		})
		return report, output.String(), err
	}

	// 1. source clone carries the manifest and the file: installed, owned, idempotent.
	home := t.TempDir()
	sourceRepo := t.TempDir()
	writeFixture(t, filepath.Join(sourceRepo, "templates", "themes", "sources.json"), manifest)
	writeFixture(t, filepath.Join(sourceRepo, "templates", "themes", "sonar-gold.json"), string(themeBody))
	target := filepath.Join(home, ".claude", "themes", "sonar-gold.json")
	first, firstOutput, err := run(home, sourceRepo, "")
	if err != nil {
		t.Fatalf("bundled theme install: %v\n%s", err, firstOutput)
	}
	if got := []byte(readFixture(t, target)); !bytes.Equal(got, themeBody) {
		t.Fatalf("installed bundled theme=%q, want %q", got, themeBody)
	}
	if first.Changed == 0 || !strings.Contains(firstOutput, "theme sonar-gold") {
		t.Fatalf("first report=%#v, want a named bundled theme write\n%s", first, firstOutput)
	}
	second, secondOutput, err := run(home, sourceRepo, "")
	if err != nil || second.Changed != 0 || !strings.Contains(secondOutput, "theme sonar-gold unchanged") {
		t.Fatalf("second run err=%v report=%#v, want changed=0 and an unchanged row\n%s", err, second, secondOutput)
	}

	// 2. the file is missing from the clone: a loud read failure, never an absence, and the install continues.
	if err := os.Remove(filepath.Join(sourceRepo, "templates", "themes", "sonar-gold.json")); err != nil {
		t.Fatal(err)
	}
	_, missingOutput, err := run(t.TempDir(), sourceRepo, "")
	if err != nil {
		t.Fatalf("missing bundled file aborted host install: %v\n%s", err, missingOutput)
	}
	if !strings.Contains(missingOutput, "theme sonar-gold read failed") || !strings.Contains(missingOutput, "sonar-gold.json") {
		t.Fatalf("missing bundled file was silent or vague:\n%s", missingOutput)
	}

	// 3. no manifest in the discovered clone: the release manifest wins and the file comes from beside it.
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/templates/themes/sources.json":
			_, _ = io.WriteString(response, manifest)
		case "/templates/themes/sonar-gold.json":
			_, _ = response.Write(themeBody)
		default:
			http.NotFound(response, request)
		}
	}))
	t.Cleanup(server.Close)
	releaseHome := t.TempDir()
	_, releaseOutput, err := run(releaseHome, t.TempDir(), server.URL+"/templates/themes/sources.json")
	if err != nil {
		t.Fatalf("release-fallback bundled install: %v\n%s", err, releaseOutput)
	}
	if got := []byte(readFixture(t, filepath.Join(releaseHome, ".claude", "themes", "sonar-gold.json"))); !bytes.Equal(got, themeBody) {
		t.Fatalf("release-fallback bundled theme=%q, want %q\n%s", got, themeBody, releaseOutput)
	}
}

func TestBundledThemeManifestValidationAndNonJSONFileFailClosedByName(t *testing.T) {
	load := func(manifest string) error {
		sourceRepo := t.TempDir()
		writeFixture(t, filepath.Join(sourceRepo, "templates", "themes", "sources.json"), manifest)
		_, err := loadThemeSources(context.Background(), Options{SourceRepo: sourceRepo})
		return err
	}
	for _, tc := range []struct{ name, manifest, want string }{
		{"path traversal", `{"bundled":{"x":{"file":"../secret.json","target":"~/.claude/themes/x.json"}}}`, `bundled theme "x" file "../secret.json" must be a bare file name beside the manifest`},
		{"name clash", `{"source_fetched":{"x":{"repo":"https://e.test","raw":"https://e.test/x.json","target":"~/.claude/themes/x.json"}},"bundled":{"x":{"file":"x.json","target":"~/.claude/themes/x.json"}}}`, `names theme "x" as both source_fetched and bundled`},
		{"missing file field", `{"bundled":{"x":{"target":"~/.claude/themes/x.json"}}}`, `bundled theme "x" is missing name, file, or target`},
	} {
		err := load(tc.manifest)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err=%v, want it to contain %q", tc.name, err, tc.want)
		}
	}

	home := t.TempDir()
	sourceRepo := t.TempDir()
	writeFixture(t, filepath.Join(sourceRepo, "templates", "themes", "sources.json"), `{"bundled":{"x":{"file":"x.json","target":"~/.claude/themes/x.json"}}}`)
	writeFixture(t, filepath.Join(sourceRepo, "templates", "themes", "x.json"), "not json\n")
	var output bytes.Buffer
	_, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, SourceRepo: sourceRepo, Stdout: &output,
		Runner: &fakeRunner{nameSyncIdle: true}, CodexHomes: []string{}, InstallThemes: true,
	})
	if err != nil {
		t.Fatalf("non-JSON bundled file aborted host install: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "theme x read failed") || !strings.Contains(output.String(), "is not valid JSON") {
		t.Fatalf("non-JSON bundled file was silent or vague:\n%s", output.String())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".claude", "themes", "x.json")); !os.IsNotExist(statErr) {
		t.Fatalf("non-JSON bundled file was installed anyway: %v", statErr)
	}
}
