package main

import (
	"bytes"
	"strings"
	"testing"
)

// The /bb hook's first duty is to NOT eat prompts. It sees every prompt typed
// into every chat, so anything that is not exactly /bb has to pass through
// untouched, and anything it cannot understand has to pass through too — a
// lost prompt is worse than a chat left open.
func TestBBPassesThroughEveryPromptItDoesNotOwn(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{name: "an ordinary prompt", payload: `{"prompt":"fix the parser"}`},
		{
			name:    "a sentence ABOUT the command",
			payload: `{"prompt":"/bb doesn't work!"}`,
		},
		{name: "a longer command that starts the same", payload: `{"prompt":"/bbq"}`},
		{name: "the command quoted mid-sentence", payload: `{"prompt":"try /bb here"}`},
		{name: "an empty prompt", payload: `{"prompt":""}`},
		{name: "a payload with no prompt field", payload: `{"session_id":"x"}`},
		{name: "not JSON at all", payload: `not json`},
		{name: "nothing on stdin", payload: ``},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			jailTest(t)
			var stdout, stderr bytes.Buffer
			code := runBB(
				nil,
				strings.NewReader(testCase.payload),
				&stdout,
				&stderr,
			)
			if code != 0 {
				t.Fatalf("rc = %d, want 0 (the prompt must reach the model)", code)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf(
					"the hook spoke: stdout=%q stderr=%q",
					stdout.String(),
					stderr.String(),
				)
			}
		})
	}
}

// Whitespace around a bare /bb is still a bare /bb: the harness delivers the
// prompt with the newline the operator typed.
func TestBBTriggersOnASurroundedCommand(t *testing.T) {
	for _, payload := range []string{
		`{"prompt":"/bb"}`,
		`{"prompt":"/bb\n"}`,
		`{"prompt":"   /bb   "}`,
	} {
		jailTest(t)
		var stdout, stderr bytes.Buffer
		// No chat surrounds this call — no TMUX, no pane — so the hide cannot
		// identify a chat and refuses. What matters here is that the prompt
		// was RECOGNISED: it is blocked (rc 2) and the refusal is said out
		// loud, rather than the chat silently staying open.
		code := runBB(nil, strings.NewReader(payload), &stdout, &stderr)
		if code != bbBlockPrompt {
			t.Fatalf("payload %q: rc = %d, want %d", payload, code, bbBlockPrompt)
		}
		if !strings.Contains(stderr.String(), "bb:") {
			t.Fatalf("payload %q: no refusal reported: %q", payload, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf(
				"the hook wrote to stdout, which is injected into the model's context: %q",
				stdout.String(),
			)
		}
	}
}

// A hide that could not identify a chat must not hide SOMETHING ELSE.
func TestBBWithoutAChatHidesNothing(t *testing.T) {
	jailTest(t)
	var stdout, stderr bytes.Buffer
	if code := runBB(
		nil,
		strings.NewReader(`{"prompt":"/bb"}`),
		&stdout,
		&stderr,
	); code != bbBlockPrompt {
		t.Fatalf("rc = %d, want %d", code, bbBlockPrompt)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"hidden"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hidden rc = %d: %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("a refused /bb still hid something: %q", stdout.String())
	}
}
