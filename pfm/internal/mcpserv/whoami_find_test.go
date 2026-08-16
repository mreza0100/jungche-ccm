package mcpserv

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"hostops/pfm/internal/resolve"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestChatWhoamiReportsIdentityOrStatesItsAbsence covers chat.sh:482-484 over
// MCP: inside tmux the answer is this chat's own session name; outside it, the
// absence is stated rather than guessed.
func TestChatWhoamiReportsIdentityOrStatesItsAbsence(t *testing.T) {
	setupBackendFixture(t)
	t.Setenv("TMUX", "")
	t.Setenv(resolve.ClaudeSessionEnv, "")
	t.Setenv(resolve.CodexThreadEnv, "")
	service, err := New("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	client := connectInMemory(t, service.Server())
	output := callTool[WhoamiOutput](
		t,
		client.clientSession,
		"chat_whoami",
		WhoamiInput{},
	)
	if output.Status != "not_found" ||
		!strings.Contains(output.Message, "not inside tmux") {
		t.Fatalf("chat_whoami outside tmux = %+v", output)
	}
	if output.Session != "" {
		t.Fatalf("chat_whoami invented a session: %+v", output)
	}
}

// TestChatFindRanksByNeedleVotesAndExcludesSelf covers chat.sh:440-478: an
// excerpt becomes up to five needles of 20+ characters, each transcript is
// ranked by how many it hits, and the ASKING session never matches itself.
func TestChatFindRanksByNeedleVotesAndExcludesSelf(t *testing.T) {
	root := setupBackendFixture(t)
	project := filepath.Join(root, "claude", "project-alpha")
	// three needles, one transcript hits all three, one hits a single one.
	writeJSONL(t, filepath.Join(project, "three.jsonl"), []any{
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{
				"content": "the migration plan must survive verbatim",
			},
		},
		map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"content": "lock namespace unification is the hard part\n" +
					"the waiter rides out the compaction to idle",
			},
		},
	})
	writeJSONL(t, filepath.Join(project, "one.jsonl"), []any{
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{
				"content": "lock namespace unification is the hard part",
			},
		},
	})
	writeJSONL(t, filepath.Join(project, "selfchat.jsonl"), []any{
		map[string]any{
			"type": "user", "cwd": "/work/alpha",
			"message": map[string]any{
				"content": "the migration plan must survive verbatim\n" +
					"lock namespace unification is the hard part\n" +
					"the waiter rides out the compaction to idle",
			},
		},
	})
	t.Setenv(resolve.ClaudeSessionEnv, "selfchat")
	t.Setenv(resolve.CodexThreadEnv, "")
	service, err := New("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	client := connectInMemory(t, service.Server())

	excerpt := strings.Join([]string{
		"> the migration plan must survive verbatim",
		"- lock namespace unification is the hard part",
		"  the waiter rides out the compaction to idle",
		"too short",
	}, "\n")
	output := callTool[FindOutput](t, client.clientSession, "chat_find", FindInput{
		Excerpt: excerpt,
	})
	if len(output.Needles) != 3 {
		t.Fatalf("needles = %+v, want the three 20+ character lines", output.Needles)
	}
	if output.SelfID != "selfchat" {
		t.Fatalf("self id = %q, want the asking session", output.SelfID)
	}
	for _, candidate := range output.Candidates {
		if candidate.ID == "selfchat" {
			t.Fatalf("the asking session matched itself: %+v", output.Candidates)
		}
	}
	if output.Count < 2 {
		t.Fatalf("find = %+v, want both other transcripts", output)
	}
	if output.Candidates[0].ID != "three" || output.Candidates[0].Hits != 3 {
		t.Fatalf("top candidate = %+v, want three with 3 hits", output.Candidates[0])
	}
	ranked := make([]int, 0, len(output.Candidates))
	for _, candidate := range output.Candidates {
		ranked = append(ranked, candidate.Hits)
	}
	for index := 1; index < len(ranked); index++ {
		if ranked[index] > ranked[index-1] {
			t.Fatalf("candidates are not ordered by hits: %v", ranked)
		}
	}

	// include_self is the escape hatch, and it brings the excluded row back.
	withSelf := callTool[FindOutput](t, client.clientSession, "chat_find", FindInput{
		Excerpt:     excerpt,
		IncludeSelf: true,
	})
	found := false
	for _, candidate := range withSelf.Candidates {
		if candidate.ID == "selfchat" {
			found = true
		}
	}
	if !found {
		t.Fatalf("include_self did not restore the asking session: %+v", withSelf)
	}
}

// TestChatInjectCarriesTheThenArgument proves the steer chain reaches the
// engine through the MCP schema, and that the /compact focus rule refuses
// before the target is even resolved.
func TestChatInjectCarriesTheThenArgument(t *testing.T) {
	setupBackendFixture(t)
	service, err := New("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	client := connectInMemory(t, service.Server())

	steerless := callTool[InjectOutput](t, client.clientSession, "chat_inject", InjectInput{
		Target:  "no-such-chat",
		Message: "/compact hold: read /tmp/hold.md",
	})
	if steerless.Code != 6 ||
		steerless.Status != "refused" ||
		steerless.Typed ||
		!strings.Contains(steerless.Message, "requires a then steer") {
		t.Fatalf("steerless /compact = %+v", steerless)
	}

	recursive := callTool[InjectOutput](t, client.clientSession, "chat_inject", InjectInput{
		Target:  "no-such-chat",
		Message: "/compact hold: read /tmp/hold.md",
		Then:    []string{"/compact again"},
	})
	if recursive.Code != 6 ||
		!strings.Contains(recursive.Message, "must not itself start with /compact") {
		t.Fatalf("recursive steer = %+v", recursive)
	}

	// With a legal steer the chain passes the guard and dies at resolution
	// instead — proof the argument travelled, and that nothing was typed.
	unresolved := callTool[InjectOutput](t, client.clientSession, "chat_inject", InjectInput{
		Target:  "no-such-chat",
		Message: "/compact hold: read /tmp/hold.md",
		Then:    []string{"resume the port"},
	})
	if unresolved.Code != 4 ||
		unresolved.Typed ||
		!strings.Contains(unresolved.Message, "matched no live chat") {
		t.Fatalf("legal steer chain = %+v", unresolved)
	}
}

// TestExtractNeedlesMirrorsTheShellAwkPass pins the needle rules themselves:
// decoration stripped, 20+ characters only, five longest, longest first.
func TestExtractNeedlesMirrorsTheShellAwkPass(t *testing.T) {
	excerpt := strings.Join([]string{
		"# short",
		"> aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r",
		"- bbbbbbbbbbbbbbbbbbbbbbbbb",
		"* cccccccccccccccccccccc   ",
		"ddddddddddddddddddddd",
		"eeeeeeeeeeeeeeeeeeeee",
		"fffffffffffffffffffff",
		"nineteen characters",
	}, "\n")
	needles := extractNeedles(excerpt)
	want := []string{
		strings.Repeat("a", 30),
		strings.Repeat("b", 25),
		strings.Repeat("c", 22),
		strings.Repeat("d", 21),
		strings.Repeat("e", 21),
	}
	if !reflect.DeepEqual(needles, want) {
		t.Fatalf("extractNeedles() = %q, want %q", needles, want)
	}
	if got := extractNeedles("all lines are short\nhere"); len(got) != 0 {
		t.Fatalf("short excerpt produced needles: %q", got)
	}
}

// TestChatCaptureBoundsAreAppliedAfterTheCapture keeps the whole-scrollback
// capture from returning an unbounded payload.
func TestChatCaptureBoundsAreAppliedAfterTheCapture(t *testing.T) {
	if _, err := os.Stat("/proc/self"); err != nil {
		t.Skip("no /proc")
	}
	setupBackendFixture(t)
	service, err := New("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	client := connectInMemory(t, service.Server())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, input := range []CaptureInput{
		{Target: "missing-target", MaxBytes: maxCaptureBytes + 1},
		{Target: "missing-target", TailLines: 5000},
	} {
		result, err := client.clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name:      "chat_capture",
			Arguments: input,
		})
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("out-of-range bound accepted: %+v", input)
		}
	}
	if got := tailBytes("héllo world", 6); got != " world" {
		t.Fatalf("tailBytes() = %q, want the last six bytes", got)
	}
	// A cut that lands inside a rune advances to the next whole one.
	if got := tailBytes("héllo", 4); got != "llo" {
		t.Fatalf("tailBytes() = %q, want a whole-rune tail", got)
	}
}
