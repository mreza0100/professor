// Package harvest is the pure-Go transport and policy core for Harvester.
//
// It deliberately does not convert documents itself. Conversion is supplied by
// Converter so the existing Python worker can remain the one implementation of
// PDF/Office/HTML extraction while Go owns transport, policy, and caching.
package harvest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrBrowserPolicyDenied marks a browser fetch the SSRF guard refused — a
// private or internal address. It is POLICY, not an outage: the terminal
// message must never tell the caller to retry a permanent refusal, and it
// must never read as proof of IP reputation.
var ErrBrowserPolicyDenied = errors.New("fetch refused by policy (private or internal address)")

// Converter turns one fetched body into markdown. Implementations must not
// mutate body. kind is the detected content kind and source is the original
// source URL/path.
type Converter interface {
	Convert(ctx context.Context, kind string, source string, body []byte) (string, error)
}

// OCRConverter is implemented by converters that can force one OCR pass for a
// single document (the dispatch escalation rung for scanned PDFs whose text
// layer converts empty). Optional: a plain Converter simply never escalates.
type OCRConverter interface {
	ConvertOCR(ctx context.Context, kind, source string, body []byte) (string, error)
}

// BrowserFetcher is implemented by adapters that can render one URL in a real
// browser (the ladder's last wall-bypass rung). Optional: a plain Converter
// never escalates to it.
type BrowserFetcher interface {
	FetchBrowser(ctx context.Context, source string) (html string, status int, err error)
}

// Options configures a Harvester. Nil HTTP clients use safe defaults.
// Every TTL field treats 0 as "use the default" and any NEGATIVE value as an
// explicit zero: CacheTTL < 0 never expires cached documents, NegativeTTL /
// NegativeTransientTTL < 0 never cache failures.
type Options struct {
	CacheDir       string
	CacheTTL       time.Duration
	Client         *http.Client
	Chrome         *http.Client
	Jina           *http.Client
	OA             *http.Client
	Converter      Converter
	MaxBytes       int64
	LocalRoots     []string
	JinaURL        string
	MaxInlineChars int
	// ProxyURL, when set, is applied to every default transport (direct,
	// Chrome, binary, Jina, and OA). UserAgent customizes only direct/Jina;
	// Chrome keeps its captured impersonation profile and OA keeps its polite
	// scholarly identity.
	ProxyURL  string
	UserAgent string
	// ResolvePublic is called once for each production dial. It is injectable
	// for deterministic DNS-rebinding tests; nil uses net.LookupIP.
	ResolvePublic func(context.Context, string) ([]net.IP, error)
	// BrowserRung opts the real-browser rung in (harvester.config.json
	// fetch.browser). The rung is OFF by default: nil or false never starts
	// the browser worker.
	BrowserRung *bool
	// NegativeTTL / NegativeTransientTTL bound the failure caches; zero uses
	// the defaults (120s / 15s).
	NegativeTTL          time.Duration
	NegativeTransientTTL time.Duration
	// Scholarly/search settings come from harvester.config.json through the
	// adapter. The core reads no process environment.
	ContactEmail          string
	GoogleBooksAPIKey     string
	CoreAPIKey            string
	SemanticScholarAPIKey string
	SearXNGURL            string
	BraveAPIKey           string
	DisableSearch         bool
}

// settings is the resolved scholarly/search/browser configuration New takes
// from Options — the one place the ladder reads it from.
type settings struct {
	contactEmail       string
	googleBooksAPIKey  string
	coreAPIKey         string
	semanticScholarKey string
	searXNGURL         string
	braveAPIKey        string
	disableSearch      bool
	browser            bool
}

// resolveTTL maps the Options TTL convention onto the cache's: 0 selects the
// default, a negative value is an explicit zero.
func resolveTTL(value, fallback time.Duration) time.Duration {
	switch {
	case value == 0:
		return fallback
	case value < 0:
		return 0
	default:
		return value
	}
}

// Default cache policy, used when an Options field is zero.
const (
	defaultCacheTTL             = 24 * time.Hour
	defaultNegativeTTL          = 120 * time.Second
	defaultNegativeTransientTTL = 15 * time.Second
	defaultMaxInlineChars       = 50000
)

// FetchOptions controls one fetch. Refresh bypasses both positive and
// negative caches; SizeOnly still fetches/caches the complete artifact.
type FetchOptions struct {
	Refresh  bool
	SizeOnly bool
}

// Result is deliberately JSON-friendly so the MCP adapter can return it
// without knowing transport internals.
type Result struct {
	Source       string   `json:"source"`
	Kind         string   `json:"kind,omitempty"`
	Content      string   `json:"content,omitempty"`
	Path         string   `json:"path,omitempty"`
	Method       string   `json:"method,omitempty"`
	CacheStatus  string   `json:"cache_status,omitempty"`
	Bytes        int64    `json:"bytes,omitempty"`
	Chars        int      `json:"chars,omitempty"`
	ContentChars int      `json:"content_chars,omitempty"`
	Tokens       int      `json:"tokens,omitempty"`
	Rungs        []string `json:"rungs,omitempty"`
	Error        string   `json:"error,omitempty"`
	ErrorKind    string   `json:"error_kind,omitempty"`
	Challenge    bool     `json:"challenge,omitempty"`
	HTTPStatus   int      `json:"http_status,omitempty"`
	Members      []Member `json:"members,omitempty"`
}

// Harvester owns transport, policy and cache state.
type Harvester struct {
	options      Options
	client       *http.Client
	chrome       *http.Client
	binaryDirect *http.Client
	binaryChrome *http.Client
	jina         *http.Client
	oa           *http.Client
	userAgent    string
	cache        *Cache
	neg          *negativeCache
	flightMu     sync.Mutex
	flights      map[string]*fetchFlight
	settings     settings
}

type fetchFlight struct {
	done   chan struct{}
	result Result
}

// New constructs a Harvester. A nil Converter is valid for callers that only
// need archive listing, search, or raw transport tests; converted fetches then
// report a useful error instead of silently returning bytes. It fails when no
// CacheDir was given and the one default (<home>/.professor/.cache) cannot be
// resolved — never by caching somewhere else.
func New(options Options) (*Harvester, error) {
	resolved := settings{
		contactEmail:       strings.TrimSpace(options.ContactEmail),
		googleBooksAPIKey:  strings.TrimSpace(options.GoogleBooksAPIKey),
		coreAPIKey:         strings.TrimSpace(options.CoreAPIKey),
		semanticScholarKey: strings.TrimSpace(options.SemanticScholarAPIKey),
		searXNGURL:         strings.TrimSpace(options.SearXNGURL),
		braveAPIKey:        strings.TrimSpace(options.BraveAPIKey),
		disableSearch:      options.DisableSearch,
		browser:            options.BrowserRung != nil && *options.BrowserRung,
	}
	if options.CacheDir == "" {
		dir, err := defaultCacheDir()
		if err != nil {
			return nil, err
		}
		options.CacheDir = dir
	}
	options.CacheTTL = resolveTTL(options.CacheTTL, defaultCacheTTL)
	options.NegativeTTL = resolveTTL(options.NegativeTTL, defaultNegativeTTL)
	options.NegativeTransientTTL = resolveTTL(options.NegativeTransientTTL, defaultNegativeTransientTTL)
	if options.MaxBytes <= 0 {
		options.MaxBytes = 50 * 1024 * 1024
	}
	if options.MaxInlineChars <= 0 {
		options.MaxInlineChars = defaultMaxInlineChars
	}
	if options.JinaURL == "" {
		options.JinaURL = "https://r.jina.ai/"
	}
	client := options.Client
	customClient := client != nil
	if client == nil {
		client = safeHTTPClient(false, options.ResolvePublic)
	}
	chrome := options.Chrome
	// A legacy adapter may pass one ordinary client in every slot. Never let
	// that silently demote the Chrome rung: equal Client/Chrome pointers mean
	// "unspecified Chrome" and are replaced by the production uTLS client.
	if chrome != nil && chrome == client {
		chrome = nil
	}
	customChrome := chrome != nil
	if chrome == nil {
		chrome = safeHTTPClient(true, options.ResolvePublic)
	}
	binaryDirect := client
	binaryChrome := chrome
	if !customClient {
		binaryDirect = safeHTTPClientTimeout(false, 60*time.Second, options.ResolvePublic)
	}
	if !customChrome {
		binaryChrome = safeHTTPClientTimeout(true, 60*time.Second, options.ResolvePublic)
	}
	jina := options.Jina
	if jina == nil {
		jina = safeHTTPClient(false, options.ResolvePublic)
	}
	oa := options.OA
	if oa == nil {
		if customClient {
			oa = client
		} else {
			// OA metadata calls use the oracle's short 15-second provider
			// timeout; body/document rungs retain their 30/45/60-second tiers.
			oa = safeHTTPClientTimeout(false, 15*time.Second, options.ResolvePublic)
		}
	} else if oa == client && customClient {
		// Keep scholarly requests on their own polite-UA client when an
		// adapter aliases OA to its direct transport.
		oa = safeHTTPClient(false, options.ResolvePublic)
	}
	if options.ProxyURL != "" {
		configureProxy(client, options.ProxyURL)
		configureProxy(chrome, options.ProxyURL)
		configureProxy(binaryDirect, options.ProxyURL)
		configureProxy(binaryChrome, options.ProxyURL)
		configureProxy(jina, options.ProxyURL)
		configureProxy(oa, options.ProxyURL)
	}
	userAgent := options.UserAgent
	if userAgent == "" {
		userAgent = defaultUA
	}
	setUserAgent(client, userAgent)
	setUserAgent(binaryDirect, userAgent)
	setUserAgent(jina, userAgent)
	return &Harvester{options: options, client: client, chrome: chrome, binaryDirect: binaryDirect, binaryChrome: binaryChrome, jina: jina, oa: oa,
		userAgent: userAgent, cache: newCache(options.CacheDir, options.CacheTTL), neg: newNegativeCache(options.NegativeTTL, options.NegativeTransientTTL), flights: make(map[string]*fetchFlight), settings: resolved}, nil
}

// Fetch executes one request with default options.
func (h *Harvester) Fetch(ctx context.Context, source string) Result {
	return h.FetchWithOptions(ctx, source, FetchOptions{})
}

// NewChromeClient exposes the production uTLS rung to adapters without
// exposing its transport internals. A nil resolver uses the system resolver.
func NewChromeClient(resolve func(context.Context, string) ([]net.IP, error)) *http.Client {
	return safeHTTPClient(true, resolve)
}
