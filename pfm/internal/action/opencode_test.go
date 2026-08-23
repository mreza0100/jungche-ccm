package action

import (
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
)

func TestSynthesizeResumeOpencodeLaunchesTheSession(t *testing.T) {
	plan, err := Synthesize(Request{
		Row: compose.Row{
			Kind: compose.ResumeOpencode,
			ID:   "ses_abc123",
			CWD:  "/work/nuts",
			Name: "math chat",
		},
		FreshSocket: "ox-1-2-3",
		Bunker:      true,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if plan.Route != ResumeOpencode {
		t.Fatalf("route = %v, want %v", plan.Route, ResumeOpencode)
	}
	if !strings.Contains(plan.Run, "--session") || !strings.Contains(plan.Run, "/work/nuts") {
		t.Errorf("run misses the session resume: %q", plan.Run)
	}
	if !strings.HasPrefix(plan.Line, "TMUX= exec tmux -L ") {
		t.Errorf("line is not a bunker session line: %q", plan.Line)
	}
	if !strings.Contains(plan.Line, "'ox-1-2-3'") || !strings.Contains(plan.Line, "'/work/nuts'") {
		t.Errorf("line misses socket or cwd: %q", plan.Line)
	}
	if strings.Contains(plan.Run, "--dangerously") {
		t.Errorf("OpenCode launch must not carry Claude/Codex autonomy flags: %q", plan.Run)
	}
}

func TestSynthesizeResumeOpencodeRequiresIdentity(t *testing.T) {
	for _, request := range []Request{
		{Row: compose.Row{Kind: compose.ResumeOpencode, CWD: "/w"}, FreshSocket: "ox-1"},
		{Row: compose.Row{Kind: compose.ResumeOpencode, ID: "s"}, FreshSocket: "ox-1"},
		{Row: compose.Row{Kind: compose.ResumeOpencode, ID: "s", CWD: "/w"}},
	} {
		if _, err := Synthesize(request); err == nil {
			t.Errorf("request %+v: expected an error", request)
		}
	}
}
