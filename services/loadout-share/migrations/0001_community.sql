CREATE TABLE IF NOT EXISTS loadouts (
  code TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  character_name TEXT NOT NULL DEFAULT '',
  character_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  likes_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS likes (
  code TEXT NOT NULL,
  visitor_key TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (code, visitor_key),
  FOREIGN KEY (code) REFERENCES loadouts(code) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS comments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  author TEXT NOT NULL DEFAULT '匿名旅人',
  body TEXT NOT NULL,
  visitor_key TEXT NOT NULL,
  created_at TEXT NOT NULL,
  deleted INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY (code) REFERENCES loadouts(code) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS comments_code_created ON comments(code, created_at DESC);
