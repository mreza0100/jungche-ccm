ALTER TABLE rollouts
  ADD COLUMN lineage_root TEXT NOT NULL DEFAULT '';

UPDATE rollouts
SET lineage_root = CASE
  WHEN session_id != '' THEN session_id
  WHEN parent_thread != '' THEN parent_thread
  ELSE id
END;

CREATE INDEX IF NOT EXISTS rollouts_lineage
  ON rollouts(lineage_root, mtime_ns DESC);
