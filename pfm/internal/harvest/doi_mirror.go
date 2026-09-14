package harvest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const doiMirrorTimeout = 45 * time.Second

// normalizeDOIMirrorURL validates the configured mirror once at startup. The
// value is copied into settings by New, so a running Harvester never consults
// mutable configuration or process environment while a fetch is in flight.
func normalizeDOIMirrorURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid doi-mirror URL %q: expected an http(s) URL", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid doi-mirror URL %q: scheme must be http or https", raw)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("invalid doi-mirror URL %q: URL userinfo is not allowed", raw)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(raw, "#") {
		return "", fmt.Errorf("invalid doi-mirror URL %q: query and fragment are not allowed", raw)
	}
	if err := assertFetchable(raw, false); err != nil {
		return "", fmt.Errorf("invalid doi-mirror URL %q: %w", raw, err)
	}
	return raw, nil
}

type doiMirrorLookup struct {
	body      []byte
	pdfURL    string
	pageURL   string
	status    int
	directPDF bool
}

type doiMirrorFailure struct {
	message   string
	kind      string
	challenge bool
	status    int
}

func (f doiMirrorFailure) result(identifier string, rungs []string) Result {
	return Result{Source: identifier, Error: "doi-mirror: " + f.message, ErrorKind: f.kind,
		Challenge: f.challenge, HTTPStatus: f.status, Rungs: append([]string(nil), rungs...)}
}

func doiMirrorClient(base *http.Client, jar http.CookieJar) *http.Client {
	clone := &http.Client{Jar: jar}
	var existingRedirect func(*http.Request, []*http.Request) error
	if base != nil {
		copy := *base
		clone = &copy
		clone.Jar = jar
		existingRedirect = clone.CheckRedirect
	}
	clone.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("doi-mirror redirect limit exceeded")
		}
		if err := assertFetchable(next.URL.String(), false); err != nil {
			return err
		}
		if existingRedirect != nil {
			return existingRedirect(next, via)
		}
		return nil
	}
	return clone
}

func doiMirrorMaxBytes(h *Harvester) int64 {
	if h != nil && h.options.MaxBytes > 0 {
		return h.options.MaxBytes
	}
	return 50 * 1024 * 1024
}

func readDOIMirrorResponse(resp *http.Response, max int64) ([]byte, int, string, error) {
	if resp == nil {
		return nil, 0, "", errors.New("doi-mirror returned no HTTP response")
	}
	status := resp.StatusCode
	contentType := resp.Header.Get("Content-Type")
	if resp.Body == nil {
		return nil, status, contentType, errors.New("doi-mirror returned an empty response body")
	}
	decoded, closeBody, err := decodedResponseBody(resp)
	if err != nil {
		return nil, status, contentType, err
	}
	defer closeBody()
	body, err := io.ReadAll(io.LimitReader(decoded, max+1))
	if err != nil {
		return nil, status, contentType, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > max {
		return nil, status, contentType, fmt.Errorf("%w (%d bytes)", errResponseTooLarge, max)
	}
	return body, status, contentType, nil
}

func (h *Harvester) doiMirrorLookup(ctx context.Context, identifier string, jar http.CookieJar) (doiMirrorLookup, doiMirrorFailure) {
	base := h.settings.doiMirrorURL
	if base == "" {
		return doiMirrorLookup{}, doiMirrorFailure{message: "provider is disabled", kind: "disabled"}
	}
	if err := assertFetchable(base, false); err != nil {
		return doiMirrorLookup{}, doiMirrorFailure{message: "lookup URL refused: " + err.Error(), kind: errorKind(err)}
	}
	form := url.Values{"request": {identifier}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, strings.NewReader(form.Encode()))
	if err != nil {
		return doiMirrorLookup{}, doiMirrorFailure{message: "could not build lookup request: " + err.Error(), kind: "invalid"}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", h.userAgent)
	client := doiMirrorClient(h.client, jar)
	resp, err := client.Do(req)
	if err != nil {
		return doiMirrorLookup{}, doiMirrorFailure{message: "lookup request failed: " + err.Error(), kind: errorKind(err)}
	}
	body, status, _, err := readDOIMirrorResponse(resp, doiMirrorMaxBytes(h))
	pageURL := base
	if resp.Request != nil && resp.Request.URL != nil {
		pageURL = resp.Request.URL.String()
	}
	if err != nil {
		kind := errorKind(err)
		if strings.Contains(err.Error(), "exceeds") {
			kind = "too_large"
		}
		return doiMirrorLookup{}, doiMirrorFailure{message: "lookup response failed: " + err.Error(), kind: kind, status: status}
	}
	if status < 400 && bytes.HasPrefix(body, []byte("%PDF-")) {
		return doiMirrorLookup{body: body, pageURL: pageURL, status: status, directPDF: true}, doiMirrorFailure{}
	}
	if doiMirrorChallenge(body, status) {
		return doiMirrorLookup{}, doiMirrorFailure{message: fmt.Sprintf("lookup returned a CAPTCHA or bot challenge (HTTP %d)", status), kind: "challenge", challenge: true, status: status}
	}
	if status >= 400 {
		return doiMirrorLookup{}, doiMirrorFailure{message: fmt.Sprintf("lookup returned HTTP %d", status), kind: "http", status: status}
	}
	pdfURL, err := doiMirrorPDFLink(body, pageURL)
	if err != nil {
		return doiMirrorLookup{}, doiMirrorFailure{message: err.Error(), kind: "missing_pdf", status: status}
	}
	return doiMirrorLookup{pdfURL: pdfURL, pageURL: pageURL, status: status}, doiMirrorFailure{}
}

func (h *Harvester) doiMirrorDownload(ctx context.Context, lookup doiMirrorLookup, jar http.CookieJar, rungs *[]string) ([]byte, int, doiMirrorFailure) {
	if lookup.pdfURL == "" {
		return nil, lookup.status, doiMirrorFailure{message: "lookup contained no supported PDF URL", kind: "missing_pdf", status: lookup.status}
	}
	var first *doiMirrorFailure
	for attempt, base := range []*http.Client{h.binaryDirectOrClient(), h.binaryChromeOrChrome()} {
		if attempt == 1 {
			*rungs = append(*rungs, "doi-mirror:chrome")
		}
		if err := assertFetchable(lookup.pdfURL, false); err != nil {
			failure := doiMirrorFailure{message: "PDF URL refused: " + err.Error(), kind: errorKind(err)}
			if attempt == 0 {
				return nil, 0, failure
			}
			return nil, 0, mergeDOIMirrorFailures(first, failure)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookup.pdfURL, nil)
		if err != nil {
			failure := doiMirrorFailure{message: "could not build PDF request: " + err.Error(), kind: "invalid"}
			if attempt == 0 {
				return nil, 0, failure
			}
			return nil, 0, mergeDOIMirrorFailures(first, failure)
		}
		req.Header.Set("Accept", "application/pdf,application/octet-stream;q=0.9,*/*;q=0.5")
		req.Header.Set("User-Agent", h.userAgent)
		req.Header.Set("Referer", lookup.pageURL)
		resp, err := doiMirrorClient(base, jar).Do(req)
		if err != nil {
			failure := doiMirrorFailure{message: "PDF request failed: " + err.Error(), kind: errorKind(err)}
			if attempt == 0 {
				first = &failure
				continue
			}
			return nil, 0, mergeDOIMirrorFailures(first, failure)
		}
		body, status, _, readErr := readDOIMirrorResponse(resp, doiMirrorMaxBytes(h))
		if readErr != nil {
			failure := doiMirrorFailure{message: "PDF response failed: " + readErr.Error(), kind: errorKind(readErr), status: status}
			if strings.Contains(readErr.Error(), "exceeds") {
				failure.kind = "too_large"
				return nil, status, failure
			}
			if attempt == 0 {
				first = &failure
				continue
			}
			return nil, status, mergeDOIMirrorFailures(first, failure)
		}
		if status < 400 && bytes.HasPrefix(body, []byte("%PDF-")) {
			return body, status, doiMirrorFailure{}
		}
		if doiMirrorChallenge(body, status) {
			failure := doiMirrorFailure{message: fmt.Sprintf("PDF request returned a CAPTCHA or bot challenge (HTTP %d)", status), kind: "challenge", challenge: true, status: status}
			if attempt == 0 {
				first = &failure
				continue
			}
			return nil, status, mergeDOIMirrorFailures(first, failure)
		}
		if status >= 400 {
			failure := doiMirrorFailure{message: fmt.Sprintf("PDF request returned HTTP %d", status), kind: "http", status: status}
			if attempt == 0 {
				first = &failure
				continue
			}
			return nil, status, mergeDOIMirrorFailures(first, failure)
		}
		if !bytes.HasPrefix(body, []byte("%PDF-")) {
			failure := doiMirrorFailure{message: "PDF URL returned non-PDF content", kind: "missing_pdf", status: status}
			if first != nil {
				failure = mergeDOIMirrorFailures(first, failure)
			}
			return nil, status, failure
		}
		return body, status, doiMirrorFailure{}
	}
	return nil, 0, doiMirrorFailure{message: "PDF download failed", kind: "connect"}
}

func mergeDOIMirrorFailures(first *doiMirrorFailure, last doiMirrorFailure) doiMirrorFailure {
	if first == nil {
		return last
	}
	last.message = first.message + "; Chrome retry: " + last.message
	last.challenge = last.challenge || first.challenge
	if last.status == 0 {
		last.status = first.status
	}
	return last
}

func (h *Harvester) fetchDOIMirror(ctx context.Context, identifier string, options FetchOptions) Result {
	if h == nil || h.settings.doiMirrorURL == "" {
		return Result{Source: identifier, Error: "doi-mirror provider is disabled", ErrorKind: "disabled"}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, doiMirrorTimeout)
	defer cancel()
	jar, err := cookiejar.New(nil)
	if err != nil {
		return doiMirrorFailure{message: "could not create cookie jar: " + err.Error(), kind: "connect"}.result(identifier, []string{"doi-mirror"})
	}
	rungs := []string{"doi-mirror"}
	lookup, failure := h.doiMirrorLookup(attemptCtx, identifier, jar)
	if failure.message != "" {
		return failure.result(identifier, rungs)
	}
	pdfBody := lookup.body
	pdfStatus := lookup.status
	pdfSource := lookup.pageURL
	if !lookup.directPDF {
		pdfBody, pdfStatus, failure = h.doiMirrorDownload(attemptCtx, lookup, jar, &rungs)
		if failure.message != "" {
			return failure.result(identifier, rungs)
		}
		pdfSource = lookup.pdfURL
	}
	if !bytes.HasPrefix(pdfBody, []byte("%PDF-")) {
		return doiMirrorFailure{message: "provider returned non-PDF content", kind: "missing_pdf", status: pdfStatus}.result(identifier, rungs)
	}
	converted, err := h.convert(attemptCtx, "pdf", pdfSource, pdfBody)
	if err != nil {
		return doiMirrorFailure{message: "PDF conversion failed: " + err.Error(), kind: "convert", status: pdfStatus}.result(identifier, rungs)
	}
	if strings.TrimSpace(converted) == "" {
		ocrConverter, ok := h.options.Converter.(OCRConverter)
		if !ok {
			return doiMirrorFailure{message: "PDF conversion produced empty text and OCR is unavailable", kind: "convert", status: pdfStatus}.result(identifier, rungs)
		}
		rungs = append(rungs, "ocr")
		converted, err = ocrConverter.ConvertOCR(attemptCtx, "pdf", pdfSource, pdfBody)
		if err != nil {
			return doiMirrorFailure{message: "PDF conversion produced empty text and OCR failed: " + err.Error(), kind: "convert", status: pdfStatus}.result(identifier, rungs)
		}
		if !usableContent(converted, "pdf") {
			return doiMirrorFailure{message: "PDF conversion and OCR produced empty text", kind: "convert", status: pdfStatus}.result(identifier, rungs)
		}
	}
	stored := h.storeResult(pdfSource, "pdf", "doi-mirror", converted, int64(len(pdfBody)), pdfStatus, rungs, options)
	if stored.Error != "" {
		return stored
	}
	stored.Source = identifier
	return stored
}

func doiMirrorChallenge(body []byte, status int) bool {
	if isChallenge(body, status) {
		return true
	}
	low := strings.ToLower(string(body))
	for _, marker := range []string{"altcha-widget", "altcha", "ddos-guard", "ddos guard", "captcha", "cloudflare"} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func doiMirrorPDFLink(body []byte, finalURL string) (string, error) {
	base, err := url.Parse(finalURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("doi-mirror final page URL is invalid")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("doi-mirror page could not be parsed: %w", err)
	}
	var link string
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, inArticle bool) {
		if link != "" {
			return
		}
		article := inArticle || (node.Type == html.ElementNode && nodeAttr(node, "id") == "article")
		if node.Type == html.ElementNode {
			switch node.Data {
			case "embed", "iframe":
				id := strings.ToLower(nodeAttr(node, "id"))
				if id == "pdf" || (article && nodeAttr(node, "src") != "") {
					link = nodeAttr(node, "src")
				}
			case "object":
				if strings.EqualFold(strings.TrimSpace(nodeAttr(node, "type")), "application/pdf") {
					link = nodeAttr(node, "data")
				}
			}
		}
		for child := node.FirstChild; child != nil && link == ""; child = child.NextSibling {
			walk(child, article)
		}
	}
	walk(doc, false)
	if strings.TrimSpace(link) == "" {
		return "", fmt.Errorf("doi-mirror page contained no supported PDF link")
	}
	resolved, err := base.Parse(strings.TrimSpace(link))
	if err != nil || resolved.Scheme == "" || resolved.Host == "" {
		return "", fmt.Errorf("doi-mirror PDF link is invalid")
	}
	resolved.Fragment = ""
	if err := assertFetchable(resolved.String(), false); err != nil {
		return "", fmt.Errorf("doi-mirror PDF link refused: %w", err)
	}
	return resolved.String(), nil
}

func nodeAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}
