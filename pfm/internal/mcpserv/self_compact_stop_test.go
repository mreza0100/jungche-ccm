package mcpserv

import (
	"strings"
	"testing"
)

// The --then waiter identifies the compaction turn by watching the pane it is
// steering: the caller's turn must end, a new turn must start, and that new one
// is the compaction. A caller that keeps working after queueing the compaction
// erases that boundary — its own turn and the compaction's become one
// indistinguishable stretch of busy — and the steer lands beside the compaction
// instead of after it.
//
// No amount of waiter cleverness recovers a boundary the caller never drew, so
// the instruction to stop is load-bearing product behaviour and is pinned here
// in BOTH places it has to appear. They are not redundant: the description is
// read once when the session starts and is far away by the time it matters; the
// result is read in the same breath as the decision it governs. This file pins
// the description; the result half belongs to whichever layer actually writes
// it, which is the engine — see TestSelfCompactScheduleTellsTheCallerToStop in
// internal/inject, covering the MCP tool and `pfm chat self-compact` in one
// place.

func TestSelfCompactToolDescriptionTellsTheCallerToStop(t *testing.T) {
	description := selfCompactDescription

	for _, want := range []string{
		"END THE TURN IMMEDIATELY",
		"run no further tool",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf(
				"chat_self_compact description is missing %q — without it a "+
					"caller keeps working after queueing the compaction, which "+
					"destroys the turn boundary the --then waiter needs.\ngot: %s",
				want, description,
			)
		}
	}
}

// "Compact yourself" was once answered with a /handoff — a fresh reboot that
// kills the session's crons and sub-agents and hides the conversation — and
// with three steers where the operator wants one. The description is the only
// thing the model reads before choosing, so it pins both the exclusivity and
// the single-steer contract in the words the model will match.
func TestSelfCompactToolDescriptionIsExclusiveAndSingleSteer(t *testing.T) {
	description := selfCompactDescription

	for _, want := range []string{
		"nothing else",
		"exactly ONE post-compact steer",
		"KEEPS the session",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf("chat_self_compact description is missing %q\ngot: %s", want, description)
		}
	}
	// The MCP surface describes itself only: which slash commands exist, and
	// who may fire them, is those commands' own business.
	for _, forbidden := range []string{"/handoff", "/reload"} {
		if strings.Contains(description, forbidden) {
			t.Fatalf("chat_self_compact description must not name %q\ngot: %s", forbidden, description)
		}
	}
}
