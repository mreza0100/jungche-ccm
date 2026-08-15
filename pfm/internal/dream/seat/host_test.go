package seat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLiteralChunksRoundTripLargeUTF8BelowTmuxCeiling(t *testing.T) {
	input := strings.Repeat("plain-αβγ-", 3000)
	chunks := literalChunks(input, tmuxLiteralChunkBytes)
	if len(chunks) < 2 {
		t.Fatalf("large prompt produced %d chunk", len(chunks))
	}
	for index, chunk := range chunks {
		if len(chunk) > tmuxLiteralChunkBytes {
			t.Fatalf("chunk %d has %d bytes", index, len(chunk))
		}
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk %d splits UTF-8", index)
		}
	}
	if got := strings.Join(chunks, ""); got != input {
		t.Fatal("chunked literal did not round-trip")
	}
}

func TestLiteralChunksKeepsSmallControlMessagesWhole(t *testing.T) {
	for _, input := range []string{"", "/rename", "seat-name"} {
		chunks := literalChunks(input, tmuxLiteralChunkBytes)
		if len(chunks) != 1 || chunks[0] != input {
			t.Fatalf("literalChunks(%q) = %q", input, chunks)
		}
	}
}

func TestCommandHostResolvesTheExactPrivatePaneRoot(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "tmux-fixture")
	script := `#!/bin/sh
set -eu
[ "$1" = "-S" ]
[ "$2" = "$DREAM_EXPECT_SOCKET" ]
[ "$3" = "list-panes" ]
[ "$4" = "-t" ]
[ "$5" = "dream-seat" ]
[ "$6" = "-F" ]
[ "$7" = '#{pane_pid}' ]
printf '%s\n' "$DREAM_PANE_ROWS"
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAM_EXPECT_SOCKET", filepath.Join(directory, "dream-socket"))
	t.Setenv("DREAM_PANE_ROWS", "4242")
	host := NewCommandHost(binary, directory)
	pid, err := host.PaneRootPID(context.Background(), "dream-socket", "dream-seat")
	if err != nil {
		t.Fatalf("PaneRootPID() error = %v", err)
	}
	if pid != 4242 {
		t.Fatalf("PaneRootPID() = %d, want 4242", pid)
	}

	t.Setenv("DREAM_PANE_ROWS", "4242\n4243")
	if _, err := host.PaneRootPID(context.Background(), "dream-socket", "dream-seat"); err == nil ||
		!strings.Contains(err.Error(), "returned 2 rows") {
		t.Fatalf("ambiguous PaneRootPID() error = %v", err)
	}
}

func TestCommandHostPastesLargeBriefAtomicallyWithoutArgvPayload(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "tmux-fixture")
	argumentsPath := filepath.Join(directory, "arguments")
	payloadPath := filepath.Join(directory, "payload")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$DREAM_TMUX_ARGUMENTS"
case "$3" in
  load-buffer)
    [ "$4" = "-" ]
    dd of="$DREAM_TMUX_PAYLOAD" 2>/dev/null
    ;;
  paste-buffer)
    [ "$4" = "-d" ]
    [ "$5" = "-p" ]
    [ "$6" = "-r" ]
    [ "$7" = "-t" ]
    [ "$8" = "dream-seat" ]
    ;;
  *) exit 64 ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DREAM_TMUX_ARGUMENTS", argumentsPath)
	t.Setenv("DREAM_TMUX_PAYLOAD", payloadPath)
	prompt := "DISTILL-FIRST-LINE\n" + strings.Repeat("large-αβγ-row\n", 3000)
	host := NewCommandHost(binary, directory)
	if err := host.PasteLiteral(
		context.Background(),
		"dream-socket",
		"dream-seat",
		prompt,
	); err != nil {
		t.Fatalf("PasteLiteral() error = %v", err)
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != prompt {
		t.Fatalf("pasted payload has %d bytes, want exact %d-byte brief", len(payload), len(prompt))
	}
	arguments, err := os.ReadFile(argumentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(arguments), "DISTILL-FIRST-LINE") {
		t.Fatal("large brief leaked into tmux argv")
	}
	for _, required := range []string{"load-buffer -", "paste-buffer -d -p -r -t dream-seat"} {
		if !strings.Contains(string(arguments), required) {
			t.Fatalf("tmux transport omitted %q: %s", required, arguments)
		}
	}
}
