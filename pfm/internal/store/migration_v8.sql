ALTER TABLE transcripts
  ADD COLUMN activity_ns INTEGER NOT NULL DEFAULT 0;

UPDATE transcripts
SET activity_ns = mtime_ns
WHERE activity_ns = 0;

CREATE INDEX IF NOT EXISTS transcripts_activity
  ON transcripts(activity_ns DESC);
