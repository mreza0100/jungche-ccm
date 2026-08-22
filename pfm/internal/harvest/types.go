// Package harvest is the pure-Go transport and policy core for Harvester.
//
// It deliberately does not convert documents itself. Conversion is supplied by
// Converter so the existing Python worker can remain the one implementation of
// PDF/Office/HTML extraction while Go owns transport, policy, and caching.
package harvest

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Converter turns one fetched body into markdown. Implementations must not
// mutate body. kind is the detected content kind and source is the original
// source URL/path.
type Converter interface {
	Convert(ctx context.Context, kind string, source string, body []byte) (string, error)
}

// OCRConverter is implemented by converters that can force one OCR pass for a
// single document (the dispatch escalation rung for scanned PDFs whose text
// layer converts empty). Optional: a plain Converter simply never escalates.
type OCRConverter interface {
	ConvertOCR(ctx context.Context, kind, source string, body []byte) (string, error)
}

// Options configures a Harvester. Nil HTTP clients use safe defaults.
type Options struct {
	CacheDir       string
	CacheTTL       time.Duration
	Client         *http.Client
	Chrome         *http.Client
	Jina           *http.Client
	OA             *http.Client
	Converter      Converter
	MaxBytes       int64
	LocalRoots     []string
	JinaURL        string
	MaxInlineChars int
	// ProxyURL, when set, is applied to every default transport (direct,
	// Chrome, binary, Jina, and OA). UserAgent customizes only direct/Jina;
	// Chrome keeps its captured impersonation profile and OA keeps its polite
	// scholarly identity.
	ProxyURL  string
	UserAgent string
	// ResolvePublic is called once for each production dial. It is injectable
	// for deterministic DNS-rebinding tests; nil uses net.LookupIP.
	ResolvePublic func(context.Context, string) ([]net.IP, error)
	// Scholarly/search settings are snapshotted by New, matching the Python
	// worker's import-time configuration. Non-empty values are explicit test
	// or embedding overrides; empty values read the process environment once.
	ContactEmail          string
	GoogleBooksAPIKey     string
	CoreAPIKey            string
	SemanticScholarAPIKey string
	SearXNGURL            string
	BraveAPIKey           string
	DisableSearch         bool
}

type envSnapshot struct {
	contactEmail       string
	googleBooksAPIKey  string
	coreAPIKey         string
	semanticScholarKey string
	searXNGURL         string
	braveAPIKey        string
	disableSearch      bool
}

func snapshotEnv() envSnapshot {
	return envSnapshot{
		contactEmail:       strings.TrimSpace(os.Getenv("HARVESTER_CONTACT_EMAIL")),
		googleBooksAPIKey:  strings.TrimSpace(os.Getenv("GOOGLE_BOOKS_API_KEY")),
		coreAPIKey:         strings.TrimSpace(os.Getenv("CORE_API_KEY")),
		semanticScholarKey: strings.TrimSpace(os.Getenv("SEMANTIC_SCHOLAR_API_KEY")),
		searXNGURL:         strings.TrimSpace(os.Getenv("SEARXNG_URL")),
		braveAPIKey:        strings.TrimSpace(os.Getenv("BRAVE_API_KEY")),
		disableSearch:      envBool("HARVESTER_DISABLE_SEARCH"),
	}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// FetchOptions controls one fetch. Refresh bypasses both positive and
// negative caches; SizeOnly still fetches/caches the complete artifact.
type FetchOptions struct {
	Refresh  bool
	SizeOnly bool
}

// Result is deliberately JSON-friendly so the MCP adapter can return it
// without knowing transport internals.
type Result struct {
	Source       string   `json:"source"`
	Kind         string   `json:"kind,omitempty"`
	Content      string   `json:"content,omitempty"`
	Path         string   `json:"path,omitempty"`
	Method       string   `json:"method,omitempty"`
	CacheStatus  string   `json:"cache_status,omitempty"`
	Bytes        int64    `json:"bytes,omitempty"`
	Chars        int      `json:"chars,omitempty"`
	ContentChars int      `json:"content_chars,omitempty"`
	Tokens       int      `json:"tokens,omitempty"`
	Rungs        []string `json:"rungs,omitempty"`
	Error        string   `json:"error,omitempty"`
	ErrorKind    string   `json:"error_kind,omitempty"`
	Challenge    bool     `json:"challenge,omitempty"`
	HTTPStatus   int      `json:"http_status,omitempty"`
	Members      []Member `json:"members,omitempty"`
}

// Harvester owns transport, policy and cache state.
type Harvester struct {
	options      Options
	client       *http.Client
	chrome       *http.Client
	binaryDirect *http.Client
	binaryChrome *http.Client
	jina         *http.Client
	oa           *http.Client
	userAgent    string
	cache        *Cache
	neg          *negativeCache
	flightMu     sync.Mutex
	flights      map[string]*fetchFlight
	env          envSnapshot
}

type fetchFlight struct {
	done   chan struct{}
	result Result
}

// New constructs a Harvester. A nil Converter is valid for callers that only
// need archive listing, search, or raw transport tests; converted fetches then
// report a useful error instead of silently returning bytes.
func New(options Options) *Harvester {
	env := snapshotEnv()
	if strings.TrimSpace(options.ContactEmail) != "" {
		env.contactEmail = strings.TrimSpace(options.ContactEmail)
	}
	if strings.TrimSpace(options.GoogleBooksAPIKey) != "" {
		env.googleBooksAPIKey = strings.TrimSpace(options.GoogleBooksAPIKey)
	}
	if strings.TrimSpace(options.CoreAPIKey) != "" {
		env.coreAPIKey = strings.TrimSpace(options.CoreAPIKey)
	}
	if strings.TrimSpace(options.SemanticScholarAPIKey) != "" {
		env.semanticScholarKey = strings.TrimSpace(options.SemanticScholarAPIKey)
	}
	if strings.TrimSpace(options.SearXNGURL) != "" {
		env.searXNGURL = strings.TrimSpace(options.SearXNGURL)
	}
	if strings.TrimSpace(options.BraveAPIKey) != "" {
		env.braveAPIKey = strings.TrimSpace(options.BraveAPIKey)
	}
	if options.DisableSearch {
		env.disableSearch = true
	}
	if options.CacheDir == "" {
		options.CacheDir = defaultCacheDir()
	}
	if options.CacheTTL == 0 {
		options.CacheTTL = cacheTTLFromEnv()
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 50 * 1024 * 1024
	}
	if options.MaxInlineChars <= 0 {
		options.MaxInlineChars = maxInlineFromEnv()
	}
	if options.LocalRoots == nil {
		options.LocalRoots = localRootsFromEnv()
	}
	if options.JinaURL == "" {
		options.JinaURL = "https://r.jina.ai/"
	}
	client := options.Client
	customClient := client != nil
	if client == nil {
		client = safeHTTPClient(false, options.ResolvePublic)
	}
	chrome := options.Chrome
	// A legacy adapter may pass one ordinary client in every slot. Never let
	// that silently demote the Chrome rung: equal Client/Chrome pointers mean
	// "unspecified Chrome" and are replaced by the production uTLS client.
	if chrome != nil && chrome == client {
		chrome = nil
	}
	customChrome := chrome != nil
	if chrome == nil {
		chrome = safeHTTPClient(true, options.ResolvePublic)
	}
	binaryDirect := client
	binaryChrome := chrome
	if !customClient {
		binaryDirect = safeHTTPClientTimeout(false, 60*time.Second, options.ResolvePublic)
	}
	if !customChrome {
		binaryChrome = safeHTTPClientTimeout(true, 60*time.Second, options.ResolvePublic)
	}
	jina := options.Jina
	if jina == nil {
		jina = safeHTTPClient(false, options.ResolvePublic)
	}
	oa := options.OA
	if oa == nil {
		if customClient {
			oa = client
		} else {
			// OA metadata calls use the oracle's short 15-second provider
			// timeout; body/document rungs retain their 30/45/60-second tiers.
			oa = safeHTTPClientTimeout(false, 15*time.Second, options.ResolvePublic)
		}
	} else if oa == client && customClient {
		// Keep scholarly requests on their own polite-UA client when an
		// adapter aliases OA to its direct transport.
		oa = safeHTTPClient(false, options.ResolvePublic)
	}
	if options.ProxyURL != "" {
		configureProxy(client, options.ProxyURL)
		configureProxy(chrome, options.ProxyURL)
		configureProxy(binaryDirect, options.ProxyURL)
		configureProxy(binaryChrome, options.ProxyURL)
		configureProxy(jina, options.ProxyURL)
		configureProxy(oa, options.ProxyURL)
	}
	userAgent := options.UserAgent
	if userAgent == "" {
		userAgent = defaultUA
	}
	setUserAgent(client, userAgent)
	setUserAgent(binaryDirect, userAgent)
	setUserAgent(jina, userAgent)
	return &Harvester{options: options, client: client, chrome: chrome, binaryDirect: binaryDirect, binaryChrome: binaryChrome, jina: jina, oa: oa,
		userAgent: userAgent, cache: newCache(options.CacheDir, options.CacheTTL), neg: newNegativeCache(negTTLFromEnv(), negTransientTTLFromEnv()), flights: make(map[string]*fetchFlight), env: env}
}

// Fetch executes one request with default options.
func (h *Harvester) Fetch(ctx context.Context, source string) Result {
	return h.FetchWithOptions(ctx, source, FetchOptions{})
}

// NewChromeClient exposes the production uTLS rung to adapters without
// exposing its transport internals. A nil resolver uses the system resolver.
func NewChromeClient(resolve func(context.Context, string) ([]net.IP, error)) *http.Client {
	return safeHTTPClient(true, resolve)
}
