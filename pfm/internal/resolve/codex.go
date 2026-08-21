package resolve

import (
	"fmt"
	"path/filepath"
)

const (
	// CodexThreadEnv is the variable a Codex session exports into the shells
	// it spawns. It names the conversation outright, which nothing else about
	// a live process does: one codex app-server process holds every thread
	// writer lock, so pid ancestry cannot tell two Codex sessions apart.
	CodexThreadEnv = "CODEX_THREAD_ID"

	// CodexBirthWindowSeconds bounds the fallback that pairs a pane with the
	// thread created at about the same moment in the same directory.
	CodexBirthWindowSeconds = 120
)

// CodexThread is one candidate conversation offered by the Codex state store.
type CodexThread struct {
	ID          string
	CWD         string
	CreatedAt   int64
	RolloutPath string
}

// CodexThreadID names the Codex conversation behind a pane. An exported
// CODEX_THREAD_ID is authoritative and is honored even when the state store
// has not caught up with it. Without one, the pane is matched to the thread
// created nearest its own start within CodexBirthWindowSeconds in the same
// directory; a tie prefers the newer thread. A pane that matches nothing
// returns an error rather than a guess, because a wrong identity would attach,
// rename, or kill the wrong chat.
func CodexThreadID(
	exported string,
	cwd string,
	birth int64,
	candidates []CodexThread,
) (CodexThread, error) {
	if exported != "" {
		for _, candidate := range candidates {
			if candidate.ID == exported {
				return candidate, nil
			}
		}
		return CodexThread{ID: exported}, nil
	}
	if cwd == "" || birth <= 0 {
		return CodexThread{}, fmt.Errorf(
			"no %s is exported and the pane has no directory and start time to match a Codex thread",
			CodexThreadEnv,
		)
	}

	wanted := filepath.Clean(cwd)
	best := CodexThread{}
	bestDistance := int64(-1)
	for _, candidate := range candidates {
		if candidate.ID == "" ||
			candidate.CWD == "" ||
			filepath.Clean(candidate.CWD) != wanted {
			continue
		}
		distance := birth - candidate.CreatedAt
		if distance < 0 {
			distance = -distance
		}
		if distance > CodexBirthWindowSeconds {
			continue
		}
		if bestDistance < 0 ||
			distance < bestDistance ||
			(distance == bestDistance && newerCodexThread(candidate, best)) {
			best = candidate
			bestDistance = distance
		}
	}
	if bestDistance < 0 {
		return CodexThread{}, fmt.Errorf(
			"no Codex thread in %q was created within %ds of the pane",
			wanted,
			CodexBirthWindowSeconds,
		)
	}
	return best, nil
}

func newerCodexThread(challenger, incumbent CodexThread) bool {
	if challenger.CreatedAt != incumbent.CreatedAt {
		return challenger.CreatedAt > incumbent.CreatedAt
	}
	return challenger.ID < incumbent.ID
}
