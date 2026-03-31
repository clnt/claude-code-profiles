package config

import (
	"os"
	"path/filepath"
)

// Paths holds all resolved filesystem paths used by ccp.
type Paths struct {
	CCPHome    string // ~/.ccp
	ClaudeHome string // ~/.claude
	ClaudeJSON string // ~/.claude.json
}

// ResolvePaths returns resolved paths, checking env var overrides first.
// Env overrides (CCP_HOME, CLAUDE_HOME, CLAUDE_JSON) are primarily for testing.
func ResolvePaths() Paths {
	home, _ := os.UserHomeDir()

	ccpHome := os.Getenv("CCP_HOME")
	if ccpHome == "" {
		ccpHome = filepath.Join(home, ".ccp")
	}

	claudeHome := os.Getenv("CLAUDE_HOME")
	if claudeHome == "" {
		claudeHome = filepath.Join(home, ".claude")
	}

	claudeJSON := os.Getenv("CLAUDE_JSON")
	if claudeJSON == "" {
		claudeJSON = filepath.Join(home, ".claude.json")
	}

	return Paths{
		CCPHome:    ccpHome,
		ClaudeHome: claudeHome,
		ClaudeJSON: claudeJSON,
	}
}

// ProfilesDir returns the path to the profiles directory.
func (p Paths) ProfilesDir() string {
	return filepath.Join(p.CCPHome, "profiles")
}

// ProfileDir returns the path to a specific profile's directory.
func (p Paths) ProfileDir(name string) string {
	return filepath.Join(p.ProfilesDir(), name)
}

// DatabasePath returns the path to the SQLite database.
func (p Paths) DatabasePath() string {
	return filepath.Join(p.CCPHome, "ccp.db")
}

// StagingDir returns a timestamped staging directory path.
func (p Paths) StagingDir(timestamp string) string {
	return filepath.Join(p.CCPHome, "staging-"+timestamp)
}

// RollbackDir returns a timestamped rollback directory path.
func (p Paths) RollbackDir(timestamp string) string {
	return filepath.Join(p.CCPHome, "rollback-"+timestamp)
}
