package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/harvestpy"
	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/testjail"
)

type noNetworkHarvestProvisioner struct{}

func (noNetworkHarvestProvisioner) Plan(platform harvestpy.Platform) (harvestpy.InstallPlan, error) {
	return harvestpy.Plan(platform)
}

func (noNetworkHarvestProvisioner) Check(context.Context, string, harvestpy.Platform) (harvestpy.CheckReport, error) {
	return harvestpy.CheckReport{Healthy: true}, nil
}

func (noNetworkHarvestProvisioner) Provision(context.Context, harvestpy.ProvisionOptions) (harvestpy.ProvisionResult, error) {
	return harvestpy.ProvisionResult{}, errors.New("test fake must not provision the pinned runtime")
}

type noNetworkHarvestDoctor struct{}

type noNetworkThemeTransport struct{}

func (noNetworkThemeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"name":"Tokyo Night","fixture":true}`)),
	}, nil
}

func (noNetworkHarvestDoctor) Inspect(string, harvestpy.Platform) (harvestpy.EnvironmentDigest, error) {
	return noNetworkHarvestDigest(), nil
}

func (noNetworkHarvestDoctor) Check(context.Context, string, harvestpy.Platform) (harvestpy.CheckReport, error) {
	digest := noNetworkHarvestDigest()
	return harvestpy.CheckReport{
		Healthy: true,
		Digest:  digest,
		Checks: map[string]harvestpy.CheckStatus{
			"interpreter":           {OK: true},
			"lock_completeness":     {OK: true},
			"live_smoke":            {OK: true},
			"live_smoke_conversion": {OK: true},
		},
	}, nil
}

func noNetworkHarvestDigest() harvestpy.EnvironmentDigest {
	return harvestpy.EnvironmentDigest{
		Schema:          1,
		Python:          "3.11.15+20260610",
		LockSHA256:      "test-lock-digest",
		InventorySHA256: "test-inventory-digest",
		InventoryCount:  1,
	}
}

// TestMain gives this package a short, canonical TMPDIR before any test builds
// a path from it. See internal/testjail for why both properties matter.
func TestMain(m *testing.M) {
	installHarvestProvisionerOverride = noNetworkHarvestProvisioner{}
	installThemeHTTPClientOverride = &http.Client{Transport: noNetworkThemeTransport{}}
	harvestDoctorOverride = noNetworkHarvestDoctor{}
	prePushGateProbeOverride = func(context.Context) prePushGate {
		return prePushGate{State: "outside-repository"}
	}
	dependencyProbeOverride = func(_ context.Context, entries []deps.Entry, _ deps.ProbeOptions) []deps.Result {
		results := make([]deps.Result, 0, len(entries))
		for _, entry := range entries {
			if !entry.AppliesTo(runtime.GOOS) {
				results = append(results, deps.Result{Entry: entry, State: deps.StateSkipped, Error: "not this platform"})
				continue
			}
			results = append(results, deps.Result{Entry: entry, State: deps.StateOK, Path: "/test/bin/" + entry.Name, Version: entry.MinVersion})
		}
		return results
	}
	hookProbeOverride = func(home string, machine pfmconfig.Config) []installer.HookProbeResult {
		expected := installer.ExpectedHooks(home, machine)
		results := make([]installer.HookProbeResult, 0, len(expected))
		for _, hook := range expected {
			results = append(results, installer.HookProbeResult{Hook: hook, State: "ok"})
		}
		return results
	}
	os.Exit(testjail.Run(m))
}
