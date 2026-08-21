package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type IdentifierKind string

const (
	IdentifierNone  IdentifierKind = ""
	IdentifierDOI   IdentifierKind = "doi"
	IdentifierISBN  IdentifierKind = "isbn"
	IdentifierPMID  IdentifierKind = "pmid"
	IdentifierPMCID IdentifierKind = "pmcid"
	IdentifierTitle IdentifierKind = "title"
)

type Candidate struct {
	URL      string  `json:"url"`
	Source   string  `json:"source,omitempty"`
	Priority int     `json:"priority,omitempty"`
	Kind     string  `json:"kind,omitempty"`
	Title    string  `json:"title,omitempty"`
	Authors  string  `json:"authors,omitempty"`
	Year     int     `json:"year,omitempty"`
	Free     string  `json:"free,omitempty"`
	Match    float64 `json:"match,omitempty"`
}

// Resolver is the legal scholarly metadata/OA resolver. It never downloads
// article bodies; candidates are sent through Harvester's normal transport.
// The optional settings are copied by New at process/server start so provider
// behavior cannot change halfway through one MCP process when its environment
// is edited by a caller.
type Resolver struct {
	Client                *http.Client
	ContactEmail          string
	GoogleBooksAPIKey     string
	CoreAPIKey            string
	SemanticScholarAPIKey string
	snapshot              bool
}

type doabMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type resolverUAContextKey struct{}

func resolverContext(ctx context.Context, r *Resolver) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, resolverUAContextKey{}, r.scholarlyUA())
}

func candidatePriority(source, status, version, kind string) int {
	bases := map[string]int{"arxiv": 0, "osf": 2, "citation_pdf_url": 4, "unpaywall": 10, "openalex": 12, "semanticscholar": 14, "europepmc": 16, "crossref": 30, "core": 40, "doaj": 45, "gutenberg": 5, "oapen": 8, "internetarchive": 18, "doab": 22, "googlebooks": 50}
	base, known := bases[source]
	if !known {
		base = 50
	}
	if kind != "pdf" {
		base += 8
	}
	if status == "bronze" {
		base += 6
	}
	if version != "" && version != "publishedVersion" {
		base += 2
	}
	return base
}

var doiPattern = regexp.MustCompile(`(?i)10\.\d{4,9}/[-._;()/:A-Z0-9]+`)
var citationPDFPattern = regexp.MustCompile(`(?is)<meta[^>]+name=["']citation_pdf_url["'][^>]+content=["']([^"']+)["']`)
var citationPDFPatternReversed = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']citation_pdf_url["']`)
var citationDOIPattern = regexp.MustCompile(`(?is)<meta[^>]+name=["'](?:citation_doi|dc\.identifier|DC\.Identifier)["'][^>]+content=["']([^"']+)["']`)
var citationTitlePattern = regexp.MustCompile(`(?is)<meta[^>]+name=["']citation_title["'][^>]+content=["']([^"']+)["']`)

func DOIFrom(text string) string {
	m := doiPattern.FindString(text)
	return strings.TrimRight(m, ".,;:)>]")
}

// ExtractMetaLinks pulls the citation DOI/PDF/title hints exposed by many
// blocked publisher landing pages. It is pure and never performs a fetch.
func ExtractMetaLinks(pageHTML string) (doi, pdf, title string) {
	if m := citationPDFPattern.FindStringSubmatch(pageHTML); len(m) > 1 {
		pdf = m[1]
	} else if m := citationPDFPatternReversed.FindStringSubmatch(pageHTML); len(m) > 1 {
		pdf = m[1]
	}
	if m := citationDOIPattern.FindStringSubmatch(pageHTML); len(m) > 1 {
		doi = DOIFrom(m[1])
	}
	if m := citationTitlePattern.FindStringSubmatch(pageHTML); len(m) > 1 {
		title = m[1]
	}
	return
}

func normalizeISBN(s string) string {
	d := strings.ToUpper(strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if r == 'X' {
			return r
		}
		return -1
	}, s))
	if len(d) > 0 && strings.Trim(d, string(d[0])) == "" {
		return "" // reject checksum-valid placeholder values such as 0000000000
	}
	if len(d) == 13 && isbn13Valid(d) {
		return d
	}
	if len(d) == 10 && isbn10Valid(d) {
		return d
	}
	return ""
}
func NormalizeISBN(s string) string { return normalizeISBN(s) }

func isbn13Valid(s string) bool {
	if !strings.HasPrefix(s, "978") && !strings.HasPrefix(s, "979") {
		return false
	}
	sum := 0
	for i, r := range s {
		v := int(r - '0')
		if i%2 == 1 {
			v *= 3
		}
		sum += v
	}
	return sum%10 == 0
}
func isbn10Valid(s string) bool {
	sum := 0
	for i, r := range s {
		v := 0
		if r == 'X' && i == 9 {
			v = 10
		} else if r >= '0' && r <= '9' {
			v = int(r - '0')
		} else {
			return false
		}
		sum += (10 - i) * v
	}
	return sum%11 == 0
}

// NormalizeIdentifier returns a canonical DOI or checksum-valid ISBN. Other
// identifiers are returned in their canonical uppercase/lowercase form.
func NormalizeIdentifier(input string) string {
	input = strings.TrimSpace(input)
	if doi := DOIFrom(input); doi != "" {
		return doi
	}
	low := strings.ToLower(input)
	if strings.HasPrefix(low, "doi:") {
		return DOIFrom(input[4:])
	}
	if strings.HasPrefix(low, "pmcid:") {
		return strings.ToUpper(strings.TrimSpace(input[6:]))
	}
	if strings.HasPrefix(low, "pmid:") {
		return strings.TrimSpace(input[5:])
	}
	if strings.HasPrefix(low, "isbn:") {
		return normalizeISBN(input[5:])
	}
	return normalizeISBN(input)
}

func ClassifyIdentifier(input string) IdentifierKind {
	trim := strings.TrimSpace(input)
	if DOIFrom(trim) != "" || strings.HasPrefix(strings.ToLower(trim), "doi:") {
		return IdentifierDOI
	}
	low := strings.ToLower(trim)
	if strings.HasPrefix(low, "pmcid:") {
		trim = strings.TrimSpace(trim[6:])
		low = strings.ToLower(trim)
	}
	if strings.HasPrefix(low, "pmc") && regexp.MustCompile(`^pmc\d+$`).MatchString(low) {
		return IdentifierPMCID
	}
	if strings.HasPrefix(strings.ToLower(trim), "pmid:") {
		trim = strings.TrimSpace(trim[5:])
	}
	if regexp.MustCompile(`^\d{7,9}$`).MatchString(trim) {
		return IdentifierPMID
	}
	if strings.HasPrefix(low, "isbn:") || normalizeISBN(trim) != "" {
		return IdentifierISBN
	}
	return IdentifierNone
}

func (r *Resolver) client() *http.Client {
	if r != nil && r.Client != nil {
		return r.Client
	}
	return safeHTTPClientTimeout(false, 15*time.Second)
}

func (r *Resolver) contact() string {
	if r != nil && r.snapshot {
		return strings.TrimSpace(r.ContactEmail)
	}
	if r != nil && strings.TrimSpace(r.ContactEmail) != "" {
		return strings.TrimSpace(r.ContactEmail)
	}
	return contactEmail()
}

func (r *Resolver) withContact(raw, key string) string {
	if r.contact() == "" {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + key + "=" + url.QueryEscape(r.contact())
}

func (r *Resolver) apiKey(value string, fallback func() string) string {
	if r != nil && r.snapshot {
		return strings.TrimSpace(value)
	}
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback()
}

func (r *Resolver) scholarlyUA() string {
	if email := r.contact(); email != "" {
		return "harvester-mcp/1.0 (mailto:" + email + ")"
	}
	return "harvester-mcp/1.0"
}

func (h *Harvester) resolver() *Resolver {
	if h == nil {
		return &Resolver{}
	}
	return &Resolver{Client: h.oa, ContactEmail: h.env.contactEmail, GoogleBooksAPIKey: h.env.googleBooksAPIKey,
		CoreAPIKey: h.env.coreAPIKey, SemanticScholarAPIKey: h.env.semanticScholarKey, snapshot: true}
}

func (r *Resolver) ResolveDOI(ctx context.Context, doi string) ([]Candidate, error) {
	doi = DOIFrom(doi)
	if doi == "" {
		return nil, fmt.Errorf("invalid DOI")
	}
	ctx = resolverContext(ctx, r)
	client := r.client()
	var out []Candidate
	// Deterministic arXiv DOI routing avoids unnecessary third-party queries.
	const arxivDOIPrefix = "10.48550/arxiv."
	if strings.HasPrefix(strings.ToLower(doi), arxivDOIPrefix) {
		id := doi[len(arxivDOIPrefix):]
		return []Candidate{{URL: "https://arxiv.org/pdf/" + id, Source: "arxiv", Priority: 0, Kind: "pdf"}}, nil
	}
	if strings.HasPrefix(strings.ToLower(doi), "10.31235/") || strings.HasPrefix(strings.ToLower(doi), "10.31234/") || strings.HasPrefix(strings.ToLower(doi), "10.31219/") || strings.HasPrefix(strings.ToLower(doi), "10.31730/") || strings.HasPrefix(strings.ToLower(doi), "10.35542/") || strings.HasPrefix(strings.ToLower(doi), "10.33767/") {
		if candidates, err := r.osf(ctx, client, doi); err == nil && len(candidates) > 0 {
			return candidates, nil
		}
	}
	// Providers are independent and the Python resolver fans them out. Gather
	// concurrently, then apply the explicit priority sort/dedupe so completion
	// order never changes the public candidate order.
	sources := []string{"unpaywall", "openalex", "semanticscholar", "europepmc", "crossref", "core", "doaj"}
	results := make([][]Candidate, len(sources))
	var wg sync.WaitGroup
	for i, source := range sources {
		wg.Add(1)
		go func(i int, source string) {
			defer wg.Done()
			var candidates []Candidate
			switch source {
			case "unpaywall":
				candidates, _ = r.unpaywall(ctx, client, doi)
			case "openalex":
				candidates, _ = r.openAlexDOI(ctx, client, doi)
			case "semanticscholar":
				candidates, _ = r.semanticScholar(ctx, client, doi)
			case "europepmc":
				candidates, _ = r.europePMCDOI(ctx, client, doi)
			case "crossref":
				candidates, _ = r.crossref(ctx, client, doi)
			case "core":
				candidates, _ = r.core(ctx, client, doi)
			case "doaj":
				candidates, _ = r.doaj(ctx, client, doi)
			}
			results[i] = candidates
		}(i, source)
	}
	wg.Wait()
	for _, candidates := range results {
		out = append(out, candidates...)
	}
	return sortCandidates(out), nil
}

func (r *Resolver) ResolveBook(ctx context.Context, query string) ([]Candidate, error) {
	query = strings.TrimSpace(strings.Trim(query, "\"'"))
	if query == "" {
		return nil, fmt.Errorf("empty book query")
	}
	ctx = resolverContext(ctx, r)
	client := r.client()
	var out []Candidate
	// OAPEN exposes direct ORIGINAL bitstreams for open academic books.
	search := query
	if isbn := normalizeISBN(query); isbn != "" {
		search = "isbn:" + isbn
	} else {
		search = "title:" + query
	}
	var oapen []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if body, status, _, fetchErr := getBody(ctx, client, "https://library.oapen.org/rest/search?query="+url.QueryEscape(search)+"&limit=5", defaultUA, 10<<20); fetchErr == nil && status < 400 {
		_ = json.Unmarshal(body, &oapen)
		for _, item := range oapen {
			if item.UUID == "" {
				continue
			}
			detailURL := "https://library.oapen.org/rest/items/" + url.PathEscape(item.UUID) + "?expand=bitstreams"
			var detail struct {
				Bitstreams []struct {
					Bundle string `json:"bundleName"`
					Mime   string `json:"mimeType"`
					Link   string `json:"retrieveLink"`
				} `json:"bitstreams"`
			}
			if err := getJSON(ctx, client, detailURL, &detail); err != nil {
				continue
			}
			for _, bs := range detail.Bitstreams {
				if bs.Bundle == "ORIGINAL" && bs.Mime == "application/pdf" && bs.Link != "" {
					link := bs.Link
					if !strings.HasPrefix(link, "http") {
						link = "https://library.oapen.org" + link
					}
					out = append(out, Candidate{URL: link, Source: "oapen", Priority: 8, Kind: "pdf", Title: item.Name})
					break
				}
			}
		}
	}
	// Gutendex is useful for public-domain title searches and has no key.
	if normalizeISBN(query) == "" {
		var data struct {
			Results []struct {
				Title     string            `json:"title"`
				Copyright bool              `json:"copyright"`
				Formats   map[string]string `json:"formats"`
			} `json:"results"`
		}
		if err := getJSON(ctx, client, "https://gutendex.com/books?search="+url.QueryEscape(query), &data); err == nil {
			for _, book := range data.Results {
				if book.Copyright {
					continue
				}
				link, kind := preferredTextFormat(book.Formats)
				if link != "" {
					out = append(out, Candidate{URL: link, Source: "gutenberg", Priority: candidatePriority("gutenberg", "", "", kind), Kind: kind, Title: book.Title})
				}
			}
		}
	}
	// OpenLibrary index -> Internet Archive full text, with the oracle's
	// public-domain gate. Never serve controlled-lending copies.
	isbn := normalizeISBN(query)
	ocaid := ""
	if isbn != "" {
		var data struct {
			OCAID string `json:"ocaid"`
		}
		if err := getJSON(ctx, client, "https://openlibrary.org/isbn/"+isbn+".json", &data); err == nil {
			ocaid = data.OCAID
		}
	} else {
		var searchData struct {
			Docs []struct {
				IA     []string `json:"ia"`
				Access string   `json:"ebook_access"`
			} `json:"docs"`
		}
		searchURL := "https://openlibrary.org/search.json?q=" + url.QueryEscape(query) + "&fields=ia,ebook_access,title&limit=5"
		if err := getJSON(ctx, client, searchURL, &searchData); err == nil {
			for _, doc := range searchData.Docs {
				if doc.Access == "public" && len(doc.IA) > 0 {
					ocaid = doc.IA[0]
					break
				}
			}
		}
	}
	if ocaid != "" {
		var meta struct {
			Metadata struct {
				Restricted string `json:"access-restricted-item"`
			} `json:"metadata"`
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		}
		if err := getJSON(ctx, client, "https://archive.org/metadata/"+url.PathEscape(ocaid), &meta); err == nil && meta.Metadata.Restricted != "true" {
			for _, file := range meta.Files {
				if strings.HasSuffix(strings.ToLower(file.Name), ".pdf") && !strings.HasSuffix(strings.ToLower(file.Name), "_encrypted.pdf") {
					out = append(out, Candidate{URL: "https://archive.org/download/" + ocaid + "/" + url.PathEscape(file.Name), Source: "internetarchive", Priority: 18, Kind: "pdf"})
				}
				if strings.HasSuffix(strings.ToLower(file.Name), "_djvu.txt") {
					out = append(out, Candidate{URL: "https://archive.org/download/" + ocaid + "/" + url.PathEscape(file.Name), Source: "internetarchive", Priority: 18, Kind: "txt"})
				}
			}
		}
	}
	// DOAB's API exposes an open download URL for academic books. A DOI URL
	// is expanded through the same legal article resolver instead of fetched as
	// an unverified publisher candidate.
	var doab []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	if isbn != "" {
		// DOAB's ISBN endpoint is a JSON POST; its search endpoint silently
		// returns no ISBN matches for several catalogues.
		_ = postJSON(ctx, client, "https://directory.doabooks.org/rest/items/find-by-metadata-field", map[string]string{"key": "oapen.relation.isbn", "value": isbn}, &doab)
	} else {
		_ = getJSON(ctx, client, "https://directory.doabooks.org/rest/search?query="+url.QueryEscape(query), &doab)
	}
	for _, item := range doab[:minInt(len(doab), 5)] {
		if item.UUID == "" {
			continue
		}
		var detail struct {
			Name       string         `json:"name"`
			Metadata   []doabMetadata `json:"metadata"`
			Bitstreams []struct {
				Metadata []doabMetadata `json:"metadata"`
			} `json:"bitstreams"`
		}
		if getJSON(ctx, client, "https://directory.doabooks.org/rest/items/"+url.PathEscape(item.UUID)+"?expand=metadata,bitstreams", &detail) != nil {
			continue
		}
		pools := make([]doabMetadata, 0, len(detail.Metadata))
		pools = append(pools, detail.Metadata...)
		for _, bitstream := range detail.Bitstreams {
			pools = append(pools, bitstream.Metadata...)
		}
		for _, m := range pools {
			if m.Key != "oapen.identifier.downloadUrl" || m.Value == "" {
				continue
			}
			if doi := DOIFrom(m.Value); doi != "" {
				if cands, resolveErr := r.ResolveDOI(ctx, doi); resolveErr == nil && len(cands) > 0 {
					out = append(out, cands...)
					continue
				}
			}
			out = append(out, Candidate{URL: m.Value, Source: "doab", Priority: 22, Kind: "pdf", Title: detail.Name})
		}
	}
	if key := r.apiKey(r.GoogleBooksAPIKey, func() string { return strings.TrimSpace(os.Getenv("GOOGLE_BOOKS_API_KEY")) }); key != "" {
		var data struct {
			Items []struct {
				Access struct {
					Public bool `json:"publicDomain"`
					PDF    struct {
						Link string `json:"downloadLink"`
					} `json:"pdf"`
				} `json:"accessInfo"`
			} `json:"items"`
		}
		bookQuery := query
		if isbn := normalizeISBN(query); isbn != "" {
			bookQuery = "isbn:" + isbn
		}
		bookURL := "https://www.googleapis.com/books/v1/volumes?q=" + url.QueryEscape(bookQuery) + "&country=US&key=" + url.QueryEscape(key)
		if err := getJSON(ctx, client, bookURL, &data); err == nil {
			for _, item := range data.Items {
				if item.Access.Public && item.Access.PDF.Link != "" {
					out = append(out, Candidate{URL: item.Access.PDF.Link, Source: "googlebooks", Priority: 50, Kind: "pdf"})
				}
			}
		}
	}
	return sortCandidates(out), nil
}

func (r *Resolver) ResolveTitle(ctx context.Context, title string) ([]Candidate, error) {
	title = strings.TrimSpace(strings.Trim(title, "\"'"))
	if title == "" {
		return nil, fmt.Errorf("empty title")
	}
	ctx = resolverContext(ctx, r)
	client := r.client()
	doi := r.titleToDOI(ctx, client, title)
	if doi != "" {
		if candidates, err := r.ResolveDOI(ctx, doi); err == nil && len(candidates) > 0 {
			return candidates, nil
		}
	}
	return r.arxivByTitle(ctx, client, title), nil
}

func (r *Resolver) titleToDOI(ctx context.Context, client *http.Client, title string) string {
	var data struct {
		Results []struct {
			DOI   string `json:"doi"`
			Name  string `json:"display_name"`
			Title string `json:"title"`
		} `json:"results"`
	}
	_ = getJSON(ctx, client, r.withContact("https://api.openalex.org/works?filter=title.search:"+url.QueryEscape(title)+"&per_page=5", "mailto"), &data)
	for _, row := range data.Results {
		name := row.Name
		if name == "" {
			name = row.Title
		}
		if titleSimilarity(title, name) >= .6 {
			if doi := DOIFrom(row.DOI); doi != "" {
				return doi
			}
		}
	}
	var crossref struct {
		Message struct {
			Items []struct {
				DOI   string   `json:"DOI"`
				Title []string `json:"title"`
			} `json:"items"`
		} `json:"message"`
	}
	if crossErr := getJSON(ctx, client, r.withContact("https://api.crossref.org/works?query.bibliographic="+url.QueryEscape(title)+"&rows=5&select=DOI,title", "mailto"), &crossref); crossErr == nil {
		for _, item := range crossref.Message.Items {
			name := ""
			if len(item.Title) > 0 {
				name = item.Title[0]
			}
			match := titleSimilarity(title, name)
			if match >= .6 && DOIFrom(item.DOI) != "" {
				return DOIFrom(item.DOI)
			}
		}
	}
	return ""
}

func (r *Resolver) arxivByTitle(ctx context.Context, client *http.Client, title string) []Candidate {
	body, status, _, err := getBody(ctx, client, "https://export.arxiv.org/api/query?search_query=ti:%22"+url.QueryEscape(title)+"%22&max_results=3", r.scholarlyUA(), 2*1024*1024)
	if err != nil || status >= 400 {
		return nil
	}
	entries := regexp.MustCompile(`(?is)<entry>(.*?)</entry>`).FindAllSubmatch(body, -1)
	space := regexp.MustCompile(`\s+`)
	for _, entry := range entries {
		part := string(entry[1])
		titleMatch := regexp.MustCompile(`(?is)<title>(.*?)</title>`).FindStringSubmatch(part)
		idMatch := regexp.MustCompile(`(?is)<id>https?://arxiv\.org/abs/([^<]+)</id>`).FindStringSubmatch(part)
		if len(titleMatch) > 1 && len(idMatch) > 1 {
			name := strings.TrimSpace(space.ReplaceAllString(titleMatch[1], " "))
			if titleSimilarity(title, name) >= .6 {
				return []Candidate{{URL: "https://arxiv.org/pdf/" + strings.TrimSpace(idMatch[1]), Source: "arxiv", Priority: 0, Kind: "pdf", Free: "green", Title: name}}
			}
		}
	}
	return nil
}

func (r *Resolver) osf(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	idx := strings.Index(strings.ToLower(doi), "osf.io/")
	if idx < 0 {
		return nil, nil
	}
	guid := doi[idx+len("osf.io/"):]
	guid = strings.Trim(guid, "/")
	if slash := strings.IndexByte(guid, '/'); slash >= 0 {
		guid = guid[:slash]
	}
	if guid == "" {
		return nil, nil
	}
	var data struct {
		Data struct {
			Relationships struct {
				PrimaryFile struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"primary_file"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := getJSONWithHeaders(ctx, client, "https://api.osf.io/v2/preprints/"+url.PathEscape(guid)+"/", map[string]string{"Accept": "application/json"}, &data); err != nil {
		return nil, err
	}
	if id := data.Data.Relationships.PrimaryFile.Data.ID; id != "" {
		return []Candidate{{URL: "https://osf.io/download/" + url.PathEscape(id) + "/", Source: "osf", Priority: 2, Kind: "pdf"}}, nil
	}
	return nil, nil
}

func (r *Resolver) FindWorks(ctx context.Context, query string, limit int) ([]Candidate, error) {
	query = strings.TrimSpace(strings.Trim(query, "\"'"))
	if query == "" {
		return []Candidate{}, nil
	}
	if limit <= 0 {
		limit = 8
	}
	ctx = resolverContext(ctx, r)
	client := r.client()
	parts := make([][]Candidate, 3)
	var wait sync.WaitGroup
	for index, gather := range []func(context.Context, *http.Client, string, int) []Candidate{
		r.findPapers,
		r.findArxiv,
		r.findBooks,
	} {
		wait.Add(1)
		go func(index int, gather func(context.Context, *http.Client, string, int) []Candidate) {
			defer wait.Done()
			parts[index] = gather(ctx, client, query, limit)
		}(index, gather)
	}
	wait.Wait()
	best := map[string]Candidate{}
	order := make([]string, 0)
	for _, part := range parts {
		for _, candidate := range part {
			if candidate.URL == "" {
				continue
			}
			prior, found := best[candidate.URL]
			if !found {
				order = append(order, candidate.URL)
			}
			if !found || candidate.Match > prior.Match {
				best[candidate.URL] = candidate
			}
		}
	}
	out := make([]Candidate, 0, len(order))
	for _, handle := range order {
		out = append(out, best[handle])
	}
	free := map[string]bool{"pd": true, "gold": true, "green": true, "diamond": true, "hybrid": true, "bronze": true, "public": true}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Match != out[j].Match {
			return out[i].Match > out[j].Match
		}
		return free[strings.ToLower(out[i].Free)] && !free[strings.ToLower(out[j].Free)]
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Resolver) findPapers(ctx context.Context, client *http.Client, query string, limit int) []Candidate {
	var data struct {
		Results []struct {
			DOI  string `json:"doi"`
			Name string `json:"display_name"`
			OA   struct {
				URL    string `json:"oa_url"`
				Status string `json:"oa_status"`
			} `json:"open_access"`
			Best        scholarlyLocation   `json:"best_oa_location"`
			Primary     scholarlyLocation   `json:"primary_location"`
			Locations   []scholarlyLocation `json:"locations"`
			Year        int                 `json:"publication_year"`
			Authorships []struct {
				Author struct {
					Name string `json:"display_name"`
				} `json:"author"`
			} `json:"authorships"`
		} `json:"results"`
	}
	raw := r.withContact("https://api.openalex.org/works?filter=title.search:"+url.QueryEscape(query)+"&per_page="+fmt.Sprint(limit), "mailto")
	if err := getJSON(ctx, client, raw, &data); err != nil {
		return nil
	}
	out := make([]Candidate, 0, len(data.Results))
	for _, work := range data.Results {
		handle := DOIFrom(work.DOI)
		if handle == "" {
			handle = work.OA.URL
		}
		if handle == "" {
			handle = bestScholarlyLocationHandle(append([]scholarlyLocation{work.Best, work.Primary}, work.Locations...))
		}
		if handle == "" {
			continue
		}
		authors := make([]string, 0, len(work.Authorships))
		for _, authorship := range work.Authorships {
			if authorship.Author.Name != "" {
				authors = append(authors, authorship.Author.Name)
			}
		}
		out = append(out, Candidate{URL: handle, Source: "openalex", Kind: "paper", Title: work.Name, Authors: formatAuthors(authors), Year: work.Year, Free: work.OA.Status, Match: roundMatch(titleMatch(query, work.Name))})
	}
	return out
}

type scholarlyLocation struct {
	PDF     string `json:"pdf_url"`
	Landing string `json:"landing_page_url"`
}

func bestScholarlyLocationHandle(locations []scholarlyLocation) string {
	for _, location := range locations {
		if location.PDF != "" {
			return location.PDF
		}
	}
	for _, location := range locations {
		match := regexp.MustCompile(`(?i)arxiv\.org/abs/([^/?#]+)`).FindStringSubmatch(location.Landing)
		if len(match) > 1 {
			return "https://arxiv.org/pdf/" + match[1]
		}
	}
	for _, location := range locations {
		if location.Landing != "" {
			return location.Landing
		}
	}
	return ""
}

func (r *Resolver) findArxiv(ctx context.Context, client *http.Client, query string, _ int) []Candidate {
	candidates := r.arxivByTitle(ctx, client, query)
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.Kind = "paper"
		candidate.Title = query
		candidate.Free = "green"
		candidate.Match = .9
		out = append(out, candidate)
	}
	return out
}

func (r *Resolver) findBooks(ctx context.Context, client *http.Client, query string, limit int) []Candidate {
	var openLibrary struct {
		Docs []struct {
			Title   string   `json:"title"`
			Authors []string `json:"author_name"`
			Year    int      `json:"first_publish_year"`
			ISBN    []string `json:"isbn"`
			Access  string   `json:"ebook_access"`
		} `json:"docs"`
	}
	raw := "https://openlibrary.org/search.json?q=" + url.QueryEscape(query) + "&fields=title,author_name,first_publish_year,isbn,ebook_access&limit=" + fmt.Sprint(limit)
	out := []Candidate{}
	if err := getJSON(ctx, client, raw, &openLibrary); err == nil {
		for _, book := range openLibrary.Docs {
			if len(book.ISBN) == 0 {
				continue
			}
			out = append(out, Candidate{URL: "isbn:" + book.ISBN[0], Source: "openlibrary", Kind: "book", Title: book.Title, Authors: formatAuthors(book.Authors), Year: book.Year, Free: book.Access, Match: roundMatch(titleMatch(query, book.Title))})
		}
	}
	var gutendex struct {
		Results []struct {
			Title     string `json:"title"`
			Copyright bool   `json:"copyright"`
			Authors   []struct {
				Name string `json:"name"`
			} `json:"authors"`
			Formats map[string]string `json:"formats"`
		} `json:"results"`
	}
	if err := getJSON(ctx, client, "https://gutendex.com/books?search="+url.QueryEscape(query), &gutendex); err == nil {
		for _, book := range gutendex.Results[:minInt(len(gutendex.Results), 3)] {
			if book.Copyright {
				continue
			}
			handle, _ := preferredTextFormat(book.Formats)
			if handle == "" {
				continue
			}
			authors := make([]string, 0, len(book.Authors))
			for _, author := range book.Authors {
				authors = append(authors, author.Name)
			}
			out = append(out, Candidate{URL: handle, Source: "gutenberg", Kind: "book", Title: book.Title, Authors: formatAuthors(authors), Free: "pd", Match: roundMatch(titleMatch(query, book.Title))})
		}
	}
	return out
}

func preferredTextFormat(formats map[string]string) (string, string) {
	for _, prefix := range []string{"text/html", "text/plain"} {
		keys := make([]string, 0)
		for mime := range formats {
			if strings.HasPrefix(mime, prefix) {
				keys = append(keys, mime)
			}
		}
		sort.Strings(keys)
		for _, mime := range keys {
			if formats[mime] != "" {
				kind := "html"
				if prefix == "text/plain" {
					kind = "txt"
				}
				return formats[mime], kind
			}
		}
	}
	return "", ""
}

func formatAuthors(names []string) string {
	filtered := names[:0]
	for _, name := range names {
		if name != "" {
			filtered = append(filtered, name)
		}
	}
	if len(filtered) > 3 {
		return filtered[0] + " et al."
	}
	return strings.Join(filtered, ", ")
}

func roundMatch(value float64) float64 { return math.Round(value*100) / 100 }

// FindWorks is the Harvester-facing scholarly search surface.
func (h *Harvester) FindWorks(ctx context.Context, query string, limit int) ([]Candidate, error) {
	return h.resolver().FindWorks(ctx, query, limit)
}

func (r *Resolver) unpaywall(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		IsOA   bool   `json:"is_oa"`
		Status string `json:"oa_status"`
		Best   struct {
			PDF     string `json:"url_for_pdf"`
			URL     string `json:"url"`
			Version string `json:"version"`
		} `json:"best_oa_location"`
		Locations []struct {
			PDF     string `json:"url_for_pdf"`
			Version string `json:"version"`
		} `json:"oa_locations"`
	}
	if err := getJSON(ctx, client, r.withContact("https://api.unpaywall.org/v2/"+url.PathEscape(doi), "email"), &data); err != nil || !data.IsOA {
		return nil, err
	}
	link := data.Best.PDF
	kind := "pdf"
	if link == "" {
		link = data.Best.URL
		kind = "html"
	}
	if link == "" {
		return nil, nil
	}
	out := []Candidate{{URL: link, Source: "unpaywall", Priority: candidatePriority("unpaywall", data.Status, data.Best.Version, kind), Kind: kind, Free: data.Status}}
	for _, location := range data.Locations {
		if location.PDF != "" {
			out = append(out, Candidate{URL: location.PDF, Source: "unpaywall", Priority: candidatePriority("unpaywall", data.Status, location.Version, "pdf") + 5, Kind: "pdf", Free: data.Status})
		}
	}
	return out, nil
}
func (r *Resolver) openAlexDOI(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		OA struct {
			URL    string `json:"oa_url"`
			Status string `json:"oa_status"`
		} `json:"open_access"`
		Locations []struct {
			IsOA    bool   `json:"is_oa"`
			PDF     string `json:"pdf_url"`
			Version string `json:"version"`
		} `json:"locations"`
	}
	if err := getJSON(ctx, client, r.withContact("https://api.openalex.org/works/https://doi.org/"+url.PathEscape(doi), "mailto"), &data); err != nil {
		return nil, err
	}
	out := []Candidate{}
	if data.OA.URL != "" {
		out = append(out, Candidate{URL: data.OA.URL, Source: "openalex", Priority: candidatePriority("openalex", data.OA.Status, "", "pdf"), Kind: "pdf", Free: data.OA.Status})
	}
	for _, l := range data.Locations {
		if l.IsOA && l.PDF != "" {
			out = append(out, Candidate{URL: l.PDF, Source: "openalex", Priority: candidatePriority("openalex", data.OA.Status, l.Version, "pdf") + 4, Kind: "pdf", Free: data.OA.Status})
		}
	}
	return out, nil
}
func (r *Resolver) crossref(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		Message struct {
			Links []struct {
				URL  string `json:"URL"`
				Type string `json:"content-type"`
			} `json:"link"`
		} `json:"message"`
	}
	if err := getJSON(ctx, client, r.withContact("https://api.crossref.org/works/"+url.PathEscape(doi), "mailto"), &data); err != nil {
		return nil, err
	}
	out := []Candidate{}
	for _, l := range data.Message.Links {
		if l.URL != "" && strings.Contains(l.Type, "pdf") {
			out = append(out, Candidate{URL: l.URL, Source: "crossref", Priority: 30, Kind: "pdf"})
		}
	}
	return out, nil
}

func (r *Resolver) semanticScholar(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		PDF *struct {
			URL    string `json:"url"`
			Status string `json:"status"`
		} `json:"openAccessPdf"`
		External map[string]string `json:"externalIds"`
	}
	headers := map[string]string{}
	if key := r.apiKey(r.SemanticScholarAPIKey, func() string { return strings.TrimSpace(os.Getenv("SEMANTIC_SCHOLAR_API_KEY")) }); key != "" {
		headers["x-api-key"] = key
	}
	if err := getJSONWithHeaders(ctx, client, "https://api.semanticscholar.org/graph/v1/paper/DOI:"+url.PathEscape(doi)+"?fields=openAccessPdf,externalIds", headers, &data); err != nil {
		return nil, err
	}
	out := []Candidate{}
	if data.PDF != nil && data.PDF.URL != "" {
		out = append(out, Candidate{URL: data.PDF.URL, Source: "semanticscholar", Priority: candidatePriority("semanticscholar", data.PDF.Status, "", "pdf"), Kind: "pdf", Free: data.PDF.Status})
	}
	if arxiv := data.External["ArXiv"]; arxiv != "" {
		out = append(out, Candidate{URL: "https://arxiv.org/pdf/" + arxiv, Source: "arxiv", Priority: 0, Kind: "pdf"})
	}
	if pmc := data.External["PubMedCentral"]; pmc != "" {
		if !strings.HasPrefix(strings.ToUpper(pmc), "PMC") {
			pmc = "PMC" + pmc
		}
		out = append(out, Candidate{URL: "https://europepmc.org/articles/" + pmc + "?pdf=render", Source: "europepmc", Priority: 16, Kind: "pdf"})
	}
	return out, nil
}
func (r *Resolver) core(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		Download string   `json:"downloadUrl"`
		Sources  []string `json:"sourceFulltextUrls"`
	}
	u := "https://api.core.ac.uk/v3/works/" + url.PathEscape(doi)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if key := r.apiKey(r.CoreAPIKey, func() string { return strings.TrimSpace(os.Getenv("CORE_API_KEY")) }); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := []Candidate{}
	if data.Download != "" {
		out = append(out, Candidate{URL: data.Download, Source: "core", Priority: 40, Kind: "pdf"})
	}
	for _, s := range data.Sources {
		if s != "" {
			out = append(out, Candidate{URL: s, Source: "core", Priority: 41, Kind: "pdf"})
		}
	}
	return out, nil
}
func (r *Resolver) doaj(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	var data struct {
		Results []struct {
			Bib struct {
				Links []struct {
					URL  string `json:"url"`
					Type string `json:"type"`
				} `json:"link"`
			} `json:"bibjson"`
		} `json:"results"`
	}
	if err := getJSON(ctx, client, "https://doaj.org/api/search/articles/doi:"+url.PathEscape(doi), &data); err != nil {
		return nil, err
	}
	out := []Candidate{}
	for _, row := range data.Results[:minInt(len(data.Results), 1)] {
		for _, link := range row.Bib.Links {
			if link.URL != "" && link.Type == "fulltext" {
				out = append(out, Candidate{URL: link.URL, Source: "doaj", Priority: candidatePriority("doaj", "", "", "html"), Kind: "html"})
			}
		}
	}
	return out, nil
}
func (r *Resolver) europePMCDOI(ctx context.Context, client *http.Client, doi string) ([]Candidate, error) {
	pmcid, err := idToPMCID(ctx, client, doi)
	if err != nil || pmcid == "" {
		return nil, err
	}
	return []Candidate{{URL: "https://europepmc.org/articles/" + pmcid + "?pdf=render", Source: "europepmc", Priority: candidatePriority("europepmc", "green", "", "pdf"), Kind: "pdf", Free: "green"}}, nil
}

// ExtractDOI is the exported spelling used by adapters that only need lexical
// DOI extraction and must not start network discovery.
func ExtractDOI(text string) string { return DOIFrom(text) }

func (r *Resolver) ResolvePMCID(ctx context.Context, pmcid string) ([]Candidate, error) {
	pmcid = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pmcid)), "pmcid:")))
	if !strings.HasPrefix(pmcid, "PMC") {
		pmcid = "PMC" + pmcid
	}
	return []Candidate{{URL: "https://europepmc.org/articles/" + pmcid + "?pdf=render", Source: "europepmc", Priority: 0, Kind: "pdf"}}, nil
}

func (r *Resolver) ResolvePMID(ctx context.Context, pmid string) ([]Candidate, error) {
	pmid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(pmid)), "pmid:"))
	if pmid == "" {
		return nil, fmt.Errorf("invalid PMID")
	}
	ctx = resolverContext(ctx, r)
	var data struct {
		Records []struct {
			PMCID string `json:"pmcid"`
		} `json:"records"`
	}
	if err := getJSON(ctx, r.client(), r.withContact("https://pmc.ncbi.nlm.nih.gov/tools/idconv/api/v1/articles/?ids="+url.QueryEscape(pmid)+"&format=json&tool=harvester-mcp", "email"), &data); err != nil {
		return nil, err
	}
	if len(data.Records) == 0 || data.Records[0].PMCID == "" {
		return nil, fmt.Errorf("PubMed ID %s has no open-access PMCID", pmid)
	}
	return r.ResolvePMCID(ctx, data.Records[0].PMCID)
}

func getJSON(ctx context.Context, client *http.Client, raw string, dst any) error {
	return getJSONWithHeaders(ctx, client, raw, nil, dst)
}

func postJSON(ctx context.Context, client *http.Client, raw string, payload any, dst any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", scholarlyUA())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func getJSONWithHeaders(ctx context.Context, client *http.Client, raw string, headers map[string]string, dst any) error {
	ua := scholarlyUA()
	if contextual, ok := ctx.Value(resolverUAContextKey{}).(string); ok && contextual != "" {
		ua = contextual
	}
	body, status, _, err := getBodyWithHeaders(ctx, client, raw, ua, headers, 10*1024*1024)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("HTTP %d", status)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func scholarlyUA() string {
	if email := contactEmail(); email != "" {
		return "harvester-mcp/1.0 (mailto:" + email + ")"
	}
	return "harvester-mcp/1.0"
}

func contactEmail() string { return strings.TrimSpace(os.Getenv("HARVESTER_CONTACT_EMAIL")) }

func withContact(raw, key string) string {
	if contactEmail() == "" {
		return raw
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + key + "=" + url.QueryEscape(contactEmail())
}

func sortCandidates(in []Candidate) []Candidate {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Priority < in[j].Priority })
	seen := map[string]bool{}
	out := []Candidate{}
	for _, c := range in {
		if c.URL != "" && !seen[c.URL] {
			seen[c.URL] = true
			out = append(out, c)
		}
	}
	return out
}

func titleSimilarity(a, b string) float64 {
	aa := titleTokens(a)
	bb := titleTokens(b)
	if len(aa) == 0 || len(bb) == 0 {
		return 0
	}
	intersect := 0
	for x := range aa {
		if bb[x] {
			intersect++
		}
	}
	return float64(intersect) / float64(len(aa)+len(bb)-intersect)
}

var titleTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func titleTokens(value string) map[string]bool {
	out := map[string]bool{}
	for _, token := range titleTokenPattern.FindAllString(strings.ToLower(value), -1) {
		out[token] = true
	}
	return out
}

func titleMatch(query, title string) float64 {
	j := titleSimilarity(query, title)
	q, t := titleTokens(query), titleTokens(title)
	if len(q) > 0 && len(q) <= len(t) {
		contained := true
		for token := range q {
			if !t[token] {
				contained = false
				break
			}
		}
		if contained && j < .85 {
			return .85
		}
	}
	return j
}

func (h *Harvester) fetchKnownID(ctx context.Context, source string, kind IdentifierKind, options FetchOptions) Result {
	canonical := NormalizeIdentifier(source)
	if canonical == "" {
		canonical = strings.TrimSpace(source)
	}
	if !options.Refresh {
		if body, cachedKind, meta, path, ok := h.cache.loadAny(canonical, []string{"pdf", "docx", "xlsx", "pptx", "csv", "json", "txt", "html"}); ok {
			return h.resultFromCache(source, cachedKind, body, meta, path)
		}
	}
	r := h.resolver()
	var candidates []Candidate
	var err error
	switch kind {
	case IdentifierDOI:
		candidates, err = r.ResolveDOI(ctx, source)
	case IdentifierISBN:
		candidates, err = r.ResolveBook(ctx, source)
	case IdentifierPMCID:
		candidates, err = r.ResolvePMCID(ctx, source)
	case IdentifierPMID:
		candidates, err = r.ResolvePMID(ctx, source)
	}
	if err != nil {
		return Result{Source: source, Error: err.Error()}
	}
	if len(candidates) == 0 && kind != IdentifierDOI {
		return Result{Source: source, Error: "no legal open-access copy found"}
	}
	trace := make([]string, 0, len(candidates))
	for _, c := range candidates {
		trace = append(trace, "oa:"+c.Source)
		result := h.fetchURLWithPolicy(ctx, c.URL, options, false)
		if result.Error == "" {
			return h.storeResultAlias(source, canonical, result, append([]string(nil), trace...), options)
		}
	}
	// Europe PMC's PDF endpoint is the preferred mirror, but scanned/HTML-only
	// records still expose a legal full-text article page. Keep that fallback
	// inside the identifier chain rather than treating the PMCID as a generic
	// publisher URL.
	if kind == IdentifierPMCID || kind == IdentifierPMID {
		for _, c := range candidates {
			if pmcid := extractPMCID(c.URL); pmcid != "" {
				for _, mirrorURL := range []string{PMCArticleURL(pmcid), "https://www.ebi.ac.uk/europepmc/webservices/rest/" + pmcid + "/fullTextXML"} {
					result := h.fetchURL(ctx, mirrorURL, options)
					if result.Error == "" {
						trace := append([]string{"mirror:" + pmcid}, result.Rungs...)
						return h.storeResultAlias(source, canonical, result, trace, options)
					}
				}
			}
		}
	}
	if kind == IdentifierDOI {
		doiURL := "https://doi.org/" + canonical
		trace = append(trace, "wayback")
		if snapshot, waybackErr := WaybackRawURL(ctx, h.oa, doiURL); waybackErr == nil && snapshot != "" {
			result := h.fetchURLWithPolicy(ctx, snapshot, options, false)
			if result.Error == "" {
				result.Method = "mirror:wayback"
				return h.storeResultAlias(source, canonical, result, append([]string(nil), trace...), options)
			}
		}
		message := fmt.Sprintf("Found DOI %s, but no free, legal full text exists in any open-access source (checked Unpaywall, OpenAlex, Semantic Scholar, Europe PMC, CORE, DOAJ, arXiv/OSF, and the Wayback Machine). The paper is likely paywalled — use `search` to find an author preprint or the publisher's page directly.", canonical)
		return Result{Source: source, Error: withRungs(message, trace), Rungs: trace}
	}
	return Result{Source: source, Error: withRungs("all legal open-access candidates failed", trace), Rungs: trace}
}

func extractPMCID(raw string) string {
	match := regexp.MustCompile(`(?i)(PMC\d+)`).FindString(raw)
	return strings.ToUpper(match)
}

func (h *Harvester) fetchOA(ctx context.Context, doi string, rungs []string, options FetchOptions) Result {
	cands, err := h.resolver().ResolveDOI(ctx, doi)
	if err != nil {
		return Result{Source: doi, Error: err.Error()}
	}
	trace := append([]string(nil), rungs...)
	for _, c := range cands {
		trace = append(trace, "oa:"+c.Source)
		result := h.fetchURLWithPolicy(ctx, c.URL, options, false)
		if result.Error == "" {
			result.Rungs = append([]string(nil), trace...)
			return result
		}
	}
	return Result{Source: doi, Error: withRungs("OA chain exhausted", trace), Rungs: trace}
}
