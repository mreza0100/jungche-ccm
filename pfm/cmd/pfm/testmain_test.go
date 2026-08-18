package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"hostops/pfm/internal/harvestpy"
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
	harvestDoctorOverride = noNetworkHarvestDoctor{}
	os.Exit(testjail.Run(m))
}
