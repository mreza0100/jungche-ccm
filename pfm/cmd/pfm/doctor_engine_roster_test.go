package main

import (
	"bytes"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

func TestDoctorEngineRosterMatrix(t *testing.T) {
	tests := []struct {
		name        string
		machine     pfmconfig.Config
		want        string
		wantWarning int
	}{
		{
			name:        "zero zero",
			machine:     pfmconfig.Config{},
			want:        "doctor: roster cc=0 cx=0 ox=0 default=none error=no engines configured: Claude roster empty; Codex roster empty; OpenCode store absent\n",
			wantWarning: 1,
		},
		{
			name:    "claude only",
			machine: pfmconfig.Config{Accounts: []pfmconfig.Account{{ID: 1}}},
			want:    "doctor: roster cc=1 cx=0 ox=0 default=cc\n",
		},
		{
			name: "codex only",
			machine: pfmconfig.Config{
				Ask:           pfmconfig.AskConfig{Engine: pfmengine.Claude},
				CodexAccounts: []pfmconfig.CodexAccount{{ID: 1}},
			},
			want: "doctor: roster cc=0 cx=1 ox=0 default=cx\n",
		},
		{
			name: "both",
			machine: pfmconfig.Config{
				Ask:           pfmconfig.AskConfig{Engine: pfmengine.Codex},
				Accounts:      []pfmconfig.Account{{ID: 1}, {ID: 2}},
				CodexAccounts: []pfmconfig.CodexAccount{{ID: 1}},
			},
			want: "doctor: roster cc=2 cx=1 ox=0 default=cx\n",
		},
		{
			name: "opencode only",
			machine: pfmconfig.Config{
				OpencodeAccounts: []pfmconfig.OpenCodeAccount{{ID: 1}},
			},
			want: "doctor: roster cc=0 cx=0 ox=1 default=ox\n",
		},
		{
			name: "opencode requested but store absent",
			machine: pfmconfig.Config{
				Ask: pfmconfig.AskConfig{Engine: pfmengine.Opencode},
			},
			want:        "doctor: roster cc=0 cx=0 ox=0 default=none error=no engines configured: Claude roster empty; Codex roster empty; OpenCode store absent\n",
			wantWarning: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			warnings := printEngineDoctor(&stdout, test.machine)
			if stdout.String() != test.want || warnings != test.wantWarning {
				t.Fatalf("printEngineDoctor() output=%q warnings=%d, want %q warnings=%d", stdout.String(), warnings, test.want, test.wantWarning)
			}
			if test.wantWarning != 0 && !strings.Contains(stdout.String(), "error=") {
				t.Fatalf("zero-engine row hid its error: %q", stdout.String())
			}
		})
	}
}

func TestDoctorEngineCapabilities(t *testing.T) {
	var stdout bytes.Buffer
	if warnings := printEngineCapabilities(&stdout); warnings != 0 {
		t.Fatalf("printEngineCapabilities() warnings=%d output=%q", warnings, stdout.String())
	}
	want := "doctor: engines cc=index,launcher,matcher,usage,headless,ask cx=index,launcher,matcher,usage,headless,ask ox=index,matcher\n"
	if stdout.String() != want {
		t.Fatalf("printEngineCapabilities()=%q, want %q", stdout.String(), want)
	}
}
