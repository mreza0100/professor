package harvest

import (
	"context"
	cryptomd5 "crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

const providerFixtureDOI = "10.1234/provider.fixture"

func TestLibGenMD5LookupKeepsCookieAndVerifiesPDFDigest(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	pdf := []byte("%PDF-1.7\nlibgen fixture\n%%EOF")
	digest := cryptomd5.Sum(pdf)
	wantMD5 := hex.EncodeToString(digest[:])
	var adsSeen, downloadSeen bool
	requireSessionHeaders := true
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ads.php":
			adsSeen = true
			if got := r.Header.Get("Referer"); got != "https://libgen.test/" {
				t.Fatalf("LibGen ads Referer = %q, want provider homepage", got)
			}
			if r.URL.Query().Get("md5") != wantMD5 {
				t.Fatalf("LibGen ads md5 = %q, want %q", r.URL.Query().Get("md5"), wantMD5)
			}
			out := response(r, http.StatusOK, "text/html", `<a href="/get.php?md5=`+wantMD5+`&key=fixture">GET</a>`)
			out.Header.Set("Set-Cookie", "sid=lookup; Path=/")
			return out, nil
		case "/get.php":
			downloadSeen = true
			if requireSessionHeaders {
				if got := r.Header.Get("Referer"); got != "https://libgen.test/ads.php?md5="+wantMD5 {
					t.Fatalf("LibGen download Referer = %q, want ads page", got)
				}
				if cookie := r.Header.Get("Cookie"); !strings.Contains(cookie, "sid=lookup") {
					t.Fatalf("LibGen download cookie = %q, want lookup cookie", cookie)
				}
			}
			return response(r, http.StatusOK, "application/pdf", string(pdf)), nil
		default:
			return response(r, http.StatusNotFound, "text/plain", "missing"), nil
		}
	})}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		LibGenURL: "https://libgen.test",
		Converter: legacyConverterFunc(func(_ context.Context, _ string, _ string, _ []byte) (string, error) { return "libgen converted", nil }),
	})
	got := h.fetchLibGenMD5(context.Background(), "10.1234/provider.fixture", wantMD5, FetchOptions{})
	if got.Error != "" || got.Method != "libgen" || got.Kind != "pdf" || !strings.Contains(got.Content, "libgen converted") {
		t.Fatalf("LibGen fetch = %#v", got)
	}
	if !adsSeen || !downloadSeen {
		t.Fatalf("LibGen stages ads=%t download=%t; want both", adsSeen, downloadSeen)
	}

	requireSessionHeaders = false
	bad := h.fetchProviderArtifact(context.Background(), "10.1234/provider.fixture", "libgen", "https://libgen.test/get.php?md5="+wantMD5, "", strings.Repeat("0", 32), FetchOptions{}, []string{"libgen"})
	if bad.Error == "" || bad.ErrorKind != "integrity" || !strings.Contains(bad.Error, "MD5 verification") {
		t.Fatalf("bad LibGen digest = %#v; want visible integrity failure", bad)
	}
}

func TestProviderDownloadLimitRejectsOversizedPartialResponse(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nthis exceeds the configured limit\n%%EOF"), nil
	})}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: client, Chrome: client, MaxBytes: 16, Converter: &fakeConverter{}})
	got := h.fetchProviderArtifact(context.Background(), providerFixtureDOI, "scidb", "https://scidb.test/file.pdf", "", "", FetchOptions{}, []string{"scidb"})
	if got.Error == "" || got.ErrorKind != "too_large" || got.Content != "" {
		t.Fatalf("oversized provider response = %#v; want bounded failure", got)
	}
}

func TestProviderDownloadRejectsShortReadInsteadOfConvertingPartialBody(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			Body:       shortReadBody{reader: strings.NewReader("%PDF-1.7\npartial")},
			Request:    r,
		}, nil
	})}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: client, Chrome: client, MaxBytes: 1024, Converter: &fakeConverter{}})
	got := h.fetchProviderArtifact(context.Background(), providerFixtureDOI, "scidb", "https://scidb.test/file.pdf", "", "", FetchOptions{}, []string{"scidb"})
	if got.Error == "" || got.Content != "" || got.Path != "" {
		t.Fatalf("short provider response was accepted: %#v", got)
	}
}

type shortReadBody struct {
	reader *strings.Reader
}

func (body shortReadBody) Read(p []byte) (int, error) {
	n, _ := body.reader.Read(p)
	if n > 0 {
		return n, io.ErrUnexpectedEOF
	}
	return 0, io.EOF
}

func (body shortReadBody) Close() error { return nil }

func TestSciDBDOILookupResolvesRelativePDFAndUsesFinalPageReferer(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	var referer string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/scidb/10.1234/provider.fixture":
			return response(r, http.StatusOK, "text/html", `<html><body><div id="article"><iframe id="pdf" src="/files/provider.pdf"></iframe></div></body></html>`), nil
		case "/files/provider.pdf":
			referer = r.Header.Get("Referer")
			return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nscidb fixture\n%%EOF"), nil
		default:
			return response(r, http.StatusNotFound, "text/plain", "missing"), nil
		}
	})}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		SciDBURL:  "https://scidb.test",
		Converter: legacyConverterFunc(func(_ context.Context, _ string, _ string, _ []byte) (string, error) { return "scidb converted", nil }),
	})
	got := h.fetchSciDBDOI(context.Background(), providerFixtureDOI, FetchOptions{})
	if got.Error != "" || got.Method != "scidb" || got.Kind != "pdf" || got.Content != "scidb converted" {
		t.Fatalf("SciDB DOI fetch = %#v", got)
	}
	if referer != "https://scidb.test/scidb/10.1234/provider.fixture" {
		t.Fatalf("SciDB PDF Referer = %q, want final lookup page", referer)
	}
}

func TestProviderPDFEmptyConversionErrorEscalatesToOCR(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nocr fixture\n%%EOF"), nil
	})}
	converter := &emptyPDFThenOCRConverter{}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: client, Chrome: client, Converter: converter})
	got := h.fetchProviderArtifact(context.Background(), providerFixtureDOI, "scidb", "https://scidb.test/file.pdf", "", "", FetchOptions{}, []string{"scidb"})
	if got.Error != "" || got.Content != "OCR recovered provider fixture" || !containsProviderString(got.Rungs, "ocr") || converter.ocrCalls != 1 {
		t.Fatalf("provider OCR recovery = %#v calls=%d", got, converter.ocrCalls)
	}
}

type emptyPDFThenOCRConverter struct{ ocrCalls int }

func (c emptyPDFThenOCRConverter) Convert(_ context.Context, kind, _ string, _ []byte) (string, error) {
	if kind == "pdf" {
		return "", errors.New("EMPTY-text conversion")
	}
	return "", nil
}

func (c *emptyPDFThenOCRConverter) ConvertOCR(_ context.Context, _ string, _ string, _ []byte) (string, error) {
	c.ocrCalls++
	return "OCR recovered provider fixture", nil
}

func TestAnnasChallengeIsTerminalAndDoesNotTryGateway(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	var gatewayCalls int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "ipfs") || strings.Contains(r.URL.Host, "dweb") {
			gatewayCalls++
		}
		return response(r, http.StatusForbidden, "text/html", "Cloudflare verification required"), nil
	})}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: client, Chrome: client, AnnasURL: "https://annas.test", Converter: &fakeConverter{}})
	got := h.fetchAnnasMD5(context.Background(), providerFixtureDOI, "9de4a86150a39b54d3e01f98678468bf", FetchOptions{})
	if got.Error == "" || got.ErrorKind != "challenge" || !got.Challenge || gatewayCalls != 0 {
		t.Fatalf("Anna challenge = %#v gatewayCalls=%d", got, gatewayCalls)
	}
}

func TestProviderSearchParsersPreserveBibliographicFields(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	md5 := "9de4a86150a39b54d3e01f98678468bf"
	libgen, err := providerRecordCandidates([]byte(`<table><tr><td><a href="/edition.php?id=4">Actual book title</a></td><td>Real Author</td><td>Publisher</td><td>2020</td><td><a href="/ads.php?md5=`+md5+`&key=x">GET</a></td></tr></table>`), "https://libgen.test/index.php", "libgen", "caller query", 4)
	if err != nil || len(libgen) != 1 || libgen[0].Title != "Actual book title" || libgen[0].Authors != "Real Author" || libgen[0].Year != 2020 || strings.Contains(libgen[0].Title, "caller query") {
		t.Fatalf("LibGen parsed candidate = %#v err=%v", libgen, err)
	}

	scholar := parseGoogleScholar([]byte(`<div class="gs_ri"><h3 class="gs_rt"><a href="https://doi.org/10.1234/provider.fixture">Fixture article</a></h3><div class="gs_a">A Author - Journal, 2020 - repository.example</div><div class="gs_or_ggsm"><a href="https://repository.example/article.pdf">[PDF]</a></div></div>`), 4)
	if len(scholar) != 1 || scholar[0].Title != "Fixture article" || scholar[0].Authors != "A Author" || scholar[0].Year != 2020 || strings.Contains(scholar[0].Authors, "repository.example") {
		t.Fatalf("Scholar parsed candidate = %#v; provider location leaked into authors", scholar)
	}
}

func TestFindWorksIncludesConfiguredProvidersAndPublicHandleFetchesSelectedPDF(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	md5 := "9de4a86150a39b54d3e01f98678468bf"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "annas.test":
			return response(r, http.StatusOK, "text/html", `<a href="/md5/`+md5+`">Anna result</a>`), nil
		case "libgen.test":
			return response(r, http.StatusOK, "text/html", `<table><tr><td><a href="/edition.php?id=4">LibGen result</a></td><td>Author</td><td>Publisher</td><td>2020</td><td><a href="/ads.php?md5=`+md5+`&key=x">GET</a></td></tr></table>`), nil
		case "scholar.test":
			if r.URL.Path == "/selected.pdf" {
				return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nselected\n%%EOF"), nil
			}
			return response(r, http.StatusOK, "text/html", `<div class="gs_ri"><h3 class="gs_rt"><a href="https://doi.org/10.1234/provider.fixture">Scholar result</a></h3><div class="gs_a">A Author - Journal, 2020 - repository.example</div><div class="gs_or_ggsm"><a href="https://scholar.test/selected.pdf">[PDF]</a></div></div>`), nil
		default:
			return response(r, http.StatusOK, "application/json", `{}`), nil
		}
	})}
	resolver := &Resolver{Client: client, AnnasURL: "https://annas.test", LibGenURL: "https://libgen.test", GoogleScholarURL: "https://scholar.test"}
	candidates, err := resolver.FindWorks(context.Background(), "fixture query", 10)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]bool{}
	var selected Candidate
	for _, candidate := range candidates {
		sources[candidate.Source] = true
		if candidate.Source == "google-scholar" {
			selected = candidate
		}
	}
	for _, source := range []string{"annas", "libgen", "google-scholar"} {
		if !sources[source] {
			t.Fatalf("FindWorks candidates omitted configured provider %q: %#v", source, candidates)
		}
	}
	if selected.URL != "https://scholar.test/selected.pdf" {
		t.Fatalf("selected Scholar URL = %q, want exact PDF location", selected.URL)
	}

	h := mustNew(t, Options{
		CacheDir: t.TempDir(), Client: client,
		Converter: legacyConverterFunc(func(_ context.Context, _ string, _ string, _ []byte) (string, error) {
			return "selected provider bytes", nil
		}),
	})
	public, err := h.PublicCandidates([]Candidate{selected})
	if err != nil || len(public) != 1 || !publicHandleRE.MatchString(public[0].URL) {
		t.Fatalf("PublicCandidates(selected) = %#v err=%v; want opaque handle", public, err)
	}
	got := h.FetchPublic(context.Background(), public[0].URL, FetchOptions{})
	if got.Error != "" || got.Content != "selected provider bytes" || strings.Contains(got.Content, "scholar.test") {
		t.Fatalf("selected public handle fetch = %#v", got)
	}
}

func TestProviderSearchCleanMissAndOutageRemainDistinct(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	missClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return response(r, http.StatusOK, "text/html", "No results found"), nil
	})}
	missResolver := &Resolver{Client: missClient, AnnasURL: "https://annas.test", LibGenURL: "https://libgen.test"}
	if got, err := missResolver.annasSearch(context.Background(), "missing", 4); err != nil || len(got) != 0 {
		t.Fatalf("Anna clean miss = %#v err=%v; want empty success", got, err)
	}
	outageResolver := &Resolver{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("provider socket outage") })}, AnnasURL: "https://annas.test"}
	if _, err := outageResolver.annasSearch(context.Background(), "missing", 4); err == nil || !strings.Contains(err.Error(), "outage") {
		t.Fatalf("Anna outage err = %v; want visible outage", err)
	}
}

func TestResolveDOIAndFallbackPreserveMetadataOutage(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	metadataOutage := errors.New("metadata provider outage")
	failing := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, metadataOutage
	})}
	resolver := &Resolver{Client: failing}
	if candidates, err := resolver.ResolveDOI(context.Background(), providerFixtureDOI); err == nil {
		t.Errorf("ResolveDOI outage = candidates=%#v err=nil; want visible outage with no candidates", candidates)
	} else if len(candidates) != 0 {
		t.Errorf("ResolveDOI outage = candidates=%#v err=%v; want no candidates with visible lookup failure", candidates, err)
	}
	chromeFailing := &http.Client{Transport: failing.Transport}
	oaFailing := &http.Client{Transport: failing.Transport}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: failing, Chrome: chromeFailing, OA: oaFailing, Converter: &fakeConverter{}})
	got := h.fetchKnownID(context.Background(), providerFixtureDOI, IdentifierDOI, FetchOptions{})
	if got.Error == "" || got.ErrorKind != "connect" || strings.Contains(strings.ToLower(got.Error), "likely paywalled") {
		t.Fatalf("DOI fallback outage = %#v; want visible connect outage", got)
	}
}

func TestGoogleScholarVersionsPageSuppliesSecondPagePDF(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	var versionRequests int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("cluster") == "123" {
			versionRequests++
			return response(r, http.StatusOK, "text/html", `<div class="gs_ri"><h3 class="gs_rt"><a href="https://doi.org/10.1234/provider.fixture">Versions fixture</a></h3><div class="gs_a">A Author - Journal, 2020 - repository.example</div><div class="gs_or_ggsm"><a href="https://repository.test/versions.pdf">[PDF]</a></div></div>`), nil
		}
		return response(r, http.StatusOK, "text/html", `<div class="gs_ri"><h3 class="gs_rt"><a href="https://doi.org/10.1234/provider.fixture">Versions fixture</a></h3><div class="gs_a">A Author - Journal, 2020 - repository.example</div><div class="gs_fl"><a href="/scholar?cluster=123">All 2 versions</a></div></div>`), nil
	})}
	resolver := &Resolver{Client: client, GoogleScholarURL: "https://scholar.test"}
	got, err := resolver.googleScholar(context.Background(), "fixture", 4)
	if err != nil || len(got) != 1 || got[0].URL != "https://repository.test/versions.pdf" || versionRequests != 1 {
		t.Fatalf("Scholar versions result = %#v err=%v requests=%d", got, err, versionRequests)
	}
}

func TestBibliographicLandingCycleStopsAfterOneHop(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	const sourceA = "https://publisher.test/a"
	const sourceB = "https://publisher.test/b"
	var aRequests, bRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case sourceA:
			aRequests++
			return response(r, http.StatusOK, "text/html", `<html><a class="document-link" href="/b">Full text</a></html>`), nil
		case sourceB:
			bRequests++
			return response(r, http.StatusOK, "text/html", `<html><a class="document-link" href="/a">Full text</a></html>`), nil
		default:
			return response(r, http.StatusNotFound, "text/plain", "missing"), nil
		}
	})
	convert := legacyConverterFunc(func(_ context.Context, kind, source string, _ []byte) (string, error) {
		if kind != "html" {
			return "", errors.New("unexpected kind")
		}
		return "# Abstract\n# Fingerprint\n# Cite this\n", nil
	})
	direct := &http.Client{Transport: transport}
	chrome := &http.Client{Transport: transport}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: direct, Chrome: chrome, Jina: direct, OA: direct, Converter: convert})
	got := h.Fetch(context.Background(), sourceA)
	if got.Error == "" {
		t.Fatalf("landing cycle unexpectedly fetched content: %#v", got)
	}
	if aRequests > 2 || bRequests > 2 {
		t.Fatalf("landing cycle was followed repeatedly: sourceA=%d sourceB=%d", aRequests, bRequests)
	}
}

func TestBibliographicLandingFollowsFullTextDocumentLink(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	withPublicDNSForSciHubTest(t)
	const landingURL = "https://publisher.test/landing"
	const documentURL = "https://publisher.test/files/fulltext.pdf"
	var documentRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case landingURL:
			return response(r, http.StatusOK, "text/html", `<html><body><h1>Title</h1><a class="document-link" href="/files/fulltext.pdf">Full text</a></body></html>`), nil
		case documentURL:
			documentRequests++
			return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nfull text\n%%EOF"), nil
		default:
			return response(r, http.StatusNotFound, "text/plain", "missing"), nil
		}
	})
	convert := legacyConverterFunc(func(_ context.Context, kind, _ string, _ []byte) (string, error) {
		if kind == "html" {
			return "# Title\n# Abstract\n# Fingerprint\n# Cite this\n", nil
		}
		return "A Sequential Analysis\n# Discussion\nfull text", nil
	})
	direct := &http.Client{Transport: transport}
	chrome := &http.Client{Transport: transport}
	h := mustNew(t, Options{CacheDir: t.TempDir(), Client: direct, Chrome: chrome, Jina: direct, OA: direct, Converter: convert})
	got := h.Fetch(context.Background(), landingURL)
	if got.Error != "" || got.Kind != "pdf" || !strings.Contains(got.Content, "A Sequential Analysis") {
		t.Fatalf("bibliographic landing fetch = %#v; want linked full text", got)
	}
	if documentRequests != 1 {
		t.Fatalf("full text document requests = %d, want one bounded landing follow", documentRequests)
	}
}

func containsProviderString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
