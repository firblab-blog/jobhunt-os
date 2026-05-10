CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE
    CHECK (length(token_hash) = 64),
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT,
  user_agent_hash TEXT
    CHECK (user_agent_hash IS NULL OR length(user_agent_hash) = 64),
  client_ip_prefix TEXT,
  CHECK (expires_at > created_at),
  CHECK (last_seen_at >= created_at)
);

CREATE INDEX idx_sessions_expires_at
  ON sessions(expires_at);

CREATE INDEX idx_sessions_last_seen_at
  ON sessions(last_seen_at);

CREATE INDEX idx_sessions_revoked_at
  ON sessions(revoked_at);
