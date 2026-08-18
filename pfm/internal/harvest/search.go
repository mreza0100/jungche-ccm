package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SearchOptions struct {
	SearXNGURL    string
	BraveAPIKey   string
	Lang          string
	Engines       string
	Count         int
	SearXNG       *http.Client
	Brave         *http.Client
	DisableSearch bool
}
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Engine  string `json:"engine,omitempty"`
}

const searchUA = "harvester-mcp/1.0"

func searchForceDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARVESTER_DISABLE_SEARCH"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
func SearchEnabled(options SearchOptions) bool {
	if options.SearXNGURL == "" {
		options.SearXNGURL = os.Getenv("SEARXNG_URL")
	}
	if options.BraveAPIKey == "" {
		options.BraveAPIKey = os.Getenv("BRAVE_API_KEY")
	}
	return !searchForceDisabled() && (options.SearXNGURL != "" || options.BraveAPIKey != "")
}
func SearchAdvertised() bool { return !searchForceDisabled() }

// Search tries configured SearXNG first and Brave second. A configured but
// empty backend returns an empty result with its backend name, while no backend
// is an explicit configuration error.
func Search(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	env := snapshotEnv()
	if options.SearXNGURL == "" {
		options.SearXNGURL = env.searXNGURL
	}
	if options.BraveAPIKey == "" {
		options.BraveAPIKey = env.braveAPIKey
	}
	options.DisableSearch = options.DisableSearch || env.disableSearch
	return searchConfigured(ctx, query, options)
}

func searchConfigured(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	if options.Count <= 0 {
		options.Count = 8
	}
	if options.Count > 20 {
		options.Count = 20
	}
	if options.SearXNGURL != "" {
		out, e := searchSearXNG(ctx, query, options)
		if e == nil && len(out) > 0 {
			return out, "searxng", nil
		}
		if e == nil && options.BraveAPIKey == "" {
			return out, "searxng", nil
		}
	}
	if options.BraveAPIKey != "" {
		out, e := searchBrave(ctx, query, options)
		if e == nil {
			return out, "brave", nil
		}
	}
	if options.DisableSearch {
		return nil, "", fmt.Errorf("search is disabled")
	}
	if options.SearXNGURL == "" && options.BraveAPIKey == "" {
		return nil, "", fmt.Errorf("search is not configured: set SEARXNG_URL or BRAVE_API_KEY")
	}
	return nil, "error", fmt.Errorf("all configured search backends failed")
}

func (h *Harvester) Search(ctx context.Context, query string, options SearchOptions) ([]SearchResult, string, error) {
	if h != nil {
		if options.SearXNGURL == "" {
			options.SearXNGURL = h.env.searXNGURL
		}
		if options.BraveAPIKey == "" {
			options.BraveAPIKey = h.env.braveAPIKey
		}
		options.DisableSearch = options.DisableSearch || h.env.disableSearch
	}
	return searchConfigured(ctx, query, options)
}
func searchSearXNG(ctx context.Context, q string, o SearchOptions) ([]SearchResult, error) {
	client := o.SearXNG
	if client == nil {
		client = safeHTTPClientTimeout(false, 20*time.Second)
	}
	u := strings.TrimRight(o.SearXNGURL, "/") + "/search?q=" + url.QueryEscape(q) + "&format=json&safesearch=0"
	if o.Lang != "" {
		u += "&language=" + url.QueryEscape(o.Lang)
	}
	if o.Engines != "" {
		u += "&engines=" + url.QueryEscape(o.Engines)
	}
	body, status, _, e := getBody(ctx, client, u, searchUA, 10*1024*1024)
	if e != nil || status >= 400 {
		if e == nil {
			e = fmt.Errorf("HTTP %d", status)
		}
		return nil, e
	}
	var data struct {
		Results []struct {
			Title, URL, Content string
			Engines             []string
			Engine              string `json:"engine"`
		} `json:"results"`
	}
	if e = json.Unmarshal(body, &data); e != nil {
		return nil, e
	}
	out := []SearchResult{}
	for _, r := range data.Results {
		if r.URL != "" {
			engine := strings.Join(r.Engines, ",")
			if engine == "" {
				engine = r.Engine
			}
			out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: truncateRunes(r.Content, 300), Engine: engine})
		}
	}
	if len(out) > o.Count {
		out = out[:o.Count]
	}
	return out, nil
}
func searchBrave(ctx context.Context, q string, o SearchOptions) ([]SearchResult, error) {
	client := o.Brave
	if client == nil {
		client = safeHTTPClientTimeout(false, 20*time.Second)
	}
	query := url.Values{"q": {q}, "count": {fmt.Sprint(o.Count)}}
	if o.Lang != "" {
		query.Set("search_lang", o.Lang)
	}
	reqURL := "https://api.search.brave.com/res/v1/web/search?" + query.Encode()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", searchUA)
	req.Header.Set("X-Subscription-Token", o.BraveAPIKey)
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var data struct {
		Web struct {
			Results []struct{ Title, URL, Description string } `json:"results"`
		} `json:"web"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&data); e != nil {
		return nil, e
	}
	out := []SearchResult{}
	for _, r := range data.Web.Results {
		if r.URL != "" {
			out = append(out, SearchResult{Title: r.Title, URL: r.URL, Snippet: truncateRunes(r.Description, 300), Engine: "brave"})
		}
	}
	return out, nil
}

func truncateRunes(value string, max int) string {
	r := []rune(value)
	if len(r) <= max {
		return value
	}
	return string(r[:max])
}
