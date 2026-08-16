CREATE TABLE IF NOT EXISTS transcripts (
  uuid TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  size INTEGER NOT NULL DEFAULT 0,
  mtime_ns INTEGER NOT NULL DEFAULT 0,
  parsed_offset INTEGER NOT NULL DEFAULT 0,
  cwd TEXT NOT NULL DEFAULT '',
  custom_title TEXT NOT NULL DEFAULT '',
  ai_title TEXT NOT NULL DEFAULT '',
  first_prompt TEXT NOT NULL DEFAULT '',
  last_prompt TEXT NOT NULL DEFAULT '',
  prompt_count INTEGER NOT NULL DEFAULT 0,
  is_bg INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS transcripts_mtime
  ON transcripts(mtime_ns DESC);

CREATE TABLE IF NOT EXISTS rollouts (
  id TEXT PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  size INTEGER NOT NULL DEFAULT 0,
  mtime_ns INTEGER NOT NULL DEFAULT 0,
  parsed_offset INTEGER NOT NULL DEFAULT 0,
  cwd TEXT NOT NULL DEFAULT '',
  user_thread INTEGER NOT NULL DEFAULT 0,
  session_id TEXT NOT NULL DEFAULT '',
  parent_thread TEXT NOT NULL DEFAULT '',
  first_prompt TEXT NOT NULL DEFAULT '',
  prompt_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS rollouts_mtime
  ON rollouts(mtime_ns DESC);

CREATE TABLE IF NOT EXISTS cx_names (
  id TEXT PRIMARY KEY,
  thread_name TEXT NOT NULL
);

-- RETIRED. Hides live in the fleet's shared store now (~/.cc/fleet.db, table
-- `hidden`, keyed by uuid) because operator state lives in the shared database
-- and a hide only one half could see is a hide that came back. This table is
-- created, populated by databases predating the move, adopted once into the
-- shared store on first open, and then never read again. It stays as the
-- rollback path: an older binary still finds its hides here.
CREATE TABLE IF NOT EXISTS hidden (
  id TEXT PRIMARY KEY,
  engine TEXT NOT NULL CHECK (engine IN ('cc', 'cx')),
  hidden_at INTEGER NOT NULL,
  baseline_prompts INTEGER
);

CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
