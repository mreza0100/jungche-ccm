// Package harvestmcp exposes the Harvester's stable six-tool MCP surface.
//
// Transport, cache and policy remain in internal/harvest. This package only
// adapts those operations to MCP and preserves the Python server's receipts.
package harvestmcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestpy"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultInlineChars = 50000
	maxFetchSources    = 50
	maxImageSources    = 50
	maxSearchResults   = 20
	maxFindResults     = 25
	maxCacheResults    = 1000
)

// Runtime contains only machine-local paths and backend settings. The MCP
// service never reads a machine config file itself; the command passes this
// already-resolved runtime in.
type Runtime struct {
	Home        string
	CacheDir    string
	LocalRoots  []string
	Python      string
	Script      string
	SearXNGURL  string
	BraveAPIKey string
	UserAgent   string
	ProxyURL    string
	Client      *http.Client
}

// Service is one independent Harvester MCP server.
type Service struct {
	server    *mcp.Server
	harvester *harvest.Harvester
	resolver  *harvest.Resolver
	runtime   Runtime
	worker    *harvestpy.Converter
}

// NewConfigured builds the stdio-independent service for tests and command
// wiring. A nil converter is not used: all document conversion is delegated
// to the pinned Python worker selected by the runtime.
func NewConfigured(version string, runtime Runtime) (*Service, error) {
	if runtime.CacheDir == "" {
		runtime.CacheDir = resolveCacheDir(runtime)
	}
	// Keep the resolver's direct client separate from harvest.New's default
	// ladder. Passing one client into every rung silently demotes Chrome and
	// changes the old resolver's polite-UA behavior.
	resolverClient := runtime.Client
	if resolverClient == nil {
		client, err := newHTTPClient(runtime)
		if err != nil {
			return nil, err
		}
		resolverClient = client
	}
	coreRuntime := runtime
	h, worker, err := newHarvester(coreRuntime)
	if err != nil {
		return nil, err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "harvester", Version: version}, &mcp.ServerOptions{
		Instructions: "Fetch, search, resolve, browse, and cache public documents with Harvester.",
	})
	service := &Service{server: server, harvester: h, resolver: &harvest.Resolver{Client: resolverClient}, runtime: runtime, worker: worker}
	service.register()
	return service, nil
}

// NewHarvester wires the Go transport to the pinned Python conversion worker.
// The returned Harvester is useful to the CLI, which shares the exact MCP
// fetch semantics without starting a second server.
func NewHarvester(runtime Runtime) (*harvest.Harvester, error) {
	harvester, _, err := newHarvester(runtime)
	return harvester, err
}

func newHarvester(runtime Runtime) (*harvest.Harvester, *harvestpy.Converter, error) {
	// A local CLI/stdin caller owns the machine. Preserve the oracle's
	// unconfined local-read posture (apart from harvest's credential denylist);
	// an empty, non-nil slice prevents the core's deployment-only env roots
	// default from accidentally confining local callers.
	if runtime.LocalRoots == nil {
		runtime.LocalRoots = []string{}
	}
	if runtime.CacheDir == "" {
		runtime.CacheDir = resolveCacheDir(runtime)
	}
	if runtime.SearXNGURL == "" {
		runtime.SearXNGURL = os.Getenv("SEARXNG_URL")
	}
	if runtime.BraveAPIKey == "" {
		runtime.BraveAPIKey = os.Getenv("BRAVE_API_KEY")
	}
	if runtime.Client != nil && runtime.Client.CheckRedirect == nil {
		runtime.Client.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
			parsed := request.URL
			return assertPublicURL(parsed)
		}
	}
	if runtime.Python == "" {
		runtime.Python = os.Getenv("PFM_HARVEST_PYTHON")
	}
	if runtime.Python == "" {
		root, rootErr := harvestStateRoot(runtime)
		if rootErr != nil {
			return nil, nil, rootErr
		}
		current := harvestpy.RuntimeRoot(root, harvestpy.Platform{GOOS: goRuntime.GOOS, GOARCH: goRuntime.GOARCH})
		runtime.Python = filepath.Join(current, "project", ".venv", "bin", "python")
		if runtime.Script == "" {
			runtime.Script = filepath.Join(current, "project", "converter.py")
		}
	}
	worker := harvestpy.NewConverter(harvestpy.Runtime{Python: runtime.Python, Script: runtime.Script})
	browserRoot, rootErr := harvestStateRoot(runtime)
	if rootErr != nil {
		return nil, nil, rootErr
	}
	converter := pythonConverter{
		worker:      worker,
		browserRoot: browserRoot,
		proxyURL:    runtime.ProxyURL,
	}
	return harvest.New(harvest.Options{
		CacheDir:   runtime.CacheDir,
		Client:     runtime.Client,
		Chrome:     nil,
		Jina:       nil,
		OA:         nil,
		Converter:  converter,
		LocalRoots: runtime.LocalRoots,
		ProxyURL:   runtime.ProxyURL,
		UserAgent:  runtime.UserAgent,
	}), worker, nil
}

// harvestStateRoot resolves the managed harvest-python state root. PFM_HARVEST_ROOT
// is a test-jail override; production lands under the user's state dir.
func harvestStateRoot(runtime Runtime) (string, error) {
	if root := os.Getenv("PFM_HARVEST_ROOT"); root != "" {
		return root, nil
	}
	home := runtime.Home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve Harvester Python home: %w", err)
		}
	}
	return filepath.Join(home, ".local", "state", "pfm", "harvest-python"), nil
}

func newHTTPClient(runtime Runtime) (*http.Client, error) {
	transport := http.DefaultTransport
	if runtime.ProxyURL != "" {
		proxy, err := url.Parse(runtime.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse Harvester proxy URL: %w", err)
		}
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("default HTTP transport cannot be configured for proxy")
		}
		copyTransport := base.Clone()
		copyTransport.Proxy = http.ProxyURL(proxy)
		transport = copyTransport
	}
	if runtime.UserAgent != "" {
		transport = userAgentTransport{base: transport, value: runtime.UserAgent}
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: transport}, nil
}

type userAgentTransport struct {
	base  http.RoundTripper
	value string
}

func (transport userAgentTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("User-Agent", transport.value)
	return transport.base.RoundTrip(clone)
}

type pythonConverter struct {
	worker *harvestpy.Converter
	// browserRoot is the managed harvest-python state root the opt-in
	// real-browser environment provisions under (env-browser/<platform>).
	browserRoot string
	proxyURL    string
}

// browserHardDeadline is the Go-side ceiling on one browser fetch. The
// 45000ms timeout travels to Python and bounds page.goto only — launch, IPC,
// the headed→headless retry, and close() answer to nobody else. Every other
// transport in this file has a hard Go ceiling; the browser gets one too.
const browserHardDeadline = 3 * time.Minute

// FetchBrowser renders one URL in system Chrome through the opt-in Patchright
// worker — the ladder's last wall-bypass rung. Go owns the SSRF decision:
// every URL Chrome touches is validated here through
// harvest.AssertFetchableStrict — strict, because Chrome re-resolves without a
// pinning hop — and a refusal is re-wrapped as harvest.ErrBrowserPolicyDenied so callers can
// tell POLICY from OUTAGE. Provisioning is lazy and only ever happens after
// HARVESTER_BROWSER=1 gated this method; a missing environment is an outage,
// never a silent skip.
func (converter pythonConverter) FetchBrowser(ctx context.Context, source string) (string, int, error) {
	runtime, err := converter.browserRuntime(ctx)
	if err != nil {
		return "", 0, err
	}
	browser := harvestpy.NewBrowserWorker(runtime)
	defer func() { _ = browser.Close() }()

	policyDenied := false
	onAsk := func(url string) error {
		if askErr := harvest.AssertFetchableStrict(url); askErr != nil {
			// Only a refusal of the INITIAL address labels the whole fetch
			// as POLICY. A denied tracker/subresource followed by an
			// unrelated failure must stay an outage — retrying can help.
			if url == source {
				policyDenied = true
			}
			return askErr
		}
		return nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, browserHardDeadline)
	defer cancel()
	html, status, fetchErr := browser.Fetch(fetchCtx, source, converter.proxyURL, 45000, onAsk)
	if fetchErr != nil && policyDenied {
		return "", 0, fmt.Errorf("%w: %v", harvest.ErrBrowserPolicyDenied, fetchErr)
	}
	return html, status, fetchErr
}

// browserPaths resolves the interpreter/worker script under the REAL host
// platform root. An empty Platform{} stringifies to "-" — a path
// provisioning never writes — so the platform is always normalized here.
func (converter pythonConverter) browserPaths() (interpreter string, script string) {
	platform := harvestpy.Platform{GOOS: goRuntime.GOOS, GOARCH: goRuntime.GOARCH}
	root := harvestpy.BrowserRuntimeRoot(converter.browserRoot, platform)
	return filepath.Join(root, "project", ".venv", "bin", "python"),
		filepath.Join(root, "project", "browser.py")
}

// browserRuntime resolves the provisioned interpreter/worker paths, lazily
// provisioning the opt-in environment on first use. Paths are resolved AFTER
// provisioning so a first-use provision is picked up on the same call. It
// NEVER downloads Chromium: patchright drives system Chrome.
func (converter pythonConverter) browserRuntime(ctx context.Context) (harvestpy.Runtime, error) {
	interpreter, script := converter.browserPaths()
	if _, statErr := os.Stat(interpreter); errors.Is(statErr, os.ErrNotExist) {
		if _, provisionErr := harvestpy.ProvisionBrowser(ctx, harvestpy.ProvisionOptions{Root: converter.browserRoot}); provisionErr != nil {
			return harvestpy.Runtime{}, fmt.Errorf("browser environment is NOT provisioned and lazy provisioning failed (%v) — run `pfm install` with HARVESTER_BROWSER=1 to provision it", provisionErr)
		}
	} else if statErr != nil {
		return harvestpy.Runtime{}, fmt.Errorf("probe browser environment interpreter %s: %w", interpreter, statErr)
	}
	return harvestpy.Runtime{Python: interpreter, Script: script}, nil
}

func (converter pythonConverter) Convert(ctx context.Context, kind, source string, body []byte) (markdown string, returnErr error) {
	directory, err := os.MkdirTemp("", "pfm-harvest-input-")
	if err != nil {
		return "", fmt.Errorf("create conversion scratch: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove conversion scratch: %w", cleanupErr))
		}
	}()
	extension := strings.TrimPrefix(filepath.Ext(source), ".")
	if extension == "" || extension == source {
		extension = kind
	}
	path := filepath.Join(directory, "input."+extension)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write conversion scratch: %w", err)
	}
	result, convertErr := converter.worker.Convert(ctx, harvestpy.Request{Path: path, Kind: kind, Source: source})
	if convertErr != nil {
		return "", convertErr
	}
	return result.Markdown, nil
}

// ConvertOCR forces ONE Tesseract pass for this document only — the dispatch's
// last-resort rung for scanned PDFs whose text layer converts empty.
func (converter pythonConverter) ConvertOCR(ctx context.Context, kind, source string, body []byte) (markdown string, returnErr error) {
	directory, err := os.MkdirTemp("", "pfm-harvest-ocr-")
	if err != nil {
		return "", fmt.Errorf("create OCR scratch: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove OCR scratch: %w", cleanupErr))
		}
	}()
	path := filepath.Join(directory, "input."+strings.TrimPrefix(filepath.Ext(source), "."))
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write OCR scratch: %w", err)
	}
	result, convertErr := converter.worker.Convert(ctx, harvestpy.Request{Path: path, Kind: kind, Source: source, OCR: true})
	if convertErr != nil {
		return "", convertErr
	}
	return result.Markdown, nil
}

// Server exposes the SDK server for in-memory protocol tests.
func (service *Service) Server() *mcp.Server { return service.server }

// Close reaps the managed pinned-Python worker. External and internal HTTP
// gateways share one Service, so callers close only the external Service.
func (service *Service) Close() error {
	if service == nil || service.worker == nil {
		return nil
	}
	return service.worker.Close()
}

// RunStdio serves the stable service over newline-delimited MCP JSON.
func (service *Service) RunStdio(ctx context.Context, input io.Reader, output io.Writer) error {
	return service.server.Run(ctx, &mcp.IOTransport{Reader: nopReaderCloser{Reader: input}, Writer: nopWriterCloser{Writer: output}})
}

// NewHTTPHandler exposes the same tool surface as stdio through streamable
// HTTP. The process-level daemon supplies authentication and endpoint routing.
func (service *Service) NewHTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return service.server },
		&mcp.StreamableHTTPOptions{
			JSONResponse:               true,
			Stateless:                  false,
			DisableLocalhostProtection: true,
		},
	)
}

// The SDK's IOTransport takes ownership of ReadCloser/WriteCloser values and
// closes both at connection shutdown. RunStdio accepts the older, deliberately
// non-owning io.Reader/io.Writer contract, so adapt them with no-op closers.
type nopReaderCloser struct{ io.Reader }

func (nopReaderCloser) Close() error { return nil }

type nopWriterCloser struct{ io.Writer }

func (nopWriterCloser) Close() error { return nil }

func (service *Service) register() {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	mcp.AddTool(service.server, &mcp.Tool{Name: "fetch", Description: fetchDescription, InputSchema: fetchInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input FetchInput) (*mcp.CallToolResult, any, error) {
		result, _, err := service.fetch(ctx, request, input)
		return result, nil, err
	})
	mcp.AddTool(service.server, &mcp.Tool{Name: "findWorks", Description: findDescription, InputSchema: findInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input FindInput) (*mcp.CallToolResult, any, error) {
		result, _, err := service.findWorks(ctx, request, input)
		return result, nil, err
	})
	if harvest.SearchAdvertised() {
		mcp.AddTool(service.server, &mcp.Tool{Name: "search", Description: searchDescription, InputSchema: searchInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, any, error) {
			result, _, err := service.search(ctx, request, input)
			return result, nil, err
		})
	}
	mcp.AddTool(service.server, &mcp.Tool{Name: "fetchImage", Description: imageDescription, InputSchema: imageInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input ImageInput) (*mcp.CallToolResult, any, error) {
		result, _, err := service.fetchImage(ctx, request, input)
		return result, nil, err
	})
	mcp.AddTool(service.server, &mcp.Tool{Name: "archive", Description: archiveDescription, InputSchema: archiveInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input ArchiveInput) (*mcp.CallToolResult, any, error) {
		result, _, err := service.archive(ctx, request, input)
		return result, nil, err
	})
	mcp.AddTool(service.server, &mcp.Tool{Name: "searchCache", Description: cacheDescription, InputSchema: cacheInputSchema(), Annotations: readOnly}, func(ctx context.Context, request *mcp.CallToolRequest, input CacheInput) (*mcp.CallToolResult, any, error) {
		result, _, err := service.searchCache(ctx, request, input)
		return result, nil, err
	})
	service.server.AddPrompt(&mcp.Prompt{Name: "fetch", Description: "Fetch a URL or local path and convert its contents to markdown", Arguments: []*mcp.PromptArgument{{Name: "url", Description: "URL or path to fetch", Required: true}}}, service.fetchPrompt)
}

const codeTick = "`"
const fetchDescription = `Convert one or many **sources** (` + codeTick + `sources` + codeTick + `, a list of 1–50) to clean Markdown, in input order, each cached on disk (type-partitioned: ` + codeTick + `html/` + codeTick + `, ` + codeTick + `pdf/` + codeTick + `, ` + codeTick + `png/` + codeTick + `, …).

A *source* is either a **location** (where something lives) or an **identity** (what the thing is — you need not know where to find it). Harvester resolves both and hands back the content.

**Locations — fetched directly to document markdown:**
- Web URL / local path / ` + codeTick + `file://` + codeTick + ` → web page, PDF, DOCX, XLSX, PPTX, CSV, or JSON.
- HTML via trafilatura. Image references stay as URLs in the markdown — ` + codeTick + `fetch` + codeTick + ` never downloads image binaries and never OCRs; to VIEW one, pass its URL to ` + codeTick + `fetchImage` + codeTick + `, which returns a local path to read with vision. PDF via pymupdf4llm (extensionless URLs like ` + codeTick + `arxiv.org/pdf/…` + codeTick + ` are header-sniffed; a scanned PDF whose text layer is empty gets one automatic OCR pass). DOCX/XLSX/PPTX via Docling, CSV/EPUB via MarkItDown, JSON pretty-printed. Credential/secret files are refused.
- An IMAGE → use the ` + codeTick + `fetchImage` + codeTick + ` tool. An ARCHIVE (.zip/.tar/.7z/.rar) → use the ` + codeTick + `archive` + codeTick + ` tool. ` + codeTick + `fetch` + codeTick + ` will redirect you if you pass one here.

**Identities — resolved to a free, legal copy, then converted:**
- **DOI** — ` + codeTick + `10.xxxx/…` + codeTick + `, ` + codeTick + `doi:…` + codeTick + `, or a ` + codeTick + `doi.org` + codeTick + ` URL.
- **Book by ISBN** — ` + codeTick + `isbn:9780262300988` + codeTick + ` (or a bare ISBN) → a free OA/public-domain copy.
- **PMID / PMCID** — a bare PubMed ID (e.g. 30220343) or a PMC accession (PMC1234567) → resolved via Europe PMC/PMC.
Harvester runs the legal open-access chain — for papers: OpenAlex, Semantic Scholar, Europe PMC, OpenAIRE, Zenodo, eLife, PLOS, NBER, Crossref, CORE, DOAJ (+ Unpaywall when HARVESTER_CONTACT_EMAIL is set; arXiv/ar5iv & OSF/SocArXiv resolve by DOI prefix); for books: OAPEN, DOAB, Internet Archive, HathiTrust (full view), Project Gutenberg — returning the first copy that yields real content. Only API-sanctioned sources; no shadow libraries.
**Have only a TITLE?** Titles are ambiguous, so ` + codeTick + `fetch` + codeTick + ` won't guess — call the **` + codeTick + `findWorks` + codeTick + ` tool first (it lists candidate works), then fetch the one you choose by its DOI/URL.

**Wall-bypass — when ANY URL is blocked, it goes down the rabbit hole:** httpx → Chrome-fingerprint impersonation (tls-client) → Jina Reader → defuddle.md → real browser (opt-in: Patchright + system Chrome behind ` + codeTick + `HARVESTER_BROWSER=1` + codeTick + `; passes passive Cloudflare managed challenges, never solves interactive CAPTCHAs) → then it extracts the DOI from the page/URL (or a ` + codeTick + `citation_pdf_url` + codeTick + ` meta tag) and runs the open-access chain → Wayback Machine. So a paywalled or bot-blocked publisher link still returns the open copy when one legally exists. Hard IP-reputation blocks need a residential exit — the server says so plainly.

**Sibling tools:** ` + codeTick + `search` + codeTick + ` (open-web search → URLs to fetch), ` + codeTick + `findWorks` + codeTick + ` (a title → candidate works to choose from), ` + codeTick + `fetchImage` + codeTick + ` (an image → a local path to read with vision), ` + codeTick + `archive` + codeTick + ` (browse a .zip/.tar/.7z/.rar), ` + codeTick + `searchCache` + codeTick + ` (search what you already fetched).

Returns the content of every source in the SAME order. Each result: a short header (source, cache_status, method, bytes, tokens, fetched_at, cache path) then the content — content past the inline cap is truncated with a note, but the cache path always holds the COMPLETE text. A failing source yields a descriptive per-item error naming what to try next; the rest still return. Set ` + codeTick + `size_only: true` + codeTick + ` to get just {size, chars, path} per source (full content still cached) — probe a source's size, then slice the cached path from disk.`
const findDescription = `Find scholarly papers and books by TITLE or free-text query — the scholarly counterpart of WebSearch. Returns a RANKED LIST of candidate works (papers + books), each with a ready-to-use ` + codeTick + `fetch:` + codeTick + ` handle; it does NOT download anything.

Use it whenever you have a TITLE or a fuzzy description rather than a URL / DOI / ISBN — ` + codeTick + `fetch` + codeTick + ` deliberately won't guess which work a title means, so ` + codeTick + `findWorks` + codeTick + ` shows the matches and you choose. Each result lists: title · authors · year · kind (paper|book) · source · free-access status · a ` + codeTick + `fetch:` + codeTick + ` handle (a DOI, an ` + codeTick + `isbn:` + codeTick + ` string, or a direct URL). Then call ` + codeTick + `fetch` + codeTick + ` with the handle of the one you want.

Two-step pattern, exactly like WebSearch → WebFetch: **findWorks → fetch**. (Papers come from OpenAlex; books from Open Library + Project Gutenberg.)`
const searchDescription = `Search the open web — a stronger, privacy-respecting replacement for the built-in WebSearch. Returns ranked results (title · URL · snippet · engine); pick the URLs you want and retrieve them with ` + codeTick + `fetch` + codeTick + `.

Backed by a self-hosted **SearXNG** that aggregates 200+ engines (less single-engine/SEO bias than a plain Google search), with the **Brave** Search API as fallback. It returns links + snippets to TRIAGE, not full content — that's ` + codeTick + `fetch` + codeTick + `'s job (search → fetch, like findWorks → fetch).

**Multilingual:** set ` + codeTick + `lang` + codeTick + ` (e.g. ` + codeTick + `zh` + codeTick + `, ` + codeTick + `ja` + codeTick + `, ` + codeTick + `pt-BR` + codeTick + `) to route the query to that language's native engines — the way to reach Chinese / Japanese / Brazilian / etc. web results that an English search never surfaces. Optionally restrict to specific ` + codeTick + `engines` + codeTick + `.`
const imageDescription = `Fetch one or many images and return their LOCAL FILE PATHS to read with your vision — figures, photos, charts, scanned pages. Each ` + codeTick + `sources` + codeTick + ` item is an image URL or a local image path; the bytes are saved under ` + codeTick + `<cache>/<ext>/` + codeTick + ` and the path returned in order — open that path with your vision to read the figure/photo. Images are NOT OCR'd or turned into text — you OPEN the returned path with your vision to see the content (a chart or photo carries information no caption can). Use this instead of ` + codeTick + `fetch` + codeTick + ` whenever the thing is a picture; ` + codeTick + `fetch` + codeTick + ` returns document markdown (image refs left as URLs) and will point you here for an image.`
const archiveDescription = `Safely browse a single archive — ` + codeTick + `.zip` + codeTick + ` / ` + codeTick + `.tar(.gz/.bz2/.xz)` + codeTick + ` / ` + codeTick + `.7z` + codeTick + ` / ` + codeTick + `.rar` + codeTick + ` — given by URL or local path.

Two-step, like ` + codeTick + `findWorks` + codeTick + ` → ` + codeTick + `fetch` + codeTick + `: call with NO ` + codeTick + `member` + codeTick + ` to get the SAFE member listing (names + sizes; nothing is extracted to disk). Then call again with one ` + codeTick + `member` + codeTick + ` name from that listing to fetch just that member, converted to Markdown. Path-traversal and symlink members are refused, member-count/size caps are enforced, and the archive is never auto-extracted. Use this instead of ` + codeTick + `fetch` + codeTick + ` for any archive.`
const cacheDescription = `Search every page already cached on disk for a regex ` + codeTick + `pattern` + codeTick + `, returning WHICH cached pages match — source URL, match count, a sample line, and the cached ` + codeTick + `md_path` + codeTick + ` — not their text. Recall what you have already fetched without re-crawling; to read a match's content, ` + codeTick + `fetch` + codeTick + ` the source again (served from cache) or read ` + codeTick + `md_path` + codeTick + ` directly from disk.`

func defaultJSON(value string) json.RawMessage { return json.RawMessage(value) }

func numberSchema(description string, minimum, maximum float64, defaultValue int) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "integer",
		Description: description,
		Minimum:     &minimum,
		Maximum:     &maximum,
		Default:     defaultJSON(strconv.Itoa(defaultValue)),
	}
}

func arraySchema(description string, minimum, maximum int) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        "array",
		Description: description,
		Items:       &jsonschema.Schema{Type: "string"},
		MinItems:    &minimum,
		MaxItems:    &maximum,
	}
}

func fetchInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object", Required: []string{"sources"},
		Properties: map[string]*jsonschema.Schema{
			"sources":   arraySchema("1–50 things to fetch, each returned as clean Markdown in the SAME order. Each is a LOCATION or an UNAMBIGUOUS identifier of a DOCUMENT: URL / local path / file://; DOI; ISBN; PMID / PMCID. Use a DIFFERENT tool for a TITLE (findWorks), an IMAGE (fetchImage), or an archive (archive). A failing item returns a descriptive per-item error and the rest still return.", 1, 50),
			"refresh":   {Type: "boolean", Description: "Force a fresh fetch: bypass the cache entirely, re-download, overwrite the cached artifact, and return the NEW content.", Default: defaultJSON("false")},
			"size_only": {Type: "boolean", Description: "When true, fetch and cache the full content but return NO body—just {size, chars, path}; full content remains cached at path.", Default: defaultJSON("false")},
		},
	}
}

func findInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Required: []string{"query"}, Properties: map[string]*jsonschema.Schema{
		"query": {Type: "string", Description: "A paper or book TITLE, or a free-text bibliographic query. Returns a ranked list of candidate works with a fetch handle; pick one and pass it to fetch."},
		"limit": numberSchema("Maximum number of candidate works to return.", 1, 25, 8),
	}}
}

func searchInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Required: []string{"query"}, Properties: map[string]*jsonschema.Schema{
		"query":   {Type: "string", Description: "The web search query."},
		"count":   numberSchema("Maximum number of results to return.", 1, 20, 8),
		"lang":    {Type: "string", Description: "Optional language/locale to bias the search (e.g. 'zh', 'ja', 'pt-BR'). Set it to reach a NON-English literature."},
		"engines": {Type: "string", Description: "Optional comma-separated SearXNG engines to restrict to (e.g. 'google,brave' or 'naver,yahoo'). Omit for the default aggregated set."},
	}}
}

func imageInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Required: []string{"sources"}, Properties: map[string]*jsonschema.Schema{
		"sources": arraySchema("1–50 image URLs or local image paths. Each is downloaded into the type-partitioned cache and its LOCAL FILE PATH is returned in order — images are NOT OCR'd.", 1, 50),
	}}
}

func archiveInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Required: []string{"source"}, Properties: map[string]*jsonschema.Schema{
		"source": {Type: "string", Description: "URL or local path of a .zip / .tar(.gz/.bz2/.xz) / .7z / .rar archive."},
		"member": {Types: []string{"string", "null"}, Description: "Omit to get the SAFE member listing (names + sizes; nothing is extracted). Give one member name from that listing to fetch just that member, converted to Markdown."},
	}}
}

func cacheInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object", Required: []string{"pattern"}, Properties: map[string]*jsonschema.Schema{
		"pattern":     {Type: "string", Description: "Regex pattern to search across every cached markdown body in the cache."},
		"max_results": numberSchema("Maximum number of matching cached pages to return.", 1, 1000, 50),
		"ignore_case": {Type: "boolean", Description: "Case-insensitive search.", Default: defaultJSON("true")},
	}}
}

type FetchInput struct {
	Sources  []string `json:"sources" jsonschema:"1–50 things to fetch, each returned as clean Markdown in the SAME order. Each is a LOCATION or an UNAMBIGUOUS identifier of a DOCUMENT: URL/local path/file:// for web pages and documents; DOI (bare, doi: prefix, or doi.org URL); ISBN; PMID/PMCID. Use findWorks for a TITLE, fetchImage for an IMAGE, and archive for ZIP/TAR/7z/RAR. A failing item returns a descriptive per-item error and the rest still return."`
	Refresh  bool     `json:"refresh,omitempty" jsonschema:"Force a fresh fetch: bypass the cache entirely, re-download, overwrite the cached artifact, and return the NEW content. Normally cached web pages and documents older than a day are refreshed automatically; images and archives stay cached until refresh."`
	SizeOnly bool     `json:"size_only,omitempty" jsonschema:"When true, fetch and cache the full content but return NO body—just {size, chars, path}, where size is an estimated token count and chars is the raw character count. The full content remains at path."`
}
type FindInput struct {
	Query string `json:"query" jsonschema:"A paper or book TITLE, or a free-text bibliographic query. Returns ranked candidate works with a fetch handle; choose one and pass that handle to fetch."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of candidate works to return, default 8 and maximum 25."`
}
type ImageInput struct {
	Sources []string `json:"sources" jsonschema:"1–50 image URLs or local image paths. Each is downloaded into the type-partitioned cache and its LOCAL FILE PATH is returned in order for vision. Images are NOT OCR'd."`
}
type ArchiveInput struct {
	Source string `json:"source" jsonschema:"URL or local path of a .zip / .tar(.gz/.bz2/.xz) / .7z / .rar archive."`
	Member string `json:"member,omitempty" jsonschema:"Omit to get the SAFE member listing (names + sizes; nothing extracted). Give one member name from that listing to fetch just that member, converted to Markdown."`
}
type SearchInput struct {
	Query   string `json:"query" jsonschema:"The web search query."`
	Count   int    `json:"count,omitempty" jsonschema:"Maximum number of results to return, default 8 and maximum 20."`
	Lang    string `json:"lang,omitempty" jsonschema:"Optional language/locale to bias the search (e.g. zh, ja, pt-BR). Set it to reach non-English literature."`
	Engines string `json:"engines,omitempty" jsonschema:"Optional comma-separated SearXNG engines to restrict to (e.g. google,brave or naver,yahoo). Omit for the default aggregated set."`
}
type CacheInput struct {
	Pattern    string `json:"pattern" jsonschema:"Regex pattern to search across every cached markdown body in the cache."`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"Maximum number of matching cached pages to return, default 50 and maximum 1000."`
	IgnoreCase *bool  `json:"ignore_case,omitempty" jsonschema:"Case-insensitive search, default true."`
}

type FetchItem struct {
	Source      string   `json:"source"`
	Content     string   `json:"content,omitempty"`
	CacheStatus string   `json:"cache_status,omitempty"`
	Method      string   `json:"method,omitempty"`
	Bytes       int64    `json:"bytes,omitempty"`
	Tokens      int      `json:"tokens,omitempty"`
	Chars       int      `json:"chars,omitempty"`
	Path        string   `json:"path,omitempty"`
	Error       string   `json:"error,omitempty"`
	Rungs       []string `json:"rungs,omitempty"`
	FetchedAt   string   `json:"fetched_at,omitempty"`
}
type FetchOutput struct {
	Items []FetchItem `json:"items"`
}
type FindOutput struct {
	Candidates []harvest.Candidate `json:"candidates"`
}
type SearchOutput struct {
	Results []harvest.SearchResult `json:"results"`
	Backend string                 `json:"backend,omitempty"`
}
type ImageItem struct {
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
	Error  string `json:"error,omitempty"`
}
type ImageOutput struct {
	Items []ImageItem `json:"items"`
}
type ArchiveOutput struct {
	Result harvest.Result `json:"result"`
}
type CacheHit struct {
	URL     string `json:"url"`
	Path    string `json:"path"`
	Matches int    `json:"matches"`
	Sample  string `json:"sample,omitempty"`
}
type CacheOutput struct {
	Matches []CacheHit `json:"matches"`
}

func (service *Service) fetch(ctx context.Context, _ *mcp.CallToolRequest, input FetchInput) (*mcp.CallToolResult, FetchOutput, error) {
	if len(input.Sources) < 1 || len(input.Sources) > maxFetchSources {
		return nil, FetchOutput{}, fmt.Errorf("sources must contain 1-%d items", maxFetchSources)
	}
	contents := make([]string, len(input.Sources))
	items := make([]FetchItem, len(input.Sources))
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for index, source := range input.Sources {
		wait.Add(1)
		go func(index int, source string) {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				contents[index] = describeFetch(source, harvest.Result{Source: source, Error: "fetch cancelled: " + ctx.Err().Error()}, input.SizeOnly)
				items[index] = FetchItem{Source: source, Error: "fetch cancelled: " + ctx.Err().Error()}
				return
			}
			defer func() { <-semaphore }()
			fetched := service.harvester.FetchWithOptions(ctx, source, harvest.FetchOptions{Refresh: input.Refresh, SizeOnly: input.SizeOnly})
			items[index] = fetchItem(fetched)
			contents[index] = describeFetch(source, fetched, input.SizeOnly)
		}(index, source)
	}
	wait.Wait()
	result := &mcp.CallToolResult{}
	for _, text := range contents {
		result.Content = append(result.Content, &mcp.TextContent{Text: text})
	}
	return result, FetchOutput{Items: items}, nil
}

func (service *Service) findWorks(ctx context.Context, _ *mcp.CallToolRequest, input FindInput) (*mcp.CallToolResult, FindOutput, error) {
	if strings.TrimSpace(input.Query) == "" {
		return nil, FindOutput{}, errors.New("query must not be empty")
	}
	if input.Limit == 0 {
		input.Limit = 8
	}
	if input.Limit < 1 || input.Limit > maxFindResults {
		return nil, FindOutput{}, fmt.Errorf("limit must be between 1 and %d", maxFindResults)
	}
	candidates, err := service.resolver.FindWorks(ctx, input.Query, input.Limit)
	if err != nil {
		return nil, FindOutput{}, err
	}
	lines := renderFind(input.Query, candidates)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: lines}}}, FindOutput{Candidates: candidates}, nil
}

func (service *Service) search(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	if strings.TrimSpace(input.Query) == "" {
		return nil, SearchOutput{}, errors.New("query must not be empty")
	}
	if input.Count == 0 {
		input.Count = 8
	}
	if input.Count < 1 || input.Count > maxSearchResults {
		return nil, SearchOutput{}, fmt.Errorf("count must be between 1 and %d", maxSearchResults)
	}
	results, backend, err := harvest.Search(ctx, input.Query, harvest.SearchOptions{SearXNGURL: service.runtime.SearXNGURL, BraveAPIKey: service.runtime.BraveAPIKey, Lang: input.Lang, Engines: input.Engines, Count: input.Count})
	if err != nil && backend != "" && backend != "error" {
		return nil, SearchOutput{Backend: backend}, err
	}
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderSearch(input.Query, results, backend)}}}, SearchOutput{Results: results, Backend: backend}, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderSearch(input.Query, results, backend)}}}, SearchOutput{Results: results, Backend: backend}, nil
}

func (service *Service) fetchImage(ctx context.Context, _ *mcp.CallToolRequest, input ImageInput) (*mcp.CallToolResult, ImageOutput, error) {
	if len(input.Sources) < 1 || len(input.Sources) > maxImageSources {
		return nil, ImageOutput{}, fmt.Errorf("sources must contain 1-%d items", maxImageSources)
	}
	items := make([]ImageItem, len(input.Sources))
	contents := make([]string, len(input.Sources))
	var wait sync.WaitGroup
	semaphore := make(chan struct{}, 8)
	for index, source := range input.Sources {
		wait.Add(1)
		go func(index int, source string) {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				items[index] = ImageItem{Source: source, Error: "fetch image cancelled: " + ctx.Err().Error()}
				contents[index] = fmt.Sprintf("# %s\nERROR: %s", source, items[index].Error)
				return
			}
			defer func() { <-semaphore }()
			item := service.fetchOneImage(ctx, source)
			items[index] = item
			body, err := json.Marshal(item)
			if err != nil {
				contents[index] = fmt.Sprintf("image receipt encoding failed for %q: %v", source, err)
				return
			}
			contents[index] = string(body)
		}(index, source)
	}
	wait.Wait()
	result := &mcp.CallToolResult{}
	for _, content := range contents {
		result.Content = append(result.Content, &mcp.TextContent{Text: content})
	}
	return result, ImageOutput{Items: items}, nil
}

func (service *Service) archive(ctx context.Context, _ *mcp.CallToolRequest, input ArchiveInput) (*mcp.CallToolResult, ArchiveOutput, error) {
	if strings.TrimSpace(input.Source) == "" {
		return nil, ArchiveOutput{}, errors.New("source must not be empty")
	}
	result, err := service.harvester.Archive(ctx, input.Source, input.Member)
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: describeFetch(input.Source, result, false)}}}, ArchiveOutput{Result: result}, nil
	}
	if input.Member == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderArchiveListing(input.Source, result.Members)}}}, ArchiveOutput{Result: result}, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: describeFetch(input.Source, result, false)}}}, ArchiveOutput{Result: result}, nil
}

func (service *Service) searchCache(_ context.Context, _ *mcp.CallToolRequest, input CacheInput) (*mcp.CallToolResult, CacheOutput, error) {
	if strings.TrimSpace(input.Pattern) == "" {
		return nil, CacheOutput{}, errors.New("pattern must not be empty")
	}
	limit := input.MaxResults
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > maxCacheResults {
		return nil, CacheOutput{}, fmt.Errorf("max_results must be between 1 and %d", maxCacheResults)
	}
	ignore := true
	if input.IgnoreCase != nil {
		ignore = *input.IgnoreCase
	}
	cacheHits, err := service.harvester.SearchCache(input.Pattern, limit, ignore)
	if err != nil {
		return nil, CacheOutput{}, err
	}
	hits := make([]CacheHit, 0, len(cacheHits))
	for _, hit := range cacheHits {
		hits = append(hits, CacheHit{URL: hit.URL, Path: hit.Path, Matches: hit.Matches, Sample: hit.Sample})
	}
	lines := make([]string, 0, len(hits)+2)
	if len(hits) == 0 {
		text := fmt.Sprintf("No cached pages match /%s/. This only searches pages already fetched — it does not search the web; use `search` or `fetch` a source first.", input.Pattern)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, CacheOutput{Matches: hits}, nil
	}
	lines = append(lines, fmt.Sprintf("%d cached page(s) match /%s/ — this lists WHICH pages match, it does not return their text; `fetch` the source or read `md_path` directly for content:", len(hits), input.Pattern), "")
	for _, hit := range hits {
		lines = append(lines, fmt.Sprintf("- %s  (%d matches)  md_path: %s", hit.URL, hit.Matches, hit.Path))
		if hit.Sample != "" {
			lines = append(lines, "    "+hit.Sample)
		}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}}}, CacheOutput{Matches: hits}, nil
}

func (service *Service) fetchPrompt(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if request == nil || request.Params == nil || request.Params.Arguments == nil || strings.TrimSpace(request.Params.Arguments["url"]) == "" {
		return nil, errors.New("URL is required")
	}
	source := request.Params.Arguments["url"]
	fetched := service.harvester.Fetch(ctx, source)
	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Contents of %s", source),
		Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{
			Text: describeFetch(source, fetched, false),
		}}},
	}, nil
}

func fetchItem(result harvest.Result) FetchItem {
	return FetchItem{Source: result.Source, Content: result.Content, CacheStatus: result.CacheStatus, Method: result.Method, Bytes: result.Bytes, Tokens: result.Tokens, Chars: result.Chars, Path: result.Path, Error: result.Error, Rungs: result.Rungs}
}

func describeFetch(source string, result harvest.Result, sizeOnly bool) string {
	if result.Error != "" {
		return "# " + source + "\nERROR: " + result.Error
	}
	// Size probes never kill an empty extraction behind a zero-sized success;
	// this wording is part of the Python scheduler contract.
	if sizeOnly {
		if strings.TrimSpace(result.Content) == "" && result.Chars == 0 && result.Bytes == 0 {
			return fmt.Sprintf("# %s\nERROR: Fetched %s but it yielded no readable content (empty after extraction) — nothing to size. Use `search` to find an alternative copy, or `findWorks` if it is a scholarly title.", source, source)
		}
		body, err := json.Marshal(map[string]any{"source": source, "size": result.Tokens, "tokens": result.Tokens, "token_count": result.Tokens, "chars": result.Chars, "path": result.Path, "cache_status": result.CacheStatus})
		if err != nil {
			return fmt.Sprintf("# %s\nERROR: encode size receipt: %v", source, err)
		}
		return string(body)
	}
	body := result.Content
	stripped := strings.TrimSpace(body)
	meta := frontmatter(result.Path)
	status := result.HTTPStatus
	if stripped == "" {
		message := ""
		if result.ErrorKind != "" || result.Challenge || status >= 400 {
			message = harvest.FailureMessage(source, status, result.ErrorKind, result.Challenge)
		}
		if message == "" {
			message = fmt.Sprintf("Fetched %s but no readable content could be extracted (JS-rendered or bot-blocked — not retrievable from this datacenter IP). Use `search` to find an alternative copy, or `findWorks` if it is a scholarly title.", source)
		}
		return "# " + source + "\nERROR: " + message
	}
	// A thin HTTP error/challenge page is a failure, not a successful body.
	lowerBody := strings.ToLower(stripped)
	challenge := strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "captcha") || strings.Contains(lowerBody, "verify you are human")
	if len([]rune(stripped)) < 500 && (status >= 400 || challenge) {
		return "# " + source + "\nERROR: " + harvest.FailureMessage(source, status, result.ErrorKind, challenge)
	}
	fetchedAt := "unknown"
	if value := meta["fetched_at"]; value != "" {
		fetchedAt = value
	}
	tokens := result.Tokens
	if value, err := strconv.Atoi(meta["token_count"]); err == nil {
		tokens = value
	}
	header := fmt.Sprintf("# %s\ncache_status: %s / method: %s / bytes: %d / tokens: %d / fetched_at: %s / path: %s", source, result.CacheStatus, result.Method, result.Bytes, tokens, fetchedAt, result.Path)
	if rungs := meta["rungs"]; rungs != "" {
		header += " / rungs: " + rungs
	} else if len(result.Rungs) > 1 {
		header += " / rungs: " + strings.Join(result.Rungs, ", ")
	}
	cap := inlineCap()
	if cap > 0 && len([]rune(body)) > cap {
		runes := []rune(body)
		body = string(runes[:cap]) + fmt.Sprintf("\n\n— [truncated: first %d of %d chars. COMPLETE text is at %s — read that file from char %d for the rest. `searchCache` locates WHICH cached pages match a pattern; it does not return text.]", cap, len(runes), result.Path, cap)
	}
	return header + "\n\n" + body
}

func inlineCap() int {
	if raw := os.Getenv("HARVESTER_MAX_INLINE_CHARS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil {
			return value
		}
		fmt.Fprintf(os.Stderr, "harvester: invalid HARVESTER_MAX_INLINE_CHARS=%q; using default %d\n", raw, defaultInlineChars)
	}
	return defaultInlineChars
}

func frontmatter(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return out
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return out
	}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			out[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return out
}

func renderFind(query string, candidates []harvest.Candidate) string {
	if len(candidates) == 0 {
		return fmt.Sprintf("No candidate works found for %q. Try a plain WebSearch, or rephrase — a more exact title helps.", query)
	}
	lines := []string{fmt.Sprintf("%d candidate work(s) for %q — pick one and call `fetch` with its `fetch:` value:", len(candidates), query), ""}
	for index, candidate := range candidates {
		values := []string{candidate.Kind}
		if candidate.Authors != "" {
			values = append(values, candidate.Authors)
		}
		if candidate.Year != 0 {
			values = append(values, strconv.Itoa(candidate.Year))
		}
		values = append(values, candidate.Source)
		if candidate.Free != "" {
			values = append(values, candidate.Free)
		}
		values = append(values, fmt.Sprintf("match %v", candidate.Match))
		meta := strings.Trim(strings.Join(values, " · "), " ·")
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, valueOr(candidate.Title, "(untitled)")), "   fetch: "+candidate.URL, "   "+meta)
	}
	return strings.Join(lines, "\n")
}

func renderArchiveListing(source string, members []harvest.Member) string {
	lines := []string{
		"# Archive: " + source,
		"",
		fmt.Sprintf("%d member(s). Fetch one with `archive(source=%q, member=\"<name>\")` — pick a name from the table below.", len(members), source),
		"",
		"| name | size (bytes) | type |",
		"| --- | --- | --- |",
	}
	for _, member := range members {
		typeName := "file"
		if member.IsDir {
			typeName = "dir"
		} else if member.IsSymlink {
			typeName = "symlink"
		}
		name := strings.ReplaceAll(member.Name, "|", "\\|")
		lines = append(lines, fmt.Sprintf("| %s | %d | %s |", name, member.UncompressedSize, typeName))
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderSearch(query string, results []harvest.SearchResult, backend string) string {
	if backend == "" {
		return "No web-search backend is configured. Set SEARXNG_URL (self-hosted SearXNG) and/or BRAVE_API_KEY to enable the `search` tool."
	}
	if backend == "error" {
		return "The web-search backend(s) are configured but unreachable or failing right now — retry shortly, or check that SEARXNG_URL is up and BRAVE_API_KEY is valid."
	}
	if len(results) == 0 {
		return fmt.Sprintf("No results for %q (via %s). Try different terms or a broader query.", query, backend)
	}
	lines := []string{fmt.Sprintf("%d result(s) for %q (via %s) — fetch the ones you want by URL:", len(results), query, backend), ""}
	for index, result := range results {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, valueOr(result.Title, "(untitled)")), "   "+result.URL)
		if result.Snippet != "" {
			lines = append(lines, "   "+result.Snippet)
		}
		if result.Engine != "" {
			lines = append(lines, "   ["+result.Engine+"]")
		}
	}
	return strings.Join(lines, "\n")
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (service *Service) fetchOneImage(ctx context.Context, source string) ImageItem {
	result := service.harvester.FetchImage(ctx, source)
	return ImageItem{Source: source, Path: result.Path, Bytes: result.Bytes, Error: result.Error}
}

func assertPublicURL(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if parsed.User != nil {
		return errors.New("URL userinfo is not allowed")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return errors.New("URL has no host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if privateIP(ip) {
			return fmt.Errorf("refusing private/internal host %s", host)
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "local" || strings.HasSuffix(host, ".local") || host == "metadata.google.internal" {
		return fmt.Errorf("refusing private/internal host %s", host)
	}
	addresses, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve URL host %s: %w", host, err)
	}
	for _, address := range addresses {
		if privateIP(address) {
			return fmt.Errorf("refusing private/internal host %s", host)
		}
	}
	return nil
}

func privateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
