package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCheckPersistsLatestReleaseForTheNextInvocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Fatalf("request method = %s, want HEAD", request.Method)
		}
		writer.Header().Set("Location", "/mreza0100/professor/releases/tag/v0.61.2")
		writer.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "update.json")
	if err := Check(context.Background(), cache, "v0.61.1", server.URL, server.Client()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	notice, found, err := Read(cache, "v0.61.1")
	if err != nil || !found || notice.Latest != "v0.61.2" || notice.Current != "v0.61.1" {
		t.Fatalf("Read(old) notice=%#v found=%t err=%v", notice, found, err)
	}
	if _, found, err := Read(cache, "v0.61.2"); err != nil || found {
		t.Fatalf("Read(current) found=%t err=%v, want no update row", found, err)
	}
}

func TestLocalHotfixVersionStillDiscoversNewRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "/mreza0100/professor/releases/tag/v0.61.5")
		writer.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	cache := filepath.Join(t.TempDir(), "update.json")
	const current = "v0.61.3-local.2"
	if err := Check(context.Background(), cache, current, server.URL, server.Client()); err != nil {
		t.Fatalf("Check(local hotfix) error = %v", err)
	}
	notice, found, err := Read(cache, current)
	if err != nil || !found || notice.Latest != "v0.61.5" || notice.Current != current {
		t.Fatalf("Read(local hotfix) notice=%#v found=%t err=%v", notice, found, err)
	}
}

func TestFailedRefreshPreservesLastSuccessfulNotice(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "/mreza0100/professor/releases/tag/v0.61.2")
		writer.WriteHeader(http.StatusFound)
	}))
	cache := filepath.Join(t.TempDir(), "update.json")
	if err := Check(context.Background(), cache, "v0.61.1", good.URL, good.Client()); err != nil {
		t.Fatal(err)
	}
	good.Close()

	failing := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failing.Close()
	if err := Check(context.Background(), cache, "v0.61.1", failing.URL, failing.Client()); err == nil {
		t.Fatal("failed refresh returned nil")
	}
	if notice, found, err := Read(cache, "v0.61.1"); err != nil || !found || notice.Latest != "v0.61.2" {
		t.Fatalf("last success was lost: notice=%#v found=%t err=%v", notice, found, err)
	}
}
