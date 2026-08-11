package naming

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"hostops/cc-fleet/internal/store"
)

// CodexNameIndex builds the two lookups a live Codex pane can be named
// through: by the rollout file it holds, and by the thread id it was
// identified by. A resumed or paginated session writes no rollout file and has
// only the second, so the id index also carries every thread the Codex session
// index names but no rollout row covers.
func CodexNameIndex(
	rollouts []store.Rollout,
	cxNames map[string]string,
) (byPath map[string]string, byID map[string]string) {
	byPath = make(map[string]string, len(rollouts))
	byID = make(map[string]string, len(rollouts)+len(cxNames))
	for _, rollout := range rollouts {
		name := CxName(
			rollout.ID,
			rollout.SessionID,
			rollout.ParentThread,
			cxNames,
			rollout.FirstPrompt,
		)
		byPath[filepath.Clean(rollout.Path)] = name
		if name != "" {
			byID[rollout.ID] = name
		}
	}
	for id := range cxNames {
		if byID[id] != "" {
			continue
		}
		if name := CxName(id, "", "", cxNames, ""); name != "" {
			byID[id] = name
		}
	}
	return byPath, byID
}

// DisplayName applies transcript naming precedence. The caller supplies the
// last custom title, last AI title, and first real prompt found by the indexer.
func DisplayName(customTitle, aiTitle, firstPrompt string) string {
	if customTitle != "" {
		return customTitle
	}
	if aiTitle != "" {
		return aiTitle
	}
	return firstPrompt
}

// LiveFallback applies the live-row naming chain. Claude sockets own a
// generated tmux session name, so their meaningful live fallback is the pane
// title; other socket types use their tmux session name.
func LiveFallback(
	indexed string,
	paneTitle string,
	sessionName string,
	lastPrompt string,
	isCCSock bool,
) string {
	if indexed != "" {
		return indexed
	}
	if isCCSock {
		if paneTitle != "" && paneTitle != "Claude Code" {
			return paneTitle
		}
	} else if sessionName != "" {
		return sessionName
	}
	if lastPrompt != "" {
		return lastPrompt
	}
	return "(unnamed)"
}

// IsJunkPrompt reports whether prompt starts with one of the injected-record
// prefixes matched by ^(<[a-z]|Caveat:|\[Request).
func IsJunkPrompt(prompt string) bool {
	if strings.HasPrefix(prompt, "Caveat:") || strings.HasPrefix(prompt, "[Request") {
		return true
	}
	return len(prompt) >= 2 &&
		prompt[0] == '<' &&
		prompt[1] >= 'a' &&
		prompt[1] <= 'z'
}

// FlattenPromptText extracts a user message's content. JSON strings become
// text directly; arrays contribute only top-level blocks whose type is text.
// Tabs and newlines are collapsed to one space, matching the transcript
// metadata records consumed by the rest of cc-fleet.
func FlattenPromptText(content json.RawMessage) string {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return squashTabsAndNewlines(text)
	}

	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(content, &rawBlocks); err != nil {
		return ""
	}

	type textBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	texts := make([]string, 0, len(rawBlocks))
	for _, rawBlock := range rawBlocks {
		var block textBlock
		if err := json.Unmarshal(rawBlock, &block); err != nil || block.Type != "text" {
			continue
		}
		texts = append(texts, block.Text)
	}
	return squashTabsAndNewlines(strings.Join(texts, " "))
}

// CxName follows Codex thread lineage before falling back to the first user
// message. It is the only naming function that clips its result.
func CxName(
	ownID string,
	sessionID string,
	parentThread string,
	names map[string]string,
	firstUserMessage string,
) string {
	for _, id := range []string{ownID, sessionID, parentThread} {
		if id == "" {
			continue
		}
		if name := names[id]; name != "" {
			return clipRunes(squashTabsAndNewlines(name), 60)
		}
	}
	return clipRunes(squashTabsAndNewlines(firstUserMessage), 60)
}

func clipRunes(value string, limit int) string {
	count := 0
	for index := range value {
		if count == limit {
			return value[:index]
		}
		count++
	}
	return value
}

func squashTabsAndNewlines(value string) string {
	first := strings.IndexAny(value, "\t\n")
	if first < 0 {
		return value
	}

	var flattened strings.Builder
	flattened.Grow(len(value))
	flattened.WriteString(value[:first])
	inRun := false
	for index := first; index < len(value); index++ {
		switch value[index] {
		case '\t', '\n':
			if !inRun {
				flattened.WriteByte(' ')
				inRun = true
			}
		default:
			flattened.WriteByte(value[index])
			inRun = false
		}
	}
	return flattened.String()
}
