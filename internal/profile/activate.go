package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
)

// Activate switches to the target profile.
//
// For directories (plugins, skills, agents): replaces real dirs in ~/.claude/
// with symlinks pointing at the target profile's copies. On first activation,
// existing real directories are moved into the target profile if it doesn't
// already have them.
//
// For files (settings.json, etc.) and project memory: copies small files.
func Activate(database *db.DB, paths config.Paths, targetName string, autoSave bool) error {
	exists, err := database.ProfileExists(targetName)
	if err != nil {
		return fmt.Errorf("check profile: %w", err)
	}
	if !exists {
		return fmt.Errorf("profile %q not found. See available profiles with: ccp list", targetName)
	}

	active, err := database.GetActiveProfile()
	if err != nil {
		return fmt.Errorf("get active profile: %w", err)
	}
	if active == targetName {
		return fmt.Errorf("profile %q is already active", targetName)
	}

	// Auto-save: only small files need saving (dirs are already in-place via symlinks)
	if autoSave && active != "" {
		activeDir := paths.ProfileDir(active)
		if _, err := os.Stat(activeDir); err == nil {
			if err := Save(database, paths, active); err != nil {
				return fmt.Errorf("auto-save current profile %q: %w", active, err)
			}
		}
	}

	targetDir := paths.ProfileDir(targetName)
	claudeDir := filepath.Join(targetDir, "claude")

	if err := os.MkdirAll(paths.ClaudeHome, 0755); err != nil {
		return fmt.Errorf("create claude home: %w", err)
	}

	// Swap symlink directories
	for _, dir := range config.ConfigSymlinkDirs {
		livePath := filepath.Join(paths.ClaudeHome, dir)
		profilePath := filepath.Join(claudeDir, dir)

		info, err := os.Lstat(livePath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// Already a symlink from a previous activation — just remove it
				os.Remove(livePath)
			} else if info.IsDir() {
				// Real directory — this is the first activation. Move it into the
				// target profile if the profile doesn't already have this dir.
				if _, err := os.Stat(profilePath); os.IsNotExist(err) {
					os.MkdirAll(claudeDir, 0755)
					if err := os.Rename(livePath, profilePath); err != nil {
						// Cross-device fallback
						fsutil.CopyDir(livePath, profilePath)
						os.RemoveAll(livePath)
					}
				} else {
					// Profile already has this dir (from ccp create) — remove the live one
					os.RemoveAll(livePath)
				}
			}
		}

		// Create symlink if the target profile has this directory
		if _, err := os.Stat(profilePath); err == nil {
			if err := os.Symlink(profilePath, livePath); err != nil {
				return fmt.Errorf("symlink %s: %w", dir, err)
			}
		}
	}

	// Copy small files
	for _, file := range config.ConfigCopyFiles {
		src := filepath.Join(claudeDir, file)
		dst := filepath.Join(paths.ClaudeHome, file)

		os.Remove(dst)

		if _, err := os.Stat(src); err == nil {
			if err := fsutil.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy %s: %w", file, err)
			}
		}
	}

	// Copy project memory directories
	if err := installProjectMemory(claudeDir, paths.ClaudeHome); err != nil {
		return fmt.Errorf("install project memory: %w", err)
	}

	// Copy claude.json
	claudeJSONSrc := filepath.Join(targetDir, "claude.json")
	if _, err := os.Stat(claudeJSONSrc); err == nil {
		if err := fsutil.CopyFile(claudeJSONSrc, paths.ClaudeJSON); err != nil {
			return fmt.Errorf("copy claude.json: %w", err)
		}
	}

	// Update database
	if err := database.SetActiveProfile(targetName); err != nil {
		return fmt.Errorf("update active profile: %w", err)
	}
	database.UpdateProfileTimestamp(targetName)

	return nil
}

// Save persists the current live config into the profile directory.
// Only saves small files and project memory — symlink dirs are already in-place.
func Save(database *db.DB, paths config.Paths, profileName string) error {
	profileDir := paths.ProfileDir(profileName)

	if err := SnapshotFilesOnly(paths, profileDir); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	return database.UpdateProfileTimestamp(profileName)
}

// installProjectMemory copies project memory dirs from the profile into ~/.claude/projects/.
func installProjectMemory(claudeDir, claudeHome string) error {
	projectsSrc := filepath.Join(claudeDir, config.ProjectsDirName)
	if _, err := os.Stat(projectsSrc); os.IsNotExist(err) {
		return nil
	}

	projectsDst := filepath.Join(claudeHome, config.ProjectsDirName)

	entries, err := os.ReadDir(projectsSrc)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		memSrc := filepath.Join(projectsSrc, entry.Name(), config.ProjectSubdirInclude)
		if _, err := os.Stat(memSrc); os.IsNotExist(err) {
			continue
		}
		memDst := filepath.Join(projectsDst, entry.Name(), config.ProjectSubdirInclude)

		os.RemoveAll(memDst)
		if err := os.MkdirAll(filepath.Dir(memDst), 0755); err != nil {
			return err
		}
		if err := fsutil.CopyDir(memSrc, memDst); err != nil {
			return fmt.Errorf("copy project memory %s: %w", entry.Name(), err)
		}
	}

	return nil
}
