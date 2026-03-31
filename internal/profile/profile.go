package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
)

var validName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// ValidateName checks if a profile name is valid.
func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must start with a letter, contain only alphanumeric/hyphen/underscore, max 64 chars", name)
	}
	return nil
}

// Create creates a new profile by snapshotting the current Claude Code config.
// If blank is true, creates an empty profile directory with no config files.
func Create(database *db.DB, paths config.Paths, name, description string, blank, setDefault bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	exists, err := database.ProfileExists(name)
	if err != nil {
		return fmt.Errorf("check existence: %w", err)
	}
	if exists {
		return fmt.Errorf("profile %q already exists", name)
	}

	profileDir := paths.ProfileDir(name)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}

	if blank {
		// Scaffold empty directories so activate can symlink to them
		claudeDir := filepath.Join(profileDir, "claude")
		for _, dir := range config.ConfigSymlinkDirs {
			if err := os.MkdirAll(filepath.Join(claudeDir, dir), 0755); err != nil {
				os.RemoveAll(profileDir)
				return fmt.Errorf("create %s: %w", dir, err)
			}
		}
	} else {
		if _, err := os.Stat(paths.ClaudeHome); os.IsNotExist(err) {
			os.RemoveAll(profileDir)
			return fmt.Errorf("claude code configuration not found at %s — is Claude Code installed?", paths.ClaudeHome)
		}

		if err := Snapshot(paths, profileDir); err != nil {
			os.RemoveAll(profileDir)
			return fmt.Errorf("snapshot config: %w", err)
		}
	}

	// Auto-set as default if it's the first profile
	if !setDefault {
		count, err := database.ProfileCount()
		if err == nil && count == 0 {
			setDefault = true
		}
	}

	if err := database.CreateProfile(name, description, setDefault); err != nil {
		os.RemoveAll(profileDir)
		return fmt.Errorf("save to database: %w", err)
	}

	return nil
}

// Delete removes a profile from the database and filesystem.
func Delete(database *db.DB, paths config.Paths, name string) error {
	active, err := database.GetActiveProfile()
	if err != nil {
		return fmt.Errorf("check active profile: %w", err)
	}
	if active == name {
		return fmt.Errorf("cannot delete the active profile %q. Switch to another profile first with: ccp use <other>", name)
	}

	if err := database.DeleteProfile(name); err != nil {
		return err
	}

	profileDir := paths.ProfileDir(name)
	if err := os.RemoveAll(profileDir); err != nil {
		return fmt.Errorf("remove profile directory: %w", err)
	}

	return nil
}
