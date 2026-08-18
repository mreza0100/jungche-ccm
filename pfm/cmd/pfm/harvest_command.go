package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"hostops/pfm/internal/harvest"
	"hostops/pfm/internal/harvestmcp"
)

func runHarvesterMCP(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("mcp harvester serve", "usage: pfm mcp harvester serve [--transport stdio|http] [--user-agent UA] [--proxy-url URL] [--ignore-robots-txt]", stderr)
	transport := flags.String("transport", "stdio", "MCP transport (stdio or http)")
	userAgent := flags.String("user-agent", "", "override the autonomous fetch User-Agent")
	proxyURL := flags.String("proxy-url", "", "HTTP proxy URL")
	ignoreRobots := false
	flags.Var(&presenceFlag{value: &ignoreRobots}, "ignore-robots-txt", "compatibility flag; Harvester does not consult robots.txt")
	host := flags.String("host", "127.0.0.1", "HTTP listen host")
	port := flags.Int("port", 8081, "HTTP listen port")
	publicURL := flags.String("public-url", "", "public URL")
	internalHost := flags.String("internal-host", "127.0.0.1", "internal host")
	internalPort := flags.Int("internal-port", 8082, "internal port (zero disables the second gateway)")
	allowUnauthenticated := flags.Bool("allow-unauthenticated", false, "allow an unauthenticated non-loopback bind")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if *transport != "stdio" && *transport != "http" {
		fmt.Fprintf(stderr, "pfm mcp harvester: unsupported transport %q (want stdio or http)\n", *transport)
		return 2
	}
	configured := harvestRuntime(runtime)
	configured.UserAgent = *userAgent
	configured.ProxyURL = *proxyURL
	if *transport == "http" {
		passphrase := os.Getenv("HARVESTER_AUTH_PASSPHRASE")
		staticToken := os.Getenv("HARVESTER_STATIC_TOKEN")
		if err := harvestmcp.ValidateRemoteBind(*host, passphrase != "" || staticToken != "", *allowUnauthenticated); err != nil {
			fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
			return 2
		}
		if *publicURL == "" {
			*publicURL = fmt.Sprintf("http://%s:%d", *host, *port)
		}
		remote, err := harvestmcp.NewRemote(harvestmcp.RemoteOptions{Runtime: configured, Version: version, PublicURL: *publicURL, Passphrase: passphrase, StaticToken: staticToken, ConfineReads: true})
		if err != nil {
			fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
			return 1
		}
		// An internal listener is only meaningful for an authenticated external
		// gateway. It deliberately shares the service but has no bearer wall.
		if *internalPort != 0 && (passphrase != "" || staticToken != "") {
			internalURL := fmt.Sprintf("http://%s:%d", *internalHost, *internalPort)
			internal, err := remote.NewInternalRemote(internalURL)
			if err != nil {
				_ = remote.Close()
				fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
				return 1
			}
			if err := harvestmcp.ValidateRemoteBind(*internalHost, false, *allowUnauthenticated); err != nil {
				_ = remote.Close()
				fmt.Fprintf(stderr, "pfm mcp harvester: internal gateway: %v\n", err)
				return 2
			}
			return harvestmcp.ServeRemotePair(context.Background(), remote, *host, *port, internal, *internalHost, *internalPort, *allowUnauthenticated)
		}
		if err := remote.ListenAndServe(context.Background(), *host, *port); err != nil {
			fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
			return 1
		}
		return 0
	}
	service, err := harvestmcp.NewConfigured(version, configured)
	if err != nil {
		fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := service.Close(); closeErr != nil {
			fmt.Fprintf(stderr, "pfm mcp harvester: close converter: %v\n", closeErr)
		}
	}()
	if err := service.RunStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(stderr, "pfm mcp harvester: %v\n", err)
		return 1
	}
	return 0
}

// presenceFlag matches Python argparse's store_true contract: the flag is
// either absent or present, never an assignment accepting false.
type presenceFlag struct{ value *bool }

func (flag *presenceFlag) String() string {
	return strconv.FormatBool(flag != nil && flag.value != nil && *flag.value)
}
func (flag *presenceFlag) Set(value string) error {
	if value != "true" {
		return fmt.Errorf("-%s does not accept a value", "ignore-robots-txt")
	}
	*flag.value = true
	return nil
}
func (*presenceFlag) IsBoolFlag() bool { return true }

// runHarvest is the command-line face of the same Harvester core served over
// MCP. Sources remain ordered: each result is printed in the order supplied.
func runHarvest(args []string, stdout, stderr io.Writer, runtime commandRuntime) int {
	flags := newFlagSet("harvest", "usage: pfm harvest [--refresh] [--size-only] [--json] <url|doi|path>...", stderr)
	refresh := flags.Bool("refresh", false, "bypass the cache and fetch fresh content")
	sizeOnly := flags.Bool("size-only", false, "fetch and cache content but print only size and cache path")
	jsonOutput := flags.Bool("json", false, "print machine-readable result objects")
	sources, code, ok := parseFlagsAnywhere(flags, args)
	if !ok {
		return code
	}
	if len(sources) == 0 || len(sources) > 50 {
		flags.Usage()
		return 2
	}
	configured := harvestRuntime(runtime)
	harvester, err := harvestmcp.NewHarvester(configured)
	if err != nil {
		fmt.Fprintf(stderr, "pfm harvest: configure: %v\n", err)
		return 1
	}
	results := make([]harvest.Result, 0, len(sources))
	for _, source := range sources {
		result := harvester.FetchWithOptions(context.Background(), source, harvest.FetchOptions{Refresh: *refresh, SizeOnly: *sizeOnly})
		results = append(results, result)
		if !*jsonOutput {
			fmt.Fprintln(stdout, renderHarvestCLI(result, *sizeOnly))
		}
	}
	if *jsonOutput {
		encoded, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "pfm harvest: encode results: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(encoded))
	}
	for _, result := range results {
		if result.Error != "" {
			return 1
		}
	}
	return 0
}

func harvestRuntime(runtime commandRuntime) harvestmcp.Runtime {
	return harvestmcp.Runtime{
		Home: runtime.Paths.Home,
		// The local CLI and stdio MCP preserve the oracle's unconfined local
		// read surface, subject to harvest.DenyLocalPath. Remote construction
		// supplies an explicit cache root instead.
	}
}

func renderHarvestCLI(result harvest.Result, sizeOnly bool) string {
	if result.Error != "" {
		return fmt.Sprintf("# %s\nERROR: %s", result.Source, result.Error)
	}
	if sizeOnly {
		return fmt.Sprintf("source: %s\nsize: %d tokens / chars: %d / path: %s / cache_status: %s", result.Source, result.Tokens, result.Chars, result.Path, result.CacheStatus)
	}
	return fmt.Sprintf("# %s\ncache_status: %s / method: %s / bytes: %d / tokens: %d / path: %s\n\n%s", result.Source, result.CacheStatus, result.Method, result.Bytes, result.Tokens, result.Path, strings.TrimSpace(result.Content))
}
