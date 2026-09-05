package codexlaunch

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConfigOptionsPreservesLayersAndLaunchDirectory(t *testing.T) {
	for _, tc := range []struct {
		args, want []string
		cwd        string
	}{
		{[]string{"resume", "id", "-c", "a=1", "--config=b=2", "--profile=work", "--enable", "feature", "--strict-config", "-C", "project", "--", "-c", "ignored"}, []string{"-c", "a=1", "--config", "b=2", "--profile", "work", "--enable", "feature", "--strict-config"}, "/start/project"},
		{[]string{"exec", "-cdeveloper_instructions='personal'", "-pwork", "-C/project", "-"}, []string{"-c", "developer_instructions='personal'", "-p", "work"}, "/project"},
	} {
		got, cwd, err := configOptions(tc.args, "/start")
		if err != nil || !reflect.DeepEqual(got, tc.want) || cwd != tc.cwd {
			t.Fatalf("got %q %q %v, want %q %q", got, cwd, err, tc.want, tc.cwd)
		}
	}
	if _, _, err := configOptions([]string{"--config"}, "/start"); err == nil {
		t.Fatal("accepted missing value")
	}
}

func TestReadDeveloperUsesConfigRPCWithoutStartingModel(t *testing.T) {
	for _, tc := range []struct{ name, response, want, failure string }{
		{"personal", `{"id":1,"result":{"config":{"developer_instructions":"personal rules"}}}`, "personal rules", ""},
		{"absent", `{"id":1,"result":{"config":{}}}`, "", ""},
		{"rejected", `{"id":1,"error":{"message":"bad config"}}`, "", "bad config"},
		{"missing", `{"id":1,"result":{}}`, "", "no config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "fake-engine")
			script := "#!/bin/sh\n[ \"$1\" = --strict-config ] && [ \"$2\" = app-server ] || exit 91\n"
			for _, method := range []string{"initialize", "initialized", "config/read"} {
				script += "IFS= read -r request\ncase \"$request\" in *'\"method\":\"" + method + "\"'*) ;; *) exit 92;; esac\n"
			}
			script += "printf '%s\\n' '" + tc.response + "'\n"
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			got, err := ReadDeveloper(context.Background(), binary, []string{"exec", "--strict-config", "-"})
			if tc.failure != "" {
				if err == nil || !strings.Contains(err.Error(), tc.failure) {
					t.Fatalf("error = %v", err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got %q %v", got, err)
			}
		})
	}
}
func TestReadDeveloperHonorsCancellation(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "fake-engine")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := ReadDeveloper(ctx, binary, nil); err == nil {
		t.Fatal("cancelled config read succeeded")
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancellation did not stop reader")
	}
}

func TestReaderHomeKeepsProfileLayersAndOriginalAccount(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "account")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	base := "developer_instructions='base'\n[projects.'/work']\ntrust_level='trusted'\n"
	for name, raw := range map[string]string{"config.toml": base, "work.config.toml": "developer_instructions='profile'\nmodel_instructions_file='../base.md'\n", "auth.json": "fixture auth"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, ignore := range []bool{false, true} {
		options := []string{"-p", "work", "-c", "developer_instructions='CLI'"}
		if ignore {
			options = append(options, "--ignore-user-config")
		}
		got, queryHome, cleanup, err := readerHome(options)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"-c", "developer_instructions='CLI'"}) || filepath.Dir(queryHome) != parent {
			t.Fatalf("bad query home/options: %q %q", queryHome, got)
		}
		document := map[string]any{}
		if err := readConfigFile(filepath.Join(queryHome, "config.toml"), document); err != nil {
			t.Fatal(err)
		}
		if ignore {
			if len(document) != 0 {
				t.Fatal("ignored user config was loaded")
			}
		} else if document["developer_instructions"] != "profile" || document["projects"] == nil {
			t.Fatalf("profile merge: %#v", document)
		}
		auth, err := os.ReadFile(filepath.Join(queryHome, "auth.json"))
		if err != nil || string(auth) != "fixture auth" {
			t.Fatal("account identity changed")
		}
		cleanup()
		if _, err := os.Stat(queryHome); !os.IsNotExist(err) {
			t.Fatal("query home retained")
		}
	}
	original, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil || string(original) != base || os.Getenv("CODEX_HOME") != home {
		t.Fatal("original account mutated")
	}
}

func TestConfigOptionsShortEquals(t *testing.T) {
	got, cwd, err := configOptions([]string{"-c=developer_instructions='personal'", "-p=work", "-C=/project"}, "/start")
	want := []string{"-c", "developer_instructions='personal'", "-p", "work"}
	if err != nil || cwd != "/project" || !reflect.DeepEqual(got, want) {
		t.Fatalf("%q %q %v", got, cwd, err)
	}
}

func TestReaderHomeCanonicalizesAccountSymlink(t *testing.T) {
	parent := t.TempDir()
	realHome := filepath.Join(parent, "real", "account")
	if err := os.MkdirAll(realHome, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(realHome, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", alias)
	_, query, cleanup, err := readerHome([]string{"--ignore-user-config"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if filepath.Dir(query) != filepath.Dir(realHome) {
		t.Fatalf("query home %s is not beside canonical account", query)
	}
}
func TestReadDeveloperRefusesOverlayKeyringIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	binary := filepath.Join(home, "fake-engine")
	script := "#!/bin/sh\nIFS= read -r request\nIFS= read -r request\nIFS= read -r request\nprintf '%s\\n' '{\"id\":1,\"result\":{\"config\":{\"cli_auth_credentials_store\":\"keyring\",\"developer_instructions\":\"wrong identity\"}}}'\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDeveloper(context.Background(), binary, []string{"exec", "--ignore-user-config"}); err == nil || !strings.Contains(err.Error(), "keyring identity") {
		t.Fatalf("unsupported credential mode not reported: %v", err)
	}
}
