// Package codexlaunch appends fleet instructions without replacing a model's base prompt.
package codexlaunch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const marker = "# Professor Codex appendix"

func PromptPath(home string) string {
	return filepath.Join(home, ".local", "share", "pfm", "install", "prompts", "codex-appendix.md")
}

// Prepare uses Codex's own config resolver, then places the merged developer
// override after caller overrides. The caller's prompt and model remain intact.
func Prepare(ctx context.Context, binary, home string, args []string, read func(context.Context, string, []string) (string, error)) ([]string, error) {
	raw, err := os.ReadFile(PromptPath(home))
	if err != nil {
		return nil, fmt.Errorf("read Professor Codex appendix (run pfm install): %w", err)
	}
	if len(raw) == 0 || len(raw) > 16384 || !utf8.Valid(raw) {
		return nil, fmt.Errorf("Professor Codex appendix must be nonempty UTF-8, at most 16 KiB")
	}
	existing, err := read(ctx, binary, args)
	if err != nil {
		return nil, err
	}
	if strings.Contains(existing, marker) {
		return nil, fmt.Errorf("Professor appendix already exists in developer_instructions; remove the stale override")
	}
	merged := existing
	if merged != "" {
		merged += "\n\n"
	}
	merged += marker + "\n\n" + strings.TrimSpace(string(raw))
	encoded, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("encode Professor appendix: %w", err)
	}
	at := len(args)
	for i, arg := range args {
		if arg == "--" {
			at = i
			break
		}
	}
	result := append([]string{}, args[:at]...)
	result = append(result, "-c", "developer_instructions="+string(encoded))
	return append(result, args[at:]...), nil
}
