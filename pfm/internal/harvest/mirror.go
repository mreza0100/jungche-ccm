package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func DOIToPMCID(ctx context.Context, client *http.Client, doi string) (string, error) {
	return idToPMCID(ctx, client, doi)
}
func PMIDToPMCID(ctx context.Context, client *http.Client, pmid string) (string, error) {
	return idToPMCID(ctx, client, pmid)
}
func idToPMCID(ctx context.Context, client *http.Client, id string) (string, error) {
	if client == nil {
		client = safeHTTPClientTimeout(false, 15*time.Second)
	}
	raw := withContact("https://pmc.ncbi.nlm.nih.gov/tools/idconv/api/v1/articles/?ids="+url.QueryEscape(id)+"&format=json&tool=harvester-mcp", "email")
	body, status, _, err := getBody(ctx, client, raw, scholarlyUA(), 1<<20)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("HTTP %d", status)
	}
	var data struct {
		Records []struct {
			PMCID string `json:"pmcid"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	if len(data.Records) == 0 || data.Records[0].PMCID == "" {
		return "", nil
	}
	return data.Records[0].PMCID, nil
}

func EuropePMCPDF(ctx context.Context, client *http.Client, pmcid string) ([]byte, error) {
	if client == nil {
		client = safeHTTPClientTimeout(false, 60*time.Second)
	}
	for _, raw := range []string{"https://europepmc.org/articles/" + url.PathEscape(pmcid) + "?pdf=render", "https://europepmc.org/api/getPdf?pmcid=" + url.QueryEscape(pmcid)} {
		body, status, _, err := getBody(ctx, client, raw, defaultUA, 50*1024*1024)
		if err == nil && status < 400 && strings.HasPrefix(string(body), "%PDF") {
			return body, nil
		}
	}
	return nil, fmt.Errorf("Europe PMC returned no PDF for %s", pmcid)
}
func EuropePMCFulltextXML(ctx context.Context, client *http.Client, pmcid string) (string, error) {
	if client == nil {
		client = safeHTTPClientTimeout(false, 30*time.Second)
	}
	body, status, _, err := getBody(ctx, client, "https://www.ebi.ac.uk/europepmc/webservices/rest/"+url.PathEscape(pmcid)+"/fullTextXML", defaultUA, 50*1024*1024)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("HTTP %d", status)
	}
	return string(body), nil
}
func EuropePMCFiguresURL(pmcid string) string {
	return "https://www.ebi.ac.uk/europepmc/webservices/rest/" + pmcid + "/supplementaryFiles"
}
func PMCArticleURL(pmcid string) string {
	return "https://pmc.ncbi.nlm.nih.gov/articles/" + pmcid + "/"
}
func WaybackRawURL(ctx context.Context, client *http.Client, source string) (string, error) {
	if client == nil {
		client = safeHTTPClientTimeout(false, 15*time.Second)
	}
	body, status, _, err := getBody(ctx, client, "https://archive.org/wayback/available?url="+url.QueryEscape(source), defaultUA, 1<<20)
	if err != nil {
		return "", err
	}
	if status >= 400 {
		return "", fmt.Errorf("HTTP %d", status)
	}
	var data struct {
		Snapshots struct {
			Closest struct {
				Available bool   `json:"available"`
				Timestamp string `json:"timestamp"`
			} `json:"closest"`
		} `json:"archived_snapshots"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	if !data.Snapshots.Closest.Available || data.Snapshots.Closest.Timestamp == "" {
		return "", nil
	}
	return "https://web.archive.org/web/" + data.Snapshots.Closest.Timestamp + "id_/" + source, nil
}
