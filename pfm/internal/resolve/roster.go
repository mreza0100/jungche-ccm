package resolve

import (
	"fmt"
	"sort"
	"strings"
)

// RosterCandidate is one composed fleet row projected onto the fields needed
// to resolve a human target name. Callers keep composition in their own layer;
// this package owns the matching judgment shared by CLI and MCP.
type RosterCandidate struct {
	Name       string
	ID         string
	Socket     string
	SocketPath string
	Session    string
	Pane       string
	Engine     string
	Live       bool
}

// RosterAmbiguityError is a refusal, not a probe failure. The candidates are
// complete enough for a caller to retry with the stable thread id.
type RosterAmbiguityError struct {
	Name       string
	Candidates []RosterCandidate
}

func (failure *RosterAmbiguityError) Error() string {
	lines := make([]string, 0, len(failure.Candidates))
	for _, candidate := range failure.Candidates {
		id := candidate.ID
		if id == "" {
			id = "<unknown>"
		}
		socket := candidate.Socket
		if socket == "" {
			socket = SessionName(candidate.SocketPath)
		}
		lines = append(lines, fmt.Sprintf(
			"thread id %s (socket %s, pane %s, name %q)",
			id, socket, candidate.Pane, candidate.Name,
		))
	}
	sort.Strings(lines)
	return fmt.Sprintf(
		"%q matches %d chats in the roster; address by thread id:\n  %s",
		failure.Name, len(failure.Candidates), strings.Join(lines, "\n  "),
	)
}

// ResolveRosterName applies the fleet's one name/id/socket matching rule.
// Exact matches outrank folded names and ID prefixes. A live row and its own
// resume row are one conversation, so the unique live row wins that pair.
func ResolveRosterName(
	candidates []RosterCandidate,
	name string,
) (RosterCandidate, bool, error) {
	exact := make([]RosterCandidate, 0, 2)
	folded := make([]RosterCandidate, 0, 2)
	session := SessionName(name)
	for _, candidate := range candidates {
		socket := candidate.Socket
		if socket == "" {
			socket = candidate.SocketPath
		}
		switch {
		case candidate.ID == name ||
			(session != "" && SessionName(socket) == session) ||
			candidate.Name == name:
			exact = append(exact, candidate)
		case strings.EqualFold(candidate.Name, name) ||
			(len(name) >= 8 && strings.HasPrefix(candidate.ID, name)):
			folded = append(folded, candidate)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = folded
	}
	if len(matches) == 0 {
		return RosterCandidate{}, false, nil
	}
	if len(matches) > 1 {
		live := make([]RosterCandidate, 0, len(matches))
		for _, candidate := range matches {
			if candidate.Live {
				live = append(live, candidate)
			}
		}
		if len(live) == 1 {
			matches = live
		}
	}
	if len(matches) > 1 {
		return RosterCandidate{}, false, &RosterAmbiguityError{
			Name: name, Candidates: append([]RosterCandidate(nil), matches...),
		}
	}
	return matches[0], true, nil
}
