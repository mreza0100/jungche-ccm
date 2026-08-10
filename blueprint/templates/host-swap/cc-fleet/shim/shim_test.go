package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShimSyntaxAndEvalProtocol(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(cwd, "cc-fleet.zsh")
	if output, err := exec.Command(zsh, "-n", shimPath).CombinedOutput(); err != nil {
		t.Fatalf("zsh -n: %v: %s", err, output)
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := `#!/bin/sh
case "$1:$2" in
  open:good) printf '%s\n' 'print -r -- evaluated' ;;
  open:bad) printf '%s\n' 'print -r -- must-not-eval'; exit 7 ;;
  ls:--tsv) printf '%s\n' 'literal tsv output' ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(
		filepath.Join(binDir, "cc-fleet"),
		[]byte(fake),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	script := "source " + quoteZsh(shimPath) + `
cc-open good
cc-ls --tsv
cc-open bad
print -r -- "rc=$?"
`
	command := exec.Command(zsh, "-c", script)
	command.Env = append(os.Environ(), "HOME="+home)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run shim protocol: %v: %s", err, output)
	}
	got := string(output)
	for _, wanted := range []string{
		"evaluated\n",
		"literal tsv output\n",
		"rc=7\n",
	} {
		if !strings.Contains(got, wanted) {
			t.Fatalf("shim output %q does not contain %q", got, wanted)
		}
	}
	if strings.Contains(got, "must-not-eval") {
		t.Fatalf("shim evaluated output from failed binary: %q", got)
	}
}

func quoteZsh(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
