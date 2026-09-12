package harvest

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

const sciHubFixtureDOI = "10.1371/journal.pone.0033693"

func withPublicDNSForSciHubTest(t *testing.T) {
	t.Helper()
	previous := lookupIP
	lookupIP = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	}
	t.Cleanup(func() { lookupIP = previous })
}

func sciHubFixtureTransport(t *testing.T, pdfReferer *string, postIdentifier *string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Host == "sci-hub.test":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				return nil, err
			}
			*postIdentifier = values.Get("request")
			finalRequest := r.Clone(r.Context())
			finalRequest.URL, _ = url.Parse("https://sci-hub.test/article/fixture")
			return response(finalRequest, http.StatusOK, "text/html", `<html><body><div id="article"><iframe id="pdf" src="//sci.bban.top/pdf/fixture.pdf#view=FitH"></iframe></div></body></html>`), nil
		case r.Method == http.MethodGet && r.URL.Host == "sci.bban.top":
			*pdfReferer = r.Header.Get("Referer")
			return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nfixture\n%%EOF"), nil
		default:
			return response(r, http.StatusNotFound, "application/json", `{}`), nil
		}
	})}
}

func TestSciHubLookupPostsIdentifierAndUsesFinalPageAsPDFReferer(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	var pdfReferer, postIdentifier string
	client := sciHubFixtureTransport(t, &pdfReferer, &postIdentifier)
	converter := &fakeConverter{}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		Converter: converter,
		SciHubURL: "https://sci-hub.test/",
	})

	got := h.fetchSciHub(context.Background(), sciHubFixtureDOI, FetchOptions{})
	if got.Error != "" {
		t.Fatalf("fetchSciHub() error = %q", got.Error)
	}
	if got.Source != sciHubFixtureDOI || got.Method != "scihub" || got.Kind != "pdf" {
		t.Fatalf("fetchSciHub() receipt = %#v", got)
	}
	if postIdentifier != sciHubFixtureDOI {
		t.Fatalf("SciHub POST request identifier = %q, want %q", postIdentifier, sciHubFixtureDOI)
	}
	if pdfReferer != "https://sci-hub.test/article/fixture" {
		t.Fatalf("PDF Referer = %q, want final lookup page URL", pdfReferer)
	}
	if converter.calls != 1 {
		t.Fatalf("converter calls = %d, want one PDF conversion", converter.calls)
	}
	if !containsString(got.Rungs, "scihub") {
		t.Fatalf("SciHub rung missing from receipt: %#v", got.Rungs)
	}
}

func TestSciHubPOSTRedirectsToDOIPageAndUsesFinalPageAsPDFReferer(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	var methods []string
	var postIdentifier, pdfReferer string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Host == "sci-hub.test" && r.URL.Path == "/":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				return nil, err
			}
			postIdentifier = values.Get("request")
			return &http.Response{
				StatusCode: http.StatusFound,
				Status:     http.StatusText(http.StatusFound),
				Header:     http.Header{"Location": []string{"/" + sciHubFixtureDOI}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case r.Method == http.MethodGet && r.URL.Host == "sci-hub.test" && r.URL.Path == "/"+sciHubFixtureDOI:
			return response(r, http.StatusOK, "text/html", `<div id="article"><iframe id="pdf" src="//sci.bban.top/pdf/redirect.pdf#view=FitH"></iframe></div>`), nil
		case r.Method == http.MethodGet && r.URL.Host == "sci.bban.top":
			pdfReferer = r.Header.Get("Referer")
			return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nredirect fixture\n%%EOF"), nil
		default:
			return nil, errors.New("unexpected SciHub request: " + r.Method + " " + r.URL.String())
		}
	})}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		Converter: &fakeConverter{},
		SciHubURL: "https://sci-hub.test/",
	})

	got := h.fetchSciHub(context.Background(), sciHubFixtureDOI, FetchOptions{})
	if got.Error != "" {
		t.Fatalf("fetchSciHub() error = %q", got.Error)
	}
	if got.Method != "scihub" || got.Kind != "pdf" {
		t.Fatalf("redirected SciHub receipt = %#v", got)
	}
	if postIdentifier != sciHubFixtureDOI {
		t.Fatalf("POST request identifier = %q, want %q", postIdentifier, sciHubFixtureDOI)
	}
	wantMethods := []string{"POST /", "GET /" + sciHubFixtureDOI, "GET /pdf/redirect.pdf"}
	if strings.Join(methods, "|") != strings.Join(wantMethods, "|") {
		t.Fatalf("redirect request sequence = %#v, want %#v", methods, wantMethods)
	}
	if pdfReferer != "https://sci-hub.test/"+sciHubFixtureDOI {
		t.Fatalf("PDF Referer = %q, want final DOI page URL", pdfReferer)
	}
}

func TestSciHubClientPreservesBaseRedirectPolicy(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("base redirect policy")
	called := false
	base := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		called = true
		return wantErr
	}}
	client := sciHubClient(base, jar)
	next, err := http.NewRequest(http.MethodGet, "https://mirror.example/final", nil)
	if err != nil {
		t.Fatal(err)
	}
	gotErr := client.CheckRedirect(next, []*http.Request{{Method: http.MethodGet}})
	if !called {
		t.Fatal("base CheckRedirect callback was not called")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("wrapped CheckRedirect error = %v, want %v", gotErr, wantErr)
	}
}

func TestSciHubBinaryPDFWithChallengeWordsIsAccepted(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Host != "sci-hub.test" {
			return nil, errors.New("unexpected binary SciHub request: " + r.Method + " " + r.URL.String())
		}
		return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nCAPTCHA Cloudflare words inside a valid PDF\n%%EOF"), nil
	})}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		Converter: &fakeConverter{},
		SciHubURL: "https://sci-hub.test/",
	})

	got := h.fetchSciHub(context.Background(), sciHubFixtureDOI, FetchOptions{})
	if got.Error != "" || got.Method != "scihub" || got.Kind != "pdf" {
		t.Fatalf("binary PDF containing challenge words = %#v", got)
	}
	if got.Challenge {
		t.Fatalf("valid binary PDF was marked as a challenge: %#v", got)
	}
}

func TestSciHubPDFLinkResolvesSupportedFormsAndRejectsPrivateTargets(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "iframe under article", body: `<div id="article"><iframe src="/pdf/paper.pdf"></iframe></div>`, want: "https://sci-hub.test/pdf/paper.pdf"},
		{name: "pdf iframe id", body: `<iframe id="pdf" src="//sci.bban.top/pdf/paper.pdf#view=FitH"></iframe>`, want: "https://sci.bban.top/pdf/paper.pdf"},
		{name: "embed under article", body: `<div id="article"><embed src="https://sci.bban.top/pdf/paper.pdf"></div>`, want: "https://sci.bban.top/pdf/paper.pdf"},
		{name: "object", body: `<object type="application/pdf" data="/pdf/paper.pdf"></object>`, want: "https://sci-hub.test/pdf/paper.pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sciHubPDFLink([]byte(tc.body), "https://sci-hub.test/article/fixture")
			if err != nil || got != tc.want {
				t.Fatalf("sciHubPDFLink() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := sciHubPDFLink([]byte(`<iframe id="pdf" src="http://127.0.0.1:8080/secret.pdf"></iframe>`), "https://sci-hub.test/article/fixture"); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("private PDF target error = %v, want explicit private-host refusal", err)
	}
	for _, href := range []string{"http://[::1", "://malformed"} {
		if _, err := sciHubPDFLink([]byte(`<iframe id="pdf" src="`+href+`"></iframe>`), "https://sci-hub.test/article/fixture"); err == nil {
			t.Fatalf("malformed PDF href %q was accepted", href)
		}
	}
}

func TestDOIFallsBackToSciHubAfterOpenAccessExhaustion(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	var pdfReferer, postIdentifier string
	client := sciHubFixtureTransport(t, &pdfReferer, &postIdentifier)
	oaClient := &http.Client{Transport: client.Transport}
	h := mustNew(t, Options{
		CacheDir:             t.TempDir(),
		Client:               client,
		Chrome:               client,
		Jina:                 client,
		OA:                   oaClient,
		Converter:            &fakeConverter{},
		ContactEmail:         "qa@example.test",
		SciHubURL:            "https://sci-hub.test/",
		NegativeTTL:          -1,
		NegativeTransientTTL: -1,
	})

	got := h.fetchKnownID(context.Background(), sciHubFixtureDOI, IdentifierDOI, FetchOptions{})
	if got.Error != "" || got.Method != "scihub" {
		t.Fatalf("DOI fallback receipt = %#v", got)
	}
	if postIdentifier != sciHubFixtureDOI || pdfReferer != "https://sci-hub.test/article/fixture" {
		t.Fatalf("SciHub fallback request identifier=%q referer=%q", postIdentifier, pdfReferer)
	}
}

func TestSuccessfulOpenAccessSkipsSciHubFallback(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	const oaPDF = "https://repo.test/paper.pdf"
	direct := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == oaPDF {
			return response(r, http.StatusOK, "application/pdf", "%PDF-1.7\nopen access\n%%EOF"), nil
		}
		return response(r, http.StatusNotFound, "application/json", `{}`), nil
	})}
	oaClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "api.unpaywall.org") {
			return jsonResponse(r, `{"is_oa":true,"oa_status":"gold","best_oa_location":{"url_for_pdf":"`+oaPDF+`","version":"publishedVersion"}}`), nil
		}
		return response(r, http.StatusNotFound, "application/json", `{}`), nil
	})}
	h := mustNew(t, Options{
		CacheDir:     t.TempDir(),
		Client:       direct,
		Chrome:       direct,
		Jina:         direct,
		OA:           oaClient,
		Converter:    &fakeConverter{},
		ContactEmail: "qa@example.test",
		SciHubURL:    "https://sci-hub.test/",
	})

	got := h.fetchKnownID(context.Background(), "10.9999/oa", IdentifierDOI, FetchOptions{})
	if got.Error != "" || got.Method == "scihub" || !containsString(got.Rungs, "oa:unpaywall") {
		t.Fatalf("successful OA receipt = %#v", got)
	}
}

func TestPMIDWithoutPMCIDFallsBackToSciHub(t *testing.T) {
	withPublicDNSForSciHubTest(t)
	var pdfReferer, postIdentifier string
	fixture := sciHubFixtureTransport(t, &pdfReferer, &postIdentifier)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "sci-hub.test" || r.URL.Host == "sci.bban.top" {
			return fixture.Transport.RoundTrip(r)
		}
		if strings.Contains(r.URL.Path, "/idconv/api/") {
			return jsonResponse(r, `{"records":[]}`), nil
		}
		return response(r, http.StatusNotFound, "application/json", `{}`), nil
	})}
	h := mustNew(t, Options{
		CacheDir:  t.TempDir(),
		Client:    client,
		Chrome:    client,
		OA:        client,
		Converter: &fakeConverter{},
		SciHubURL: "https://sci-hub.test/",
	})

	got := h.fetchKnownID(context.Background(), "1234567", IdentifierPMID, FetchOptions{})
	if got.Error != "" || got.Method != "scihub" {
		t.Fatalf("PMID fallback receipt = %#v", got)
	}
	if postIdentifier != "1234567" || pdfReferer != "https://sci-hub.test/article/fixture" {
		t.Fatalf("PMID SciHub request identifier=%q referer=%q", postIdentifier, pdfReferer)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
