package harvest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

func extractSciDBPDF(body []byte) (string, bool) {
	if match := sciDBViewerFileRe.FindSubmatch(body); len(match) > 1 {
		if decoded, err := url.QueryUnescape(string(match[1])); err == nil && decoded != "" {
			return decoded, true
		}
	}
	if doc, err := html.Parse(bytes.NewReader(body)); err == nil {
		var found string
		var walk func(*html.Node, bool)
		walk = func(node *html.Node, inArticle bool) {
			if found != "" {
				return
			}
			article := inArticle || (node.Type == html.ElementNode && strings.EqualFold(nodeAttr(node, "id"), "article"))
			if node.Type == html.ElementNode {
				switch node.Data {
				case "embed", "iframe":
					candidate := nodeAttr(node, "src")
					id := strings.ToLower(nodeAttr(node, "id"))
					if candidate != "" && (id == "pdf" || article || strings.Contains(strings.ToLower(candidate), ".pdf")) {
						found = candidate
					}
				case "object":
					if strings.EqualFold(strings.TrimSpace(nodeAttr(node, "type")), "application/pdf") {
						found = nodeAttr(node, "data")
					}
				}
			}
			for child := node.FirstChild; child != nil && found == ""; child = child.NextSibling {
				walk(child, article)
			}
		}
		walk(doc, false)
		if found != "" {
			return found, true
		}
	}
	if match := sciDBPDFRe.Find(body); match != nil {
		return string(match), true
	}
	return "", false
}

func resolveProviderURL(base, raw string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return "", fmt.Errorf("provider page URL is invalid")
	}
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("provider file URL is invalid: %w", err)
	}
	resolved := baseURL.ResolveReference(ref)
	resolved.Fragment = ""
	if err := assertFetchable(resolved.String(), false); err != nil {
		return "", err
	}
	return resolved.String(), nil
}

func (h *Harvester) fetchSciDBDOI(ctx context.Context, doi string, options FetchOptions) Result {
	if h.settings.sciDBURL == "" {
		return Result{Source: doi, Error: "SciDB is disabled", ErrorKind: "disabled"}
	}
	pageURL := strings.TrimRight(h.settings.sciDBURL, "/") + "/scidb/" + escapeDOIPath(doi)
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	response, err := h.providerGet(providerCtx, pageURL, nil, providerHTMLMaxBody)
	if err != nil {
		return providerResult(doi, "scidb", "page request failed: "+err.Error(), errorKind(err), 0, false, []string{"scidb"})
	}
	if response.status >= 400 {
		challenge := providerChallenge(response.body, response.status)
		kind := "http"
		if challenge {
			kind = "challenge"
		}
		return providerResult(doi, "scidb", fmt.Sprintf("page returned HTTP %d", response.status), kind, response.status, challenge, []string{"scidb"})
	}
	if providerChallenge(response.body, response.status) && !bytes.HasPrefix(response.body, []byte("%PDF-")) {
		return providerResult(doi, "scidb", "page returned a challenge page", "challenge", response.status, true, []string{"scidb"})
	}
	pdfURL, ok := extractSciDBPDF(response.body)
	if !ok {
		return providerResult(doi, "scidb", "page contained no PDF link", "missing", response.status, false, []string{"scidb"})
	}
	basePageURL := response.finalURL
	if basePageURL == "" {
		basePageURL = pageURL
	}
	resolvedPDF, err := resolveProviderURL(basePageURL, pdfURL)
	if err != nil {
		return providerResult(doi, "scidb", "PDF URL refused: "+err.Error(), errorKind(err), response.status, false, []string{"scidb"})
	}
	return h.fetchProviderArtifactWithPolicy(providerCtx, doi, "scidb", resolvedPDF, response.finalURL, "", options, []string{"scidb"}, true)
}

func (h *Harvester) fetchDOIShadow(ctx context.Context, doi string, options FetchOptions) (Result, bool) {
	var last Result
	attempted := false
	if strings.TrimSpace(h.settings.sciDBURL) != "" {
		attempted = true
		last = h.fetchSciDBDOI(ctx, doi, options)
		if last.Error == "" {
			return last, true
		}
	}
	if strings.TrimSpace(h.settings.libGenURL) != "" {
		attempted = true
		last = h.fetchLibGenDOI(ctx, doi, options)
		if last.Error == "" {
			return last, true
		}
	}
	return last, attempted
}

func decodeProviderObjects(body []byte) (map[string]map[string]any, error) {
	var objects map[string]map[string]any
	if err := json.Unmarshal(body, &objects); err == nil && objects != nil {
		return objects, nil
	}
	var empty []any
	if err := json.Unmarshal(body, &empty); err == nil && len(empty) == 0 {
		return map[string]map[string]any{}, nil
	}
	return nil, errors.New("provider JSON response has an unexpected shape")
}

func firstMapString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func extractMD5sFromRecord(record map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(map[string]any)
	walk = func(current map[string]any) {
		if md5 := firstMapString(current, "md5", "MD5"); md5Re.MatchString(md5) {
			md5 = strings.ToLower(md5)
			if !seen[md5] {
				seen[md5] = true
				out = append(out, md5)
			}
		}
		keys := make([]string, 0, len(current))
		for key := range current {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch value := current[key].(type) {
			case map[string]any:
				walk(value)
			case []any:
				for _, item := range value {
					if nested, ok := item.(map[string]any); ok {
						walk(nested)
					}
				}
			}
		}
	}
	walk(record)
	sort.Strings(out)
	return out
}

func (h *Harvester) libGenRecord(ctx context.Context, md5 string) (string, string, error) {
	base := strings.TrimRight(h.settings.libGenURL, "/")
	if base == "" {
		return "", "", errors.New("LibGen is disabled")
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	adsURL := base + "/ads.php?md5=" + url.QueryEscape(md5)
	// The download page requires a same-site navigation Referer. Some mirrors
	// otherwise return HTTP 200 with no body, concealing the available GET link.
	response, err := h.providerGet(providerCtx, adsURL, http.Header{"Referer": {base + "/"}}, providerHTMLMaxBody)
	if err != nil {
		return "", response.finalURL, err
	}
	if response.status >= 400 {
		challenge := providerChallenge(response.body, response.status)
		kind := "http"
		if challenge {
			kind = "challenge"
		}
		return "", response.finalURL, &providerLookupError{message: fmt.Sprintf("LibGen ads page returned HTTP %d", response.status), kind: kind, status: response.status, challenge: challenge}
	}
	if providerChallenge(response.body, response.status) {
		return "", response.finalURL, &providerLookupError{message: "LibGen ads page returned a challenge page", kind: "challenge", status: response.status, challenge: true}
	}
	match := libGenGetRe.FindSubmatch(response.body)
	if len(match) >= 3 {
		getURL := string(match[0])
		parsed, err := url.Parse(getURL)
		if err != nil {
			return "", response.finalURL, &providerLookupError{message: fmt.Sprintf("LibGen download link invalid: %v", err), kind: "malformed", status: response.status}
		}
		if !parsed.IsAbs() {
			baseURL, parseErr := url.Parse(response.finalURL)
			if parseErr != nil {
				return "", response.finalURL, &providerLookupError{message: fmt.Sprintf("LibGen download link base invalid: %v", parseErr), kind: "malformed", status: response.status}
			}
			parsed = baseURL.ResolveReference(parsed)
		}
		parsed.Fragment = ""
		if err := assertFetchable(parsed.String(), false); err != nil {
			return "", response.finalURL, &providerLookupError{message: err.Error(), kind: errorKind(err), status: response.status}
		}
		return parsed.String(), response.finalURL, nil
	}
	// Some mirrors expose the generated endpoint directly instead of rendering
	// the ads page. Probe it once as a bounded fallback; an HTML/empty response
	// is still a provider failure and is never accepted as a file.
	directURL := base + "/get.php?md5=" + url.QueryEscape(md5)
	direct, directErr := h.providerGet(providerCtx, directURL, http.Header{"Referer": {response.finalURL}}, sciHubMaxBytes(h))
	if directErr == nil && direct.status < 400 && bytes.HasPrefix(direct.body, []byte("%PDF-")) {
		return direct.finalURL, response.finalURL, nil
	}
	if directErr != nil {
		return "", response.finalURL, &providerLookupError{message: "LibGen ads page contained no download link; direct get.php failed: " + directErr.Error(), kind: errorKind(directErr), status: response.status}
	}
	challenge := providerChallenge(direct.body, direct.status)
	kind := "missing"
	if challenge {
		kind = "challenge"
	}
	return "", response.finalURL, &providerLookupError{message: "LibGen ads page contained no download link", kind: kind, status: direct.status, challenge: challenge}
}

func (h *Harvester) fetchLibGenMD5(ctx context.Context, source, md5 string, options FetchOptions) Result {
	if !md5Re.MatchString(md5) {
		return providerResult(source, "libgen", "invalid MD5", "invalid", 0, false, []string{"libgen"})
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	fileURL, referer, err := h.libGenRecord(providerCtx, strings.ToLower(md5))
	if err != nil {
		return providerLookupFailure(source, "libgen", err, []string{"libgen"})
	}
	return h.fetchProviderArtifact(providerCtx, source, "libgen", fileURL, referer, md5, options, []string{"libgen"})
}

func (h *Harvester) fetchLibGenDOI(ctx context.Context, doi string, options FetchOptions) Result {
	if h.settings.libGenURL == "" {
		return Result{Source: doi, Error: "LibGen is disabled", ErrorKind: "disabled"}
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	endpoint := strings.TrimRight(h.settings.libGenURL, "/") + "/json.php?object=e&doi=" + url.QueryEscape(doi) + "&addkeys=*"
	response, err := h.providerGet(providerCtx, endpoint, nil, providerHTMLMaxBody)
	if err != nil {
		return providerResult(doi, "libgen", "DOI lookup failed: "+err.Error(), errorKind(err), 0, false, []string{"libgen"})
	}
	if response.status >= 400 {
		challenge := providerChallenge(response.body, response.status)
		kind := "http"
		if challenge {
			kind = "challenge"
		}
		return providerResult(doi, "libgen", fmt.Sprintf("DOI lookup returned HTTP %d", response.status), kind, response.status, challenge, []string{"libgen"})
	}
	if providerChallenge(response.body, response.status) {
		return providerResult(doi, "libgen", "DOI lookup returned a challenge page", "challenge", response.status, true, []string{"libgen"})
	}
	objects, err := decodeProviderObjects(response.body)
	if err != nil {
		return providerResult(doi, "libgen", "DOI lookup failed: "+err.Error(), "malformed", response.status, false, []string{"libgen"})
	}
	keys := make([]string, 0, len(objects))
	for key := range objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var md5s []string
	for _, key := range keys {
		md5s = append(md5s, extractMD5sFromRecord(objects[key])...)
	}
	seen := map[string]bool{}
	var last Result
	attempted := 0
	for _, md5 := range md5s {
		if seen[md5] || attempted >= providerCandidateMax {
			continue
		}
		seen[md5] = true
		attempted++
		result := h.fetchLibGenMD5(providerCtx, doi, md5, options)
		if result.Error == "" {
			return result
		}
		last = result
	}
	if last.Error != "" {
		return last
	}
	return providerResult(doi, "libgen", "catalog has no exact DOI record", "missing", response.status, false, []string{"libgen"})
}

func (h *Harvester) fetchAnnasMD5(ctx context.Context, source, md5 string, options FetchOptions) Result {
	base := strings.TrimRight(h.settings.annasURL, "/")
	if base == "" {
		return Result{Source: source, Error: "Anna's Archive is disabled", ErrorKind: "disabled"}
	}
	if !md5Re.MatchString(md5) {
		return providerResult(source, "annas", "invalid MD5", "invalid", 0, false, []string{"annas"})
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	recordURL := base + "/md5/" + strings.ToLower(md5)
	response, err := h.providerGet(providerCtx, recordURL, nil, providerHTMLMaxBody)
	if err != nil {
		return providerResult(source, "annas", "record lookup failed: "+err.Error(), errorKind(err), 0, false, []string{"annas"})
	}
	if response.status >= 400 {
		challenge := providerChallenge(response.body, response.status)
		kind := "http"
		if challenge {
			kind = "challenge"
		}
		return providerResult(source, "annas", fmt.Sprintf("record lookup returned HTTP %d", response.status), kind, response.status, challenge, []string{"annas"})
	}
	if providerChallenge(response.body, response.status) {
		return providerResult(source, "annas", "record lookup returned a challenge page", "challenge", response.status, true, []string{"annas"})
	}
	cid, ok := extractAnnasCID(response.body)
	if !ok {
		return providerResult(source, "annas", "record contained no keyless IPFS CID", "missing", response.status, false, []string{"annas"})
	}
	var last Result
	for _, gateway := range []string{"https://dweb.link/ipfs/", "https://ipfs.io/ipfs/"} {
		fileURL := gateway + cid
		result := h.fetchProviderArtifactWithPolicy(providerCtx, source, "annas", fileURL, response.finalURL, md5, options, []string{"annas"}, false)
		if result.Error == "" {
			return result
		}
		last = result
	}
	if last.Error != "" {
		return last
	}
	return providerResult(source, "annas", "no IPFS gateway served the record", "missing", response.status, false, []string{"annas"})
}

func extractAnnasCID(body []byte) (string, bool) {
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`\bbaf[a-z2-7]{10,}\b`),
		regexp.MustCompile(`\bQm[1-9A-HJ-NP-Za-km-z]{44}\b`),
	} {
		if match := pattern.Find(body); match != nil {
			return string(match), true
		}
	}
	return "", false
}

func (h *Harvester) fetchProviderRecord(ctx context.Context, source string, options FetchOptions) (Result, bool) {
	match := providerMD5Re.FindStringSubmatch(source)
	if len(match) < 2 {
		return Result{}, false
	}
	u, err := url.Parse(source)
	if err != nil {
		return Result{}, false
	}
	providers := []struct {
		name string
		base string
	}{
		{name: "annas", base: h.settings.annasURL},
		{name: "libgen", base: h.settings.libGenURL},
	}
	for _, provider := range providers {
		baseURL, parseErr := url.Parse(provider.base)
		if parseErr != nil || baseURL.Host == "" || !strings.EqualFold(baseURL.Host, u.Host) {
			continue
		}
		var result Result
		if provider.name == "annas" {
			result = h.fetchAnnasMD5(ctx, source, match[1], options)
		} else {
			result = h.fetchLibGenMD5(ctx, source, match[1], options)
		}
		if result.Error == "" {
			result = h.storeResultAlias(source, source, result, result.Rungs, options)
		}
		return result, true
	}
	return Result{}, false
}

func (r *Resolver) annasSearch(ctx context.Context, query string, limit int) ([]Candidate, error) {
	base := r.configuredProviderBase("annas")
	if base == "" {
		return nil, nil
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	endpoint := base + "/search?q=" + url.QueryEscape(query)
	response, err := r.providerHarvester().providerGet(providerCtx, endpoint, nil, providerHTMLMaxBody)
	if err != nil {
		return nil, err
	}
	if response.status >= 400 {
		if providerChallenge(response.body, response.status) {
			return nil, errors.New("Anna's search returned a challenge page")
		}
		return nil, fmt.Errorf("Anna's search returned HTTP %d", response.status)
	}
	return providerRecordCandidates(response.body, response.finalURL, "annas", query, limit)
}

func (r *Resolver) libGenSearch(ctx context.Context, query string, limit int) ([]Candidate, error) {
	base := r.configuredProviderBase("libgen")
	if base == "" {
		return nil, nil
	}
	providerCtx, cancel := providerContext(ctx)
	defer cancel()
	endpoints := []string{base + "/index.php?req=" + url.QueryEscape(query)}
	endpoints = append(endpoints, base+"/search.php?req="+url.QueryEscape(query)+"&column=title")
	var lastErr error
	cleanMiss := false
	for _, endpoint := range endpoints {
		response, err := r.providerHarvester().providerGet(providerCtx, endpoint, nil, providerHTMLMaxBody)
		if err != nil {
			lastErr = err
			continue
		}
		if response.status >= 400 {
			if providerChallenge(response.body, response.status) {
				lastErr = errors.New("LibGen search returned a challenge page")
			} else {
				lastErr = fmt.Errorf("LibGen search returned HTTP %d", response.status)
			}
			continue
		}
		candidates, parseErr := providerRecordCandidates(response.body, response.finalURL, "libgen", query, limit)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		if len(candidates) > 0 {
			return candidates, nil
		}
		cleanMiss = true
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if cleanMiss {
		return nil, nil
	}
	return nil, nil
}

func providerRecordCandidates(body []byte, pageURL, source, query string, limit int) ([]Candidate, error) {
	if limit <= 0 || limit > providerCandidateMax {
		limit = providerCandidateMax
	}
	if providerChallenge(body, http.StatusOK) {
		return nil, errors.New("provider search returned a challenge page")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("provider search HTML malformed: %w", err)
	}
	page, err := url.Parse(pageURL)
	if err != nil || page.Host == "" {
		return nil, errors.New("provider search returned an invalid final URL")
	}
	seen := map[string]bool{}
	out := make([]Candidate, 0, limit)
	appendCandidate := func(md5, title, authors string, year int) {
		md5 = strings.ToLower(md5)
		if !md5Re.MatchString(md5) || seen[md5] || len(out) >= limit {
			return
		}
		seen[md5] = true
		u := *page
		u.Path = "/md5/" + md5
		u.RawQuery = ""
		u.Fragment = ""
		out = append(out, Candidate{URL: u.String(), Source: source, Priority: 90, Kind: "book", Title: strings.TrimSpace(title), Authors: strings.TrimSpace(authors), Year: year, Match: .5})
	}

	// LibGen renders records as table rows. Associate the md5 download link with
	// the title/author/year cells in that same row, rather than inventing the
	// caller's query as metadata.
	var walkRows func(*html.Node)
	walkRows = func(node *html.Node) {
		if len(out) >= limit {
			return
		}
		if node.Type == html.ElementNode && node.Data == "tr" {
			var cells []*html.Node
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.ElementNode && child.Data == "td" {
					cells = append(cells, child)
				}
			}
			if len(cells) > 0 {
				title := ""
				for _, anchor := range descendantsByTag(cells[0], "a") {
					if strings.Contains(nodeAttr(anchor, "href"), "edition.php") && strings.TrimSpace(nodeText(anchor)) != "" {
						title = nodeText(anchor)
						break
					}
				}
				authors := ""
				if len(cells) > 1 {
					authors = nodeText(cells[1])
				}
				year := 0
				if len(cells) > 3 {
					if match := scholarYearRe.FindString(nodeText(cells[3])); match != "" {
						fmt.Sscanf(match, "%d", &year)
					}
				}
				for _, anchor := range descendantsByTag(node, "a") {
					if md5 := providerMD5FromHref(nodeAttr(anchor, "href"), source); md5 != "" {
						appendCandidate(md5, title, authors, year)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walkRows(child)
		}
	}
	walkRows(doc)

	// Anna's cards and legacy LibGen layouts do not always use a table. Their
	// record anchor itself still carries the md5; use its visible label only.
	var walkAnchors func(*html.Node)
	walkAnchors = func(node *html.Node) {
		if len(out) >= limit {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" {
			if md5 := providerMD5FromHref(nodeAttr(node, "href"), source); md5 != "" {
				appendCandidate(md5, nodeText(node), "", 0)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walkAnchors(child)
		}
	}
	walkAnchors(doc)
	if len(out) > 0 {
		return out, nil
	}
	text := strings.ToLower(nodeText(doc))
	for _, marker := range []string{"no results", "no records", "nothing found", "0 results", "0 files", "not found"} {
		if strings.Contains(text, marker) {
			return nil, nil
		}
	}
	return nil, errors.New("provider search page contained no recognizable records")
}

func providerMD5FromHref(raw, source string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if match := providerMD5Re.FindStringSubmatch(u.Path); len(match) > 1 {
		return string(match[1])
	}
	if source != "libgen" {
		return ""
	}
	if !strings.Contains(strings.ToLower(u.Path), "ads.php") && !strings.Contains(strings.ToLower(u.Path), "get.php") && !strings.Contains(strings.ToLower(u.Path), "file.php") && !strings.Contains(strings.ToLower(u.Path), "index.php") {
		return ""
	}
	md5 := u.Query().Get("md5")
	if md5Re.MatchString(md5) {
		return md5
	}
	return ""
}
