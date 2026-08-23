package action

import (
	"strings"
	"testing"

	"hostops/pfm/internal/compose"
	pfmconfig "hostops/pfm/internal/config"
)

func TestSynthesizeNewOpencodeLaunchesFreshConfiguredSeat(t *testing.T) {
	machine := pfmconfig.Config{
		Version:          pfmconfig.Version,
		OpencodeAccounts: []pfmconfig.OpenCodeAccount{{ID: 1, Home: "/opencode"}},
		OpenCode:         pfmconfig.OpenCode{Binary: "/opt/opencode stable"},
		Sources:          map[string]pfmconfig.Source{"opencode.binary": pfmconfig.SourceFile},
	}
	plan, err := Synthesize(Request{
		Row:            compose.Row{Kind: compose.NewOpencode, CWD: "/work/nuts"},
		PrimaryAccount: 1,
		FreshSocket:    "ox-1-2-4",
		Config:         machine,
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if plan.Route != NewOpencode {
		t.Fatalf("route = %v, want %v", plan.Route, NewOpencode)
	}
	if !strings.Contains(plan.Run, Quote(machine.OpenCode.Binary)) || strings.Contains(plan.Run, "--session") {
		t.Fatalf("fresh OpenCode run = %q", plan.Run)
	}
	if !strings.Contains(plan.Line, "'ox-1-2-4'") || !strings.Contains(plan.Line, "'/work/nuts'") {
		t.Fatalf("fresh OpenCode line = %q", plan.Line)
	}
}

func TestSynthesizeNewOpencodeRequiresSeatAndLaunchContext(t *testing.T) {
	machine := pfmconfig.Config{
		Version:          pfmconfig.Version,
		OpencodeAccounts: []pfmconfig.OpenCodeAccount{{ID: 1, Home: "/opencode"}},
		OpenCode:         pfmconfig.OpenCode{Binary: "opencode"},
	}
	for _, test := range []struct {
		name    string
		request Request
		want    string
	}{
		{
			name: "account must be configured",
			request: Request{
				Row:            compose.Row{Kind: compose.NewOpencode, CWD: "/work"},
				PrimaryAccount: 9,
				FreshSocket:    "ox-1",
				Config:         machine,
			},
			want: "account 9 is not in the configured roster",
		},
		{
			name: "cwd is required",
			request: Request{
				Row:            compose.Row{Kind: compose.NewOpencode},
				PrimaryAccount: 1,
				FreshSocket:    "ox-1",
				Config:         machine,
			},
			want: "requires cwd and fresh socket",
		},
		{
			name: "fresh socket is required",
			request: Request{
				Row:            compose.Row{Kind: compose.NewOpencode, CWD: "/work"},
				PrimaryAccount: 1,
				Config:         machine,
			},
			want: "requires cwd and fresh socket",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Synthesize(test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

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
