package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = "1"

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates the SQLite database at the given path.
// It runs migrations if needed.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB connection for use in queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	// Create tables if they don't exist (idempotent)
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			name        TEXT PRIMARY KEY,
			description TEXT DEFAULT '',
			created_at  DATETIME NOT NULL,
			updated_at  DATETIME NOT NULL,
			is_default  INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS state (
			key   TEXT PRIMARY KEY,
			value TEXT
		);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// Set schema version if not present
	_, err = db.conn.Exec(`
		INSERT OR IGNORE INTO state (key, value) VALUES ('schema_version', ?)
	`, currentSchemaVersion)
	if err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}

	return nil
}
