package harvest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FetchWithOptions routes local files, URLs, and scholarly identifiers.
func (h *Harvester) FetchWithOptions(ctx context.Context, source string, options FetchOptions) Result {
	key := canonicalNegativeKey("fetch", strings.TrimSpace(source))
	if !options.Refresh {
		if cached, ok := h.neg.get(key); ok {
			if options.SizeOnly {
				cached.Content = ""
			}
			return cached
		}
		h.flightMu.Lock()
		if flight, ok := h.flights[key]; ok {
			h.flightMu.Unlock()
			select {
			case <-flight.done:
				result := flight.result
				if options.SizeOnly {
					result.Content = ""
				}
				return result
			case <-ctx.Done():
				return Result{Source: source, Error: ctx.Err().Error()}
			}
		}
		flight := &fetchFlight{done: make(chan struct{})}
		h.flights[key] = flight
		h.flightMu.Unlock()
		result := h.fetchUnshared(ctx, source, options)
		h.flightMu.Lock()
		flight.result = result
		close(flight.done)
		delete(h.flights, key)
		h.flightMu.Unlock()
		if result.Error != "" {
			h.neg.put(key, result)
		}
		h.recordStat(source, result) // scoreboard: every terminal outcome lands in stats.jsonl
		if options.SizeOnly && result.Error == "" {
			result.Content = ""
		}
		return result
	}
	result := h.fetchUnshared(ctx, source, options)
	h.recordStat(source, result)
	return result
}

func (h *Harvester) fetchUnshared(ctx context.Context, source string, options FetchOptions) Result {
	source = strings.TrimSpace(source)
	if source == "" {
		return Result{Source: source, Error: "source is empty"}
	}
	var result Result
	if id := ClassifyIdentifier(source); id != IdentifierNone {
		result = h.fetchKnownID(ctx, source, id, options)
	} else if isLocalSource(source) {
		result = h.fetchLocal(ctx, source, options)
	} else {
		// A bare title is intentionally ambiguous. The Python fetch tool never
		// guesses a work from title text; callers must use FindWorks and then fetch
		// the selected handle. Existing paths and unambiguous identifiers were
		// handled above.
		if strings.HasPrefix(strings.ToLower(source), "title:") {
			value := strings.Trim(strings.TrimSpace(source[len("title:"):]), "\"'")
			result = Result{Source: source, Error: fmt.Sprintf("%q is a title — use the `findWorks` tool to list candidate works (it returns a fetch handle for each), then fetch the one you pick. `fetch` retrieves locations and UNAMBIGUOUS identifiers (URL, file path, DOI, ISBN), never a title.", value)}
		} else if parsed, parseErr := url.Parse(source); parseErr == nil && parsed.Scheme != "" && parsed.Host != "" {
			result = h.fetchURL(ctx, source, options)
		} else {
			result = Result{Source: source, Error: fmt.Sprintf("%q is a title — use the `findWorks` tool to list candidate works (it returns a fetch handle for each), then fetch the one you pick. `fetch` retrieves locations and UNAMBIGUOUS identifiers (URL, file path, DOI, ISBN), never a title.", source)}
		}
	}
	if options.SizeOnly && result.Error == "" {
		result.Content = ""
	}
	return result
}

func canonicalNegativeKey(media, source string) string {
	if media == "" {
		media = "fetch"
	}
	if doi := DOIFrom(source); doi != "" {
		return media + ":doi:" + strings.ToLower(doi)
	}
	return media + ":" + source
}

func isLocalSource(source string) bool {
	if strings.HasPrefix(strings.ToLower(source), "file://") {
		return true
	}
	u, err := url.Parse(source)
	if err != nil {
		return true
	}
	if u.Scheme != "" || u.Host != "" {
		return false
	}
	if _, statErr := os.Stat(source); statErr == nil {
		return true
	}
	// Recognized file extensions remain local-path syntax even when the file is
	// missing; callers then receive a useful missing-path diagnostic instead of
	// an ambiguous scholarly-title hint.
	if DetectKind(source) != "html" {
		return true
	}
	return filepath.IsAbs(source) || strings.HasPrefix(source, ".") || strings.ContainsAny(source, `/\\`)
}

func (h *Harvester) fetchTitle(ctx context.Context, title string, options FetchOptions) Result {
	candidates, err := h.resolver().ResolveTitle(ctx, title)
	if err != nil {
		return Result{Source: title, Error: err.Error()}
	}
	for _, candidate := range candidates {
		var result Result
		if doi := DOIFrom(candidate.URL); doi != "" && !strings.HasPrefix(strings.ToLower(candidate.URL), "http://") && !strings.HasPrefix(strings.ToLower(candidate.URL), "https://") {
			result = h.fetchOA(ctx, doi, nil, options)
		} else {
			result = h.fetchURL(ctx, candidate.URL, options)
		}
		if result.Error == "" {
			result.Source = title
			result.Rungs = append([]string{"title:" + candidate.Source}, result.Rungs...)
			return result
		}
	}
	return Result{Source: title, Error: fmt.Sprintf("no confident work match for %q; use findWorks", title)}
}

func (h *Harvester) fetchURL(ctx context.Context, source string, options FetchOptions) Result {
	return h.fetchURLWithPolicy(ctx, source, options, true)
}

func (h *Harvester) fetchURLWithPolicy(ctx context.Context, source string, options FetchOptions, allowOAPivot bool) Result {
	if err := assertFetchable(source); err != nil {
		if strings.Contains(err.Error(), "unsupported URL scheme") {
			return Result{Source: source, Error: fmt.Sprintf("unsupported URL scheme in %q — fetch handles http(s):// and file:// URLs, local paths, DOIs, and ISBNs.", source)}
		}
		return Result{Source: source, Error: err.Error()}
	}
	if isPubMedSearchURL(source) {
		return Result{Source: source, Error: fmt.Sprintf("%s is a PubMed search/results URL, not an article — use the `findWorks` tool (or `search`) to get candidate works, each with a fetch handle.", source)}
	}
	// Cache is looked up by the eventual kind for stable type partitioning. A URL
	// extension gives us an early key; response sniffing may move it to another
	// partition after the fetch.
	guess := kindFromName(source)
	if !options.Refresh {
		if body, meta, path, ok := h.cache.load(source, guess); ok {
			return h.resultFromCache(source, guess, body, meta, path)
		}
		// Extensionless scholarly/PDF URLs are classified from response headers
		// on the first request. Search every type partition on subsequent calls
		// so the sniffed kind remains cacheable without a sidecar index.
		if body, kind, meta, path, ok := h.cache.loadAny(source, []string{"pdf", "docx", "xlsx", "pptx", "csv", "json", "txt", "epub", "html"}); ok {
			return h.resultFromCache(source, kind, body, meta, path)
		}
	}
	rungs := []string{}
	lastStatus := 0
	var lastErr error
	var lastPage []byte
	lastErrorKind := ""
	lastChallenge := false
	lastContentChars := 0
	emptyPDFConvert := false
	var emptyPDFBody []byte
	wrongPDF := false
	directClient, chromeClient := h.client, h.chrome
	switch guess {
	case "pdf", "docx", "xlsx", "pptx", "csv", "zip", "tar", "7z", "rar":
		directClient, chromeClient = h.binaryDirectOrClient(), h.binaryChromeOrChrome()
	}
	// Ladder note: the Python reference also carries a defuddle.md reader rung and an
	// opt-in real-browser rung (Patchright + system Chrome); on this engine defuddle is
	// wired below and Chrome impersonation is tls-client at the wire level. There is no
	// Go real-browser rung by design.
	for _, rung := range []struct {
		name   string
		client *http.Client
		target string
		ua     string
	}{
		{"direct", directClient, source, h.userAgent},
		{"chrome-impersonation", chromeClient, source, chromeUA},
	} {
		rungs = append(rungs, rung.name)
		body, status, contentType, err := getBody(ctx, rung.client, rung.target, rung.ua, h.options.MaxBytes)
		if err != nil {
			lastErr = err
			lastErrorKind = errorKind(err)
			continue
		}
		lastStatus = status
		if status >= 400 && lastErrorKind == "" {
			lastErrorKind = "http"
		}
		if isChallenge(body, status) {
			lastChallenge = true
		}
		kind := classifyFetchedKind(source, contentType, body)
		if guess == "pdf" {
			if strings.HasPrefix(string(body), "%PDF-") {
				kind = "pdf"
			} else {
				wrongPDF = true
				if (kind == "html" || kind == "txt") && len(body) > 0 {
					lastPage = append(lastPage[:0], body...)
				}
				continue
			}
		}
		if (kind == "html" || kind == "txt") && len(body) > 0 {
			lastPage = append(lastPage[:0], body...)
		}
		if kind == "image" || isImageKind(kind) {
			return Result{Source: source, Kind: "image", Error: fmt.Sprintf("%s is an image — use the `fetchImage` tool, not `fetch`.", source), HTTPStatus: status, ErrorKind: "wrong_kind"}
		}
		if kind == "zip" || kind == "tar" || kind == "7z" || kind == "rar" {
			// An EPUB is zip-SHAPED but is a book; OA book sources (OAPEN/DOAB/
			// Gutenberg/Zenodo) serve EPUB constantly. Detect it by its uncompressed
			// `mimetype` member and convert it instead of throwing the found book away.
			if kind == "zip" && LooksLikeEpub(body) {
				kind = "epub"
			} else {
				return Result{Source: source, Kind: "archive", Error: fmt.Sprintf("%s is a %s archive — use the `archive` tool, not `fetch`.", source, kind), HTTPStatus: status, ErrorKind: "wrong_kind"}
			}
		}
		converted, err := h.convert(ctx, kind, source, body)
		if err != nil {
			continue
		}
		if kind == "pdf" && strings.TrimSpace(converted) == "" {
			emptyPDFConvert = true
			if len(emptyPDFBody) == 0 {
				emptyPDFBody = append([]byte(nil), body...)
			}
		}
		lastContentChars = contentChars(converted)
		binary4xxOK := status >= 400 && kind == "pdf" && strings.HasPrefix(string(body), "%PDF-")
		if len(body) == 0 || isChallenge(body, status) || (status >= 400 && kind != "html" && kind != "txt" && !binary4xxOK) {
			continue
		}
		if !usableContent(converted, kind) {
			continue
		}
		// The HTML ladder escalates thin extraction results. A short page is
		// commonly a JS shell or bot wall even when the HTTP status is 200;
		// plain text and converted binary documents are not subject to this
		// threshold because their bytes are already the requested artifact.
		if kind == "html" && contentChars(converted) < 500 {
			continue
		}
		if kind == "html" {
			if localized, localizeErr := h.LocalizeImages(ctx, converted, source); localizeErr == nil {
				converted = localized
			}
		}
		method := rung.name
		if kind == "txt" {
			method = "plain-text"
		}
		return h.storeResult(source, kind, method, converted, int64(len(body)), status, rungs, options)
	}
	if !isPrivateURL(source) && guess != "pdf" {
		rungs = append(rungs, "jina")
		target := strings.TrimRight(h.options.JinaURL, "/") + "/" + source
		body, status, _, err := getBody(ctx, h.jina, target, h.userAgent, h.options.MaxBytes)
		if err != nil {
			lastErr = err
			lastErrorKind = errorKind(err)
		}
		lastStatus = status
		if err == nil && status < 400 && !isChallenge(body, status) {
			// Jina Reader already returns clean Markdown. Feeding it back into an
			// HTML converter loses headings and code blocks, so preserve it as the
			// original HTML-source kind for cache/type semantics.
			kind := "html"
			converted, convErr := stripJinaEnvelope(string(body)), error(nil)
			if convErr == nil && usableContent(converted, kind) {
				return h.storeResult(source, kind, "jina", converted, int64(len(body)), status, rungs, options)
			}
		}
	}
	// defuddle.md — a second keyless reader beside Jina (different infra,
	// different blocks), tried before the legal mirror pivot.
	if !isPrivateURL(source) && guess != "pdf" {
		rungs = append(rungs, "defuddle")
		target := "https://defuddle.md/" + source
		body, status, _, err := getBody(ctx, h.client, target, h.userAgent, h.options.MaxBytes)
		if err == nil && status < 400 && !isChallenge(body, status) {
			converted := stripDefuddleEnvelope(string(body))
			if usableContent(converted, "html") && contentChars(converted) > lastContentChars {
				return h.storeResult(source, "html", "defuddle-reader", converted, int64(len(body)), status, rungs, options)
			}
		}
	}
	// A publisher wall often leaves citation_pdf_url/citation_doi metadata in
	// the HTML error body. Reuse that body before giving up; this is the same
	// legal mirror pivot used for DOI inputs, and avoids a redundant page fetch.
	if len(lastPage) > 0 {
		metaDOI, metaPDF, _ := ExtractMetaLinks(string(lastPage))
		if metaPDF != "" && metaPDF != source {
			if result := h.fetchURLWithPolicy(ctx, metaPDF, options, allowOAPivot); result.Error == "" {
				trace := append(append([]string(nil), rungs...), "oa:citation_pdf_url")
				return h.storeResultAlias(source, source, result, trace, options)
			}
		}
		if allowOAPivot && metaDOI != "" && !strings.EqualFold(metaDOI, DOIFrom(source)) {
			if result := h.fetchOA(ctx, metaDOI, rungs, options); result.Error == "" {
				return result
			} else if len(result.Rungs) > len(rungs) {
				rungs = result.Rungs
			}
		}
	}
	// Scholarly URLs get one legal OA pivot after the web ladder. This is
	// intentionally opt-in by recognizable DOI; arbitrary URLs must not trigger
	// broad discovery traffic.
	if doi := DOIFrom(source); allowOAPivot && doi != "" {
		if result := h.fetchOA(ctx, doi, rungs, options); result.Error == "" {
			return result
		} else if len(result.Rungs) > len(rungs) {
			rungs = result.Rungs
		}
	}
	// Any public source may have a legal Wayback snapshot, not only a DOI
	// landing page. Avoid recursing when the snapshot itself fails.
	if !strings.Contains(strings.ToLower(source), "web.archive.org") && !isPrivateURL(source) {
		rungs = append(rungs, "wayback")
		if snapshot, wbErr := WaybackRawURL(ctx, h.oa, source); wbErr == nil && snapshot != "" {
			if result := h.fetchURLWithPolicy(ctx, snapshot, options, false); result.Error == "" {
				result.Source = source
				result.Rungs = append([]string(nil), rungs...)
				return result
			}
		}
	}
	// LAST resort for a remote PDF whose text layer converted EMPTY (a scan):
	// ONE forced-OCR pass. Every faster rescue just failed; OCR is slow, so it
	// fires exactly when nothing else worked. Needs an OCR-capable converter —
	// a plain Converter skips this rung and the ladder's error stands.
	ocrRan, ocrBackendFailed := false, false
	if emptyPDFConvert && len(emptyPDFBody) > 0 {
		if ocrConverter, ok := h.options.Converter.(OCRConverter); ok {
			rungs = append(rungs, "ocr")
			ocrRan = true
			ocrConverted, ocrErr := ocrConverter.ConvertOCR(ctx, "pdf", source, emptyPDFBody)
			switch {
			case ocrErr != nil:
				ocrBackendFailed = true
				log.Printf("harvest: OCR escalation backend failed for %s: %v", source, ocrErr)
			case usableContent(ocrConverted, "pdf"):
				return h.storeResult(source, "pdf", "pdf:ocr", ocrConverted, int64(len(emptyPDFBody)), lastStatus, rungs, options)
			}
		}
	}
	message := failureMessage(source, lastStatus, lastErrorKind, lastChallenge)
	if wrongPDF {
		message = fmt.Sprintf("%s has a .pdf address but did not return a PDF (non-PDF content — likely an HTML paywall/login wall or a bot-block). Use `search` to find an open-access copy.", source)
	}
	if emptyPDFConvert {
		// A BROKEN OCR backend and an OCR pass that legitimately found no text
		// are different answers; collapsing them would let an outage read as
		// "this PDF has nothing in it".
		switch {
		case ocrBackendFailed:
			message = fmt.Sprintf("Downloaded the PDF from %s but it converted to EMPTY text, and the OCR escalation could not RUN (converter backend error — see the server log). That is a tool outage, not proof the PDF is textless: retry, or use `search` to find an alternative copy.", source)
		case ocrRan:
			message = fmt.Sprintf("Downloaded the PDF from %s but it converted to EMPTY text. It is likely scanned/image-only, corrupt, or password-protected — an OCR pass was already attempted on this copy and produced nothing. Use `search` to find an alternative copy.", source)
		default:
			message = fmt.Sprintf("Downloaded the PDF from %s but it converted to EMPTY text. It is likely scanned/image-only, corrupt, or password-protected — if it's a scanned/image-only PDF, set HARVESTER_PDF_OCR=1 to OCR it. Use `search` to find an alternative copy.", source)
		}
	}
	if lastContentChars == 0 && len(lastPage) > 0 {
		lastContentChars = contentChars(string(lastPage))
	}
	if lastErr != nil && strings.Contains(strings.ToLower(lastErr.Error()), "private") {
		message = lastErr.Error()
	}
	message = withRungs(message, rungs)
	return Result{Source: source, HTTPStatus: lastStatus, Error: message, ErrorKind: lastErrorKind, Challenge: lastChallenge, Chars: lastContentChars, ContentChars: lastContentChars, Rungs: rungs}
}

func isPubMedSearchURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Hostname(), "pubmed.ncbi.nlm.nih.gov") {
		return false
	}
	return strings.Contains(strings.ToLower(u.Path), "/search") || strings.Contains(strings.ToLower(u.RawQuery), "term=")
}

func (h *Harvester) fetchLocal(ctx context.Context, source string, options FetchOptions) Result {
	path := source
	if strings.HasPrefix(strings.ToLower(path), "file://") {
		decoded, err := fileURLPath(path)
		if err != nil {
			return Result{Source: source, Error: err.Error()}
		}
		path = decoded
	}
	if reason := DenyLocalPath(path, h.options.LocalRoots); reason != "" {
		return Result{Source: source, Error: reason}
	}
	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Result{Source: source, Error: fmt.Sprintf("read local file %s: %v", path, err)}
	}
	kind := classifyFetchedKind(path, "", body)
	if kindFromName(path) == "pdf" && !strings.HasPrefix(string(body), "%PDF-") {
		return Result{Source: source, Kind: "pdf", Error: fmt.Sprintf("%s has a .pdf extension but is not a PDF file — check its actual contents before fetching it again.", source)}
	}
	if !options.Refresh {
		if cached, meta, cachePath, ok := h.cache.load(source, kind); ok {
			return h.resultFromCache(source, kind, cached, meta, cachePath)
		}
	}
	converted, err := h.convert(ctx, kind, source, body)
	if err != nil {
		return Result{Source: source, Kind: kind, Error: err.Error()}
	}
	if !usableContent(converted, kind) {
		return Result{Source: source, Kind: kind, Error: "conversion produced no usable content"}
	}
	return h.storeResult(source, kind, "local", converted, int64(len(body)), 0, []string{"local"}, options)
}

func (h *Harvester) convert(ctx context.Context, kind, source string, body []byte) (string, error) {
	if kind == "txt" {
		return string(body), nil
	}
	if h.options.Converter == nil {
		return "", errors.New("no injected converter configured for " + kind)
	}
	return h.options.Converter.Convert(ctx, kind, source, body)
}

func classifyFetchedKind(source, contentType string, body []byte) string {
	kind := classifyKind(source, contentType, body)
	if kind != "html" && kind != "txt" {
		return kind
	}
	if IsPlainText(source, contentType, string(body)) {
		return "txt"
	}
	return "html"
}

func usableContent(content, kind string) bool {
	if content == "" {
		return false
	}
	if kind == "html" || kind == "txt" {
		return len(strings.TrimSpace(content)) >= 1
	}
	return true
}

func (h *Harvester) storeResult(source, kind, method, content string, bytes int64, statusCode int, rungs []string, options FetchOptions) Result {
	path, err := h.cache.save(source, kind, method, content, rungs)
	if err != nil {
		return Result{Source: source, Kind: kind, Error: err.Error(), Rungs: rungs}
	}
	status := "miss"
	if options.Refresh {
		status = "refresh"
	}
	// The public receipt reports the on-disk artifact size, including its
	// provenance frontmatter, just like the Python cache result. Fall back to
	// source bytes only if a filesystem stat races with cleanup.
	if info, statErr := os.Stat(path); statErr == nil {
		bytes = info.Size()
	}
	chars := contentChars(content)
	return Result{Source: source, Kind: kind, Content: truncateInline(content, h.options.MaxInlineChars), Path: path, Method: method,
		CacheStatus: status, Bytes: bytes, Chars: chars, ContentChars: chars, Tokens: estimateTokens(content), HTTPStatus: statusCode, Rungs: rungs}
}

func (h *Harvester) storeResultAlias(source, canonicalSource string, result Result, rungs []string, options FetchOptions) Result {
	content := result.Content
	if result.Path != "" {
		if raw, err := os.ReadFile(result.Path); err == nil {
			_, content = parseFrontmatter(string(raw))
		}
	}
	stored := h.storeResult(canonicalSource, result.Kind, result.Method, content, result.Bytes, result.HTTPStatus, rungs, options)
	stored.Source = source
	return stored
}

func (h *Harvester) resultFromCache(source, kind string, content string, meta map[string]string, path string) Result {
	rungs := []string{}
	if raw := meta["rungs"]; raw != "" {
		for _, rung := range strings.Split(raw, ",") {
			if s := strings.TrimSpace(rung); s != "" {
				rungs = append(rungs, s)
			}
		}
	}
	bytes := int64(0)
	if info, statErr := os.Stat(path); statErr == nil {
		bytes = info.Size()
	} else {
		bytes = int64(len(content))
	}
	tokens := estimateTokens(content)
	if stored, parseErr := strconv.Atoi(meta["token_count"]); parseErr == nil && stored >= 0 {
		tokens = stored
	}
	chars := contentChars(content)
	return Result{Source: source, Kind: kind, Content: truncateInline(content, h.options.MaxInlineChars), Path: path, Method: meta["method"], CacheStatus: "hit",
		Bytes: bytes, Chars: chars, ContentChars: chars, Tokens: tokens, Rungs: rungs}
}

func contentChars(content string) int { return len([]rune(strings.TrimSpace(content))) }

func kindFromName(source string) string {
	return DetectKind(source)
}

func isPrivateURL(source string) bool {
	// Jina's forwarding decision is lexical in the oracle. A public hostname
	// whose local resolver is unavailable must still be offered to Jina; the
	// direct/chrome transports perform the stronger DNS-pinned fetch check.
	return IsPrivateHost(source)
}

func rungsSummary(rungs []string) string { return strings.Join(rungs, ", ") }

func rungsPhrase(rungs []string) string {
	parts := make([]string, 0, len(rungs))
	for index := 0; index < len(rungs); {
		if !strings.HasPrefix(rungs[index], "oa:") {
			parts = append(parts, rungs[index])
			index++
			continue
		}
		end := index
		for end < len(rungs) && strings.HasPrefix(rungs[end], "oa:") {
			end++
		}
		count := end - index
		label := "source"
		if count != 1 {
			label = "sources"
		}
		parts = append(parts, fmt.Sprintf("oa-mirror(%d %s)", count, label))
		index = end
	}
	return strings.Join(parts, ", ")
}

func withRungs(message string, rungs []string) string {
	if len(rungs) <= 1 {
		return message
	}
	return message + " Rungs tried: " + rungsPhrase(rungs) + " — re-fetching will not help."
}
