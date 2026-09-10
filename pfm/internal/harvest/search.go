package harvest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearchOptions struct {
	SearXNGURL    string
	BraveAPIKey   string
	Lang          string
	Engines       string
	Count         int
	SearXNG       *http.Client
	Brave         *http.Client
	DisableSearch bool
}
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Engine  string `json:"engine,omitempty"`
}

const searchUA = "harvester-mcp/1.0"

// SearchEnabled reports whether a search can run: not disabled, and at least
// one backend configured.
func SearchEnabled(options SearchOptions) bool {
	return !options.DisableSearch && (options.SearXNGURL != "" || options.BraveAPIKey != "")
}

// Search tries configured SearXNG first and Brave second. A configured but
// empty backend returns an empty result with its backend name, while no backend
// is an explicit configuration error. When every configured backend fails the
// error joins each backend's own failure, so the caller sees what broke.
func Search(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	return searchConfigured(ctx, query, options)
}

func searchConfigured(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	if options.DisableSearch {
		return nil, "", errors.New("search is disabled (search.enabled=false in harvester.config.json)")
	}
	if options.SearXNGURL == "" && options.BraveAPIKey == "" {
		return nil, "", errors.New("search is not configured: set search.searxngURL and/or search.braveApiKey in harvester.config.json")
	}
	if options.Count <= 0 {
		options.Count = 8
	}
	if options.Count > 20 {
		options.Count = 20
	}
	var failures []error
	if options.SearXNGURL != "" {
		out, e := searchSearXNG(ctx, query, options)
		if e == nil && len(out) > 0 {
			return out, "searxng", nil
		}
		if e == nil && options.BraveAPIKey == "" {
			return out, "searxng", nil
		}
		if e != nil {
			failures = append(failures, fmt.Errorf("searxng %s: %w", options.SearXNGURL, e))
		}
	}
	if options.BraveAPIKey != "" {
		out, e := searchBrave(ctx, query, options)
		if e == nil {
			return out, "brave", nil
		}
		failures = append(failures, fmt.Errorf("brave: %w", e))
	}
	return nil, "error", errors.Join(failures...)
}

func (h *Harvester) Search(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	if h != nil {
		if options.SearXNGURL == "" {
			options.SearXNGURL = h.settings.searXNGURL
		}
		if options.BraveAPIKey == "" {
			options.BraveAPIKey = h.settings.braveAPIKey
		}
		options.DisableSearch = options.DisableSearch || h.settings.disableSearch
	}
	return searchConfigured(ctx, query, options)
}

// searxngClient trusts exactly one origin: the operator-configured SearXNG.
// The SearXNG URL is configuration, not content — a loopback or LAN SearXNG is
// its normal deployment — so this client skips the private-address pin that
// guards every fetch. The trust cannot travel: the dialer connects only to the
// configured host:port, no proxy is consulted, and every redirect is refused.
// Fetch, Brave, and every ladder rung keep the full SSRF guard.
func searxngClient(configured string, timeout time.Duration) (*http.Client, error) {
	origin, err := url.Parse(configured)
	if err != nil || origin.Hostname() == "" || (origin.Scheme != "http" && origin.Scheme != "https") {
		return nil, fmt.Errorf("configured SearXNG URL %q is not an absolute http(s) URL", configured)
	}
	port := origin.Port()
	if port == "" {
		port = "80"
		if origin.Scheme == "https" {
			port = "443"
		}
	}
	allowed := net.JoinHostPort(origin.Hostname(), port)
	dialer := &net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != allowed {
				return nil, fmt.Errorf("refusing to dial %s: the search client is pinned to the configured SearXNG origin %s", address, allowed)
			}
			return dialer.DialContext(ctx, network, address)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(next *http.Request, _ []*http.Request) error {
			return fmt.Errorf("refusing redirect from the configured SearXNG to %s", next.URL.Redacted())
		},
	}, nil
}

func searchSearXNG(ctx context.Context, q string, o SearchOptions) ([]SearchResult, error) {
	client := o.SearXNG
	if client == nil {
		trusted, err := searxngClient(o.SearXNGURL, 20*time.Second)
		if err != nil {
			return nil, err
		}
		client = trusted
	}
	u := strings.TrimRight(o.SearXNGURL, "/") + "/search?q=" + url.QueryEscape(q) + "&format=json&safesearch=0"
	if o.Lang != "" {
		u += "&language=" + url.QueryEscape(o.Lang)
	}
	if o.Engines != "" {
		u += "&engines=" + url.QueryEscape(o.Engines)
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return nil, fmt.Errorf("build request: %w", e)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", searchUA)
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	const maxSearchBody = 10 * 1024 * 1024
	body, e := io.ReadAll(io.LimitReader(resp.Body, maxSearchBody+1))
	if e != nil {
		return nil, fmt.Errorf("read response: %w", e)
	}
	if len(body) > maxSearchBody {
		return nil, fmt.Errorf("response exceeds %d bytes", maxSearchBody)
	}
	var data struct {
		Results []struct {
			Title, URL, Content string
			Engines             []string
			Engine              string `json:"engine"`
		} `json:"results"`
	}
	if e = json.Unmarshal(body, &data); e != nil {
		return nil, fmt.Errorf("decode SearXNG JSON (is format=json enabled in its settings.yml?): %w", e)
	}
	out := []SearchResult{}
	for _, r := range data.Results {
		if r.URL != "" {
			engine := strings.Join(r.Engines, ",")
			if engine == "" {
				engine = r.Engine
			}
			out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: truncateRunes(r.Content, 300), Engine: engine})
		}
	}
	if len(out) > o.Count {
		out = out[:o.Count]
	}
	return out, nil
}
func searchBrave(ctx context.Context, q string, o SearchOptions) ([]SearchResult, error) {
	client := o.Brave
	if client == nil {
		client = safeHTTPClientTimeout(false, 20*time.Second)
	}
	query := url.Values{"q": {q}, "count": {fmt.Sprint(o.Count)}}
	if o.Lang != "" {
		query.Set("search_lang", o.Lang)
	}
	reqURL := "https://api.search.brave.com/res/v1/web/search?" + query.Encode()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", searchUA)
	req.Header.Set("X-Subscription-Token", o.BraveAPIKey)
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var data struct {
		Web struct {
			Results []struct{ Title, URL, Description string } `json:"results"`
		} `json:"web"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&data); e != nil {
		return nil, e
	}
	out := []SearchResult{}
	for _, r := range data.Web.Results {
		if r.URL != "" {
			out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: truncateRunes(r.Description, 300), Engine: "brave"})
		}
	}
	return out, nil
}

func truncateRunes(value string, max int) string {
	r := []rune(value)
	if len(r) <= max {
		return value
	}
	return string(r[:max])
}
