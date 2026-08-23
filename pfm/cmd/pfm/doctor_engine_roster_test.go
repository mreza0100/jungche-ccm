package main

import (
	"bytes"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
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
			want:        "doctor: engines claude=0 codex=0 opencode=0 default=none error=no engines configured: Claude roster empty; Codex roster empty; OpenCode store absent\n",
			wantWarning: 1,
		},
		{
			name:    "claude only",
			machine: pfmconfig.Config{Accounts: []pfmconfig.Account{{ID: 1}}},
			want:    "doctor: engines claude=1 codex=0 opencode=0 default=claude\n",
		},
		{
			name: "codex only",
			machine: pfmconfig.Config{
				Ask:           pfmconfig.AskConfig{Engine: "claude"},
				CodexAccounts: []pfmconfig.CodexAccount{{ID: 1}},
			},
			want: "doctor: engines claude=0 codex=1 opencode=0 default=codex\n",
		},
		{
			name: "both",
			machine: pfmconfig.Config{
				Ask:           pfmconfig.AskConfig{Engine: "codex"},
				Accounts:      []pfmconfig.Account{{ID: 1}, {ID: 2}},
				CodexAccounts: []pfmconfig.CodexAccount{{ID: 1}},
			},
			want: "doctor: engines claude=2 codex=1 opencode=0 default=codex\n",
		},
		{
			name: "opencode only",
			machine: pfmconfig.Config{
				OpencodeAccounts: []pfmconfig.OpenCodeAccount{{ID: 1}},
			},
			want: "doctor: engines claude=0 codex=0 opencode=1 default=opencode\n",
		},
		{
			name: "opencode requested but store absent",
			machine: pfmconfig.Config{
				Ask: pfmconfig.AskConfig{Engine: "opencode"},
			},
			want:        "doctor: engines claude=0 codex=0 opencode=0 default=none error=no engines configured: Claude roster empty; Codex roster empty; OpenCode store absent\n",
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
