package domain

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS airport (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL,
    sub_url            TEXT NOT NULL,
    source_type        TEXT NOT NULL DEFAULT 'URL',
    manual_content     TEXT,
    enabled            INTEGER NOT NULL DEFAULT 1,
    cached_raw_content TEXT,
    cached_userinfo    TEXT,
    last_fetched_at    TEXT,
    last_fetch_ok      INTEGER NOT NULL DEFAULT 0,
    last_fetch_error   TEXT
);

CREATE TABLE IF NOT EXISTS profile (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    slug                TEXT NOT NULL UNIQUE,
    target_format       TEXT NOT NULL,
    external_config     TEXT,
    enabled             INTEGER NOT NULL DEFAULT 1,
    cached_content      TEXT,
    cached_content_type TEXT,
    last_generated_at   TEXT
);

CREATE TABLE IF NOT EXISTS profile_airports (
    profile_id INTEGER NOT NULL,
    airport_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, airport_id)
);

CREATE TABLE IF NOT EXISTS app_settings (
    id                             INTEGER PRIMARY KEY,
    refresh_interval_seconds      INTEGER NOT NULL DEFAULT 3600,
    subconverter_timeout_seconds  INTEGER NOT NULL DEFAULT 30,
    airport_fetch_timeout_seconds INTEGER NOT NULL DEFAULT 20,
    subconverter_base_url         TEXT NOT NULL DEFAULT 'http://subconverter:25500',
    internal_base_url             TEXT NOT NULL DEFAULT 'http://app:8080',
    public_base_url               TEXT,
    internal_secret                TEXT
);
`

// Open opens (creating if needed) the SQLite file at path and applies the schema.
// A single connection is enforced: SQLite serializes writers anyway, and this
// avoids "database is locked" errors under modernc.org/sqlite without WAL tuning.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}
