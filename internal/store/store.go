// Package store is the persistence layer. It supports both SQLite (pure-Go,
// single-file, zero-config) and Postgres (multi-user) behind one API. The DSN
// scheme selects the driver; SQL is written portably (TEXT timestamps, INTEGER
// booleans, `?` placeholders rebound per dialect).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib" // postgres driver "pgx"
	_ "modernc.org/sqlite"             // pure-Go sqlite driver "sqlite"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// Dialect identifies the active backend for the handful of places SQL differs.
type Dialect string

const (
	SQLite   Dialect = "sqlite"
	Postgres Dialect = "postgres"
)

type Store struct {
	db      *sqlx.DB
	dialect Dialect
}

// Models. Booleans are stored as INTEGER (0/1) and timestamps as RFC3339 TEXT
// for cross-dialect portability.

type User struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"-"`
}

type Tag struct {
	ID        string `db:"id"`
	GUID      string `db:"guid"`
	UserID    string `db:"user_id"`
	Name      string `db:"name"`
	ShowEmail bool   `db:"-"`
	ShowPhone bool   `db:"-"`
	Phone     string `db:"phone"`
	CreatedAt time.Time
}

type FoundReport struct {
	ID          string
	TagID       string
	FinderName  string
	FinderEmail string
	FinderPhone string
	Message     string
	RemoteIP    string
	UserAgent   string
	FinderKey   string // opaque per-finder id (cookie), for re-report throttling
	CreatedAt   time.Time

	// IP-derived approximate location.
	HasGeo    bool
	GeoCountry string
	GeoRegion  string
	GeoCity    string
	GeoLat     float64
	GeoLon     float64

	// Precise location, only when the finder explicitly consented (browser geolocation).
	HasPrecise      bool
	PreciseLat      float64
	PreciseLon      float64
	PreciseAccuracy float64 // meters
}

// ScanEvent records a QR scan (a load of the public /found/<guid> page) with the
// connection metadata, for owner/admin analytics.
type ScanEvent struct {
	ID         string
	TagID      string
	RemoteIP   string
	UserAgent  string
	HasGeo     bool
	GeoCountry string
	GeoRegion  string
	GeoCity    string
	GeoLat     float64
	GeoLon     float64
	CreatedAt  time.Time
}

// UserStat is an admin-panel row: a user plus activity counts.
type UserStat struct {
	Email       string    `db:"email"`
	CreatedAt   time.Time `db:"-"`
	CreatedRaw  string    `db:"created_at"`
	TagCount    int       `db:"tag_count"`
	ReportCount int       `db:"report_count"`
}

// Open resolves the DSN, opens the pool, and applies migrations.
func Open(dsn string) (*Store, error) {
	dialect, driver, conn, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Connect(driver, conn)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", dialect, err)
	}
	if dialect == SQLite {
		// Single writer; enable WAL + FK enforcement + sane busy handling.
		db.SetMaxOpenConns(1)
		for _, pragma := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=5000",
			"PRAGMA foreign_keys=ON",
		} {
			if _, err := db.Exec(pragma); err != nil {
				return nil, fmt.Errorf("sqlite pragma: %w", err)
			}
		}
	}
	s := &Store{db: db, dialect: dialect}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Dialect() Dialect { return s.dialect }

// parseDSN maps a user-facing URL onto a driver + connection string.
//
//	sqlite://./data/lost.db      -> driver "sqlite", path ./data/lost.db
//	sqlite:///data/lost.db       -> driver "sqlite", path /data/lost.db
//	postgres://user:pw@host/db   -> driver "pgx", full URL
func parseDSN(dsn string) (Dialect, string, string, error) {
	switch {
	case strings.HasPrefix(dsn, "sqlite://"):
		path := strings.TrimPrefix(dsn, "sqlite://")
		if path == "" || path == ":memory:" {
			return SQLite, "sqlite", ":memory:", nil
		}
		// Ensure the parent directory exists for a file DB.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", "", "", fmt.Errorf("create db dir: %w", err)
		}
		return SQLite, "sqlite", path, nil
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		if _, err := url.Parse(dsn); err != nil {
			return "", "", "", fmt.Errorf("invalid postgres dsn: %w", err)
		}
		return Postgres, "pgx", dsn, nil
	default:
		return "", "", "", fmt.Errorf("unsupported LOST_DB_URL scheme (want sqlite:// or postgres://): %q", dsn)
	}
}

// rebind converts `?` placeholders to the dialect's native form.
func (s *Store) rebind(q string) string { return s.db.Rebind(q) }

// nowStr is the canonical timestamp encoding used across the schema.
func nowStr() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// context helper kept short for handlers
func (s *Store) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

var _ = sql.ErrNoRows
