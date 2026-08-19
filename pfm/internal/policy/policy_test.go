package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	path := ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestAutonomyDefaultsToPromptedWithNoConfig is the regression guard: absent
// any operator opt-in, autonomy must resolve false. This is the security
// property the whole package exists to enforce.
func TestAutonomyDefaultsToPromptedWithNoConfig(t *testing.T) {
	home := t.TempDir()
	if Autonomy(home) {
		t.Fatal("Autonomy() = true with no env var and no config file")
	}
	if flags := AutonomyFlags(home); flags != nil {
		t.Fatalf("AutonomyFlags() = %v, want nil when off", flags)
	}
}

func TestAutonomyMalformedConfigIsPrompted(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "{not valid json")
	if Autonomy(home) {
		t.Fatal("Autonomy() = true reading a malformed config; a broken file must never fall open")
	}
}

func TestAutonomyConfigMissingFieldIsPrompted(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "{}")
	if Autonomy(home) {
		t.Fatal("Autonomy() = true with the autonomy field absent")
	}
}

func TestAutonomyConfigTrueEnablesBypass(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `{"autonomy": true}`)
	if !Autonomy(home) {
		t.Fatal("Autonomy() = false with config autonomy:true")
	}
	flags := AutonomyFlags(home)
	if len(flags) != len(Flags) {
		t.Fatalf("AutonomyFlags() = %v, want %v", flags, Flags)
	}
}

func TestAutonomyConfigFalseStaysPrompted(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `{"autonomy": false}`)
	if Autonomy(home) {
		t.Fatal("Autonomy() = true with config autonomy:false")
	}
}

// TestAutonomyEnvOverridesConfig proves PFM_AUTONOMY takes precedence over
// the config file in both directions, matching the documented resolution
// order (env, then config, then off).
func TestAutonomyEnvOverridesConfig(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `{"autonomy": false}`)
	t.Setenv(EnvAutonomy, "1")
	if !Autonomy(home) {
		t.Fatal("PFM_AUTONOMY=1 did not override a config file saying false")
	}
	t.Setenv(EnvAutonomy, "0")
	writeConfig(t, home, `{"autonomy": true}`)
	if Autonomy(home) {
		t.Fatal("PFM_AUTONOMY=0 did not override a config file saying true")
	}
}

func TestAutonomyEnvTruthySpellings(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "on", "On", "yes", " yes "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvAutonomy, value)
			if !Autonomy(t.TempDir()) {
				t.Fatalf("PFM_AUTONOMY=%q did not resolve true", value)
			}
		})
	}
}

func TestAutonomyEnvFalsySpellings(t *testing.T) {
	for _, value := range []string{"0", "false", "off", "no", "", "garbage"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(EnvAutonomy, value)
			if Autonomy(t.TempDir()) {
				t.Fatalf("PFM_AUTONOMY=%q resolved true, want false", value)
			}
		})
	}
}

func TestAutonomyFlagsForUserResolvesRealHome(t *testing.T) {
	// Only exercises the no-env, no-config path (a real ~/.config/pfm/config.json
	// may or may not exist on the machine running the tests); the assertion is
	// that it never panics and returns a flags slice or nil, matching Flags.
	flags := AutonomyFlagsForUser()
	if flags != nil && len(flags) != len(Flags) {
		t.Fatalf("AutonomyFlagsForUser() = %v, want nil or %v", flags, Flags)
	}
}

func TestConfigPathIsUnderDotConfigPfm(t *testing.T) {
	home := "/home/tester"
	want := filepath.Join(home, ".config", "pfm", "config.json")
	if got := ConfigPath(home); got != want {
		t.Fatalf("ConfigPath(%q) = %q, want %q", home, got, want)
	}
}
