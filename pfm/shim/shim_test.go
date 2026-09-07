// Package shim is a TEST HOME, not a source directory: it holds no shim of its
// own. Tests always read the installer asset embedded into the pfm binary.
package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"hostops/pfm/internal/testjail"
)

func TestShimSyntaxAndResource(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)
	if output, err := exec.Command(zsh, "-n", shimPath).CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v: %s", err, output)
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeShimFile(t, filepath.Join(binDir, "pfm"), "#!/bin/sh\nprintf 'pfm=%s\\n' \"$*\"\n")
	script := "source " + quoteZsh(shimPath) + "\n" +
		"source " + quoteZsh(shimPath) + "\n" +
		`"$HOME/.local/bin/pfm" --version` + "\n"
	command := exec.Command(zsh, "-c", script)
	command.Env = append(os.Environ(), "HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "pfm=--version\n" {
		t.Fatalf("source shim twice: err=%v output=%q", err, output)
	}
}

func TestShimCanBeResourcedWithForeignReadOnlyPFMBinParameter(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeShimFile(t, filepath.Join(binDir, "pfm"), "#!/bin/sh\nprintf 'shim-ok\\n'\n")
	shimPath := embeddedShimPath(t)
	script := `typeset -gr _PFM_BIN=/foreign/read-only/value
source ` + quoteZsh(shimPath) + `
source ` + quoteZsh(shimPath) + `
"$HOME/.local/bin/pfm"
`
	command := exec.Command(zsh, "-c", script)
	command.Env = append(os.Environ(), "HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "shim-ok\n" {
		t.Fatalf("resource shim with foreign read-only parameter: err=%v output=%q", err, output)
	}
}

// A Codex fleet chat owns a bare terminal. Replacing that shell with the tmux
// client makes terminal teardown structural: when the harness closes the
// server, no parent shell remains to reveal an accidental prompt.
func TestBareCodexLaunchExecsTheTerminalOwner(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script is not installed")
	}
	home := t.TempDir()
	fakeBin := filepath.Join(home, "fake-bin")
	if err := os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeShimFile(t, filepath.Join(home, ".local", "bin", "pfm"), "#!/bin/sh\nprintf '1\\n'\n")
	writeShimFile(t, filepath.Join(fakeBin, "tmux"), `#!/bin/sh
case " $* " in *" attach "*) printf '%s\n' "$$" > "$SHIM_TMUX_PID" ;; esac
`)
	shellPID := filepath.Join(home, "shell.pid")
	tmuxPID := filepath.Join(home, "tmux.pid")
	driver := filepath.Join(home, "driver.zsh")
	writeShimFile(t, driver,
		"source "+quoteZsh(embeddedShimPath(t))+"\n"+
			"print -r -- \"$$\" > \"$SHIM_SHELL_PID\"\n"+
			"cx\n",
	)
	command := testjail.PTYCommand(zsh, "-fi", driver)
	command.Env = append(
		os.Environ(),
		"HOME="+home,
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"TMUX=",
		"SHIM_SHELL_PID="+shellPID,
		"SHIM_TMUX_PID="+tmuxPID,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("drive interactive shim: %v: %s", err, output)
	}
	want := strings.TrimSpace(readShimFile(t, shellPID))
	got := strings.TrimSpace(readShimFile(t, tmuxPID))
	if got != want {
		t.Fatalf("tmux pid=%s, shell pid=%s: Codex launch forked and left an outer terminal shell", got, want)
	}
}

func TestShimAutoOpenDefersDisarmsAndMapsLegacyValuesToPicker(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)
	home := t.TempDir()
	fakeBin := filepath.Join(home, "fake-bin")
	for _, directory := range []string{filepath.Join(home, ".local", "bin"), fakeBin} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeShimFile(t, filepath.Join(home, ".local", "bin", "pfm"), `#!/bin/sh
if [ "$#" -eq 0 ]; then printf 'picker\n' >> "$SHIM_AUTO_LOG"; fi
`)
	writeShimFile(t, filepath.Join(fakeBin, "tmux"), `#!/bin/sh
printf 'launch %s\n' "$*" >> "$SHIM_AUTO_LOG"
`)
	writeShimFile(t, filepath.Join(fakeBin, "rm"), `#!/bin/sh
printf 'EVALUATED %s\n' "$*" >> "$SHIM_AUTO_LOG"
`)

	cases := []struct {
		name    string
		environ []string
		want    []string
		absent  []string
	}{
		{name: "managed profile", environ: []string{"PFM_AUTO_OPEN=pfm"}, want: []string{"picker"}},
		{name: "legacy managed profile", environ: []string{"CC_AUTO_OPEN=pfm"}, want: []string{"picker"}},
		{name: "legacy truthy", environ: []string{"CC_AUTO_OPEN=1"}, want: []string{"picker"}},
		{name: "retired cc", environ: []string{"CC_AUTO_OPEN=cc"}, want: []string{"picker"}, absent: []string{"launch "}},
		{name: "retired cc2", environ: []string{"CC_AUTO_OPEN=cc2"}, want: []string{"picker"}, absent: []string{"launch "}},
		{name: "retired VS Code spelling", environ: []string{"VSCODE_AUTO_CC=1"}, want: []string{"picker"}},
		{name: "Codex survives", environ: []string{"PFM_AUTO_OPEN=cx"}, want: []string{"launch ", "codex"}},
		{name: "unknown is never evaluated", environ: []string{"PFM_AUTO_OPEN=rm -rf /"}, want: []string{"picker"}, absent: []string{"EVALUATED"}},
		{name: "unset", absent: []string{"picker", "launch "}},
		{name: "inside chat", environ: []string{"PFM_AUTO_OPEN=pfm", "CLAUDECODE=1"}, absent: []string{"picker", "launch "}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "log")
			writeShimFile(t, log, "")
			got := runAutoOpenShell(t, zsh, shimPath, home, fakeBin, log, testCase.environ,
				"print -r -- one\nprint -r -- two\nprint -r -- three\n")
			for _, wanted := range testCase.want {
				if !strings.Contains(got, wanted) {
					t.Fatalf("auto-open log %q lacks %q", got, wanted)
				}
			}
			for _, unwanted := range testCase.absent {
				if strings.Contains(got, unwanted) {
					t.Fatalf("auto-open log %q contains %q", got, unwanted)
				}
			}
			if pickers, launches := strings.Count(got, "picker\n"), strings.Count(got, "new-session"); pickers > 1 || launches > 1 {
				t.Fatalf("auto-open fired more than once (pickers=%d launches=%d): %q", pickers, launches, got)
			}
		})
	}

	log := filepath.Join(home, "inherit-log")
	writeShimFile(t, log, "")
	inherited := runAutoOpenShell(t, zsh, shimPath, home, fakeBin, log,
		[]string{"PFM_AUTO_OPEN=pfm", "CC_AUTO_OPEN=1", "VSCODE_AUTO_CC=1"},
		`print -r -- "leaked=${PFM_AUTO_OPEN-no}${CC_AUTO_OPEN-no}${VSCODE_AUTO_CC-no}" >> "$SHIM_AUTO_LOG"`+"\n")
	if !strings.Contains(inherited, "leaked=nonono") {
		t.Fatalf("auto-open left its variables in the environment: %q", inherited)
	}
}

func runAutoOpenShell(t *testing.T, zsh, shimPath, home, fakeBin, log string, environ []string, body string) string {
	t.Helper()
	command := exec.Command(zsh, "-f", "-i", "+m")
	command.Stdin = strings.NewReader("source " + quoteZsh(shimPath) + "\n" + body)
	command.Env = append([]string{
		"HOME=" + home,
		"PATH=" + fakeBin + ":/usr/bin:/bin",
		"TERM=dumb",
		"SHIM_AUTO_LOG=" + log,
	}, environ...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run auto-open shell: %v: %s", err, output)
	}
	return readShimFile(t, log)
}

func writeShimFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func readShimFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func embeddedShimPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "internal", "installer", "assets", "shim", "pfm.zsh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("embedded shim asset %s: %v", path, err)
	}
	return path
}

func quoteZsh(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
