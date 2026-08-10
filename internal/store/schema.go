package store

import "fmt"

// migrate creates tables idempotently. The schema is intentionally within the
// common subset of SQLite and Postgres: TEXT ids/timestamps, INTEGER booleans,
// no dialect-specific types or defaults.
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id          TEXT PRIMARY KEY,
			email       TEXT NOT NULL UNIQUE,
			created_at  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id          TEXT PRIMARY KEY,
			guid        TEXT NOT NULL UNIQUE,
			user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name        TEXT NOT NULL DEFAULT '',
			show_email  INTEGER NOT NULL DEFAULT 0,
			show_phone  INTEGER NOT NULL DEFAULT 0,
			phone       TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_user ON tags(user_id)`,
		`CREATE TABLE IF NOT EXISTS login_tokens (
			id          TEXT PRIMARY KEY,
			token_hash  TEXT NOT NULL UNIQUE,
			email       TEXT NOT NULL,
			expires_at  TEXT NOT NULL,
			used_at     TEXT,
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_login_tokens_hash ON login_tokens(token_hash)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id          TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at  TEXT NOT NULL,
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		`CREATE TABLE IF NOT EXISTS found_reports (
			id           TEXT PRIMARY KEY,
			tag_id       TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			finder_name  TEXT NOT NULL DEFAULT '',
			finder_email TEXT NOT NULL DEFAULT '',
			finder_phone TEXT NOT NULL DEFAULT '',
			message      TEXT NOT NULL DEFAULT '',
			remote_ip    TEXT NOT NULL DEFAULT '',
			created_at   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_found_reports_tag ON found_reports(tag_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
