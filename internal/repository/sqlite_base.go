package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"configcenter/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLite struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLite, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + filepath.ToSlash(path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	store := &SQLite{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLite) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		schema,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite: %w", err)
		}
	}
	return nil
}

func (s *SQLite) Close() error { return s.db.Close() }

func (s *SQLite) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func mapSQLError(err error, resource string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, resource+" not found")
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") {
		return domain.NewError(domain.CodeConflict, resource+" already exists")
	}
	if strings.Contains(message, "foreign key constraint") {
		return domain.NewError(domain.CodeInvalid, resource+" references an invalid parent")
	}
	return fmt.Errorf("database operation failed: %v", err)
}

func timeText(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

const schema = `
CREATE TABLE IF NOT EXISTS applications (
 id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE,
 description TEXT NOT NULL DEFAULT '', access_token_hash TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS environments (
 id INTEGER PRIMARY KEY AUTOINCREMENT, application_id INTEGER NOT NULL REFERENCES applications(id),
 name TEXT NOT NULL, code TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
 current_version INTEGER NOT NULL DEFAULT 0, draft_revision INTEGER NOT NULL DEFAULT 0,
 last_published_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 UNIQUE(application_id, code)
);
CREATE TABLE IF NOT EXISTS draft_items (
 environment_id INTEGER NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
 config_key TEXT NOT NULL, config_value TEXT NOT NULL, value_type TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '', sensitive INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(environment_id, config_key)
);
CREATE TABLE IF NOT EXISTS releases (
 id INTEGER PRIMARY KEY AUTOINCREMENT, application_id INTEGER NOT NULL REFERENCES applications(id),
 environment_id INTEGER NOT NULL REFERENCES environments(id), version INTEGER NOT NULL,
 items_json BLOB NOT NULL, content BLOB NOT NULL, checksum TEXT NOT NULL,
 change_summary TEXT NOT NULL, source_version INTEGER, operation TEXT NOT NULL,
 operator TEXT NOT NULL, created_at TEXT NOT NULL,
 UNIQUE(environment_id, version)
);
CREATE INDEX IF NOT EXISTS idx_releases_env_created ON releases(environment_id, created_at DESC);
CREATE TABLE IF NOT EXISTS audit_logs (
 id INTEGER PRIMARY KEY AUTOINCREMENT, resource_type TEXT NOT NULL, resource_id INTEGER NOT NULL,
 action TEXT NOT NULL, operator TEXT NOT NULL, request_id TEXT NOT NULL,
 summary TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audits_created ON audit_logs(created_at DESC);
`
