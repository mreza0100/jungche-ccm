package harvestmcp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	goRuntime "runtime"
	"strings"
	"testing"

	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestpy"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStableSixToolSurfaceAndFetchPrompt(t *testing.T) {
	service, err := NewConfigured("test", Runtime{Home: t.TempDir(), CacheDir: filepath.Join(t.TempDir(), "cache")})
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := service.Server().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "fixture", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	want := []string{"archive", "fetch", "fetchImage", "findWorks", "search", "searchCache"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %#v, want %#v", got, want)
	}
	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "fetch" {
		t.Fatalf("prompts = %#v, want one fetch prompt", prompts.Prompts)
	}
}

func TestOracleReceiptRenderers(t *testing.T) {
	listing := renderArchiveListing("/tmp/sample.zip", []harvest.Member{{Name: "a|b.txt", UncompressedSize: 7}})
	if want := `archive(source="/tmp/sample.zip", member="<name>")`; !contains(listing, want) {
		t.Fatalf("archive listing does not teach archive member call: %q", listing)
	}
	if !contains(listing, `| a\|b.txt | 7 | file |`) {
		t.Fatalf("archive listing does not escape table member: %q", listing)
	}
	if got := renderSearch("q", nil, "error"); got != "The web-search backend(s) are configured but unreachable or failing right now — retry shortly, or check that SEARXNG_URL is up and BRAVE_API_KEY is valid." {
		t.Fatalf("search failure receipt = %q", got)
	}
}

func TestDescribeLegacyFailureKindsNameTheSameRecovery(t *testing.T) {
	tests := []struct {
		name   string
		result harvest.Result
		want   []string
	}{
		{
			name:   "invalid URL",
			result: harvest.Result{ErrorKind: "invalid"},
			want:   []string{"Invalid URL", "`search`"},
		},
		{
			name:   "timeout",
			result: harvest.Result{ErrorKind: "timeout"},
			want:   []string{"timed out", "`search`"},
		},
		{
			name:   "challenge",
			result: harvest.Result{Challenge: true, HTTPStatus: 200},
			want:   []string{"challenge", "`search`"},
		},
		{
			name:   "HTTP 404",
			result: harvest.Result{Content: "tiny", ContentChars: 4, HTTPStatus: 404},
			want:   []string{"HTTP 404", "`search`", "`findWorks`"},
		},
		{
			name:   "thin extraction",
			result: harvest.Result{HTTPStatus: 200},
			want:   []string{"no readable content", "`search`", "`findWorks`"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := describeFetch("https://fixture.example/source", test.result, false)
			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Fatalf("describe receipt missing %q: %q", want, got)
				}
			}
		})
	}
}

func TestCacheDirectoryPrecedenceMatchesHarvesterOracle(t *testing.T) {
	t.Setenv("WEBFETCH_DIR", "/fixture/webfetch")
	t.Setenv("HARVESTER_CACHE_DIR", "relative-harvest")
	if got := resolveCacheDir(Runtime{}); got != "/fixture/webfetch" {
		t.Fatalf("WEBFETCH_DIR precedence = %q", got)
	}
	t.Setenv("WEBFETCH_DIR", "")
	if got := resolveCacheDir(Runtime{}); got != filepath.Join(mustWorkingDir(t), "relative-harvest") {
		t.Fatalf("relative HARVESTER_CACHE_DIR = %q", got)
	}
	t.Setenv("HARVESTER_CACHE_DIR", "/fixture/absolute")
	if got := resolveCacheDir(Runtime{}); got != "/fixture/absolute" {
		t.Fatalf("absolute HARVESTER_CACHE_DIR = %q", got)
	}
	t.Setenv("HARVESTER_CACHE_DIR", "")
	if got := resolveCacheDir(Runtime{}); got != ".cache" {
		t.Fatalf("default cache = %q", got)
	}
}

func mustWorkingDir(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return working
}

func contains(value, needle string) bool {
	return strings.Contains(value, needle)
}

// TestBrowserPathsResolveTheNormalizedPlatform pins the HIGH review finding:
// production must probe env-browser/<GOOS>-<GOARCH>/ — never the "-" sentinel
// an empty Platform{} stringifies to, which provisioning never writes.
func TestBrowserPathsResolveTheNormalizedPlatform(t *testing.T) {
	root := t.TempDir()
	converter := pythonConverter{browserRoot: root}
	interpreter, script := converter.browserPaths()
	normalized := harvestpy.Platform{GOOS: goRuntime.GOOS, GOARCH: goRuntime.GOARCH}
	wantRoot := harvestpy.BrowserRuntimeRoot(root, normalized)
	wantInterpreter := filepath.Join(wantRoot, "project", ".venv", "bin", "python")
	wantScript := filepath.Join(wantRoot, "project", "browser.py")
	if interpreter != wantInterpreter || script != wantScript {
		t.Fatalf("browser paths=%q/%q, want the normalized-platform root %q", interpreter, script, wantRoot)
	}
	if strings.Contains(interpreter, `"-/`) || strings.HasSuffix(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(interpreter)))), "-") {
		t.Fatalf("browser path uses the empty-Platform %q sentinel: %q", harvestpy.Platform{}.String(), interpreter)
	}
}
