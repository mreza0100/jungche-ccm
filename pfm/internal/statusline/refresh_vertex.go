package statusline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hostops/pfm/internal/paths"
)

const (
	vertexMetric                  = "aiplatform.googleapis.com/publisher/online_serving/token_count"
	vertexService                 = "C7E2-9256-1C43"
	cachedInputFactor             = 0.10
	storageEURPerMillionTokenHour = 0.877
)

var vertexSKUMap = map[string][2]string{
	"gemini-2.5-flash-lite": {
		"Gemini 2.5 Flash Lite Text Input - Predictions",
		"Gemini 2.5 Flash Lite Text Output - Predictions",
	},
	"gemini-2.5-flash": {
		"Gemini 2.5 Flash GA Text Input - Predictions",
		"Gemini 2.5 Flash GA Text Output - Predictions",
	},
	"gemini-2.5-pro": {
		"Gemini 2.5 Pro Text Input - Predictions",
		"Gemini 2.5 Pro Text Output - Predictions",
	},
	"gemini-3.5-flash": {
		"Gemini 3.5 Flash Regional Text Input - Predictions",
		"Gemini 3.5 Flash Regional Text Output - Predictions",
	},
}

var vertexFallbackEUR = map[string][2]float64{
	"gemini-2.5-flash-lite": {0.093, 0.37},
	"gemini-2.5-flash":      {0.28, 2.33},
	"gemini-2.5-pro":        {1.16, 9.30},
	"gemini-3.5-flash":      {1.53, 9.21},
}

// VertexOptions describes the detached Vertex cache refresh. Every URL and
// credential seam is replaceable so tests consume recorded JSON only.
type VertexOptions struct {
	Now       func() time.Time
	Project   string
	Locations []string

	CachePath      string
	PriceCachePath string
	Client         *http.Client
	AccessToken    func(context.Context) (string, error)

	MonitoringBaseURL string
	BillingBaseURL    string
	AIBaseURL         string
	Log               io.Writer
}

type vertexRow struct {
	Day          string
	Model        string
	InputFresh   int64
	InputCached  int64
	InputUnknown int64
	Output       int64
}

type monitoringResponse struct {
	TimeSeries    []monitoringSeries `json:"timeSeries"`
	NextPageToken string             `json:"nextPageToken"`
}

type monitoringSeries struct {
	Metric struct {
		Labels map[string]string `json:"labels"`
	} `json:"metric"`
	Resource struct {
		Labels map[string]string `json:"labels"`
	} `json:"resource"`
	Points []struct {
		Interval struct {
			EndTime string `json:"endTime"`
		} `json:"interval"`
		Value struct {
			Int64Value json.RawMessage `json:"int64Value"`
		} `json:"value"`
	} `json:"points"`
}

type billingResponse struct {
	SKUs          []billingSKU `json:"skus"`
	NextPageToken string       `json:"nextPageToken"`
}

type billingSKU struct {
	Description string `json:"description"`
	PricingInfo []struct {
		PricingExpression struct {
			TieredRates []struct {
				UnitPrice struct {
					Units string `json:"units"`
					Nanos int64  `json:"nanos"`
				} `json:"unitPrice"`
			} `json:"tieredRates"`
		} `json:"pricingExpression"`
	} `json:"pricingInfo"`
}

type storageResponse struct {
	CachedContents []struct {
		UsageMetadata struct {
			TotalTokenCount int64 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		CreateTime string `json:"createTime"`
	} `json:"cachedContents"`
	NextPageToken string `json:"nextPageToken"`
}

// RefreshVertex writes the same today|7d|epoch cache the retired Python pair
// produced. A storage-inventory failure is fatal and preserves the old cache;
// writing a fresh-looking token-only estimate would under-report materially.
func RefreshVertex(ctx context.Context, options VertexOptions) error {
	options = normalizeVertex(options)
	defer removeRefreshLock(options.CachePath)
	if options.Project == "" {
		return fmt.Errorf("vertex project is empty")
	}
	if options.AccessToken == nil {
		return fmt.Errorf("Vertex access-token reader is nil")
	}
	token, err := options.AccessToken(ctx)
	if err != nil {
		return fmt.Errorf("get Vertex access token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("get Vertex access token: command returned an empty token")
	}
	rows, err := fetchVertexRows(ctx, options, token, 7)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("no Vertex token_count data in the window")
	}
	prices := loadVertexPrices(ctx, options, token)
	now := options.Now()
	today := now.In(time.Local).Format("2006-01-02")
	todayEUR, weekEUR := 0.0, 0.0
	unknownTokens := int64(0)
	unpriced := map[string]int64{}
	for _, row := range rows {
		unknownTokens += row.InputUnknown
		rate, ok := longestVertexRate(row.Model, prices)
		if !ok {
			unpriced[row.Model] += row.InputFresh + row.InputCached + row.Output
			continue
		}
		cost := (float64(row.InputFresh)*rate[0] +
			float64(row.InputCached)*rate[0]*cachedInputFactor +
			float64(row.Output)*rate[1]) / 1_000_000
		weekEUR += cost
		if row.Day == today {
			todayEUR += cost
		}
	}
	storageToday, storageWeek, err := fetchStorageEUR(ctx, options, token)
	if err != nil {
		return fmt.Errorf("storage lookup failed; refusing token-only spend cache: %w", err)
	}
	todayEUR += storageToday
	weekEUR += storageWeek
	if len(unpriced) > 0 {
		unpricedTokens := int64(0)
		for _, tokens := range unpriced {
			unpricedTokens += tokens
		}
		fmt.Fprintf(options.Log, "WARNING: %d tokens from unpriced Vertex models excluded: %v\n", unpricedTokens, sortedKeys(unpriced))
	}
	if unknownTokens > 0 {
		fmt.Fprintf(options.Log, "WARNING: %d Vertex input tokens have an unrecognised cache label and are excluded\n", unknownTokens)
	}
	body := []byte(fmt.Sprintf("%.2f|%.2f|%d", todayEUR, weekEUR, now.Unix()))
	return atomicWrite(options.CachePath, body, 0o600)
}

func normalizeVertex(options VertexOptions) VertexOptions {
	if options.Now == nil {
		options.Now = time.Now
	}
	if len(options.Locations) == 0 {
		options.Locations = []string{"europe-west4"}
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 60 * time.Second}
	}
	if options.MonitoringBaseURL == "" {
		options.MonitoringBaseURL = "https://monitoring.googleapis.com/v3"
	}
	if options.BillingBaseURL == "" {
		options.BillingBaseURL = "https://cloudbilling.googleapis.com/v1"
	}
	cacheDir := os.TempDir()
	if jailHome := os.Getenv(paths.EnvHome); jailHome != "" {
		cacheDir = filepath.Join(jailHome, "tmp")
	}
	if options.CachePath == "" {
		options.CachePath = filepath.Join(cacheDir, "cc-vertex-spend")
	}
	if options.PriceCachePath == "" {
		options.PriceCachePath = filepath.Join(cacheDir, "cc-vertex-prices-eur.json")
	}
	if options.Log == nil {
		options.Log = io.Discard
	}
	return options
}

func fetchVertexRows(
	ctx context.Context,
	options VertexOptions,
	token string,
	days int,
) ([]vertexRow, error) {
	now := options.Now().In(time.Local)
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	start := tomorrow.AddDate(0, 0, -days)
	values := url.Values{
		"filter":                         {fmt.Sprintf("metric.type = %q", vertexMetric)},
		"interval.startTime":             {start.Format(time.RFC3339)},
		"interval.endTime":               {tomorrow.Format(time.RFC3339)},
		"aggregation.alignmentPeriod":    {"86400s"},
		"aggregation.perSeriesAligner":   {"ALIGN_SUM"},
		"aggregation.crossSeriesReducer": {"REDUCE_SUM"},
		"aggregation.groupByFields": {
			"metric.label.type",
			"metric.label.explicit_caching",
			"resource.label.model_user_id",
		},
		"view": {"FULL"},
	}
	base := strings.TrimRight(options.MonitoringBaseURL, "/") +
		"/projects/" + url.PathEscape(options.Project) + "/timeSeries"
	series := make([]monitoringSeries, 0)
	page := ""
	for {
		query := cloneValues(values)
		if page != "" {
			query.Set("pageToken", page)
		}
		var response monitoringResponse
		if err := getJSON(ctx, options.Client, base+"?"+query.Encode(), token, &response); err != nil {
			return nil, fmt.Errorf("fetch Vertex token series: %w", err)
		}
		series = append(series, response.TimeSeries...)
		page = response.NextPageToken
		if page == "" {
			break
		}
	}

	type totals struct{ fresh, cached, unknown, output int64 }
	grouped := map[string]map[string]*totals{}
	for _, seriesItem := range series {
		kind := seriesItem.Metric.Labels["type"]
		cacheLabel := strings.ToLower(strings.TrimSpace(seriesItem.Metric.Labels["explicit_caching"]))
		model := seriesItem.Resource.Labels["model_user_id"]
		if model == "" {
			model = "all-models"
		}
		for _, point := range seriesItem.Points {
			end, err := time.Parse(time.RFC3339Nano, point.Interval.EndTime)
			if err != nil {
				return nil, fmt.Errorf("invalid Vertex point endTime %q: %w", point.Interval.EndTime, err)
			}
			value, err := rawInt64(point.Value.Int64Value)
			if err != nil {
				return nil, fmt.Errorf("invalid Vertex point value: %w", err)
			}
			day := end.In(time.Local).Add(-time.Second).Format("2006-01-02")
			if grouped[day] == nil {
				grouped[day] = map[string]*totals{}
			}
			if grouped[day][model] == nil {
				grouped[day][model] = &totals{}
			}
			total := grouped[day][model]
			if kind != "input" {
				total.output += value
				continue
			}
			switch cacheBucket(cacheLabel) {
			case "cached":
				total.cached += value
			case "fresh":
				total.fresh += value
			default:
				total.unknown += value
			}
		}
	}
	rows := make([]vertexRow, 0)
	for _, day := range sortedKeys(grouped) {
		for _, model := range sortedKeys(grouped[day]) {
			total := grouped[day][model]
			rows = append(rows, vertexRow{
				Day: day, Model: model,
				InputFresh: total.fresh, InputCached: total.cached,
				InputUnknown: total.unknown, Output: total.output,
			})
		}
	}
	return rows, nil
}

func loadVertexPrices(
	ctx context.Context,
	options VertexOptions,
	token string,
) map[string][2]float64 {
	if info, err := os.Stat(options.PriceCachePath); err == nil &&
		options.Now().Sub(info.ModTime()) < 24*time.Hour {
		body, readErr := os.ReadFile(options.PriceCachePath)
		prices := map[string][2]float64{}
		if readErr == nil && json.Unmarshal(body, &prices) == nil {
			return prices
		}
		fmt.Fprintln(options.Log, "price cache unreadable — refetching")
	}
	prices, err := fetchVertexPrices(ctx, options, token)
	if err != nil {
		fmt.Fprintf(options.Log, "live catalog price fetch failed — using embedded fallback: %v\n", err)
		return clonePrices(vertexFallbackEUR)
	}
	body, marshalErr := json.Marshal(prices)
	if marshalErr != nil {
		fmt.Fprintf(options.Log, "marshal live Vertex price cache: %v\n", marshalErr)
	} else if writeErr := atomicWrite(options.PriceCachePath, body, 0o600); writeErr != nil {
		fmt.Fprintf(options.Log, "write live Vertex price cache: %v\n", writeErr)
	}
	return prices
}

func fetchVertexPrices(
	ctx context.Context,
	options VertexOptions,
	token string,
) (map[string][2]float64, error) {
	wanted := map[string][2]string{}
	for model, descriptions := range vertexSKUMap {
		wanted[descriptions[0]] = [2]string{model, "input"}
		wanted[descriptions[1]] = [2]string{model, "output"}
	}
	found := map[string]map[string]float64{}
	page := ""
	for count := 0; count < 8; count++ {
		query := url.Values{"pageSize": {"5000"}, "currencyCode": {"EUR"}}
		if page != "" {
			query.Set("pageToken", page)
		}
		endpoint := strings.TrimRight(options.BillingBaseURL, "/") +
			"/services/" + vertexService + "/skus?" + query.Encode()
		var response billingResponse
		if err := getJSON(ctx, options.Client, endpoint, token, &response); err != nil {
			return nil, err
		}
		for _, sku := range response.SKUs {
			tag, ok := wanted[sku.Description]
			if !ok || len(sku.PricingInfo) == 0 ||
				len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
				continue
			}
			unit := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
			unitText := unit.Units
			if unitText == "" {
				unitText = "0"
			}
			units, err := strconv.ParseFloat(unitText, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid catalog units for %q: %w", sku.Description, err)
			}
			perMillion := (units + float64(unit.Nanos)/1e9) * 1_000_000
			if found[tag[0]] == nil {
				found[tag[0]] = map[string]float64{}
			}
			found[tag[0]][tag[1]] = perMillion
		}
		page = response.NextPageToken
		if page == "" {
			break
		}
	}
	prices := clonePrices(vertexFallbackEUR)
	for model, kinds := range found {
		inputRate, inputOK := kinds["input"]
		outputRate, outputOK := kinds["output"]
		if inputOK && outputOK {
			prices[model] = [2]float64{inputRate, outputRate}
		}
	}
	return prices, nil
}

func fetchStorageEUR(
	ctx context.Context,
	options VertexOptions,
	token string,
) (float64, float64, error) {
	now := options.Now()
	local := now.In(time.Local)
	todayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	weekStart := now.Add(-7 * 24 * time.Hour)
	rate := storageEURPerMillionTokenHour / 1_000_000
	todayEUR, weekEUR := 0.0, 0.0
	for _, location := range options.Locations {
		page := ""
		for count := 0; count < 50; count++ {
			base := options.AIBaseURL
			if base == "" {
				base = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1", location)
			}
			endpoint := strings.TrimRight(base, "/") + "/projects/" +
				url.PathEscape(options.Project) + "/locations/" + url.PathEscape(location) + "/cachedContents"
			if page != "" {
				endpoint += "?" + url.Values{"pageToken": {page}}.Encode()
			}
			var response storageResponse
			requestContext, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := getJSON(requestContext, options.Client, endpoint, token, &response)
			cancel()
			if err != nil {
				return 0, 0, err
			}
			for _, cached := range response.CachedContents {
				if cached.UsageMetadata.TotalTokenCount <= 0 || cached.CreateTime == "" {
					continue
				}
				created, err := time.Parse(time.RFC3339Nano, cached.CreateTime)
				if err != nil {
					return 0, 0, fmt.Errorf("invalid cached content createTime %q: %w", cached.CreateTime, err)
				}
				todayHours := now.Sub(laterTime(created, todayStart)).Hours()
				weekHours := now.Sub(laterTime(created, weekStart)).Hours()
				if todayHours > 0 {
					todayEUR += float64(cached.UsageMetadata.TotalTokenCount) * todayHours * rate
				}
				if weekHours > 0 {
					weekEUR += float64(cached.UsageMetadata.TotalTokenCount) * weekHours * rate
				}
			}
			page = response.NextPageToken
			if page == "" {
				break
			}
		}
	}
	return todayEUR, weekEUR, nil
}

func getJSON(
	ctx context.Context,
	client *http.Client,
	endpoint, token string,
	target any,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("GET %s returned %s: %s", endpoint, response.Status, strings.TrimSpace(string(body)))
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<20))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func rawInt64(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("missing int64Value")
	}
	var text string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	} else {
		text = string(raw)
	}
	return strconv.ParseInt(text, 10, 64)
}

func cacheBucket(label string) string {
	switch label {
	case "cached", "true":
		return "cached"
	case "", "uncached", "not_cached", "false":
		return "fresh"
	default:
		return "unknown"
	}
}

func longestVertexRate(model string, prices map[string][2]float64) ([2]float64, bool) {
	prefixes := make([]string, 0, len(prices))
	for prefix := range prices {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(left, right int) bool { return len(prefixes[left]) > len(prefixes[right]) })
	for _, prefix := range prefixes {
		if strings.HasPrefix(model, prefix) {
			return prices[prefix], true
		}
	}
	return [2]float64{}, false
}

func clonePrices(source map[string][2]float64) map[string][2]float64 {
	clone := make(map[string][2]float64, len(source))
	for model, price := range source {
		clone[model] = price
	}
	return clone
}

func cloneValues(source url.Values) url.Values {
	clone := make(url.Values, len(source))
	for key, values := range source {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

func laterTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

// DefaultVertexPaths returns the cache paths used by the render segment.
func DefaultVertexPaths(cacheDir string) (string, string) {
	return filepath.Join(cacheDir, "cc-vertex-spend"),
		filepath.Join(cacheDir, "cc-vertex-prices-eur.json")
}
