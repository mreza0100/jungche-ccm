package harvest

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// Parity with harvester tests/test_wave3.py — the wave additions must behave
// identically on both sides of the seam.

func TestWaveLooksLikeEpubBoundaries(t *testing.T) {
	epub := buildMinimalEpub(t)
	if !LooksLikeEpub(epub) {
		t.Fatalf("a real EPUB must carry its mimetype marker in the first 200 bytes")
	}
	plain := []byte("PK\x03\x04 plain zip, not a book")
	if LooksLikeEpub(plain) {
		t.Fatalf("plain zip bytes must not be mistaken for an EPUB")
	}
	buried := append([]byte("PK\x03\x04"), append(make([]byte, 4096), []byte(epubContentMarker)...)...)
	if LooksLikeEpub(buried) {
		t.Fatalf("the marker buried in member DATA is not an EPUB — only the stored mimetype position counts")
	}
}

func buildMinimalEpub(t *testing.T) []byte {
	t.Helper()
	// Hand-rolled minimal EPUB: mimetype STORED first (spec order), then container + OPF.
	var out []byte
	appendFile := func(name string, data []byte, store bool) {
		out = append(out, 'P', 'K', 3, 4)
		flags := byte(0)
		if store {
			flags = 0
		}
		out = append(out, 14, flags, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		out = append(out, byte(len(name)), 0)
		out = append(out, name...)
		out = append(out, data...)
	}
	appendFile("mimetype", []byte(epubContentMarker), true)
	appendFile("OEBPS/c1.xhtml", []byte("<html><body><h1>Hello</h1></body></html>"), false)
	return out
}

func TestWavePlosNberOfflineDeterministic(t *testing.T) {
	plos := plosCandidates("10.1371/journal.pcbi.1003285")
	if len(plos) != 1 || plos[0].URL != "https://journals.plos.org/pcbi/article/file?id=10.1371%2Fjournal.pcbi.1003285&type=printable" && plos[0].URL != "https://journals.plos.org/pcbi/article/file?id=10.1371/journal.pcbi.1003285&type=printable" {
		t.Fatalf("plos candidates=%#v", plos)
	}
	if plosCandidates("10.1038/s41586-020-2649-2") != nil {
		t.Fatalf("non-PLOS DOI must yield nothing")
	}
	nber := nberCandidates("10.3386/W33186")
	if len(nber) != 1 || nber[0].URL != "https://www.nber.org/system/files/working_papers/w33186/w33186.pdf" {
		t.Fatalf("nber candidates=%#v", nber)
	}
	if nberCandidates("10.3386/not-a-wp-id") != nil {
		t.Fatalf("malformed NBER id must yield nothing")
	}
}

func TestWaveStripDefuddleEnvelope(t *testing.T) {
	raw := "---\ntitle: \"T\"\nsource: \"u\"\nword_count: 17\n---\n\nBody line.\n"
	if got := strings.TrimSpace(stripDefuddleEnvelope(raw)); got != "Body line." {
		t.Fatalf("stripDefuddleEnvelope=%q", got)
	}
	if got := stripDefuddleEnvelope("# Just markdown"); got != "# Just markdown" {
		t.Fatalf("envelope-less body must pass through, got %q", got)
	}
}

func TestWaveUnpaywallSkipsKeylessRuns(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(r, `{}`), nil
	})}
	resolver := &Resolver{Client: client} // no ContactEmail — keyless run
	cands, err := resolver.unpaywall(context.Background(), client, "10.1234/example")
	if err != nil || cands != nil || calls != 0 {
		t.Fatalf("keyless unpaywall must SKIP cleanly: cands=%#v calls=%d err=%v", cands, calls, err)
	}
}

func TestWaveZenodoOnlyMatchingRecordDocumentFiles(t *testing.T) {
	payload := `{"hits":{"hits":[
		{"doi":"10.5281/zenodo.13235113","files":[
			{"key":"article.pdf","links":{"self":"https://zenodo.org/api/records/13235113/files/article.pdf/content"}},
			{"key":"dataset.zip","links":{"self":"https://zenodo.org/api/records/13235113/files/dataset.zip/content"}}]},
		{"doi":"10.5281/zenodo.99999999","files":[
			{"key":"other.epub","links":{"self":"https://zenodo.org/api/records/9/files/other.epub/content"}}]}]}}`
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(r, payload), nil
	})}
	resolver := &Resolver{Client: client}
	got, err := resolver.zenodo(context.Background(), client, "10.5281/zenodo.13235113")
	if err != nil || len(got) != 1 || !strings.HasSuffix(got[0].URL, "files/article.pdf/content") {
		t.Fatalf("zenodo candidates=%#v err=%v", got, err)
	}
}
