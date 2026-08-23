package inject

import (
	"testing"

	"hostops/pfm/internal/resolve"
)

func TestPaneCommandEngineRecognizesOpencode(t *testing.T) {
	cases := []struct {
		command  string
		binaries []string
		want     string
	}{
		{"opencode", nil, resolve.OpencodeEngine},
		{"/home/me/.local/bin/opencode", nil, resolve.OpencodeEngine},
		{"ocx", []string{"claude", "codex", "ocx"}, resolve.OpencodeEngine},
		{"codex", []string{"claude", "codex"}, "cx"},
		{"claude", nil, "cc"},
		{"2.1.47", nil, "cc"},
		{"vim", nil, ""},
	}
	for _, c := range cases {
		if got := paneCommandEngine(c.command, c.binaries...); got != c.want {
			t.Errorf("paneCommandEngine(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestTargetFromPartsRecognizesOpencodeSockets(t *testing.T) {
	t.Setenv("PFM_TEST_PROBE_SOCKETS", "")
	for _, socket := range []string{"/tmp/tmux-0/ox-1-2-3,99,0"} {
		target := targetFromParts(socket, "%5")
		if target.Engine != resolve.OpencodeEngine {
			t.Errorf("targetFromParts(%q).Engine = %q, want ox", socket, target.Engine)
		}
	}
	if got := targetFromParts("/tmp/tmux-0/cc-9,1,0", "%1").Engine; got != "cc" {
		t.Errorf("cc socket misread as %q", got)
	}
}

func TestTargetFromPartsProbeOpencodeSocket(t *testing.T) {
	t.Setenv("PFM_TEST_PROBE_SOCKETS", "1")
	target := targetFromParts("/tmp/jail/probe-ox-7,3,0", "%2")
	if target.Engine != resolve.OpencodeEngine {
		t.Errorf("probe socket engine = %q, want ox", target.Engine)
	}
}

func TestEngineNameCoversOpencode(t *testing.T) {
	if got := engineName(resolve.OpencodeEngine); got != "OpenCode" {
		t.Errorf("engineName(ox) = %q, want OpenCode", got)
	}
}

func TestAutoFileThresholdInheritsCodexBoundForOpencode(t *testing.T) {
	engine := &Engine{options: withDefaults(Options{
		ClaudeAutoFileMax: 500,
		CodexAutoFileMax:  300,
	})}
	if got := engine.autoFileThreshold(resolve.OpencodeEngine); got != 300 {
		t.Errorf("ox threshold = %d, want Codex's 300", got)
	}
}
