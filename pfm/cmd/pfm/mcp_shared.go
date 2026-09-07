package main

import (
	"context"
	"fmt"
	pfmengine "hostops/pfm/internal/engine"
	"io"
	"os"
	"strings"

	"hostops/pfm/internal/compose"
	"hostops/pfm/internal/mcpserv"
	"hostops/pfm/internal/store"
	"hostops/pfm/internal/transcript"
)

// mcpRuntime is the one bridge from the command package into MCP. Every
// callback below calls the same command primitive used by the CLI; MCP owns
// only argument/result adaptation.
func mcpRuntime(runtime commandRuntime) mcpserv.Runtime {
	return mcpserv.Runtime{
		Paths:          runtime.Paths,
		Accounts:       runtime.Config.Accounts,
		ConfigPath:     runtime.Config.Path,
		ClaudeBinary:   runtime.Config.Claude.Binary,
		CodexBinary:    runtime.Config.Codex.Binary,
		OpencodeBinary: runtime.Config.OpenCode.Binary,
		Operations:     mcpSharedOperations(runtime),
		Dispatch: func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
			if len(args) == 0 || args[0] != "chat" {
				fmt.Fprintln(stderr, "pfm: MCP dispatch requires chat argv")
				return 2
			}
			return runChatWithRuntime(args[1:], strings.NewReader(""), stdout, stderr, runtime)
		},
	}
}

func mcpSharedOperations(runtime commandRuntime) mcpserv.SharedOperations {
	return mcpserv.SharedOperations{
		List: func(ctx context.Context, input mcpserv.LSInput) (mcpserv.LSOutput, error) {
			if input.All && input.Killed {
				return mcpserv.LSOutput{}, fmt.Errorf("all and killed are mutually exclusive")
			}
			database, err := store.Open(store.WithWarningWriter(os.Stderr))
			if err != nil {
				return mcpserv.LSOutput{}, err
			}
			defer database.Close()
			view := compose.DefaultView
			if input.All {
				view = compose.AllView
			} else if input.Killed {
				view = compose.KilledView
			}
			limit := input.Limit
			if limit == 0 {
				limit = defaultMCPListLimit
			}
			if limit < 1 || limit > maxMCPListLimit {
				return mcpserv.LSOutput{}, fmt.Errorf(
					"limit must be between 1 and %d", maxMCPListLimit,
				)
			}
			// Query is the interactive picker's search box, NOT a filter: it
			// only seeds ui.Snapshot.InitialQuery. Passing the caller's
			// project into it returned the whole fleet while the schema
			// promised a filter, so the filter is applied here, over the rows.
			scan, err := scanFleet(ctx, database, scanRequest{
				View: view, Runtime: &runtime,
			}, os.Stderr)
			if err != nil {
				return mcpserv.LSOutput{}, err
			}
			filter := strings.ToLower(strings.TrimSpace(input.Project))
			rows := make([]mcpserv.ChatRow, 0, len(scan.Output.Rows))
			matched := 0
			truncated := false
			for _, row := range scan.Output.Rows {
				if excludedFromMCPList(row.Kind) {
					continue
				}
				if filter != "" && !matchesProjectFilter(row, filter) {
					continue
				}
				matched++
				if len(rows) >= limit {
					truncated = true
					continue
				}
				state := mcpRowState(row)
				session := row.SessionName
				if session == "" {
					session = row.ID
				}
				account := row.Account
				if account == 0 && len(row.Accounts) > 0 {
					account = row.Accounts[0]
				}
				rows = append(rows, mcpserv.ChatRow{
					Session: session, ID: row.ID, Engine: compose.EngineForKind(row.Kind),
					State: state, Dir: row.CWD, Project: row.Project, Name: row.Name,
					Account: account, Kind: row.Kind.String(), Killed: row.Killed,
					Socket: row.Socket, Pane: row.PaneID,
				})
			}
			return mcpserv.LSOutput{
				Rows: rows, Count: len(rows), Matched: matched,
				Truncated: truncated, KilledCount: scan.Output.KilledCount,
				Filter: input.Project,
			}, nil
		},
		Find: func(ctx context.Context, input mcpserv.FindInput) (mcpserv.FindOutput, error) {
			match, alternatives, err := findTranscriptContent([]byte(input.Excerpt), runtime)
			if err != nil {
				return mcpserv.FindOutput{}, err
			}
			matches := append([]transcriptMatch{match}, alternatives...)
			limit := input.Limit
			if limit == 0 {
				limit = 10
			}
			if limit < 1 || limit > 50 {
				return mcpserv.FindOutput{}, fmt.Errorf("limit must be between 1 and 50")
			}
			if len(matches) > limit {
				matches = matches[:limit]
			}
			candidates := make([]mcpserv.FindCandidate, 0, len(matches))
			for _, item := range matches {
				candidates = append(candidates, mcpserv.FindCandidate{
					ID: item.ID, Path: item.Path, Engine: string(pfmengine.Claude),
					Date: item.Last, Hits: item.Hits, Confirmed: true,
				})
			}
			return mcpserv.FindOutput{
				Candidates: candidates, Count: len(candidates),
				Needles: excerptNeedles(input.Excerpt),
			}, nil
		},
		Read: func(ctx context.Context, input mcpserv.ReadInput) (mcpserv.ReadOutput, error) {
			lastN := input.LastN
			if lastN == 0 {
				lastN = 1
			}
			if lastN < 1 || lastN > 200 {
				return mcpserv.ReadOutput{}, fmt.Errorf("last_n must be between 1 and 200")
			}
			maxBytes := input.MaxBytes
			if maxBytes == 0 {
				maxBytes = 64 << 10
			}
			if maxBytes < 1 || maxBytes > 1<<20 {
				return mcpserv.ReadOutput{}, fmt.Errorf("max_bytes must be between 1 and 1048576")
			}
			chat, entries, truncated, err := readChatEntries(ctx, input.Source, lastN, runtime)
			if err != nil {
				return mcpserv.ReadOutput{}, err
			}
			turns, bytes, budgetTruncated := boundMCPTurns(entries, maxBytes)
			truncated = truncated || budgetTruncated
			return mcpserv.ReadOutput{
				ID: chat.ID, Path: chat.Path, Engine: string(chat.Engine),
				Turns: turns, Count: len(turns), Truncated: truncated, Bytes: bytes,
			}, nil
		},
	}
}

func boundMCPTurns(entries []transcript.Entry, maxBytes int) ([]mcpserv.Turn, int, bool) {
	kept := make([]mcpserv.Turn, 0, len(entries))
	used := 0
	truncated := false
	for index := len(entries) - 1; index >= 0; index-- {
		available := maxBytes - used
		if available <= 0 {
			truncated = true
			break
		}
		entry := entries[index]
		text := entry.Text
		if len(text) > available {
			text = transcript.Truncate(text, available)
			truncated = true
		}
		used += len(text)
		kept = append(kept, mcpserv.Turn{
			Role: entry.Role, Text: text, Timestamp: entry.Timestamp,
		})
	}
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return kept, used, truncated
}

// defaultMCPListLimit keeps a full killed history (626 rows and growing on
// this host) from blowing the caller's tool-result budget in one answer;
// maxMCPListLimit is the ceiling an explicit caller may raise it to.
const (
	defaultMCPListLimit = 200
	maxMCPListLimit     = 1000
)

// excludedFromMCPList drops only the picker's "start a new chat" placeholder
// rows. A Booting row is a REAL chat that is still coming up, and is listed
// with state "booting".
func excludedFromMCPList(kind compose.Kind) bool {
	return kind == compose.NewClaude || kind == compose.NewCodex || kind == compose.NewOpencode
}

// mcpRowState is the one place a chat_ls row's state is decided.
//
// The killed-but-live arm is the honesty half of the kill fix: a kill that
// closed nothing still wrote its tombstone, so a row came back asserting BOTH
// things at once — killed:true beside state "idle" and kind "live-claude" —
// and no caller could tell a real kill from a de-listing. A contradiction is
// reported as a contradiction, never smoothed into one of its two halves.
func mcpRowState(row compose.Row) string {
	switch {
	case row.Killed && isLiveKind(row.Kind):
		return "killed-but-live"
	case row.Kind == compose.Booting:
		// A booting chat HAS a socket and already answers chat_inject by
		// name. Excluding it made chat_ls report a chat that exists as
		// simply absent for its first minute.
		return "booting"
	case isLiveKind(row.Kind):
		return "idle"
	default:
		return "resumable"
	}
}

// matchesProjectFilter is the chat_ls project filter: a case-insensitive
// substring of either the row's project label or its full directory, so both
// "professor" and "/home/x/.professor" select the same rows.
func matchesProjectFilter(row compose.Row, lowered string) bool {
	return strings.Contains(strings.ToLower(row.Project), lowered) ||
		strings.Contains(strings.ToLower(row.CWD), lowered)
}
