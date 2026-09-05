package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVSCodeOwnedLegacyAutoOpenProfileUpgradesToPFM(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, "settings.json")
	writeFixture(t, settings, `{
  "terminal.integrated.profiles.linux": {
    "PFM": {"path":"/bin/zsh","args":["-l"],"env":{"CC_AUTO_OPEN":"pfm"}}
  },
  "terminal.integrated.defaultProfile.linux": "PFM"
}`)
	record := vscodeOwnershipDocument{
		Version: vscodeOwnershipVersion,
		Files: []vscodeOwnershipRecord{{
			Path: settings, Platform: "linux", ProfileOwned: true, DefaultOwned: true,
		}},
	}
	ledger, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", vscodeOwnershipName), string(ledger))

	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Runner: &fakeRunner{}, Stdout: &bytes.Buffer{},
		vscodePlatform: "linux", vscodeSettingsPaths: []string{settings},
	}); err != nil {
		t.Fatal(err)
	}
	got := readFixture(t, settings)
	if !strings.Contains(got, `"PFM_AUTO_OPEN": "pfm"`) || strings.Contains(got, "CC_AUTO_OPEN") {
		t.Fatalf("owned legacy PFM profile was not upgraded exactly:\n%s", got)
	}
}

func TestVSCodeCustomizedLegacyAutoOpenProfileIsPreserved(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, "settings.json")
	original := `{"terminal.integrated.profiles.linux":{"PFM":{"path":"/bin/zsh","args":["-l"],"env":{"CC_AUTO_OPEN":"operator-choice"}}}}`
	writeFixture(t, settings, original)
	record := vscodeOwnershipDocument{
		Version: vscodeOwnershipVersion,
		Files:   []vscodeOwnershipRecord{{Path: settings, Platform: "linux", ProfileOwned: true}},
	}
	ledger, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", vscodeOwnershipName), string(ledger))

	if _, err := Run(context.Background(), Options{
		Mode: ModeApply, Home: home, Runner: &fakeRunner{}, Stdout: &bytes.Buffer{},
		vscodePlatform: "linux", vscodeSettingsPaths: []string{settings},
	}); err != nil {
		t.Fatal(err)
	}
	got := readFixture(t, settings)
	if !strings.Contains(got, `"CC_AUTO_OPEN":"operator-choice"`) || strings.Contains(got, "PFM_AUTO_OPEN") {
		t.Fatalf("customized profile was overwritten:\n%s", got)
	}
	ownershipPath := filepath.Join(home, ".local", "share", "pfm", "install", vscodeOwnershipName)
	owned, err := os.ReadFile(ownershipPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var updated vscodeOwnershipDocument
	if err := json.Unmarshal(owned, &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Files) != 1 || updated.Files[0].ProfileOwned {
		t.Fatalf("customized profile ownership was not relinquished: %#v", updated)
	}
}

func TestVSCodeUninstallRemovesOwnedLegacyProfile(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, "settings.json")
	writeFixture(t, settings, `{"terminal.integrated.profiles.linux":{"zsh":{"path":"/bin/zsh"},"PFM":{"path":"/bin/zsh","args":["-l"],"env":{"CC_AUTO_OPEN":"pfm"}}}}`)
	writeVSCodeOwnershipFixture(t, home, vscodeOwnershipRecord{
		Path: settings, Platform: "linux", ProfileOwned: true,
	})

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, Runner: &fakeRunner{}, Stdout: &bytes.Buffer{},
		vscodePlatform: "linux", vscodeSettingsPaths: []string{settings},
	}); err != nil {
		t.Fatal(err)
	}
	got := readFixture(t, settings)
	if strings.Contains(got, `"PFM"`) || !strings.Contains(got, `"zsh"`) {
		t.Fatalf("uninstall did not selectively remove the exact owned legacy profile:\n%s", got)
	}
}

func TestVSCodeUninstallPreservesEditedLegacyProfile(t *testing.T) {
	home := t.TempDir()
	settings := filepath.Join(home, "settings.json")
	original := `{"terminal.integrated.profiles.linux":{"PFM":{"path":"/operator/zsh","args":["-l"],"env":{"CC_AUTO_OPEN":"pfm"}}}}`
	writeFixture(t, settings, original)
	writeVSCodeOwnershipFixture(t, home, vscodeOwnershipRecord{
		Path: settings, Platform: "linux", ProfileOwned: true,
	})

	if _, err := Run(context.Background(), Options{
		Mode: ModeUninstall, Home: home, Runner: &fakeRunner{}, Stdout: &bytes.Buffer{},
		vscodePlatform: "linux", vscodeSettingsPaths: []string{settings},
	}); err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, settings); got != original {
		t.Fatalf("uninstall changed the operator-edited legacy profile:\n%s", got)
	}
}

func writeVSCodeOwnershipFixture(t *testing.T, home string, record vscodeOwnershipRecord) {
	t.Helper()
	ledger, err := json.Marshal(vscodeOwnershipDocument{
		Version: vscodeOwnershipVersion,
		Files:   []vscodeOwnershipRecord{record},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(home, ".local", "share", "pfm", "install", vscodeOwnershipName), string(ledger))
}
