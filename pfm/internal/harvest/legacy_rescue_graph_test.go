package harvest

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLegacyUnresolvedBotChallengeIsNeverCached(t *testing.T) {
	challenge := "<html><body>Are you a robot? verify you are human</body></html>"
	wallTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusForbidden, "text/html", challenge), nil
	})
	missingTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusNotFound, "application/json", `{}`), nil
	})
	h := New(Options{
		ContactEmail: "test@example.org",
		CacheDir:     t.TempDir(),
		Client:       &http.Client{Transport: wallTransport},
		Chrome:       &http.Client{Transport: wallTransport},
		Jina:         &http.Client{Transport: missingTransport},
		OA:           &http.Client{Transport: missingTransport},
		Converter:    legacyConverterFunc(func(_ context.Context, _ string, _ string, raw []byte) (string, error) { return string(raw), nil }),
	})
	result := h.Fetch(context.Background(), "https://blocked.example.test/article")
	if result.Error == "" || !strings.Contains(strings.ToLower(result.Error), "challenge") || !strings.Contains(result.Error, "Rungs tried: direct, chrome-impersonation, jina, defuddle, wayback") {
		t.Fatalf("unresolved challenge receipt=%#v", result)
	}
	var artifacts []string
	_ = filepath.Walk(h.options.CacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			artifacts = append(artifacts, path)
		}
		return nil
	})
	if len(artifacts) != 0 {
		t.Fatalf("unresolved challenge entered cache: %#v", artifacts)
	}
}

func TestLegacyChallengeCanRecoverAtChromeAndPersistsTrace(t *testing.T) {
	direct := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusForbidden, "text/html", "<html>checking your browser</html>"), nil
	})
	rich := "<html><body><h1>Recovered article</h1>" + strings.Repeat("scientific evidence ", 100) + "</body></html>"
	chrome := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, "text/html", rich), nil
	})
	h := New(Options{
		CacheDir:  t.TempDir(),
		Client:    &http.Client{Transport: direct},
		Chrome:    &http.Client{Transport: chrome},
		Converter: legacyConverterFunc(func(_ context.Context, _ string, _ string, raw []byte) (string, error) { return string(raw), nil }),
	})
	result := h.Fetch(context.Background(), "https://blocked.example.test/recovered")
	if result.Error != "" || result.Method != "chrome-impersonation" || strings.Join(result.Rungs, ",") != "direct,chrome-impersonation" {
		t.Fatalf("Chrome recovery=%#v", result)
	}
	raw, err := os.ReadFile(result.Path)
	if err != nil || !strings.Contains(string(raw), "rungs: direct, chrome-impersonation") {
		t.Fatalf("Chrome trace artifact=%q err=%v", string(raw), err)
	}
}

func TestLegacyCitationPDFMetaRescueCachesThePublisherSource(t *testing.T) {
	const publisher = "https://publisher.example.test/walled-article"
	const mirror = "https://repository.example.test/open-paper.pdf"
	wall := `<html><head><meta name="citation_pdf_url" content="` + mirror + `"></head><body>checking your browser</body></html>`
	var publisherCalls atomic.Int64
	var mirrorCalls atomic.Int64
	documentTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case publisher:
			publisherCalls.Add(1)
			return response(request, http.StatusForbidden, "text/html", wall), nil
		case mirror:
			mirrorCalls.Add(1)
			return response(request, http.StatusOK, "application/pdf", "%PDF-1.7 open fixture"), nil
		default:
			return response(request, http.StatusNotFound, "application/json", `{}`), nil
		}
	})
	missingTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusNotFound, "application/json", `{}`), nil
	})
	h := New(Options{
		CacheDir: t.TempDir(),
		Client:   &http.Client{Transport: documentTransport},
		Chrome:   &http.Client{Transport: documentTransport},
		Jina:     &http.Client{Transport: missingTransport},
		OA:       &http.Client{Transport: missingTransport},
		Converter: legacyConverterFunc(func(_ context.Context, kind, _ string, _ []byte) (string, error) {
			if kind == "pdf" {
				return "# Open paper\n\nRecovered through citation metadata.", nil
			}
			return "", nil
		}),
	})
	first := h.Fetch(context.Background(), publisher)
	wantRungs := "direct,chrome-impersonation,jina,defuddle,oa:citation_pdf_url"
	if first.Error != "" || first.Source != publisher || strings.Join(first.Rungs, ",") != wantRungs {
		t.Fatalf("citation PDF rescue=%#v want rungs=%s", first, wantRungs)
	}
	firstPublisherCalls, firstMirrorCalls := publisherCalls.Load(), mirrorCalls.Load()
	second := h.Fetch(context.Background(), publisher)
	if second.Error != "" || second.CacheStatus != "hit" || publisherCalls.Load() != firstPublisherCalls || mirrorCalls.Load() != firstMirrorCalls {
		t.Fatalf("citation PDF publisher cache=%#v publisher=%d->%d mirror=%d->%d", second, firstPublisherCalls, publisherCalls.Load(), firstMirrorCalls, mirrorCalls.Load())
	}
}

func TestLegacySelfReferentialOACandidateCannotRecurse(t *testing.T) {
	const source = "https://publisher.example.test/10.1234/recursive"
	const candidate = "https://mirror.example.test/10.1234/recursive.pdf"
	var providerCalls atomic.Int64
	oaTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Host, "unpaywall") {
			providerCalls.Add(1)
			return jsonResponse(request, `{"is_oa":true,"oa_status":"green","best_oa_location":{"url_for_pdf":"`+candidate+`","version":"publishedVersion"}}`), nil
		}
		return jsonResponse(request, `{}`), nil
	})
	wallTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusForbidden, "text/html", "<html>checking your browser</html>"), nil
	})
	missingTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusNotFound, "application/json", `{}`), nil
	})
	h := New(Options{
		ContactEmail: "test@example.org", // unpaywall is gated on an operator email
		CacheDir:     t.TempDir(),
		Client:       &http.Client{Transport: wallTransport},
		Chrome:       &http.Client{Transport: wallTransport},
		Jina:         &http.Client{Transport: missingTransport},
		OA:           &http.Client{Transport: oaTransport},
		Converter:    legacyConverterFunc(func(_ context.Context, _ string, _ string, raw []byte) (string, error) { return string(raw), nil }),
	})
	result := h.Fetch(context.Background(), source)
	if result.Error == "" || providerCalls.Load() != 1 {
		t.Fatalf("recursive OA result=%#v unpaywall calls=%d", result, providerCalls.Load())
	}
	if !strings.Contains(strings.Join(result.Rungs, ","), "oa:unpaywall") || result.Rungs[len(result.Rungs)-1] != "wayback" {
		t.Fatalf("recursive OA trace=%#v", result.Rungs)
	}
}
