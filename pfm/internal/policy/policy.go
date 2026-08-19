// Package policy resolves the local permission posture for launching chats.
//
// Upstream hardcodes the Claude permission bypass at every launch site. That is a fleet-wide
// decision with no off switch, so it lives here instead: one resolver, read by the zsh shim and by
// every Go launch path, defaulting to OFF. A missing or unreadable config means prompted mode —
// the safe answer, never the permissive one.
package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// EnvAutonomy overrides the config file for one process tree.
const EnvAutonomy = "PFM_AUTONOMY"

// Flags is the bypass pair. Both halves are required: --allow-… enables the bypass and
// --dangerously-… performs it, so passing one alone is inert.
var Flags = []string{"--allow-dangerously-skip-permissions", "--dangerously-skip-permissions"}

type config struct {
	Autonomy *bool `json:"autonomy"`
}

// ConfigPath is where the posture is stored.
func ConfigPath(home string) string {
	return filepath.Join(home, ".config", "pfm", "config.json")
}

// Autonomy reports whether chats launch with the permission bypass. Precedence: PFM_AUTONOMY, then
// ~/.config/pfm/config.json, then false.
func Autonomy(home string) bool {
	if value, present := os.LookupEnv(EnvAutonomy); present {
		return truthy(value)
	}
	content, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		return false
	}
	var parsed config
	// A malformed config is reported as prompted mode rather than ignored into the permissive
	// default: the whole point of the file is to make the bypass deliberate.
	if err := json.Unmarshal(content, &parsed); err != nil || parsed.Autonomy == nil {
		return false
	}
	return *parsed.Autonomy
}

// AutonomyFlags is Flags when autonomy is on and empty otherwise, so a caller can splat it
// unconditionally into an argument list.
func AutonomyFlags(home string) []string {
	if !Autonomy(home) {
		return nil
	}
	return append([]string(nil), Flags...)
}

// AutonomyFlagsForUser resolves the home directory itself, for launch sites that already shell out
// and hold no resolved paths. An unresolvable home yields prompted mode.
func AutonomyFlagsForUser() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return AutonomyFlags(home)
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}
