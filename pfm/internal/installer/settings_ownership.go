package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const settingsHookOwnershipVersion = 1

type settingsHookKey struct {
	Event   string
	Matcher string
	Command string
}

type settingsHookCounts map[settingsHookKey]int

type settingsHookOwnershipRecord struct {
	Path    string `json:"path"`
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
	Count   int    `json:"count"`
}

type settingsHookOwnershipDocument struct {
	Version int                           `json:"version"`
	Hooks   []settingsHookOwnershipRecord `json:"hooks"`
}

func countSettingsHookCommands(document map[string]any) settingsHookCounts {
	counts := settingsHookCounts{}
	events, _ := document["hooks"].(map[string]any)
	for event, eventValue := range events {
		entries, _ := eventValue.([]any)
		for _, entryValue := range entries {
			entry, _ := entryValue.(map[string]any)
			matcher, _ := entry["matcher"].(string)
			hooks, _ := entry["hooks"].([]any)
			for _, hookValue := range hooks {
				hook, _ := hookValue.(map[string]any)
				command, _ := hook["command"].(string)
				if command != "" {
					counts[settingsHookKey{Event: event, Matcher: matcher, Command: command}]++
				}
			}
		}
	}
	return counts
}

func nextSettingsHookOwnership(
	before, after, owned settingsHookCounts,
	pfmBinary string,
	uninstall bool,
) settingsHookCounts {
	if uninstall {
		return settingsHookCounts{}
	}
	// A settings.json is "pure" when EVERY hook command it carries, once
	// this run's wiring has converged it, is one of the installer's own
	// canonical (event, matcher, command) triples — nothing foreign,
	// nothing sitting at a non-canonical spot. Only a whole document that
	// clean could have been produced entirely by a pfm install, whether or
	// not the ownership ledger tracked it; a single stray or misplaced
	// command (an operator's own hook, a not-yet-migrated legacy entry)
	// means the file's history is not provably ours. Purity is computed
	// once per document, not per key, and is stable across repeat runs:
	// wiring never removes foreign content, so an impure file never
	// "becomes" pure just because a later apply happens to be a no-op.
	pure := true
	for key := range after {
		if !installerOwnedHookKey(key, pfmBinary) {
			pure = false
			break
		}
	}
	next := settingsHookCounts{}
	for key, afterCount := range after {
		if !installerOwnedHookCommand(key.Command, pfmBinary) {
			continue
		}
		added := afterCount - before[key]
		if added < 0 {
			added = 0
		}
		count := owned[key] + added
		// The delta above already covers a hook this run wrote at a NEW
		// slot — including a legacy hook whose text `rewriteCommandFields`
		// converts to canonical in place without moving it to the
		// template's own event (e.g. a `dreamer-nudge.sh` SessionStart
		// entry rewritten to the canonical nudge command but left under
		// SessionStart, not UserPromptSubmit; see
		// TestDreamHookMigrationIsMigrateOnlyAndUninstallPreservesManualHooks).
		// What it never credits is a hook nothing changed this run AND the
		// ledger never recorded — an already fully-wired host predating the
		// ownership ledger, or one converged by an install version before
		// it existed for that hook
		// (TestInstallOwnershipLedgerClaimsHooksAlreadyPresentInSettings).
		// That hook belongs to the installer when — and ONLY when — it
		// sits at exactly the canonical triple a template wires AND the
		// whole document is pure: a command that merely shares its TEXT
		// with a template while parked under a different event or matcher,
		// in a document that also carries an operator's own hook, is never
		// claimed this way (the PostToolUse/UserPromptSubmit manual copies
		// TestDreamHookMigrationIsMigrateOnlyAndUninstallPreservesManualHooks
		// pins as permanently NOT owned); uninstall only ever removes what
		// `owned` lists, so a wrongly claimed key is a wrongly deleted
		// operator hook.
		if count == 0 && afterCount > 0 && pure && installerOwnedHookKey(key, pfmBinary) {
			count = afterCount
		}
		if count > afterCount {
			count = afterCount
		}
		if count > 0 {
			next[key] = count
		}
	}
	return next
}

func installerOwnedHookCommand(command, pfmBinary string) bool {
	home := filepath.Dir(filepath.Dir(filepath.Dir(pfmBinary)))
	for _, hook := range claudeHookTemplates(home) {
		if hook.Command == command {
			return true
		}
	}
	return codexHookTemplate(home).Command == command
}

func installerOwnedHookKey(key settingsHookKey, pfmBinary string) bool {
	home := filepath.Dir(filepath.Dir(filepath.Dir(pfmBinary)))
	for _, hook := range claudeHookTemplates(home) {
		if hook.Event == key.Event && hook.Matcher == key.Matcher && hook.Command == key.Command {
			return true
		}
	}
	codex := codexHookTemplate(home)
	return codex.Event == key.Event && codex.Matcher == key.Matcher && codex.Command == key.Command
}

func removeOwnedSettingsHooks(document map[string]any, owned settingsHookCounts) bool {
	if len(owned) == 0 {
		return false
	}
	remaining := make(settingsHookCounts, len(owned))
	for key, count := range owned {
		remaining[key] = count
	}
	changed := false
	events, _ := document["hooks"].(map[string]any)
	for event, eventValue := range events {
		entries, _ := eventValue.([]any)
		keptEntries := entries[:0]
		for _, entryValue := range entries {
			entry, _ := entryValue.(map[string]any)
			matcher, _ := entry["matcher"].(string)
			hooks, _ := entry["hooks"].([]any)
			keptHooks := hooks[:0]
			entryChanged := false
			for _, hookValue := range hooks {
				hook, _ := hookValue.(map[string]any)
				command, _ := hook["command"].(string)
				key := settingsHookKey{Event: event, Matcher: matcher, Command: command}
				if remaining[key] > 0 {
					remaining[key]--
					changed = true
					entryChanged = true
					continue
				}
				keptHooks = append(keptHooks, hookValue)
			}
			if entryChanged && len(keptHooks) == 0 {
				continue
			}
			if entryChanged {
				entry["hooks"] = keptHooks
			}
			keptEntries = append(keptEntries, entryValue)
		}
		events[event] = keptEntries
	}
	return changed
}

func readSettingsHookOwnership(path string) (map[string]settingsHookCounts, []byte, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]settingsHookCounts{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var document settingsHookOwnershipDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, nil, fmt.Errorf("decode ownership ledger: %w", err)
	}
	if document.Version != settingsHookOwnershipVersion {
		return nil, nil, fmt.Errorf("unsupported ownership ledger version %d", document.Version)
	}
	ownership := map[string]settingsHookCounts{}
	for index, record := range document.Hooks {
		if record.Path == "" || record.Event == "" || record.Command == "" || record.Count < 1 {
			return nil, nil, fmt.Errorf("invalid ownership record %d", index)
		}
		if ownership[record.Path] == nil {
			ownership[record.Path] = settingsHookCounts{}
		}
		key := settingsHookKey{Event: record.Event, Matcher: record.Matcher, Command: record.Command}
		ownership[record.Path][key] += record.Count
	}
	return ownership, raw, nil
}

func encodeSettingsHookOwnership(ownership map[string]settingsHookCounts) ([]byte, error) {
	document := settingsHookOwnershipDocument{Version: settingsHookOwnershipVersion}
	for path, counts := range ownership {
		for key, count := range counts {
			if count < 1 {
				continue
			}
			document.Hooks = append(document.Hooks, settingsHookOwnershipRecord{
				Path: path, Event: key.Event, Matcher: key.Matcher, Command: key.Command, Count: count,
			})
		}
	}
	sort.Slice(document.Hooks, func(left, right int) bool {
		leftRecord, rightRecord := document.Hooks[left], document.Hooks[right]
		leftKey := leftRecord.Path + "\x00" + leftRecord.Event + "\x00" + leftRecord.Matcher + "\x00" + leftRecord.Command
		rightKey := rightRecord.Path + "\x00" + rightRecord.Event + "\x00" + rightRecord.Matcher + "\x00" + rightRecord.Command
		return leftKey < rightKey
	})
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode ownership ledger: %w", err)
	}
	return append(encoded, '\n'), nil
}

func sameSettingsHookOwnership(existing []byte, ownership map[string]settingsHookCounts) (bool, []byte, error) {
	encoded, err := encodeSettingsHookOwnership(ownership)
	if err != nil {
		return false, nil, err
	}
	return bytes.Equal(existing, encoded), encoded, nil
}
