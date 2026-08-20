CREATE TABLE IF NOT EXISTS epic_injections (
  session_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  injected_at TEXT NOT NULL,
  PRIMARY KEY (session_id, slug)
);
