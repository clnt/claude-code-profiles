package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Profile represents a row in the profiles table.
type Profile struct {
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	IsDefault   bool
}

// CreateProfile inserts a new profile into the database.
func (db *DB) CreateProfile(name, description string, isDefault bool) error {
	now := time.Now().UTC()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if isDefault {
		if _, err := tx.Exec(`UPDATE profiles SET is_default = 0 WHERE is_default = 1`); err != nil {
			return fmt.Errorf("clear default: %w", err)
		}
	}

	_, err = tx.Exec(
		`INSERT INTO profiles (name, description, created_at, updated_at, is_default) VALUES (?, ?, ?, ?, ?)`,
		name, description, now, now, boolToInt(isDefault),
	)
	if err != nil {
		return fmt.Errorf("insert profile: %w", err)
	}

	return tx.Commit()
}

// GetProfile returns a single profile by name.
func (db *DB) GetProfile(name string) (*Profile, error) {
	row := db.conn.QueryRow(
		`SELECT name, description, created_at, updated_at, is_default FROM profiles WHERE name = ?`,
		name,
	)
	return scanProfile(row)
}

// ListProfiles returns all profiles ordered by name.
func (db *DB) ListProfiles() ([]Profile, error) {
	rows, err := db.conn.Query(
		`SELECT name, description, created_at, updated_at, is_default FROM profiles ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("query profiles: %w", err)
	}
	defer rows.Close()

	var profiles []Profile
	for rows.Next() {
		var p Profile
		var isDefault int
		if err := rows.Scan(&p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &isDefault); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		p.IsDefault = isDefault != 0
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// DeleteProfile removes a profile from the database.
func (db *DB) DeleteProfile(name string) error {
	result, err := db.conn.Exec(`DELETE FROM profiles WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("profile %q not found", name)
	}
	return nil
}

// UpdateProfileTimestamp updates the updated_at field for a profile.
func (db *DB) UpdateProfileTimestamp(name string) error {
	_, err := db.conn.Exec(
		`UPDATE profiles SET updated_at = ? WHERE name = ?`,
		time.Now().UTC(), name,
	)
	return err
}

// SetDefault sets the given profile as the default, clearing any previous default.
func (db *DB) SetDefault(name string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Verify profile exists
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM profiles WHERE name = ?`, name).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("profile %q not found", name)
	}

	if _, err := tx.Exec(`UPDATE profiles SET is_default = 0 WHERE is_default = 1`); err != nil {
		return fmt.Errorf("clear default: %w", err)
	}
	if _, err := tx.Exec(`UPDATE profiles SET is_default = 1 WHERE name = ?`, name); err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return tx.Commit()
}

// GetDefault returns the default profile, or nil if none is set.
func (db *DB) GetDefault() (*Profile, error) {
	row := db.conn.QueryRow(
		`SELECT name, description, created_at, updated_at, is_default FROM profiles WHERE is_default = 1`,
	)
	p, err := scanProfile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// ProfileExists returns true if a profile with the given name exists.
func (db *DB) ProfileExists(name string) (bool, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM profiles WHERE name = ?`, name).Scan(&count)
	return count > 0, err
}

// ProfileCount returns the total number of profiles.
func (db *DB) ProfileCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count)
	return count, err
}

// State operations

// GetState returns the value for a state key, or empty string if not found.
func (db *DB) GetState(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetState sets a state key-value pair (upsert).
func (db *DB) SetState(key, value string) error {
	_, err := db.conn.Exec(
		`INSERT INTO state (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`,
		key, value, value,
	)
	return err
}

// GetActiveProfile returns the name of the active profile, or empty string if none.
func (db *DB) GetActiveProfile() (string, error) {
	return db.GetState("active_profile")
}

// SetActiveProfile sets the active profile name in the state table.
func (db *DB) SetActiveProfile(name string) error {
	return db.SetState("active_profile", name)
}

// ClearActiveProfile removes the active profile state.
func (db *DB) ClearActiveProfile() error {
	_, err := db.conn.Exec(`DELETE FROM state WHERE key = 'active_profile'`)
	return err
}

// helpers

func scanProfile(row *sql.Row) (*Profile, error) {
	var p Profile
	var isDefault int
	err := row.Scan(&p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &isDefault)
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault != 0
	return &p, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
