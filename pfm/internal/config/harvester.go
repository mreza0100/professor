package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// FileName is the machine config's name inside the pfm config directory.
	FileName = "pfm.config.json"
	// LegacyFileName is the pre-split name. A machine that still has only this
	// file keeps working until `pfm install` migrates it (PlanMigration).
	LegacyFileName = "config.json"
	// HarvesterFileName holds every Harvester setting, beside FileName.
	HarvesterFileName = "harvester.config.json"
	// legacyBackupName is where the migration parks the pre-split file.
	legacyBackupName = "config.json.pre-split"

	// DefaultMCPPort is the loopback daemon port (chat + harvester, no auth).
	DefaultMCPPort = 18377
	// legacyDefaultMCPPort is the port `pfm config init` wrote explicitly into
	// every pre-split file; the migration moves exactly this value.
	legacyDefaultMCPPort = 8377
	// DefaultHarvesterExternalPort is the authenticated external gateway port.
	DefaultHarvesterExternalPort = 18378

	// SourceLegacy marks a value still read from the pre-split pfm config.
	SourceLegacy Source = "legacy pfm config — run `pfm install --yes` to migrate"
)

// HarvesterConfig is the fully materialized harvester.config.json. It is the
// ONLY source of Harvester settings: the harvest packages read no process
// environment, and the daemon units carry none.
type HarvesterConfig struct {
	Enabled   bool
	External  HarvesterExternal
	Search    HarvesterSearch
	Scholarly HarvesterScholarly
	Fetch     HarvesterFetch
	Convert   HarvesterConvert
	Cache     HarvesterCache
	Output    HarvesterOutput

	Path   string
	Exists bool
}

// HarvesterExternal is the authenticated gateway the daemon opens beside its
// loopback port. It serves the harvester only — never the chat MCP.
type HarvesterExternal struct {
	Enabled     bool
	Host        string
	Port        int
	PublicURL   string
	Passphrase  string
	StaticToken string
	// StateDir holds the OAuth client/token store; empty resolves beside the
	// cache root at serve time.
	StateDir string
}

type HarvesterSearch struct {
	Enabled bool
	// SearXNGURL is operator configuration, so its exact origin is trusted by
	// the search request (a private/loopback SearXNG is the normal deployment).
	SearXNGURL  string
	BraveAPIKey string
}

type HarvesterScholarly struct {
	ContactEmail          string
	GoogleBooksAPIKey     string
	CoreAPIKey            string
	SemanticScholarAPIKey string
}

type HarvesterFetch struct {
	Browser   bool
	UserAgent string
	ProxyURL  string
}

// HarvesterConvert steers the pinned Python converter. Go hands both flags to
// the worker process explicitly; converter.py itself is unchanged.
type HarvesterConvert struct {
	// PDFOCR forces an OCR pass on every PDF (scanned/image-only documents).
	PDFOCR bool
	// PDFLayout keeps the PDF's physical layout in the extracted text.
	PDFLayout bool
}

// HarvesterCache: an empty Dir resolves to harvest's single default,
// <home>/.professor/.cache. A zero TTL is meaningful: TTL 0 never expires a
// cached document; NegativeTTL / NegativeTransientTTL 0 never cache failures.
type HarvesterCache struct {
	Dir                  string
	TTL                  time.Duration
	NegativeTTL          time.Duration
	NegativeTransientTTL time.Duration
}

type HarvesterOutput struct {
	MaxInlineChars int
}

type rawHarvester struct {
	Enabled   *bool                  `json:"enabled,omitempty"`
	External  *rawHarvesterExternal  `json:"external,omitempty"`
	Search    *rawHarvesterSearch    `json:"search,omitempty"`
	Scholarly *rawHarvesterScholarly `json:"scholarly,omitempty"`
	Fetch     *rawHarvesterFetch     `json:"fetch,omitempty"`
	Convert   *rawHarvesterConvert   `json:"convert,omitempty"`
	Cache     *rawHarvesterCache     `json:"cache,omitempty"`
	Output    *rawHarvesterOutput    `json:"output,omitempty"`
}

type rawHarvesterExternal struct {
	Enabled   *bool             `json:"enabled,omitempty"`
	Host      *string           `json:"host,omitempty"`
	Port      *int              `json:"port,omitempty"`
	PublicURL *string           `json:"publicURL,omitempty"`
	Auth      *rawHarvesterAuth `json:"auth,omitempty"`
	StateDir  *string           `json:"stateDir,omitempty"`
}

type rawHarvesterAuth struct {
	Passphrase  *string `json:"passphrase,omitempty"`
	StaticToken *string `json:"staticToken,omitempty"`
}

type rawHarvesterSearch struct {
	Enabled     *bool   `json:"enabled,omitempty"`
	SearXNGURL  *string `json:"searxngURL,omitempty"`
	BraveAPIKey *string `json:"braveApiKey,omitempty"`
}

type rawHarvesterScholarly struct {
	ContactEmail          *string `json:"contactEmail,omitempty"`
	GoogleBooksAPIKey     *string `json:"googleBooksApiKey,omitempty"`
	CoreAPIKey            *string `json:"coreApiKey,omitempty"`
	SemanticScholarAPIKey *string `json:"semanticScholarApiKey,omitempty"`
}

type rawHarvesterFetch struct {
	Browser   *bool   `json:"browser,omitempty"`
	UserAgent *string `json:"userAgent,omitempty"`
	ProxyURL  *string `json:"proxyURL,omitempty"`
}

type rawHarvesterConvert struct {
	PDFOCR    *bool `json:"pdfOcr,omitempty"`
	PDFLayout *bool `json:"pdfLayout,omitempty"`
}

type rawHarvesterCache struct {
	Dir                         *string `json:"dir,omitempty"`
	TTLSeconds                  *int    `json:"ttlSeconds,omitempty"`
	NegativeTTLSeconds          *int    `json:"negativeTtlSeconds,omitempty"`
	NegativeTransientTTLSeconds *int    `json:"negativeTransientTtlSeconds,omitempty"`
}

type rawHarvesterOutput struct {
	MaxInlineChars *int `json:"maxInlineChars,omitempty"`
}

// DefaultHarvester is the effective configuration when harvester.config.json
// is absent. The harvester MCP ships disabled, like every registered server.
func DefaultHarvester() HarvesterConfig {
	return HarvesterConfig{
		External: HarvesterExternal{Host: "127.0.0.1", Port: DefaultHarvesterExternalPort},
		Search:   HarvesterSearch{Enabled: true},
		Cache: HarvesterCache{
			TTL:                  24 * time.Hour,
			NegativeTTL:          120 * time.Second,
			NegativeTransientTTL: 15 * time.Second,
		},
		Output: HarvesterOutput{MaxInlineChars: 50000},
	}
}

// HarvesterPath is harvester.config.json beside the given pfm config file.
func HarvesterPath(pfmConfigPath string) string {
	return filepath.Join(filepath.Dir(pfmConfigPath), HarvesterFileName)
}

// harvesterSourceKeys lists every harvester key `pfm config show` reports.
var harvesterSourceKeys = []string{
	"harvester.enabled",
	"harvester.external.enabled", "harvester.external.host", "harvester.external.port",
	"harvester.external.publicURL", "harvester.external.auth.passphrase",
	"harvester.external.auth.staticToken", "harvester.external.stateDir",
	"harvester.search.enabled", "harvester.search.searxngURL", "harvester.search.braveApiKey",
	"harvester.scholarly.contactEmail", "harvester.scholarly.googleBooksApiKey",
	"harvester.scholarly.coreApiKey", "harvester.scholarly.semanticScholarApiKey",
	"harvester.fetch.browser", "harvester.fetch.userAgent", "harvester.fetch.proxyURL",
	"harvester.convert.pdfOcr", "harvester.convert.pdfLayout",
	"harvester.cache.dir", "harvester.cache.ttlSeconds", "harvester.cache.negativeTtlSeconds",
	"harvester.cache.negativeTransientTtlSeconds",
	"harvester.output.maxInlineChars",
}

// HarvesterSourceKeys returns the reported harvester keys in display order.
func HarvesterSourceKeys() []string { return append([]string(nil), harvesterSourceKeys...) }

// loadHarvester reads harvester.config.json over defaults into result.
// legacyEnabled is the pre-split pfm config's mcp.servers.harvester.enabled,
// honored only while the harvester file does not set enabled itself.
func loadHarvester(result *Config, home string, legacyEnabled *bool) error {
	harvester := result.Harvester
	harvester.Path = HarvesterPath(result.Path)
	defer func() { result.Harvester = harvester }()
	if legacyEnabled != nil {
		harvester.Enabled = *legacyEnabled
		result.Sources["harvester.enabled"] = SourceLegacy
	}

	info, statErr := os.Stat(harvester.Path)
	if errors.Is(statErr, fs.ErrNotExist) {
		return nil
	}
	if statErr != nil {
		return fmt.Errorf("inspect harvester config %s: %w", harvester.Path, statErr)
	}
	content, err := os.ReadFile(harvester.Path)
	if err != nil {
		return fmt.Errorf("read harvester config %s: %w", harvester.Path, err)
	}
	harvester.Exists = true
	var raw rawHarvester
	if err := decodeStrict(content, &raw); err != nil {
		return configJSONError(harvester.Path, err, int64(len(content)))
	}
	path := harvester.Path
	file := func(key string) { result.Sources["harvester."+key] = SourceFile }
	if raw.Enabled != nil {
		harvester.Enabled = *raw.Enabled
		file("enabled")
	}
	if external := raw.External; external != nil {
		if external.Enabled != nil {
			harvester.External.Enabled = *external.Enabled
			file("external.enabled")
		}
		if external.Host != nil {
			host := strings.TrimSpace(*external.Host)
			if host == "" || strings.ContainsAny(host, " /\x00") {
				return fmt.Errorf("harvester config %s: external.host must be a bare host or IP, got %q", path, *external.Host)
			}
			harvester.External.Host = host
			file("external.host")
		}
		if external.Port != nil {
			if *external.Port < 1 || *external.Port > 65535 {
				return fmt.Errorf("harvester config %s: external.port must be between 1 and 65535", path)
			}
			harvester.External.Port = *external.Port
			file("external.port")
		}
		if external.PublicURL != nil {
			value := strings.TrimRight(strings.TrimSpace(*external.PublicURL), "/")
			if value != "" {
				if err := validateHTTPURL(value, false); err != nil {
					return fmt.Errorf("harvester config %s: external.publicURL %w", path, err)
				}
			}
			harvester.External.PublicURL = value
			file("external.publicURL")
		}
		if external.Auth != nil {
			if external.Auth.Passphrase != nil {
				harvester.External.Passphrase = *external.Auth.Passphrase
				file("external.auth.passphrase")
			}
			if external.Auth.StaticToken != nil {
				harvester.External.StaticToken = strings.TrimSpace(*external.Auth.StaticToken)
				file("external.auth.staticToken")
			}
		}
		if external.StateDir != nil && strings.TrimSpace(*external.StateDir) != "" {
			dir, err := expandHomePath(strings.TrimSpace(*external.StateDir), home)
			if err != nil {
				return fmt.Errorf("harvester config %s: external.stateDir %w", path, err)
			}
			harvester.External.StateDir = dir
			file("external.stateDir")
		}
	}
	if search := raw.Search; search != nil {
		if search.Enabled != nil {
			harvester.Search.Enabled = *search.Enabled
			file("search.enabled")
		}
		if search.SearXNGURL != nil {
			value := strings.TrimRight(strings.TrimSpace(*search.SearXNGURL), "/")
			if value != "" {
				if err := validateHTTPURL(value, true); err != nil {
					return fmt.Errorf("harvester config %s: search.searxngURL %w", path, err)
				}
			}
			harvester.Search.SearXNGURL = value
			file("search.searxngURL")
		}
		if search.BraveAPIKey != nil {
			harvester.Search.BraveAPIKey = strings.TrimSpace(*search.BraveAPIKey)
			file("search.braveApiKey")
		}
	}
	if scholarly := raw.Scholarly; scholarly != nil {
		for key, pair := range map[string]struct {
			raw    *string
			target *string
		}{
			"contactEmail":          {scholarly.ContactEmail, &harvester.Scholarly.ContactEmail},
			"googleBooksApiKey":     {scholarly.GoogleBooksAPIKey, &harvester.Scholarly.GoogleBooksAPIKey},
			"coreApiKey":            {scholarly.CoreAPIKey, &harvester.Scholarly.CoreAPIKey},
			"semanticScholarApiKey": {scholarly.SemanticScholarAPIKey, &harvester.Scholarly.SemanticScholarAPIKey},
		} {
			if pair.raw != nil {
				*pair.target = strings.TrimSpace(*pair.raw)
				file("scholarly." + key)
			}
		}
	}
	if fetch := raw.Fetch; fetch != nil {
		if fetch.Browser != nil {
			harvester.Fetch.Browser = *fetch.Browser
			file("fetch.browser")
		}
		if fetch.UserAgent != nil {
			if strings.ContainsAny(*fetch.UserAgent, "\r\n\x00") {
				return fmt.Errorf("harvester config %s: fetch.userAgent must be one line", path)
			}
			harvester.Fetch.UserAgent = strings.TrimSpace(*fetch.UserAgent)
			file("fetch.userAgent")
		}
		if fetch.ProxyURL != nil {
			value := strings.TrimSpace(*fetch.ProxyURL)
			if value != "" {
				parsed, err := url.Parse(value)
				if err != nil || parsed.Scheme == "" || parsed.Host == "" {
					return fmt.Errorf("harvester config %s: fetch.proxyURL must be an absolute proxy URL, got %q", path, value)
				}
			}
			harvester.Fetch.ProxyURL = value
			file("fetch.proxyURL")
		}
	}
	if convert := raw.Convert; convert != nil {
		if convert.PDFOCR != nil {
			harvester.Convert.PDFOCR = *convert.PDFOCR
			file("convert.pdfOcr")
		}
		if convert.PDFLayout != nil {
			harvester.Convert.PDFLayout = *convert.PDFLayout
			file("convert.pdfLayout")
		}
	}
	if cache := raw.Cache; cache != nil {
		if cache.Dir != nil && strings.TrimSpace(*cache.Dir) != "" {
			dir, err := expandHomePath(strings.TrimSpace(*cache.Dir), home)
			if err != nil {
				return fmt.Errorf("harvester config %s: cache.dir %w", path, err)
			}
			harvester.Cache.Dir = dir
			file("cache.dir")
		}
		for key, pair := range map[string]struct {
			raw    *int
			target *time.Duration
		}{
			"ttlSeconds":                  {cache.TTLSeconds, &harvester.Cache.TTL},
			"negativeTtlSeconds":          {cache.NegativeTTLSeconds, &harvester.Cache.NegativeTTL},
			"negativeTransientTtlSeconds": {cache.NegativeTransientTTLSeconds, &harvester.Cache.NegativeTransientTTL},
		} {
			if pair.raw == nil {
				continue
			}
			if *pair.raw < 0 {
				return fmt.Errorf("harvester config %s: cache.%s must be 0 or more (0 = never expire / never cache failures), got %d", path, key, *pair.raw)
			}
			*pair.target = time.Duration(*pair.raw) * time.Second
			file("cache." + key)
		}
	}
	if raw.Output != nil && raw.Output.MaxInlineChars != nil {
		if *raw.Output.MaxInlineChars < 1 {
			return fmt.Errorf("harvester config %s: output.maxInlineChars must be at least 1, got %d", path, *raw.Output.MaxInlineChars)
		}
		harvester.Output.MaxInlineChars = *raw.Output.MaxInlineChars
		file("output.maxInlineChars")
	}

	if harvester.External.Enabled {
		if harvester.External.PublicURL == "" {
			return fmt.Errorf("harvester config %s: external.enabled requires external.publicURL (the URL clients reach the gateway at)", path)
		}
		if harvester.External.Passphrase == "" && harvester.External.StaticToken == "" {
			return fmt.Errorf("harvester config %s: external.enabled requires external.auth.passphrase and/or external.auth.staticToken — the external gateway is never unauthenticated", path)
		}
	}
	if harvester.holdsSecret() && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("harvester config %s holds secrets but is readable by others (mode %04o); run: chmod 600 %s", path, info.Mode().Perm(), path)
	}
	return nil
}

func (harvester HarvesterConfig) holdsSecret() bool {
	return harvester.External.Passphrase != "" || harvester.External.StaticToken != "" ||
		harvester.Search.BraveAPIKey != "" || harvester.Scholarly.GoogleBooksAPIKey != "" ||
		harvester.Scholarly.CoreAPIKey != "" || harvester.Scholarly.SemanticScholarAPIKey != ""
}

// validateHTTPURL accepts an absolute http(s) URL with a host and no
// userinfo. allowPath permits a sub-path mount (a SearXNG behind a prefix).
func validateHTTPURL(value string, allowPath bool) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("must be an absolute http(s) URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("must use http or https, got %q", value)
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("must include a host, got %q", value)
	}
	if parsed.User != nil {
		return fmt.Errorf("must not carry userinfo, got %q", value)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must not carry a query or fragment, got %q", value)
	}
	if !allowPath && parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("must be an origin without a path, got %q", value)
	}
	return nil
}

// MCPServerSource reports where a registered server's enabled flag came from.
// The harvester's lives in harvester.config.json.
func (config Config) MCPServerSource(name string) Source {
	if name == "harvester" {
		return config.Source("harvester.enabled")
	}
	return config.Source("mcp.servers." + name + ".enabled")
}

// Migration is the one-time move from the pre-split layout: config.json →
// pfm.config.json, mcp.servers.harvester → harvester.config.json, and the
// init-written loopback port 8377 → 18377. `pfm install` plans it in the
// preview and applies it before it wires clients, so registrations and the
// daemon agree on the port.
type Migration struct {
	LegacyPath string // non-empty: rename this file to Path
	Path       string
	// StrayLegacyPath is a pre-split config.json left beside an existing
	// pfm.config.json — an interrupted migration's leftover; it is parked.
	StrayLegacyPath string
	// HarvesterEnabled is the legacy mcp.servers.harvester.enabled to move.
	HarvesterEnabled *bool
	MovePort         bool
	// PortKept names why an init-written 8377 stays: its target collides.
	PortKept        string
	harvesterPath   string
	harvesterExists bool
}

// Empty reports whether the machine already has the current layout.
func (migration Migration) Empty() bool {
	return migration.LegacyPath == "" && migration.StrayLegacyPath == "" && migration.HarvesterEnabled == nil && !migration.MovePort
}

func (migration Migration) rewrites() bool {
	return migration.LegacyPath != "" || migration.HarvesterEnabled != nil || migration.MovePort
}

// Steps renders each planned change as one human line, in apply order.
func (migration Migration) Steps() []string {
	var steps []string
	if migration.LegacyPath != "" {
		steps = append(steps, fmt.Sprintf("rename %s → %s (pre-split copy kept as %s)", migration.LegacyPath, migration.Path, legacyBackupName))
	}
	if migration.HarvesterEnabled != nil {
		steps = append(steps, fmt.Sprintf("move mcp.servers.harvester.enabled=%t → %s", *migration.HarvesterEnabled, migration.harvesterPath))
	}
	if migration.MovePort {
		steps = append(steps, fmt.Sprintf("move mcp.http.port %d → %d (client registrations re-wire in this same install)", legacyDefaultMCPPort, DefaultMCPPort))
	}
	if migration.PortKept != "" {
		steps = append(steps, migration.PortKept)
	}
	if migration.StrayLegacyPath != "" {
		steps = append(steps, fmt.Sprintf("park leftover pre-split %s as %s (%s already holds the migrated config)", migration.StrayLegacyPath, legacyBackupName, migration.Path))
	}
	return steps
}

// Preview returns config as the installer will wire it after ApplyMigration,
// so an install preview wires exactly the port the apply will. Path stays on
// the file that exists today: the preview's own reads (the retired authToken
// probe among them) must see real content, not a file the apply has yet to write.
func (migration Migration) Preview(config Config) Config {
	if migration.MovePort {
		config.MCP.HTTP.Port = DefaultMCPPort
	}
	return config
}

// PlanMigration inspects the machine's pfm config without modifying it.
func PlanMigration(config Config) (Migration, error) {
	migration := Migration{Path: config.Path}
	if !config.Exists {
		return migration, nil
	}
	if filepath.Base(config.Path) == LegacyFileName {
		migration.LegacyPath = config.Path
		migration.Path = filepath.Join(filepath.Dir(config.Path), FileName)
	} else {
		stray := filepath.Join(filepath.Dir(config.Path), LegacyFileName)
		exists, err := pathExists(stray)
		if err != nil {
			return Migration{}, err
		}
		if exists {
			migration.StrayLegacyPath = stray
		}
	}
	migration.harvesterPath = HarvesterPath(migration.Path)
	top, err := readTopLevel(config.Path)
	if err != nil {
		return Migration{}, err
	}
	mcpObject, err := decodeObject(top["mcp"], config.Path, "mcp")
	if err != nil {
		return Migration{}, err
	}
	servers, err := decodeObject(mcpObject["servers"], config.Path, "mcp.servers")
	if err != nil {
		return Migration{}, err
	}
	if content, found := servers["harvester"]; found {
		var server rawMCPServer
		if err := decodeStrict(content, &server); err != nil {
			return Migration{}, fmt.Errorf("decode config %s mcp.servers.harvester: %w", config.Path, err)
		}
		enabled := false
		if server.Enabled != nil {
			enabled = *server.Enabled
		}
		migration.HarvesterEnabled = &enabled
	}
	if content, found := mcpObject["http"]; found {
		var httpValue rawMCPHTTP
		if err := json.Unmarshal(content, &httpValue); err != nil {
			return Migration{}, fmt.Errorf("decode config %s mcp.http: %w", config.Path, err)
		}
		migration.MovePort = httpValue.Port == legacyDefaultMCPPort
	}
	if external := config.Harvester.External; migration.MovePort && external.Enabled && external.Port == DefaultMCPPort {
		// Moving the loopback port onto the external gateway's port would turn
		// a working machine into one whose config refuses to load.
		migration.MovePort = false
		migration.PortKept = fmt.Sprintf("keep mcp.http.port %d: external.port in %s already uses %d", legacyDefaultMCPPort, migration.harvesterPath, DefaultMCPPort)
	}
	if _, err := os.Stat(migration.harvesterPath); err == nil {
		migration.harvesterExists = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Migration{}, fmt.Errorf("inspect harvester config %s: %w", migration.harvesterPath, err)
	}
	return migration, nil
}

// ApplyMigration performs a planned migration. Every write is atomic; the
// pre-split file is parked only after its successor landed. An interruption
// between the two leaves both files, which the next PlanMigration reports as
// StrayLegacyPath and parks — never silently treated as done.
func ApplyMigration(migration Migration) error {
	if migration.Empty() {
		return nil
	}
	if migration.rewrites() {
		if err := rewriteMigratedConfig(migration); err != nil {
			return err
		}
	}
	park := migration.LegacyPath
	if park == "" {
		park = migration.StrayLegacyPath
	}
	if park == "" {
		return nil
	}
	backup := filepath.Join(filepath.Dir(park), legacyBackupName)
	if exists, err := pathExists(backup); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("park pre-split config %s: %s already exists; compare the two, remove the one you no longer need, and rerun", park, backup)
	}
	if err := os.Rename(park, backup); err != nil {
		return fmt.Errorf("park pre-split config %s as %s: %w", park, backup, err)
	}
	return nil
}

func rewriteMigratedConfig(migration Migration) error {
	source := migration.Path
	if migration.LegacyPath != "" {
		source = migration.LegacyPath
	}
	top, err := readTopLevel(source)
	if err != nil {
		return err
	}
	mcpObject, err := decodeObject(top["mcp"], source, "mcp")
	if err != nil {
		return err
	}
	if migration.HarvesterEnabled != nil {
		if err := moveHarvesterEnabled(migration); err != nil {
			return err
		}
		servers, err := decodeObject(mcpObject["servers"], source, "mcp.servers")
		if err != nil {
			return err
		}
		delete(servers, "harvester")
		if len(servers) == 0 {
			delete(mcpObject, "servers")
		} else {
			mcpObject["servers"], _ = json.Marshal(servers)
		}
	}
	if migration.MovePort {
		mcpObject["http"], _ = json.Marshal(map[string]int{"port": DefaultMCPPort})
	}
	if len(mcpObject) == 0 {
		delete(top, "mcp")
	} else {
		top["mcp"], _ = json.Marshal(mcpObject)
	}
	content, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("encode migrated config %s: %w", migration.Path, err)
	}
	return writeAtomic(migration.Path, append(content, '\n'))
}

func pathExists(path string) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return true, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else {
		return false, fmt.Errorf("inspect %s: %w", path, err)
	}
}

// moveHarvesterEnabled writes the legacy flag into harvester.config.json. An
// existing harvester file that already sets enabled wins; its other keys are
// preserved byte-for-byte in meaning.
func moveHarvesterEnabled(migration Migration) error {
	top := map[string]json.RawMessage{}
	if migration.harvesterExists {
		existing, err := readTopLevel(migration.harvesterPath)
		if err != nil {
			return err
		}
		top = existing
	}
	if _, set := top["enabled"]; !set {
		top["enabled"], _ = json.Marshal(*migration.HarvesterEnabled)
	}
	content, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("encode harvester config %s: %w", migration.harvesterPath, err)
	}
	return writeAtomic(migration.harvesterPath, append(content, '\n'))
}

func readTopLevel(path string) (map[string]json.RawMessage, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	top := map[string]json.RawMessage{}
	if err := json.Unmarshal(content, &top); err != nil {
		return nil, configJSONError(path, err)
	}
	return top, nil
}

func decodeObject(content json.RawMessage, path, key string) (map[string]json.RawMessage, error) {
	object := map[string]json.RawMessage{}
	if len(content) == 0 {
		return object, nil
	}
	if err := json.Unmarshal(content, &object); err != nil {
		return nil, fmt.Errorf("decode config %s %s: %w", path, key, err)
	}
	return object, nil
}

// SetHarvesterEnabled atomically flips enabled in harvester.config.json,
// preserving every other key. Repeating the effective value is a no-op.
func SetHarvesterEnabled(config Config, enabled bool) (bool, error) {
	if config.Harvester.Enabled == enabled && config.Source("harvester.enabled") != SourceLegacy {
		return false, nil
	}
	path := config.Harvester.Path
	if path == "" {
		path = HarvesterPath(config.Path)
	}
	top := map[string]json.RawMessage{}
	if config.Harvester.Exists {
		existing, err := readTopLevel(path)
		if err != nil {
			return false, err
		}
		top = existing
	}
	top["enabled"], _ = json.Marshal(enabled)
	content, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode harvester config %s: %w", path, err)
	}
	if err := writeAtomic(path, append(content, '\n')); err != nil {
		return false, err
	}
	return true, nil
}

// MarshalHarvester renders the effective harvester config as the JSON shape
// of harvester.config.json. redact replaces every secret for display.
func MarshalHarvester(harvester HarvesterConfig, redact bool) ([]byte, error) {
	secret := func(value string) string {
		if redact && value != "" {
			return "<redacted>"
		}
		return value
	}
	value := map[string]any{
		"enabled": harvester.Enabled,
		"external": map[string]any{
			"enabled": harvester.External.Enabled, "host": harvester.External.Host, "port": harvester.External.Port,
			"publicURL": harvester.External.PublicURL, "stateDir": harvester.External.StateDir,
			"auth": map[string]any{
				"passphrase": secret(harvester.External.Passphrase), "staticToken": secret(harvester.External.StaticToken),
			},
		},
		"search": map[string]any{
			"enabled": harvester.Search.Enabled, "searxngURL": harvester.Search.SearXNGURL,
			"braveApiKey": secret(harvester.Search.BraveAPIKey),
		},
		"scholarly": map[string]any{
			"contactEmail":          harvester.Scholarly.ContactEmail,
			"googleBooksApiKey":     secret(harvester.Scholarly.GoogleBooksAPIKey),
			"coreApiKey":            secret(harvester.Scholarly.CoreAPIKey),
			"semanticScholarApiKey": secret(harvester.Scholarly.SemanticScholarAPIKey),
		},
		"fetch": map[string]any{
			"browser": harvester.Fetch.Browser, "userAgent": harvester.Fetch.UserAgent,
			"proxyURL": harvester.Fetch.ProxyURL,
		},
		"convert": map[string]any{"pdfOcr": harvester.Convert.PDFOCR, "pdfLayout": harvester.Convert.PDFLayout},
		"cache": map[string]any{
			"dir":                         harvester.Cache.Dir,
			"ttlSeconds":                  int(harvester.Cache.TTL / time.Second),
			"negativeTtlSeconds":          int(harvester.Cache.NegativeTTL / time.Second),
			"negativeTransientTtlSeconds": int(harvester.Cache.NegativeTransientTTL / time.Second),
		},
		"output": map[string]any{"maxInlineChars": harvester.Output.MaxInlineChars},
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

// WriteDefaultHarvester installs the default harvester.config.json. An
// existing file is protected unless force is requested.
func WriteDefaultHarvester(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("harvester config %s already exists; use --force to overwrite", path)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect harvester config %s: %w", path, err)
	}
	content, err := MarshalHarvester(DefaultHarvester(), false)
	if err != nil {
		return fmt.Errorf("encode harvester defaults: %w", err)
	}
	return writeAtomic(path, content)
}
