package harvest

// This file is the protocol boundary for results that leave the harvester
// process.  The transport and cache deliberately retain their provenance for
// diagnostics; this layer turns that private receipt into a separate,
// provenance-free artifact before an adapter can return it.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	publicHandlePrefix = "harvest:"
	publicHandleHexLen = sha256.Size * 2
	publicReadLimit    = 50 * 1024 * 1024
	publicSearchLimit  = 10000
)

var (
	publicHandleRE = regexp.MustCompile(`^harvest:[0-9a-f]{64}$`)
	publicBareDOI  = regexp.MustCompile(`(?i)^10\.\d{4,9}/[-._;()/:A-Z0-9]+$`)
	publicPMID     = regexp.MustCompile(`^(?:pmid:)?\d{7,9}$`)
	publicPMCID    = regexp.MustCompile(`(?i)^(?:pmcid:)?pmc\d+$`)
	publicISBN     = regexp.MustCompile(`(?i)^(?:isbn:)?[0-9x][0-9x -]{8,16}$`)
)

type publicHandleRecord struct {
	Target string `json:"target"`
}

// FetchPublic resolves a private opaque handle or a permitted local path,
// fetches through the existing core, and publishes an isolated result.
func (h *Harvester) FetchPublic(ctx context.Context, source string, options FetchOptions) Result {
	resolved, err := h.ResolvePublicSource(source)
	if err != nil {
		log.Printf("harvest: public source resolution failed for %q: %v", source, err)
		kind := "invalid"
		switch {
		case strings.Contains(err.Error(), "does not exist"):
			kind = "missing"
		case strings.Contains(err.Error(), "not permitted"), strings.Contains(err.Error(), "internal retrieval"):
			kind = "refused"
		case strings.Contains(err.Error(), "cache directory"), strings.Contains(err.Error(), "handle is unavailable"):
			kind = "internal"
		}
		return h.PublicResult(source, Result{Source: source, Error: "public source resolution failed", ErrorKind: kind}, options.SizeOnly)
	}
	return h.PublicResult(source, h.FetchWithOptions(ctx, resolved, options), options.SizeOnly)
}

// PublicResult publishes a core result without exposing acquisition method,
// rung traces, cache metadata, or private filesystem paths.
func (h *Harvester) PublicResult(source string, result Result, sizeOnly bool) Result {
	if result.Error != "" {
		log.Printf("harvest: public result failure for %q: kind=%q status=%d challenge=%t error=%v", source, result.ErrorKind, result.HTTPStatus, result.Challenge, result.Error)
		return PublicFailure(source, result)
	}

	out := publicSuccessSkeleton(source, result)
	var body string
	// Generated converter metadata is removed from the public body. Keep its
	// size in the inline budget so stripping a long private Source line cannot
	// turn a clipped core result into an apparently complete one.
	inlineLimit := h.options.MaxInlineChars
	metadataRemoved := 0
	stripPublicMetadata := func(value string) string {
		before := contentChars(value)
		cleaned := stripGeneratedSourceMetadata(value)
		after := contentChars(cleaned)
		if before > after {
			metadataRemoved += before - after
		}
		return cleaned
	}
	if result.Path != "" {
		raw, err := h.readPublicArtifact(result.Path)
		if err != nil {
			return h.publicExportFailure(source, result, "read cached artifact", err)
		}
		if publicBinaryResult(result.Kind, result.Path, raw) {
			ext, ok := publicBinaryExtension(result.Kind, result.Path, raw)
			if !ok {
				return h.publicExportFailure(source, result, "validate binary artifact", errors.New("unrecognized binary kind"))
			}
			publicPath, err := h.publicArtifactPath(source, result.Kind, result.Path, ext)
			if err != nil {
				return h.publicExportFailure(source, result, "choose public artifact path", err)
			}
			if err := h.writePublicFile(publicPath, raw); err != nil {
				return h.publicExportFailure(source, result, "write public artifact", err)
			}
			body = result.Content
			if strings.EqualFold(result.Kind, "archive") {
				if len(result.Members) > 0 {
					body = h.publicArchiveListing(source, result.Members)
				} else {
					body = h.rewriteArchiveSource(source, result.Source, body)
				}
			}
			if body != "" {
				body, err = h.rewritePublicImages(source, body, result.Path)
				if err != nil {
					return h.publicExportFailure(source, result, "export embedded image", err)
				}
			}
			out.Path = publicPath
			out.Bytes = int64(len(raw))
		} else {
			fetchedAt := ""
			body = string(raw)
			meta, parsed := parseFrontmatter(string(raw))
			if strings.HasPrefix(string(raw), "---\n") && meta["source"] != "harvester" && (meta["url"] != "" || meta["method"] != "" || meta["rungs"] != "") {
				return h.publicExportFailure(source, result, "validate cached artifact metadata", errors.New("artifact provenance is not a harvester document"))
			}
			if meta["source"] == "harvester" {
				body = parsed
				fetchedAt = meta["fetched_at"]
			}
			body = stripPublicMetadata(body)
			body, err = h.rewritePublicImages(source, body, result.Path)
			if err != nil {
				return h.publicExportFailure(source, result, "export embedded image", err)
			}
			publicPath, err := h.publicArtifactPath(source, result.Kind, result.Path, ".md")
			if err != nil {
				return h.publicExportFailure(source, result, "choose public artifact path", err)
			}
			if err := h.writePublicMarkdown(publicPath, body, fetchedAt); err != nil {
				return h.publicExportFailure(source, result, "write public artifact", err)
			}
			out.Path = publicPath
		}
	} else {
		if result.Kind != "archive_member" && result.Kind != "archive" {
			return h.publicExportFailure(source, result, "publish result without complete artifact", errors.New("result has no complete artifact path"))
		}
		body = result.Content
		if strings.TrimSpace(body) == "" {
			return h.publicExportFailure(source, result, "publish empty artifact", errors.New("successful result has no artifact"))
		}
		if strings.EqualFold(result.Kind, "archive") {
			body = h.rewriteArchiveSource(source, result.Source, body)
		}
		body = stripPublicMetadata(body)
		var err error
		body, err = h.rewritePublicImages(source, body, "")
		if err != nil {
			return h.publicExportFailure(source, result, "export embedded image", err)
		}
		publicPath, err := h.publicArtifactPath(source, result.Kind, "", ".md")
		if err != nil {
			return h.publicExportFailure(source, result, "choose public artifact path", err)
		}
		if err := h.writePublicMarkdown(publicPath, body); err != nil {
			return h.publicExportFailure(source, result, "write public artifact", err)
		}
		out.Path = publicPath
	}

	out.Chars = contentChars(body)
	out.ContentChars = out.Chars
	out.Tokens = estimateTokens(body)
	if metadataRemoved > 0 && inlineLimit > 0 {
		inlineLimit -= metadataRemoved
		if inlineLimit < 1 {
			inlineLimit = 1
		}
	}
	out.Content = truncateInline(body, inlineLimit)
	if sizeOnly {
		out.Content = ""
	}
	info, err := os.Stat(out.Path)
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("public artifact is not a regular file")
		}
		return h.publicExportFailure(source, result, "stat public artifact", err)
	}
	out.Bytes = info.Size()
	return out
}

func (h *Harvester) publicArchiveListing(source string, members []Member) string {
	display := h.publicArchiveDisplay(source)
	return formatArchiveListing(display, members)
}

func (h *Harvester) publicArchiveDisplay(source string) string {
	display := strings.TrimSpace(source)
	if strings.HasPrefix(strings.ToLower(display), "http://") || strings.HasPrefix(strings.ToLower(display), "https://") {
		if handle, err := h.PublicHandle(display); err == nil {
			display = handle
		} else {
			display = "requested archive"
		}
	} else if strings.HasPrefix(display, "/") || strings.HasPrefix(strings.ToLower(display), "file://") {
		display = "requested archive"
	}
	return display
}

func (h *Harvester) rewriteArchiveSource(source, resolved, body string) string {
	if strings.TrimSpace(body) == "" {
		return body
	}
	display := h.publicArchiveDisplay(source)
	if strings.TrimSpace(resolved) != "" {
		body = strings.ReplaceAll(body, resolved, display)
	}
	return body
}

// The Python HTML converter adds a short metadata block before the article.
// Remove only its generated Source field, and only before that block's
// separator; article citations later in the document remain untouched.
func stripGeneratedSourceMetadata(body string) string {
	lines := strings.Split(body, "\n")
	separator := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return body
	}
	prefix := make([]string, 0, separator)
	removed := false
	for _, line := range lines[:separator] {
		if strings.HasPrefix(strings.TrimSpace(line), "**Source:**") {
			removed = true
			continue
		}
		prefix = append(prefix, line)
	}
	if !removed {
		return body
	}
	for len(prefix) > 0 && strings.TrimSpace(prefix[len(prefix)-1]) == "" {
		prefix = prefix[:len(prefix)-1]
	}
	suffix := lines[separator+1:]
	for len(suffix) > 0 && strings.TrimSpace(suffix[0]) == "" {
		suffix = suffix[1:]
	}
	return strings.Join(append(prefix, suffix...), "\n")
}

func publicSuccessSkeleton(source string, result Result) Result {
	return Result{
		Source:      source,
		Kind:        publicKind(result.Kind),
		CacheStatus: publicCacheStatus(result.CacheStatus),
		HTTPStatus:  result.HTTPStatus,
		Members:     append([]Member(nil), result.Members...),
	}
}

func publicCacheStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "hit", "miss", "refresh":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "public"
	}
}

func publicKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pdf", "docx", "xlsx", "pptx", "csv", "json", "epub", "html", "txt", "md", "jpg", "png", "gif", "webp", "bmp", "tiff", "svg", "image", "zip", "tar", "7z", "rar", "archive", "archive_member":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return ""
	}
}

func publicBinaryResult(kind, path string, body []byte) bool {
	low := strings.ToLower(strings.TrimSpace(kind))
	if isImageKind(low) || low == "archive" || low == "zip" || low == "tar" || low == "7z" || low == "rar" {
		return true
	}
	if low == "archive_member" {
		return isImageKind(classifyKind(path, "", body)) || isImageKind(SniffMagic(body))
	}
	return false
}

func publicBinaryExtension(kind, path string, body []byte) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(kind))
	byKind := map[string]string{
		"jpg": ".jpg", "png": ".png", "gif": ".gif", "webp": ".webp", "bmp": ".bmp", "tiff": ".tiff", "svg": ".svg",
		"zip": ".zip", "tar": ".tar", "7z": ".7z", "rar": ".rar",
	}
	if ext, ok := byKind[low]; ok {
		return ext, true
	}
	if low == "image" || low == "archive_member" || low == "archive" {
		detected := classifyKind(path, "", body)
		if detected == "html" {
			if magic := SniffMagic(body); magic != "" {
				detected = magic
			}
		}
		if detected == "image" || SniffMagic(body) == "image" {
			ext := strings.ToLower(filepath.Ext(strings.Split(strings.Split(path, "?")[0], "#")[0]))
			switch ext {
			case ".jpg", ".jpeg":
				return ".jpg", true
			case ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".svg":
				if ext == ".tif" {
					return ".tiff", true
				}
				return ext, true
			}
			switch {
			case bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")):
				return ".png", true
			case bytes.HasPrefix(body, []byte("\xff\xd8\xff")):
				return ".jpg", true
			case bytes.HasPrefix(body, []byte("GIF87a")), bytes.HasPrefix(body, []byte("GIF89a")):
				return ".gif", true
			case len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
				return ".webp", true
			case bytes.HasPrefix(body, []byte("BM")):
				return ".bmp", true
			case bytes.HasPrefix(body, []byte("II*\x00")), bytes.HasPrefix(body, []byte("MM\x00*")):
				return ".tiff", true
			case bytes.HasPrefix(bytes.TrimSpace(body), []byte("<svg")), bytes.HasPrefix(bytes.TrimSpace(body), []byte("<?xml")) && bytes.Contains(body, []byte("<svg")):
				return ".svg", true
			}
		}
		for candidate, ext := range map[string]string{"zip": ".zip", "tar": ".tar", "7z": ".7z", "rar": ".rar"} {
			if detected == candidate {
				return ext, true
			}
		}
	}
	return "", false
}

func (h *Harvester) publicExportFailure(source string, result Result, operation string, err error) Result {
	log.Printf("harvest: public export %s failed for %q (path=%q): %v", operation, source, result.Path, err)
	failure := Result{Source: source, Kind: publicKind(result.Kind), ErrorKind: "internal", Error: "public export failed"}
	return PublicFailure(source, failure)
}

// PublicFailure retains only safe failure fields and the caller's input.
// Receipt writers use this boundary when a failure occurs after FetchPublic.
func PublicFailure(source string, result Result) Result {
	kind := publicErrorKind(result)
	out := Result{Source: source, Error: PublicFailureMessage(result), ErrorKind: kind}
	if result.HTTPStatus >= 400 && result.HTTPStatus < 600 {
		out.HTTPStatus = result.HTTPStatus
	}
	if kind == "challenge" {
		out.Challenge = true
	}
	return out
}

func publicErrorKind(result Result) string {
	low := strings.ToLower(strings.TrimSpace(result.ErrorKind))
	switch low {
	case "timeout", "timed_out", "deadline":
		return "timeout"
	case "dns", "name_resolution":
		return "dns"
	case "connect", "connection", "network":
		return "connect"
	case "challenge", "cloudflare", "captcha":
		return "challenge"
	case "blocked", "refused", "policy":
		return "refused"
	case "missing", "not_found", "unresolvable_path":
		return "missing"
	case "conversion", "convert", "ocr":
		return "conversion"
	case "oversized", "too_large", "payload_too_large":
		return "oversized"
	case "cancelled", "canceled":
		return "cancelled"
	case "invalid", "wrong_kind", "unsupported":
		if low == "wrong_kind" {
			return "wrong_kind"
		}
		return "invalid"
	case "internal", "storage", "cache":
		return "internal"
	}
	if result.Challenge {
		return "challenge"
	}
	if result.HTTPStatus == 404 || result.HTTPStatus == 410 {
		return "missing"
	}
	if result.HTTPStatus == 408 || result.HTTPStatus == 504 {
		return "timeout"
	}
	if result.HTTPStatus == 401 || result.HTTPStatus == 403 {
		return "refused"
	}
	err := strings.ToLower(result.Error)
	switch {
	case strings.Contains(err, "context canceled"), strings.Contains(err, "context cancelled"):
		return "cancelled"
	case strings.Contains(err, "no such host"), strings.Contains(err, "dns"):
		return "dns"
	case strings.Contains(err, "timed out"), strings.Contains(err, "timeout"):
		return "timeout"
	case strings.Contains(err, "connection refused"), strings.Contains(err, "connection reset"), strings.Contains(err, "connect:"):
		return "connect"
	case strings.Contains(err, "too large"), strings.Contains(err, "exceeds"), strings.Contains(err, "maximum"):
		return "oversized"
	case strings.Contains(err, "convert"), strings.Contains(err, "ocr"):
		return "conversion"
	case strings.Contains(err, "not found"), strings.Contains(err, "missing"):
		return "missing"
	case strings.Contains(err, "invalid url"), strings.Contains(err, "unsupported url"), strings.Contains(err, "source is empty"):
		return "invalid"
	case strings.Contains(err, "findworks"), strings.Contains(err, "find works"), strings.Contains(err, "title — use"):
		return "ambiguous"
	case strings.Contains(err, "fetchimage"), strings.Contains(err, "fetch image"), strings.Contains(err, "archive tool"), strings.Contains(err, "use the `archive`"):
		return "wrong_kind"
	case strings.Contains(err, "cache"), strings.Contains(err, "storage"), strings.Contains(err, "read local file"):
		return "internal"
	}
	return "failed"
}

// PublicFailureMessage returns a safe, actionable diagnostic. It intentionally
// does not include Result.Source or Result.Error: both can contain a provider
// URL, private path, or a provider's internal wording.
func PublicFailureMessage(result Result) string {
	switch publicErrorKind(result) {
	case "timeout":
		return "The source timed out. Retry later or choose another work."
	case "dns":
		return "The source could not be resolved. Retry later or choose another work."
	case "connect":
		return "The connection failed. Retry later or choose another work."
	case "challenge":
		return "The source is protected by an access challenge. Choose another copy."
	case "refused":
		return "The request was refused by access policy. Use a public URL or choose another copy."
	case "missing":
		return "The requested document was not found. Use findWorks, select a result, and fetch it again."
	case "conversion":
		return "The document could not be converted or OCR'd. Try another copy."
	case "oversized":
		return "The document is too large to process. Choose a smaller copy."
	case "cancelled":
		return "The request was cancelled."
	case "invalid":
		return "The input is invalid. Use findWorks, select a result, and fetch it."
	case "ambiguous":
		return "The title is ambiguous. Use findWorks, select a result, and fetch it."
	case "wrong_kind":
		return "This source is an image or archive. Use fetchImage or archive for this media."
	case "internal":
		return "Harvester could not read or publish its stored result. Retry later."
	default:
		return "Retrieval failed. Retry or choose another work."
	}
}

// PublicCandidates strips provider labels and replaces every location URL
// with a persistent opaque handle. Exact scholarly identifiers remain useful
// identity handles and are never inferred from arbitrary PDF URLs.
func (h *Harvester) PublicCandidates(candidates []Candidate) ([]Candidate, error) {
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		copy := candidate
		copy.Source = ""
		copy.Priority = 0
		if strings.TrimSpace(candidate.URL) != "" {
			if publicIdentityHandle(candidate.URL) {
				copy.URL = strings.TrimSpace(candidate.URL)
			} else {
				handle, err := h.PublicHandle(candidate.URL)
				if err != nil {
					return nil, err
				}
				copy.URL = handle
			}
		}
		out = append(out, copy)
	}
	return out, nil
}

func publicIdentityHandle(source string) bool {
	s := strings.TrimSpace(source)
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	if publicBareDOI.MatchString(s) {
		return true
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "doi:") {
		return publicBareDOI.MatchString(strings.TrimSpace(s[4:]))
	}
	if publicPMID.MatchString(low) || publicPMCID.MatchString(low) {
		return true
	}
	if publicISBN.MatchString(s) {
		value := s
		if strings.HasPrefix(low, "isbn:") {
			value = strings.TrimSpace(s[5:])
		}
		return NormalizeISBN(value) != ""
	}
	parsed, err := url.Parse(s)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.User == nil && parsed.Port() == "" &&
		(strings.EqualFold(parsed.Hostname(), "doi.org") || strings.EqualFold(parsed.Hostname(), "dx.doi.org")) && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Opaque == "" {
		return publicBareDOI.MatchString(strings.TrimPrefix(parsed.Path, "/"))
	}
	return false
}

// PublicHandle validates and stores a public HTTP(S) target behind a stable
// handle. The target is intentionally kept only in the private mapping.
func (h *Harvester) PublicHandle(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("public handle requires a public HTTP(S) URL")
	}
	if err := assertFetchable(source, false); err != nil {
		log.Printf("harvest: public handle rejected %q: %v", source, err)
		return "", errors.New("public handle requires a public HTTP(S) URL")
	}
	root, err := h.cacheRoot()
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(source))
	handle := publicHandlePrefix + hex.EncodeToString(key[:])
	dir := filepath.Join(root, ".private", "handles")
	if err := h.ensurePrivateHandleDir(root, dir); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hex.EncodeToString(key[:])+".json")
	mapping, err := json.Marshal(publicHandleRecord{Target: source})
	if err != nil {
		log.Printf("harvest: public handle mapping encode failed for %q: %v", source, err)
		return "", errors.New("could not create public source handle")
	}
	if err := h.writeAtomic(path, mapping, 0o600); err != nil {
		log.Printf("harvest: public handle mapping write failed for %q: %v", source, err)
		return "", errors.New("could not create public source handle")
	}
	return handle, nil
}

// ResolvePublicSource decodes a handle and confines local requests. Public
// exports are readable; private cache/provenance paths are never readable.
func (h *Harvester) ResolvePublicSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, publicHandlePrefix) {
		if !publicHandleRE.MatchString(source) {
			return "", errors.New("invalid public source handle")
		}
		target, err := h.readPublicHandle(source)
		if err != nil {
			return "", err
		}
		if err := assertFetchable(target, false); err != nil {
			log.Printf("harvest: stored public handle target rejected %q: %v", target, err)
			return "", errors.New("stored public source is no longer fetchable")
		}
		return target, nil
	}
	// Match FetchWithOptions' precedence: an existing/explicit local file wins
	// over identifier syntax (an ISBN-named file is still a file), while a bare
	// DOI such as 10.1234/example must never be mistaken for a missing path.
	if !isDefiniteLocalSource(source) && ClassifyIdentifier(source) != IdentifierNone {
		return source, nil
	}
	if !isLocalSource(source) {
		return source, nil
	}
	path := source
	if strings.HasPrefix(strings.ToLower(path), "file://") {
		decoded, err := fileURLPath(path)
		if err != nil {
			return "", errors.New("invalid local source")
		}
		path = decoded
	}
	canonical, err := canonicalPublicPath(path)
	if err != nil {
		return "", errors.New("local source cannot be resolved")
	}
	root, err := h.cacheRoot()
	if err != nil {
		return "", err
	}
	lexicalPath, lexicalErr := filepath.Abs(filepath.Clean(path))
	if lexicalErr != nil {
		return "", errors.New("local source cannot be resolved")
	}
	lexicalCacheRoot, lexicalErr := filepath.Abs(filepath.Clean(root))
	if lexicalErr != nil {
		return "", errors.New("cache directory cannot be resolved")
	}
	lexicalPublicRoot := filepath.Join(lexicalCacheRoot, "public")
	if isPathInside(lexicalPath, lexicalCacheRoot) && !isPathInside(lexicalPath, lexicalPublicRoot) {
		return "", errors.New("internal retrieval metadata is not available")
	}
	if isPathInside(lexicalPath, lexicalPublicRoot) {
		if symlinked, symlinkErr := symlinkBelow(lexicalPath, lexicalPublicRoot); symlinkErr != nil || symlinked {
			return "", errors.New("public export path is not a safe namespace")
		}
	}
	publicRoot, err := h.publicRoot()
	if err != nil {
		return "", errors.New("public export directory cannot be resolved")
	}
	cacheRoot, err := canonicalPublicPath(root)
	if err != nil {
		return "", errors.New("cache directory cannot be resolved")
	}
	if insideAny(canonical, []string{cacheRoot}) {
		if !insideAny(canonical, []string{publicRoot}) {
			return "", errors.New("internal retrieval metadata is not available")
		}
		if info, statErr := os.Stat(canonical); statErr != nil || !info.Mode().IsRegular() {
			return "", errors.New("public export does not exist")
		}
		return canonical, nil
	}
	if _, statErr := os.Stat(canonical); statErr != nil {
		return "", errors.New("local source does not exist")
	}
	if reason := DenyLocalPath(canonical, h.options.LocalRoots); reason != "" {
		return "", errors.New("local source is not permitted")
	}
	return canonical, nil
}

func (h *Harvester) readPublicHandle(source string) (string, error) {
	root, err := h.cacheRoot()
	if err != nil {
		return "", err
	}
	key := strings.TrimPrefix(source, publicHandlePrefix)
	dir := filepath.Join(root, ".private", "handles")
	if err := ensureNamespaceDir(filepath.Dir(dir), false); err != nil {
		return "", errors.New("public source handle is invalid")
	}
	if err := ensureNamespaceDir(dir, false); err != nil {
		return "", errors.New("public source handle is invalid")
	}
	canonicalDir, dirErr := canonicalPublicPath(dir)
	path := filepath.Join(dir, key+".json")
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("public source handle is invalid")
	}
	canonicalPath, pathErr := canonicalPublicPath(path)
	if dirErr != nil || pathErr != nil || !isPathInside(canonicalPath, canonicalDir) {
		return "", errors.New("public source handle is invalid")
	}
	data, err := readBoundedFile(path, 16*1024)
	if err != nil {
		log.Printf("harvest: public handle mapping read failed for %q: %v", source, err)
		return "", errors.New("public source handle is unavailable")
	}
	var record publicHandleRecord
	if err := json.Unmarshal(data, &record); err != nil || strings.TrimSpace(record.Target) == "" {
		return "", errors.New("public source handle is invalid")
	}
	target := strings.TrimSpace(record.Target)
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", errors.New("public source handle is invalid")
	}
	return target, nil
}

// SearchCachePublic searches private cache entries and exports every match
// before returning it. Public exports are skipped so they cannot consume the
// caller's result limit or become a second search index.
func (h *Harvester) SearchCachePublic(pattern string, maxResults int, ignoreCase bool) ([]CacheSearchResult, error) {
	if maxResults <= 0 {
		maxResults = 50
	}
	entries, err := h.searchPrivateCache(pattern, publicSearchLimit, ignoreCase)
	if err != nil {
		return nil, err
	}
	publicRoot, err := h.publicRoot()
	if err != nil {
		return nil, err
	}
	out := make([]CacheSearchResult, 0, maxResults)
	for _, entry := range entries {
		if isPathInside(entry.Path, publicRoot) {
			continue
		}
		if len(out) >= maxResults {
			break
		}
		identity := strings.TrimSpace(entry.URL)
		if identity == "" || identity == entry.Path {
			return nil, errors.New("cache match has no public document identity")
		}
		exported := h.PublicResult(identity, Result{Source: identity, Path: entry.Path}, false)
		if exported.Error != "" || exported.Path == "" {
			return nil, errors.New("cache match could not be exported safely")
		}
		publicBody, err := h.readPublicBody(exported.Path)
		if err != nil {
			return nil, errors.New("cache match could not be read safely")
		}
		rx, err := publicSearchRegexp(pattern, ignoreCase)
		if err != nil {
			return nil, err
		}
		matches := rx.FindAllString(publicBody, -1)
		if len(matches) == 0 {
			continue
		}
		sample := ""
		for _, line := range strings.Split(publicBody, "\n") {
			if rx.MatchString(line) {
				sample = strings.TrimSpace(line)
				if len([]rune(sample)) > 200 {
					sample = string([]rune(sample)[:200])
				}
				break
			}
		}
		display, err := h.publicDisplayHandle(identity, exported.Path)
		if err != nil {
			return nil, err
		}
		out = append(out, CacheSearchResult{URL: display, Path: exported.Path, Matches: len(matches), Sample: sample})
	}
	return out, nil
}

func (h *Harvester) searchPrivateCache(pattern string, _ int, ignoreCase bool) ([]CacheSearchResult, error) {
	if h == nil || h.cache == nil {
		return nil, errors.New("harvester cache is unavailable")
	}
	rx, err := publicSearchRegexp(pattern, ignoreCase)
	if err != nil {
		return nil, err
	}
	// Walk the resolved cache root so a configured cache directory symlink is
	// valid, while symlinks introduced below that root still fail closed.
	root, rootErr := canonicalPublicPath(h.cache.root)
	if rootErr != nil {
		return nil, fmt.Errorf("search cache: resolve cache directory: %w", rootErr)
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return []CacheSearchResult{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("search cache: %w", err)
	}
	publicRoot := filepath.Join(root, "public")
	privateRoot := filepath.Join(root, ".private")
	out := make([]CacheSearchResult, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			log.Printf("harvest: private cache search found unsafe symlink %q", path)
			return errors.New("private cache search found an unsafe symlink")
		}
		if entry.IsDir() {
			if path != root && isPathInside(path, publicRoot) {
				return fs.SkipDir
			}
			if path != root && isPathInside(path, privateRoot) {
				return fs.SkipDir
			}
			return nil
		}
		if isPathInside(path, publicRoot) || isPathInside(path, privateRoot) {
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		raw, readErr := readBoundedFile(path, h.publicLimit())
		if readErr != nil {
			log.Printf("harvest: private cache search cannot read %q: %v", path, readErr)
			return errors.New("private cache search could not read an artifact")
		}
		meta, body := parseFrontmatter(string(raw))
		if meta["source"] == "harvester" {
			body = stripGeneratedSourceMetadata(body)
		}
		hits := rx.FindAllString(body, -1)
		if len(hits) == 0 {
			return nil
		}
		sample := ""
		for _, line := range strings.Split(body, "\n") {
			if rx.MatchString(line) {
				sample = strings.TrimSpace(line)
				if len([]rune(sample)) > 200 {
					sample = string([]rune(sample)[:200])
				}
				break
			}
		}
		display := meta["url"]
		if display == "" {
			display = path
		}
		out = append(out, CacheSearchResult{URL: display, Path: path, Matches: len(hits), Sample: sample})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search cache: %w", err)
	}
	return out, nil
}

func publicSearchRegexp(pattern string, ignoreCase bool) (*regexp.Regexp, error) {
	flags := pattern
	if ignoreCase {
		flags = "(?i)" + pattern
	}
	rx, err := regexp.Compile(flags)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return rx, nil
}

func (h *Harvester) readPublicBody(path string) (string, error) {
	raw, err := h.readPublicArtifact(path)
	if err != nil {
		return "", err
	}
	if meta, body := parseFrontmatter(string(raw)); meta["source"] == "harvester" {
		return body, nil
	}
	return string(raw), nil
}

func (h *Harvester) publicDisplayHandle(identity, exportedPath string) (string, error) {
	if publicIdentityHandle(identity) {
		return identity, nil
	}
	if isLocalSource(identity) || strings.HasPrefix(strings.ToLower(identity), "file://") {
		return exportedPath, nil
	}
	if err := assertFetchable(identity, false); err != nil {
		log.Printf("harvest: cache identity is not a public URL %q: %v", identity, err)
		return "", errors.New("cache match has no public identity")
	}
	return h.PublicHandle(identity)
}

func (h *Harvester) cacheRoot() (string, error) {
	if h == nil || strings.TrimSpace(h.options.CacheDir) == "" {
		return "", errors.New("harvester cache directory is unavailable")
	}
	root, err := filepath.Abs(filepath.Clean(h.options.CacheDir))
	if err != nil {
		return "", errors.New("harvester cache directory is unavailable")
	}
	return root, nil
}

func (h *Harvester) publicRoot() (string, error) {
	root, err := h.cacheRoot()
	if err != nil {
		return "", err
	}
	return publicNamespace(root, false)
}

// publicNamespace permits the configured cache root itself to be a symlink,
// but never permits the public namespace to be one. Otherwise public/ could
// silently point at .private/ and turn private cache files into public files.
func publicNamespace(root string, create bool) (string, error) {
	path := filepath.Join(root, "public")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if !create {
			return canonicalPublicPath(path)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return "", err
		}
		info, err = os.Lstat(path)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("public directory is not a safe directory")
	}
	if create {
		if err := os.Chmod(path, 0o700); err != nil {
			return "", err
		}
	}
	return canonicalPublicPath(path)
}

func ensureNamespaceDir(path string, create bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if !create {
			return os.ErrNotExist
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("private namespace is not a safe directory")
	}
	if create {
		return os.Chmod(path, 0o700)
	}
	return nil
}

func (h *Harvester) publicArtifactPath(source, kind, oldPath, ext string) (string, error) {
	root, err := h.cacheRoot()
	if err != nil {
		return "", err
	}
	canonical := oldPath
	if oldPath != "" {
		if resolved, err := canonicalPublicPath(oldPath); err == nil {
			canonical = resolved
		}
	}
	key := sha256.Sum256([]byte("harvester-public\x00" + source + "\x00" + kind + "\x00" + canonical))
	return filepath.Join(root, "public", hex.EncodeToString(key[:])+ext), nil
}

func (h *Harvester) writePublicMarkdown(path, body string, fetchedAt ...string) error {
	stamp := time.Now().UTC().Format(time.RFC3339)
	if len(fetchedAt) > 0 {
		candidate := strings.TrimSpace(fetchedAt[0])
		if _, err := time.Parse(time.RFC3339, candidate); err == nil {
			stamp = candidate
		}
	}
	meta := "---\nfetched_at: " + stamp + "\ntoken_count: " + fmt.Sprint(estimateTokens(body)) + "\nsource: harvester\n---\n\n"
	return h.writePublicFile(path, []byte(meta+body))
}

func (h *Harvester) writePublicFile(path string, data []byte) error {
	root, err := h.cacheRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	publicRoot := filepath.Join(root, "public")
	if _, err := publicNamespace(root, true); err != nil {
		return fmt.Errorf("create public directory: %w", err)
	}
	if symlinked, symlinkErr := symlinkBelow(path, publicRoot); symlinkErr != nil || symlinked {
		return errors.New("public artifact path is not a safe namespace")
	}
	canonicalPublic, err := canonicalPublicPath(publicRoot)
	if err != nil {
		return errors.New("public directory cannot be resolved")
	}
	canonicalPath, err := canonicalPublicPath(path)
	if err != nil || !isPathInside(canonicalPath, canonicalPublic) {
		return errors.New("public artifact path escaped its directory")
	}
	return h.writeAtomic(path, data, 0o600)
}

func (h *Harvester) writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".public-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (h *Harvester) ensurePrivateHandleDir(root, dir string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	private := filepath.Dir(dir)
	if err := ensureNamespaceDir(private, true); err != nil {
		return fmt.Errorf("create private namespace: %w", err)
	}
	if err := ensureNamespaceDir(dir, true); err != nil {
		return fmt.Errorf("create private handle directory: %w", err)
	}
	canonical, err := canonicalPublicPath(dir)
	canonicalRoot, rootErr := canonicalPublicPath(root)
	if err != nil || rootErr != nil || !isPathInside(canonical, canonicalRoot) {
		return errors.New("private handle directory escaped the cache")
	}
	return nil
}

func (h *Harvester) readPublicArtifact(path string) ([]byte, error) {
	canonical, err := canonicalPublicPath(path)
	if err != nil {
		return nil, err
	}
	root, err := h.cacheRoot()
	if err != nil {
		return nil, err
	}
	cacheRoot, err := canonicalPublicPath(root)
	if err != nil {
		return nil, err
	}
	lexicalPath, lexicalErr := filepath.Abs(filepath.Clean(path))
	if lexicalErr != nil {
		return nil, lexicalErr
	}
	if isPrivateMetadataPath(lexicalPath, root) {
		return nil, errors.New("internal metadata is not a document artifact")
	}
	lexicalPublicRoot := filepath.Join(root, "public")
	if isPathInside(lexicalPath, lexicalPublicRoot) {
		if symlinked, symlinkErr := symlinkBelow(lexicalPath, lexicalPublicRoot); symlinkErr != nil || symlinked {
			return nil, errors.New("public artifact path is not a safe namespace")
		}
	}
	if isPathInside(lexicalPath, root) {
		if symlinked, symlinkErr := symlinkBelow(lexicalPath, root); symlinkErr != nil || symlinked {
			return nil, errors.New("artifact path is not a safe namespace")
		}
	}
	if isPathInside(lexicalPath, root) && !isPathInside(canonical, cacheRoot) {
		return nil, errors.New("artifact path escaped the cache")
	}
	if isPathInside(canonical, cacheRoot) {
		rel, _ := filepath.Rel(cacheRoot, canonical)
		first := strings.ToLower(strings.Split(filepath.ToSlash(rel), "/")[0])
		if first == ".private" || first == "stats" || first == "auth" || first == "handles" || first == "searchcache" {
			return nil, errors.New("internal metadata is not a document artifact")
		}
	} else if reason := DenyLocalPath(canonical, h.options.LocalRoots); reason != "" {
		return nil, errors.New("artifact path is not permitted")
	}
	return readBoundedFile(canonical, h.publicLimit())
}

func (h *Harvester) publicLimit() int64 {
	if h != nil && h.options.MaxBytes > 0 {
		return h.options.MaxBytes
	}
	return publicReadLimit
}

func isPrivateMetadataPath(path, root string) bool {
	for _, name := range []string{".private", "stats", "auth", "handles", "searchcache"} {
		if isPathInside(path, filepath.Join(root, name)) {
			return true
		}
	}
	return false
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = publicReadLimit
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("artifact is not a regular file")
	} else if info.Size() > limit {
		return nil, errors.New("artifact exceeds public size limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("artifact exceeds public size limit")
	}
	return data, nil
}

func (h *Harvester) rewritePublicImages(source, body, basePath string) (string, error) {
	matches := markdownImageRE.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body, nil
	}
	replacements := make(map[string]string)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		raw := body[match[2]:match[3]]
		if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") || strings.HasPrefix(strings.ToLower(raw), "http://") || strings.HasPrefix(strings.ToLower(raw), "https://") {
			continue
		}
		path, local := publicImagePath(raw, basePath)
		if !local {
			continue
		}
		canonical, err := canonicalPublicPath(path)
		if err != nil {
			return "", errors.New("embedded image cannot be resolved")
		}
		root, err := h.cacheRoot()
		if err != nil {
			return "", err
		}
		lexicalPath, lexicalErr := filepath.Abs(filepath.Clean(path))
		if lexicalErr != nil || isPrivateMetadataPath(lexicalPath, root) {
			return "", errors.New("embedded image is inside private metadata")
		}
		if isPathInside(lexicalPath, root) {
			if symlinked, symlinkErr := symlinkBelow(lexicalPath, root); symlinkErr != nil || symlinked {
				return "", errors.New("embedded image is not a safe namespace")
			}
		}
		cacheRoot, err := canonicalPublicPath(root)
		if err != nil {
			return "", err
		}
		publicRoot, err := h.publicRoot()
		if err != nil {
			return "", err
		}
		if isPathInside(canonical, publicRoot) {
			continue
		}
		if isPathInside(canonical, cacheRoot) {
			// Private cache links must be copied. A link that cannot be
			// copied is a failed export, never a leaked private path.
		} else if reason := DenyLocalPath(canonical, h.options.LocalRoots); reason != "" {
			return "", errors.New("embedded local image is not permitted")
		}
		data, err := readBoundedFile(canonical, maxImageBytes)
		if err != nil {
			return "", errors.New("embedded image cannot be read")
		}
		kind := classifyKind(canonical, "", data)
		if !isImageKind(kind) {
			kind = SniffMagic(data)
		}
		if !isImageKind(kind) {
			return "", errors.New("embedded link is not an image")
		}
		ext, ok := publicBinaryExtension(kind, canonical, data)
		if !ok {
			return "", errors.New("embedded image type is unsupported")
		}
		publicPath, err := h.publicArtifactPath(source, "image", canonical, ext)
		if err != nil {
			return "", err
		}
		if err := h.writePublicFile(publicPath, data); err != nil {
			return "", err
		}
		replacements[raw] = publicPath
	}
	if len(replacements) == 0 {
		return body, nil
	}
	rewritten := markdownImageRE.ReplaceAllStringFunc(body, func(fragment string) string {
		parts := markdownImageRE.FindStringSubmatch(fragment)
		if len(parts) < 2 {
			return fragment
		}
		if replacement := replacements[parts[1]]; replacement != "" {
			return strings.Replace(fragment, parts[1], replacement, 1)
		}
		return fragment
	})
	for original, replacement := range replacements {
		rewritten = strings.ReplaceAll(rewritten, original, replacement)
	}
	return rewritten, nil
}

func publicImagePath(raw, basePath string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		path, err := fileURLPath(trimmed)
		return path, err == nil
	}
	location := strings.Split(strings.Split(trimmed, "?")[0], "#")[0]
	if filepath.IsAbs(location) {
		return location, true
	}
	if basePath != "" && (strings.HasPrefix(location, ".") || strings.Contains(location, string(os.PathSeparator))) {
		return filepath.Join(filepath.Dir(basePath), location), true
	}
	return "", false
}

func isPathInside(path, root string) bool {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	return insideAny(path, []string{root})
}

func symlinkBelow(path, root string) (bool, error) {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	root, err = filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, errors.New("path is outside namespace")
	}
	current := root
	if rel == "." {
		return false, nil
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return false, nil
		}
		if statErr != nil {
			return false, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}

func canonicalPublicPath(raw string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return "", err
	}
	current := abs
	missing := []string{}
	for {
		resolved, evalErr := filepath.EvalSymlinks(current)
		if evalErr == nil {
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", evalErr
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", evalErr
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}
