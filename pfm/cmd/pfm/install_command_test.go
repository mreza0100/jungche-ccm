package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/pfm/internal/installer"
	"hostops/pfm/internal/paths"
)

func TestInstallGateScopesDryRunIdleAndRunningService(t *testing.T) {
	t.Run("bare preview ignores reachable manager", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))
		var stdout, stderr bytes.Buffer
		if code := runInstall(nil, &stdout, &stderr); code != 0 {
			t.Fatalf("preview code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
			t.Fatalf("preview wrote files: entries=%v err=%v", entries, err)
		}
		const confirmation = "if you agree, run again: pfm install --yes\n"
		if !strings.HasSuffix(stdout.String(), confirmation) || strings.Count(stdout.String(), confirmation) != 1 {
			t.Fatalf("preview confirmation=%q, want one final line %q", stdout.String(), confirmation)
		}
	})

	t.Run("idle reachable manager applies with yes", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		script := "#!/bin/sh\nif [ \"$*\" = \"--user is-active --quiet pfm-name-sync.service\" ]; then exit 1; fi\nexit 0\n"
		if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))
		var stdout, stderr bytes.Buffer
		if code := runInstall([]string{"--yes"}, &stdout, &stderr); code != 0 {
			t.Fatalf("idle yes code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("running service refuses actionably", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(t.TempDir(), "systemctl")
		if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", home)
		t.Setenv("PATH", filepath.Dir(bin))

		var stdout, stderr bytes.Buffer
		if code := runInstall([]string{"--yes"}, &stdout, &stderr); code != 97 {
			t.Fatalf("runInstall() code=%d, want 97; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "systemctl --user stop pfm-name-sync.service") {
			t.Fatalf("stderr=%q, want actionable running-service refusal", stderr.String())
		}
		entries, err := os.ReadDir(home)
		if err != nil || len(entries) != 0 {
			t.Fatalf("rc 97 refusal wrote files: entries=%v err=%v", entries, err)
		}
	})
}

func TestInstallUsesOnlyTheNewSurface(t *testing.T) {
	for _, retired := range []string{"-" + "-apply", "-" + "-uninstall", "-" + "-dry-run"} {
		t.Run(retired, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			var stdout, stderr bytes.Buffer
			if code := runInstall([]string{retired}, &stdout, &stderr); code != 2 {
				t.Fatalf("runInstall(%q) code=%d stdout=%q stderr=%q, want unknown-flag usage", retired, code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "usage: pfm install [--yes] [--force] [--config-dir DIR]") {
				t.Fatalf("runInstall(%q) stderr=%q, want new usage", retired, stderr.String())
			}
		})
	}
}

func TestInstallPreviewAndYesUseTheSameInstallerClassification(t *testing.T) {
	previous := runInstaller
	t.Cleanup(func() { runInstaller = previous })
	var calls []installer.Options
	runInstaller = func(_ context.Context, options installer.Options) (installer.Report, error) {
		calls = append(calls, options)
		return installer.Report{}, nil
	}
	runtime := commandRuntime{Paths: paths.Values{Home: t.TempDir()}}
	configDir := filepath.Join(t.TempDir(), "config")
	var previewOut, previewErr bytes.Buffer
	if code := runInstall([]string{"--force", "--config-dir", configDir}, &previewOut, &previewErr, runtime); code != 0 {
		t.Fatalf("preview code=%d stdout=%q stderr=%q", code, previewOut.String(), previewErr.String())
	}
	var applyOut, applyErr bytes.Buffer
	if code := runInstall([]string{"--yes", "--force", "--config-dir", configDir}, &applyOut, &applyErr, runtime); code != 0 {
		t.Fatalf("yes code=%d stdout=%q stderr=%q", code, applyOut.String(), applyErr.String())
	}
	if len(calls) != 2 {
		t.Fatalf("installer calls=%d, want preview and yes", len(calls))
	}
	if calls[0].Mode != installer.ModeDryRun || calls[1].Mode != installer.ModeApply {
		t.Fatalf("installer modes=%v/%v, want dry-run/apply", calls[0].Mode, calls[1].Mode)
	}
	preview, apply := calls[0], calls[1]
	preview.Mode = installer.ModeApply
	preview.Stdout = nil
	apply.Stdout = nil
	if !reflect.DeepEqual(preview, apply) {
		t.Fatalf("preview and yes options classify differently:\npreview=%#v\nyes=%#v", preview, apply)
	}
	if got := previewOut.String(); !strings.HasSuffix(got, "if you agree, run again: pfm install --yes\n") {
		t.Fatalf("preview output=%q, want exact confirmation suffix", got)
	}
	if strings.Contains(applyOut.String(), "if you agree, run again:") {
		t.Fatalf("yes output unexpectedly contains preview confirmation: %q", applyOut.String())
	}
}

func TestUninstallVerbAcceptsConfigDirAndUsesUninstallMode(t *testing.T) {
	previous := runInstaller
	t.Cleanup(func() { runInstaller = previous })
	var got installer.Options
	runInstaller = func(_ context.Context, options installer.Options) (installer.Report, error) {
		got = options
		return installer.Report{}, nil
	}
	configDir := filepath.Join(t.TempDir(), "config")
	runtime := commandRuntime{Paths: paths.Values{Home: t.TempDir()}}
	var stdout, stderr bytes.Buffer
	if code := runUninstall([]string{"--config-dir", configDir}, &stdout, &stderr, runtime); code != 0 {
		t.Fatalf("uninstall code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got.Mode != installer.ModeUninstall || got.ConfigDir != configDir || got.Home != runtime.Paths.Home {
		t.Fatalf("uninstall options=%#v, want mode uninstall, config %q, home %q", got, configDir, runtime.Paths.Home)
	}
}

func TestRootHelpListsUninstall(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "  uninstall") {
		t.Fatalf("help=%q, want top-level uninstall", stdout.String())
	}
}
