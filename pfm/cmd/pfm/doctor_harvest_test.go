package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/harvestpy"
)

type harvestDoctorFake struct {
	digest   harvestpy.EnvironmentDigest
	inspect  error
	check    harvestpy.CheckReport
	checkErr error
}

func (fake harvestDoctorFake) Inspect(root string, platform harvestpy.Platform) (harvestpy.EnvironmentDigest, error) {
	return fake.digest, fake.inspect
}

func (fake harvestDoctorFake) Check(ctx context.Context, root string, platform harvestpy.Platform) (harvestpy.CheckReport, error) {
	return fake.check, fake.checkErr
}

func doctorHarvestDigest() harvestpy.EnvironmentDigest {
	return harvestpy.EnvironmentDigest{
		Schema:          1,
		Target:          "linux-amd64",
		Python:          "3.11.15+20260610",
		LockSHA256:      "lock-digest",
		InventorySHA256: "inventory-digest",
		InventoryCount:  187,
		State:           "ready",
	}
}

func TestDoctorHarvestReportsPinnedInterpreterLockInventoryAndLiveSmokeHealthy(t *testing.T) {
	digest := doctorHarvestDigest()
	fake := harvestDoctorFake{
		digest: digest,
		check: harvestpy.CheckReport{
			Healthy: true,
			Digest:  digest,
			Checks: map[string]harvestpy.CheckStatus{
				"interpreter":           {OK: true},
				"lock_completeness":     {OK: true},
				"dependency_check":      {OK: true},
				"inventory":             {OK: true},
				"live_smoke":            {OK: true},
				"live_smoke_conversion": {OK: true},
			},
		},
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local", "state", "pfm", "harvest-python"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	warnings := printHarvestPythonDoctor(context.Background(), &output, home, harvestpy.Platform{GOOS: "linux", GOARCH: "amd64"}, fake)
	if warnings != 0 {
		t.Fatalf("healthy doctor warnings=%d, want 0\n%s", warnings, output.String())
	}
	for _, want := range []string{
		"doctor: harvestpy interpreter=(file)",
		"3.11.15+20260610",
		"doctor: harvestpy lock=(file) complete",
		"lock-digest",
		"doctor: harvestpy inventory=(file) complete count=187",
		"inventory-digest",
		"doctor: harvestpy live_smoke=(file) healthy",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("healthy doctor output missing %q:\n%s", want, output.String())
		}
	}
}

func TestDoctorHarvestDistinguishesBrokenEnvironmentAndSmoke(t *testing.T) {
	digest := doctorHarvestDigest()
	fake := harvestDoctorFake{
		digest: digest,
		check: harvestpy.CheckReport{
			Healthy: false,
			Digest:  digest,
			Checks: map[string]harvestpy.CheckStatus{
				"lock_completeness": {Error: "installed inventory differs from provisioned lock"},
				"live_smoke":        {Error: "harvestpy smoke subprocess: interpreter missing"},
			},
		},
		checkErr: errors.New("harvestpy environment check failed"),
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local", "state", "pfm", "harvest-python"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	warnings := printHarvestPythonDoctor(context.Background(), &output, home, harvestpy.Platform{GOOS: "linux", GOARCH: "amd64"}, fake)
	if warnings == 0 {
		t.Fatalf("broken doctor warnings=%d, want nonzero\n%s", warnings, output.String())
	}
	text := output.String()
	// digest.State is "ready" here (doctorHarvestDigest): the lock check
	// failed against a provision that DID finish, so the failure word is
	// "broken" — "incomplete" is reserved for digest.State == "incomplete".
	for _, want := range []string{
		"doctor: harvestpy lock=(file) broken error=installed inventory differs from provisioned lock",
		"doctor: harvestpy live_smoke=(file) broken error=harvestpy smoke subprocess: interpreter missing",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("broken doctor output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "live_smoke=(file) healthy") || strings.Contains(text, "lock=(file) complete") {
		t.Fatalf("broken doctor reused healthy rendering:\n%s", text)
	}
	if strings.Contains(text, "lock=(file) incomplete") {
		t.Fatalf("broken doctor rendered the interrupted-provision word for a State==ready digest:\n%s", text)
	}
}

func TestDoctorHarvestLockIncompleteReflectsInterruptedProvisionState(t *testing.T) {
	digest := doctorHarvestDigest()
	digest.State = "incomplete"
	digest.LockSHA256 = ""
	fake := harvestDoctorFake{
		digest: digest,
		check: harvestpy.CheckReport{
			Healthy: false,
			Digest:  digest,
			Checks: map[string]harvestpy.CheckStatus{
				"lock_completeness": {Error: "provision interrupted: lock file missing"},
			},
		},
		checkErr: errors.New("harvestpy environment check failed"),
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local", "state", "pfm", "harvest-python"), 0o700); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	warnings := printHarvestPythonDoctor(context.Background(), &output, home, harvestpy.Platform{GOOS: "linux", GOARCH: "amd64"}, fake)
	if warnings == 0 {
		t.Fatalf("interrupted-provision doctor warnings=%d, want nonzero\n%s", warnings, output.String())
	}
	text := output.String()
	if want := "doctor: harvestpy lock=(file) incomplete error=provision interrupted: lock file missing"; !strings.Contains(text, want) {
		t.Fatalf("interrupted-provision doctor output missing %q:\n%s", want, text)
	}
	if strings.Contains(text, "lock=(file) broken") {
		t.Fatalf("interrupted-provision doctor rendered the finished-provision word for a State==incomplete digest:\n%s", text)
	}
}

func TestDoctorHarvestMissingRootIsSkipped(t *testing.T) {
	var output strings.Builder
	warnings := printHarvestPythonDoctor(
		context.Background(),
		&output,
		t.TempDir(),
		harvestpy.Platform{GOOS: "linux", GOARCH: "amd64"},
		harvestDoctorFake{
			inspect:  errors.New("harvestpy root is absent"),
			checkErr: errors.New("harvestpy root is absent"),
		},
	)
	if warnings != 0 {
		t.Fatalf("missing harvest root warnings=%d, want 0\n%s", warnings, output.String())
	}
	if got, want := output.String(), "doctor: harvestpy skipped\n"; got != want {
		t.Fatalf("missing harvest root output=%q, want %q", got, want)
	}
}

func TestDoctorHarvestUnreadableRootIsNotSkipped(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, ".local")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("state", filepath.Join(local, "state")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	warnings := printHarvestPythonDoctor(
		context.Background(),
		&output,
		home,
		harvestpy.Platform{GOOS: "linux", GOARCH: "amd64"},
		harvestDoctorFake{
			inspect:  os.ErrPermission,
			checkErr: os.ErrPermission,
		},
	)
	if warnings == 0 {
		t.Fatalf("unreadable harvest root warnings=0, want a visible probe failure\n%s", output.String())
	}
	if strings.Contains(output.String(), "doctor: harvestpy skipped") {
		t.Fatalf("unreadable harvest root rendered as skipped:\n%s", output.String())
	}
}
