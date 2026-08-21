CREATE TABLE IF NOT EXISTS chat_summaries (
  transcript_path TEXT NOT NULL,
  last_offset INTEGER NOT NULL,
  summary TEXT NOT NULL,
  PRIMARY KEY (transcript_path, last_offset)
);
