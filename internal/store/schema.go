package store

import (
	"fmt"
	"strings"
)

// migrate creates tables idempotently. The schema is intentionally within the
// common subset of SQLite and Postgres: TEXT ids/timestamps, INTEGER booleans,
// REAL coordinates, no dialect-specific types or defaults.
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
			id               TEXT PRIMARY KEY,
			tag_id           TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			finder_name      TEXT NOT NULL DEFAULT '',
			finder_email     TEXT NOT NULL DEFAULT '',
			finder_phone     TEXT NOT NULL DEFAULT '',
			message          TEXT NOT NULL DEFAULT '',
			remote_ip        TEXT NOT NULL DEFAULT '',
			user_agent       TEXT NOT NULL DEFAULT '',
			finder_key       TEXT NOT NULL DEFAULT '',
			has_geo          INTEGER NOT NULL DEFAULT 0,
			geo_country      TEXT NOT NULL DEFAULT '',
			geo_region       TEXT NOT NULL DEFAULT '',
			geo_city         TEXT NOT NULL DEFAULT '',
			geo_lat          REAL NOT NULL DEFAULT 0,
			geo_lon          REAL NOT NULL DEFAULT 0,
			has_precise      INTEGER NOT NULL DEFAULT 0,
			precise_lat      REAL NOT NULL DEFAULT 0,
			precise_lon      REAL NOT NULL DEFAULT 0,
			precise_accuracy REAL NOT NULL DEFAULT 0,
			created_at       TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_found_reports_tag ON found_reports(tag_id)`,
		`CREATE INDEX IF NOT EXISTS idx_found_reports_throttle ON found_reports(tag_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS scan_events (
			id          TEXT PRIMARY KEY,
			tag_id      TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			remote_ip   TEXT NOT NULL DEFAULT '',
			user_agent  TEXT NOT NULL DEFAULT '',
			has_geo     INTEGER NOT NULL DEFAULT 0,
			geo_country TEXT NOT NULL DEFAULT '',
			geo_region  TEXT NOT NULL DEFAULT '',
			geo_city    TEXT NOT NULL DEFAULT '',
			geo_lat     REAL NOT NULL DEFAULT 0,
			geo_lon     REAL NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scan_events_tag ON scan_events(tag_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Additive column upgrades for pre-existing databases (created before these
	// columns existed). Duplicate-column errors are expected and ignored.
	for _, col := range []string{
		"ALTER TABLE found_reports ADD COLUMN user_agent TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE found_reports ADD COLUMN finder_key TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE found_reports ADD COLUMN has_geo INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN geo_country TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE found_reports ADD COLUMN geo_region TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE found_reports ADD COLUMN geo_city TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE found_reports ADD COLUMN geo_lat REAL NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN geo_lon REAL NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN has_precise INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN precise_lat REAL NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN precise_lon REAL NOT NULL DEFAULT 0",
		"ALTER TABLE found_reports ADD COLUMN precise_accuracy REAL NOT NULL DEFAULT 0",
	} {
		if _, err := s.db.Exec(col); err != nil && !isDuplicateColumn(err) {
			return fmt.Errorf("migrate alter: %w", err)
		}
	}
	return nil
}

// isDuplicateColumn matches the "column already exists" error on both SQLite
// ("duplicate column name") and Postgres ("already exists").
func isDuplicateColumn(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "duplicate column") || strings.Contains(m, "already exists")
}
