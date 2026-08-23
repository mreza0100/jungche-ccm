package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"hostops/pfm/internal/store"
)

// The OpenCode mirror. OpenCode keeps its sessions in a SQLite database at
// <root>/opencode.db (tables `session`, `project`, `session_input`), which is
// LIVE while any OpenCode TUI runs. The reader opens it READ-ONLY and reads
// both queries inside ONE transaction: WAL concurrency makes that a
// consistent, non-blocking snapshot, and a bounded busy timeout turns a
// hot-writer moment into an error instead of a hang. An index seconds behind
// a live chat is the same contract the Claude and Codex walkers already have.

type opencodeRow struct {
	sessionID      string
	title          string
	directory      string
	projectDir     sql.NullString
	parentID       sql.NullString
	agent          string
	model          string
	tokensInput    int64
	tokensOutput   int64
	cost           float64
	timeCreatedMS  int64
	timeUpdatedMS  int64
	timeArchivedMS sql.NullInt64
	promptCount    int64
	firstPrompt    sql.NullString
}

// ReadOpencodeSessions reads OpenCode's session store into OcSession rows.
// A missing opencode.db means the engine is not installed — that is a real,
// checkable answer ("no store exists"), not a silent failure, so it returns
// zero sessions and a nil error; a PRESENT database that cannot be opened or
// parsed is an error and says so.
func ReadOpencodeSessions(ctx context.Context, root string) (
	sessions []store.OcSession,
	returnErr error,
) {
	dbPath := filepath.Join(root, "opencode.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat opencode store %s: %w", dbPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("opencode store %s is a directory", dbPath)
	}

	// Direct read-only access to the LIVE store: WAL-mode readers never block
	// writers or tear state, and one transaction pins one consistent snapshot
	// across both queries. A bounded busy timeout converts a hot-writer
	// moment into an error we report, never an unbounded wait.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open opencode store read-only: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			returnErr = errors.Join(
				returnErr,
				fmt.Errorf("close opencode store read-only: %w", closeErr),
			)
		}
	}()
	rows, err := db.QueryContext(ctx, `
WITH input_counts AS (
  SELECT session_id, COUNT(*) AS prompt_count
  FROM session_input
  GROUP BY session_id
),
first_times AS (
  SELECT session_id, MIN(time_created) AS first_time
  FROM session_input
  GROUP BY session_id
),
first_ids AS (
  SELECT i.session_id, MIN(i.id) AS first_id
  FROM session_input i
  JOIN first_times f
    ON f.session_id = i.session_id AND f.first_time = i.time_created
  GROUP BY i.session_id
),
input_summary AS (
  SELECT c.session_id, c.prompt_count, i.prompt AS first_prompt
  FROM input_counts c
  JOIN first_ids f ON f.session_id = c.session_id
  JOIN session_input i ON i.id = f.first_id
)
SELECT s.id, s.title, s.directory, p.worktree, s.parent_id, s.agent, s.model,
       s.tokens_input, s.tokens_output, s.cost,
       s.time_created, s.time_updated, s.time_archived,
	   COALESCE(i.prompt_count, 0), i.first_prompt
FROM session s
LEFT JOIN project p ON p.id = s.project_id
LEFT JOIN input_summary i ON i.session_id = s.id`)
	if err != nil {
		return nil, fmt.Errorf("query opencode sessions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			returnErr = errors.Join(
				returnErr,
				fmt.Errorf("close opencode session rows: %w", closeErr),
			)
		}
	}()

	sessions = make([]store.OcSession, 0)
	for rows.Next() {
		var row opencodeRow
		if err := rows.Scan(
			&row.sessionID, &row.title, &row.directory,
			&row.projectDir, &row.parentID, &row.agent, &row.model,
			&row.tokensInput, &row.tokensOutput, &row.cost,
			&row.timeCreatedMS, &row.timeUpdatedMS, &row.timeArchivedMS,
			&row.promptCount, &row.firstPrompt,
		); err != nil {
			return nil, fmt.Errorf("scan opencode session: %w", err)
		}
		sessions = append(sessions, store.OcSession{
			ID:             row.sessionID,
			Title:          row.title,
			Directory:      row.directory,
			ProjectDir:     nonEmpty(row.projectDir),
			ParentID:       nonEmpty(row.parentID),
			Agent:          row.agent,
			Model:          compactModel(row.model),
			FirstPrompt:    clip(nonEmpty(row.firstPrompt)),
			PromptCount:    row.promptCount,
			TokensInput:    row.tokensInput,
			TokensOutput:   row.tokensOutput,
			CostMillicents: int64(row.cost * 1000),
			TimeCreatedMS:  row.timeCreatedMS,
			TimeUpdatedMS:  row.timeUpdatedMS,
			TimeArchivedMS: row.timeArchivedMS.Int64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate opencode sessions: %w", err)
	}
	return sessions, nil
}

func nonEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

// compactModel flattens OpenCode's model JSON ({"id":...,"providerID":...})
// to its provider/model word; anything unparsable travels verbatim rather
// than being dropped silently.
func compactModel(raw string) string {
	if raw == "" {
		return ""
	}
	var decoded struct {
		ID         string `json:"id"`
		ProviderID string `json:"providerID"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil || decoded.ID == "" {
		return raw
	}
	if decoded.ProviderID == "" {
		return decoded.ID
	}
	return decoded.ProviderID + "/" + decoded.ID
}

// clip bounds a first prompt to what a picker row can show.
func clip(prompt string) string {
	runes := []rune(prompt)
	const max = 200
	if len(runes) > max {
		return string(runes[:max])
	}
	return prompt
}

// syncOpencodeMirror replaces the oc_sessions mirror with one pass's view.
func syncOpencodeMirror(
	ctx context.Context,
	database *store.Store,
	root string,
	counters *Counters,
) error {
	sessions, err := ReadOpencodeSessions(ctx, root)
	if err != nil {
		return fmt.Errorf("read opencode sessions: %w", err)
	}
	if err := database.ReplaceOcSessions(ctx, sessions); err != nil {
		return fmt.Errorf("replace opencode mirror: %w", err)
	}
	counters.OcSessions = len(sessions)
	return nil
}
