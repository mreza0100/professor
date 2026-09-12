package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestHarvesterDefaultsWhenFileAbsent(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	path := filepath.Join(t.TempDir(), FileName)
	got, err := Load(path, home, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := got.Harvester
	if h.Exists || h.Enabled || h.External.Enabled || h.External.Port != DefaultHarvesterExternalPort ||
		h.External.Host != "127.0.0.1" || !h.Search.Enabled || h.Cache.TTL != 24*time.Hour ||
		h.Cache.NegativeTTL != 120*time.Second || h.Cache.NegativeTransientTTL != 15*time.Second ||
		h.Scholarly != (HarvesterScholarly{}) ||
		h.Output.MaxInlineChars != 50000 || h.Cache.Dir != "" {
		t.Fatalf("defaults = %+v", h)
	}
	if got.MCP.HTTP.Port != DefaultMCPPort {
		t.Fatalf("loopback port = %d, want %d", got.MCP.HTTP.Port, DefaultMCPPort)
	}
	if h.Path != filepath.Join(filepath.Dir(path), HarvesterFileName) {
		t.Fatalf("harvester path = %q", h.Path)
	}
}

func TestHarvesterFileLoadsEverySetting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{
  "enabled": true,
  "external": {"enabled": true, "host": "0.0.0.0", "port": 19000, "publicURL": "https://harvester.example.com/",
               "auth": {"passphrase": "open sesame", "staticToken": "tok"}, "stateDir": "~/state"},
  "search": {"enabled": true, "searxngURL": "http://127.0.0.1:8888/", "braveApiKey": "brave"},
	  "scholarly": {"contactEmail": "ops@example.com", "googleBooksApiKey": "g", "coreApiKey": "c", "semanticScholarApiKey": "s", "sciHubURL": "  https://mirror.example/base  ", "annasURL": "https://annas.example", "sciDBURL": "https://scidb.example", "libGenURL": "https://libgen.example", "googleScholarURL": "https://scholar.example"},
  "fetch": {"browser": true, "userAgent": "UA/1", "proxyURL": "http://proxy.example:3128"},
  "convert": {"pdfOcr": true, "pdfLayout": true},
  "cache": {"dir": "~/cache", "ttlSeconds": 60, "negativeTtlSeconds": 5, "negativeTransientTtlSeconds": 2},
  "output": {"maxInlineChars": 1234}
}`, 0o600)
	got, err := Load(path, home, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	h := got.Harvester
	if !h.Enabled || !got.MCPServers["harvester"].Enabled {
		t.Fatalf("enabled not mirrored: harvester=%t server=%t", h.Enabled, got.MCPServers["harvester"].Enabled)
	}
	want := HarvesterExternal{Enabled: true, Host: "0.0.0.0", Port: 19000, PublicURL: "https://harvester.example.com",
		Passphrase: "open sesame", StaticToken: "tok", StateDir: filepath.Join(home, "state")}
	if h.External != want {
		t.Fatalf("external = %+v, want %+v", h.External, want)
	}
	if h.Search.SearXNGURL != "http://127.0.0.1:8888" || h.Search.BraveAPIKey != "brave" {
		t.Fatalf("search = %+v", h.Search)
	}
	if h.Scholarly != (HarvesterScholarly{ContactEmail: "ops@example.com", GoogleBooksAPIKey: "g", CoreAPIKey: "c", SemanticScholarAPIKey: "s", SciHubURL: "https://mirror.example/base", AnnasURL: "https://annas.example", SciDBURL: "https://scidb.example", LibGenURL: "https://libgen.example", GoogleScholarURL: "https://scholar.example"}) {
		t.Fatalf("scholarly = %+v", h.Scholarly)
	}
	if !h.Fetch.Browser || h.Fetch.UserAgent != "UA/1" || h.Fetch.ProxyURL != "http://proxy.example:3128" {
		t.Fatalf("fetch = %+v", h.Fetch)
	}
	if h.Cache.Dir != filepath.Join(home, "cache") || h.Cache.TTL != time.Minute || h.Cache.NegativeTTL != 5*time.Second || h.Cache.NegativeTransientTTL != 2*time.Second {
		t.Fatalf("cache = %+v", h.Cache)
	}
	if !h.Convert.PDFOCR || !h.Convert.PDFLayout {
		t.Fatalf("convert = %+v", h.Convert)
	}
	if h.Output.MaxInlineChars != 1234 {
		t.Fatalf("output = %+v", h.Output)
	}
	for _, key := range HarvesterSourceKeys() {
		if got.Source(key) != SourceFile {
			t.Errorf("source %s = %q, want file", key, got.Source(key))
		}
	}
}

func TestHarvesterSciHubURLRoundTripsWithoutChangingSiblingSettings(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{
  "enabled": true,
  "search": {"enabled": false, "searxngURL": "http://127.0.0.1:8888"},
	  "scholarly": {"contactEmail": "ops@example.com", "sciHubURL": " https://mirror.example/scihub ", "annasURL": " https://annas.example ", "sciDBURL": "https://scidb.example", "libGenURL": "https://libgen.example", "googleScholarURL": "https://scholar.example"},
  "fetch": {"userAgent": "fixture-agent"},
  "cache": {"ttlSeconds": 77},
  "output": {"maxInlineChars": 321}
}`, 0o600)
	got, err := Load(path, home, nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Harvester.Scholarly.SciHubURL != "https://mirror.example/scihub" ||
		got.Harvester.Scholarly.AnnasURL != "https://annas.example" ||
		got.Harvester.Scholarly.SciDBURL != "https://scidb.example" ||
		got.Harvester.Scholarly.LibGenURL != "https://libgen.example" ||
		got.Harvester.Scholarly.GoogleScholarURL != "https://scholar.example" {
		t.Fatalf("SciHubURL = %q, want trimmed configured URL", got.Harvester.Scholarly.SciHubURL)
	}
	if got.Harvester.Search.Enabled || got.Harvester.Search.SearXNGURL != "http://127.0.0.1:8888" ||
		got.Harvester.Scholarly.ContactEmail != "ops@example.com" || got.Harvester.Fetch.UserAgent != "fixture-agent" ||
		got.Harvester.Cache.TTL != 77*time.Second || got.Harvester.Output.MaxInlineChars != 321 {
		t.Fatalf("unrelated settings changed: %+v", got.Harvester)
	}
	if got.Source("harvester.scholarly.sciHubURL") != SourceFile {
		t.Fatalf("SciHub source = %q, want %q", got.Source("harvester.scholarly.sciHubURL"), SourceFile)
	}
	marshaled, err := MarshalHarvester(got.Harvester, false)
	if err != nil {
		t.Fatalf("MarshalHarvester() error = %v", err)
	}
	var shape struct {
		Search    map[string]any `json:"search"`
		Scholarly map[string]any `json:"scholarly"`
	}
	if err := json.Unmarshal(marshaled, &shape); err != nil {
		t.Fatalf("MarshalHarvester JSON = %v", err)
	}
	if shape.Scholarly["sciHubURL"] != "https://mirror.example/scihub" {
		t.Fatalf("marshaled sciHubURL = %#v", shape.Scholarly["sciHubURL"])
	}
	for key, want := range map[string]string{
		"annasURL": "https://annas.example", "sciDBURL": "https://scidb.example",
		"libGenURL": "https://libgen.example", "googleScholarURL": "https://scholar.example",
	} {
		if shape.Scholarly[key] != want {
			t.Fatalf("marshaled %s = %#v, want %q", key, shape.Scholarly[key], want)
		}
	}
	if shape.Search["searxngURL"] != "http://127.0.0.1:8888" || shape.Search["enabled"] != false {
		t.Fatalf("marshaled sibling settings changed: %#v", shape.Search)
	}
}

func TestHarvesterFileRefusesUnsafeOrInvalidSettings(t *testing.T) {
	cases := map[string]struct {
		content string
		mode    os.FileMode
		want    string
	}{
		"external without auth": {`{"external":{"enabled":true,"publicURL":"https://h.example.com"}}`, 0o600, "never unauthenticated"},
		"external without url":  {`{"external":{"enabled":true,"auth":{"staticToken":"t"}}}`, 0o600, "requires external.publicURL"},
		"external url path":     {`{"external":{"publicURL":"https://h.example.com/mcp"}}`, 0o600, "without a path"},
		"searxng query":         {`{"search":{"searxngURL":"http://127.0.0.1:8888/?x=1"}}`, 0o600, "query or fragment"},
		"searxng scheme":        {`{"search":{"searxngURL":"ftp://127.0.0.1"}}`, 0o600, "http or https"},
		"scihub scheme":         {`{"scholarly":{"sciHubURL":"ftp://mirror.example"}}`, 0o600, "http or https"},
		"scihub relative":       {`{"scholarly":{"sciHubURL":"mirror.example/path"}}`, 0o600, "http or https"},
		"scihub userinfo":       {`{"scholarly":{"sciHubURL":"https://user:pass@mirror.example"}}`, 0o600, "must not carry userinfo"},
		"scihub query":          {`{"scholarly":{"sciHubURL":"https://mirror.example/?token=x"}}`, 0o600, "query or fragment"},
		"scihub fragment":       {`{"scholarly":{"sciHubURL":"https://mirror.example/#pdf"}}`, 0o600, "query or fragment"},
		"negative ttl":          {`{"cache":{"ttlSeconds":-1}}`, 0o600, "0 or more"},
		"zero inline":           {`{"output":{"maxInlineChars":0}}`, 0o600, "at least 1"},
		"relative cache dir":    {`{"cache":{"dir":"cache"}}`, 0o600, "must be absolute"},
		"unknown key":           {`{"search":{"searxng":"http://x"}}`, 0o600, "unknown field"},
		"retired env name":      {`{"SEARXNG_URL":"http://x"}`, 0o600, "unknown field"},
		"world-readable secret": {`{"search":{"braveApiKey":"k"}}`, 0o644, "chmod 600"},
		"port collision":        {`{"external":{"enabled":true,"port":18377,"publicURL":"https://h.example.com","auth":{"staticToken":"t"}}}`, 0o600, "collides with mcp.http.port"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, HarvesterFileName), test.content, test.mode)
			_, err := Load(filepath.Join(dir, FileName), filepath.Join(dir, "home"), nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestHarvesterScholarlyProviderURLsRejectUnsafeComponents(t *testing.T) {
	for _, field := range []string{"annasURL", "sciDBURL", "libGenURL", "googleScholarURL"} {
		for name, value := range map[string]string{
			"scheme":   "ftp://mirror.example",
			"userinfo": "https://user:pass@mirror.example",
			"query":    "https://mirror.example/?token=x",
			"fragment": "https://mirror.example/#pdf",
		} {
			t.Run(field+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				content := `{"scholarly":{"` + field + `":"` + value + `"}}`
				writeFile(t, filepath.Join(dir, HarvesterFileName), content, 0o600)
				if _, err := Load(filepath.Join(dir, FileName), filepath.Join(dir, "home"), nil); err == nil {
					t.Fatalf("Load accepted unsafe %s=%q", field, value)
				}
			})
		}
	}
}

func TestHarvesterSciHubURLWhitespaceOnlyDisablesFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"scholarly":{"sciHubURL":"   "}}`, 0o600)
	got, err := Load(filepath.Join(dir, FileName), filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Harvester.Scholarly.SciHubURL != "" {
		t.Fatalf("SciHubURL = %q, want empty disabled value", got.Harvester.Scholarly.SciHubURL)
	}
}

func TestHarvesterWorldReadableWithoutSecretsLoads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"search":{"searxngURL":"http://127.0.0.1:8888"}}`, 0o644)
	if _, err := Load(filepath.Join(dir, FileName), filepath.Join(dir, "home"), nil); err != nil {
		t.Fatalf("a secret-free harvester file must load at 0644: %v", err)
	}
}

func TestHarvesterFileEnabledWinsOverLegacyKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, path, `{"version":2,"mcp":{"servers":{"harvester":{"enabled":true}}}}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"enabled":false}`, 0o600)
	got, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Harvester.Enabled || got.MCPServers["harvester"].Enabled || got.MCPServerSource("harvester") != SourceFile {
		t.Fatalf("harvester enabled=%t server=%t source=%q, want the harvester file's false",
			got.Harvester.Enabled, got.MCPServers["harvester"].Enabled, got.MCPServerSource("harvester"))
	}
}

func TestLoadFallsBackToPreSplitFileUntilMigrated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	legacy := filepath.Join(home, ".config", "pfm", LegacyFileName)
	writeFile(t, legacy, `{"version":2,"theme":"tokyo-night"}`, 0o600)
	got, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != legacy || !got.Exists || got.Theme != "tokyo-night" {
		t.Fatalf("pre-split fallback: path=%q exists=%t theme=%q", got.Path, got.Exists, got.Theme)
	}
	current := filepath.Join(home, ".config", "pfm", FileName)
	writeFile(t, current, `{"version":2,"theme":"default"}`, 0o600)
	got, err = Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != current || got.Theme != "default" {
		t.Fatalf("current file must win once present: path=%q theme=%q", got.Path, got.Theme)
	}
}

func TestMigrationSplitsRenamesAndMovesPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	legacy := filepath.Join(dir, LegacyFileName)
	writeFile(t, legacy, `{"version":2,"theme":"tokyo-night","mcp":{"http":{"port":8377},"servers":{"chat":{"enabled":true},"harvester":{"enabled":true}}}}`, 0o600)
	before, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.Empty() || len(migration.Steps()) != 3 {
		t.Fatalf("plan = %+v steps=%q, want rename + harvester move + port", migration, migration.Steps())
	}
	if preview := migration.Preview(before); preview.MCP.HTTP.Port != DefaultMCPPort || preview.Path != legacy {
		t.Fatalf("preview port=%d path=%q", preview.MCP.HTTP.Port, preview.Path)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("pre-split file still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyBackupName)); err != nil {
		t.Fatalf("pre-split backup missing: %v", err)
	}
	after, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.Path != filepath.Join(dir, FileName) || after.Theme != "tokyo-night" || after.MCP.HTTP.Port != DefaultMCPPort ||
		!after.MCPServers["chat"].Enabled || !after.Harvester.Enabled || after.MCPServerSource("harvester") != SourceFile {
		t.Fatalf("after migration: path=%q theme=%q port=%d chat=%t harvester=%t source=%q", after.Path, after.Theme,
			after.MCP.HTTP.Port, after.MCPServers["chat"].Enabled, after.Harvester.Enabled, after.MCPServerSource("harvester"))
	}
	content, err := os.ReadFile(after.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "harvester") {
		t.Fatalf("pfm.config.json still carries the harvester key:\n%s", content)
	}
	again, err := PlanMigration(after)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Empty() {
		t.Fatalf("second plan = %q, want empty", again.Steps())
	}
}

func TestMigrationKeepsCustomPortAndExistingHarvesterFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, path, `{"version":2,"mcp":{"http":{"port":9999},"servers":{"harvester":{"enabled":true}}}}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"enabled":false,"search":{"searxngURL":"http://127.0.0.1:8888"}}`, 0o600)
	before, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.MovePort || migration.LegacyPath != "" || migration.HarvesterEnabled == nil {
		t.Fatalf("plan = %+v", migration)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	after, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.MCP.HTTP.Port != 9999 || after.Harvester.Enabled || after.Harvester.Search.SearXNGURL != "http://127.0.0.1:8888" {
		t.Fatalf("after: port=%d enabled=%t searxng=%q", after.MCP.HTTP.Port, after.Harvester.Enabled, after.Harvester.Search.SearXNGURL)
	}
}

func TestSetMCPServerHarvesterWritesHarvesterFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, path, `{"version":2}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"search":{"searxngURL":"http://127.0.0.1:8888"}}`, 0o600)
	loaded, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := SetMCPServer(loaded, "harvester", true)
	if err != nil || !changed {
		t.Fatalf("SetMCPServer = %t, %v", changed, err)
	}
	pfmContent, _ := os.ReadFile(path)
	if strings.Contains(string(pfmContent), "harvester") {
		t.Fatalf("pfm.config.json gained a harvester key:\n%s", pfmContent)
	}
	var harvester map[string]any
	content, _ := os.ReadFile(filepath.Join(dir, HarvesterFileName))
	if err := json.Unmarshal(content, &harvester); err != nil {
		t.Fatal(err)
	}
	if harvester["enabled"] != true || harvester["search"] == nil {
		t.Fatalf("harvester file = %s", content)
	}
}

func TestHarvesterSecretsNeverRenderInDisplay(t *testing.T) {
	harvester := DefaultHarvester()
	harvester.External.Passphrase = "open sesame"
	harvester.External.StaticToken = "tok-123"
	harvester.Search.BraveAPIKey = "brave-456"
	harvester.Scholarly.CoreAPIKey = "core-789"
	content, err := MarshalHarvester(harvester, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"open sesame", "tok-123", "brave-456", "core-789"} {
		if strings.Contains(string(content), secret) {
			t.Fatalf("redacted harvester output leaks %q:\n%s", secret, content)
		}
	}
	generic := RedactSecrets([]byte(`{"braveApiKey":"b","auth":{"passphrase":"p"}}`))
	if strings.Contains(string(generic), `"b"`) || strings.Contains(string(generic), `"p"`) {
		t.Fatalf("RedactSecrets leaked apikey/passphrase: %s", generic)
	}
}

// A migration interrupted between writing pfm.config.json and parking
// config.json leaves both files; the next plan must park the leftover, never
// treat the machine as already migrated.
func TestInterruptedMigrationLeftoverIsParkedNotIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	migrated := `{"version":2,"mcp":{"http":{"port":18377}}}` + "\n"
	writeFile(t, filepath.Join(dir, FileName), migrated, 0o600)
	stray := filepath.Join(dir, LegacyFileName)
	writeFile(t, stray, `{"version":2,"mcp":{"http":{"port":8377},"servers":{"harvester":{"enabled":true}}}}`, 0o600)
	loaded, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if migration.Empty() || migration.StrayLegacyPath != stray {
		t.Fatalf("plan = %+v, want the leftover %s parked", migration, stray)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatalf("leftover still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyBackupName)); err != nil {
		t.Fatalf("leftover not parked: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(dir, FileName)); err != nil || string(content) != migrated {
		t.Fatalf("parking rewrote the migrated config: %q %v", content, err)
	}
}

// Moving the loopback port onto a port the external gateway already holds
// would leave a config that refuses to load; the plan keeps the old port.
func TestMigrationKeepsPortWhenExternalGatewayHoldsTheTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	writeFile(t, filepath.Join(dir, LegacyFileName), `{"version":2,"mcp":{"http":{"port":8377}}}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName),
		`{"enabled":true,"external":{"enabled":true,"port":18377,"publicURL":"https://h.example.test","auth":{"staticToken":"t"}}}`, 0o600)
	before, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.MovePort || migration.PortKept == "" {
		t.Fatalf("plan = %+v, want the port kept with a reason", migration)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	after, err := Load("", home, nil)
	if err != nil {
		t.Fatalf("migrated config no longer loads: %v", err)
	}
	if after.MCP.HTTP.Port != legacyDefaultMCPPort || after.Path != filepath.Join(dir, FileName) {
		t.Fatalf("after: port=%d path=%q", after.MCP.HTTP.Port, after.Path)
	}
}
