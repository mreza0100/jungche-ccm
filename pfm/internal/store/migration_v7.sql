CREATE TABLE IF NOT EXISTS oc_sessions (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  directory TEXT NOT NULL DEFAULT '',
  project_dir TEXT NOT NULL DEFAULT '',
  parent_id TEXT NOT NULL DEFAULT '',
  agent TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  first_prompt TEXT NOT NULL DEFAULT '',
  prompt_count INTEGER NOT NULL DEFAULT 0,
  tokens_input INTEGER NOT NULL DEFAULT 0,
  tokens_output INTEGER NOT NULL DEFAULT 0,
  cost_millicents INTEGER NOT NULL DEFAULT 0,
  time_created_ms INTEGER NOT NULL DEFAULT 0,
  time_updated_ms INTEGER NOT NULL DEFAULT 0,
  time_archived_ms INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS oc_sessions_updated
  ON oc_sessions(time_updated_ms DESC);
