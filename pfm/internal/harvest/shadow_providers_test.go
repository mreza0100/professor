package harvest

import (
	"context"
	cryptomd5 "crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"testing"
)

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
