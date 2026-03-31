package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
)

// Activate switches to the target profile using a stage-backup-install-commit pattern.
// If autoSave is true and there is an active profile, it saves current config first.
func Activate(database *db.DB, paths config.Paths, targetName string, autoSave bool) error {
	// Verify target exists
	exists, err := database.ProfileExists(targetName)
	if err != nil {
		return fmt.Errorf("check profile: %w", err)
	}
	if !exists {
		return fmt.Errorf("profile %q not found. See available profiles with: ccp list", targetName)
	}

	// Check if already active
	active, err := database.GetActiveProfile()
	if err != nil {
		return fmt.Errorf("get active profile: %w", err)
	}
	if active == targetName {
		return fmt.Errorf("profile %q is already active", targetName)
	}

	// Auto-save current config to active profile
	if autoSave && active != "" {
		activeDir := paths.ProfileDir(active)
		if _, err := os.Stat(activeDir); err == nil {
			if err := Save(database, paths, active); err != nil {
				return fmt.Errorf("auto-save current profile %q: %w", active, err)
			}
		}
	}

	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	stagingDir := paths.StagingDir(timestamp)
	rollbackDir := paths.RollbackDir(timestamp)

	// Cleanup on any exit path
	defer os.RemoveAll(stagingDir)

	// Phase 1: Stage - copy target profile files to staging
	targetDir := paths.ProfileDir(targetName)
	if err := stageProfile(targetDir, stagingDir); err != nil {
		return fmt.Errorf("stage profile: %w", err)
	}

	// Phase 2: Backup - move current config to rollback
	if err := backupCurrentConfig(paths, rollbackDir); err != nil {
		return fmt.Errorf("backup current config: %w", err)
	}

	// Phase 3: Install - move staged files into ~/.claude/ and copy claude.json
	if err := installProfile(stagingDir, paths); err != nil {
		// Rollback on failure
		rollbackErr := restoreFromRollback(rollbackDir, paths)
		if rollbackErr != nil {
			return fmt.Errorf("install failed: %w (rollback also failed: %v)", err, rollbackErr)
		}
		return fmt.Errorf("install failed (rolled back): %w", err)
	}

	// Phase 4: Commit - update database
	if err := database.SetActiveProfile(targetName); err != nil {
		// Rollback
		restoreFromRollback(rollbackDir, paths)
		return fmt.Errorf("update active profile: %w", err)
	}
	database.UpdateProfileTimestamp(targetName)

	// Phase 5: Cleanup
	os.RemoveAll(rollbackDir)

	return nil
}

// Save snapshots the current config into the given profile directory.
func Save(database *db.DB, paths config.Paths, profileName string) error {
	profileDir := paths.ProfileDir(profileName)

	// Clear existing profile files (but not the directory itself)
	claudeDir := filepath.Join(profileDir, "claude")
	os.RemoveAll(claudeDir)
	os.Remove(filepath.Join(profileDir, "claude.json"))

	if err := Snapshot(paths, profileDir); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	return database.UpdateProfileTimestamp(profileName)
}

// stageProfile copies the target profile's config files into the staging directory.
func stageProfile(profileDir, stagingDir string) error {
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return err
	}

	// Copy claude/ directory if it exists
	claudeSrc := filepath.Join(profileDir, "claude")
	if _, err := os.Stat(claudeSrc); err == nil {
		claudeDst := filepath.Join(stagingDir, "claude")
		if err := fsutil.CopyDir(claudeSrc, claudeDst); err != nil {
			return fmt.Errorf("copy claude dir: %w", err)
		}
	}

	// Copy claude.json if it exists
	jsonSrc := filepath.Join(profileDir, "claude.json")
	if _, err := os.Stat(jsonSrc); err == nil {
		jsonDst := filepath.Join(stagingDir, "claude.json")
		if err := fsutil.CopyFile(jsonSrc, jsonDst); err != nil {
			return fmt.Errorf("copy claude.json: %w", err)
		}
	}

	return nil
}

// backupCurrentConfig moves config-relevant items from ~/.claude/ to the rollback directory.
func backupCurrentConfig(paths config.Paths, rollbackDir string) error {
	if err := os.MkdirAll(rollbackDir, 0755); err != nil {
		return err
	}

	claudeRollback := filepath.Join(rollbackDir, "claude")
	if err := os.MkdirAll(claudeRollback, 0755); err != nil {
		return err
	}

	// Move whitelisted items from ~/.claude/ to rollback
	for _, item := range config.ConfigIncludeList {
		src := filepath.Join(paths.ClaudeHome, item)
		if _, err := os.Lstat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(claudeRollback, item)
		if err := fsutil.MoveEntry(src, dst); err != nil {
			return fmt.Errorf("backup %s: %w", item, err)
		}
	}

	// Move projects/*/memory/ directories
	projectsDir := filepath.Join(paths.ClaudeHome, config.ProjectsDirName)
	if _, err := os.Stat(projectsDir); err == nil {
		entries, err := os.ReadDir(projectsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				memSrc := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
				if _, err := os.Stat(memSrc); os.IsNotExist(err) {
					continue
				}
				memDst := filepath.Join(claudeRollback, config.ProjectsDirName, entry.Name(), config.ProjectSubdirInclude)
				if err := os.MkdirAll(filepath.Dir(memDst), 0755); err != nil {
					return fmt.Errorf("mkdir for project backup: %w", err)
				}
				if err := fsutil.MoveEntry(memSrc, memDst); err != nil {
					return fmt.Errorf("backup project memory %s: %w", entry.Name(), err)
				}
			}
		}
	}

	// Backup ~/.claude.json
	if _, err := os.Stat(paths.ClaudeJSON); err == nil {
		dst := filepath.Join(rollbackDir, "claude.json")
		if err := fsutil.CopyFile(paths.ClaudeJSON, dst); err != nil {
			return fmt.Errorf("backup claude.json: %w", err)
		}
	}

	return nil
}

// installProfile moves files from staging into the real config locations.
func installProfile(stagingDir string, paths config.Paths) error {
	// Ensure ~/.claude/ exists
	if err := os.MkdirAll(paths.ClaudeHome, 0755); err != nil {
		return fmt.Errorf("create claude home: %w", err)
	}

	// Install claude/ contents
	claudeSrc := filepath.Join(stagingDir, "claude")
	if _, err := os.Stat(claudeSrc); err == nil {
		entries, err := os.ReadDir(claudeSrc)
		if err != nil {
			return fmt.Errorf("read staged claude dir: %w", err)
		}
		for _, entry := range entries {
			src := filepath.Join(claudeSrc, entry.Name())
			dst := filepath.Join(paths.ClaudeHome, entry.Name())

			// For projects, merge rather than replace (to preserve session data)
			if entry.Name() == config.ProjectsDirName && entry.IsDir() {
				if err := installProjects(src, paths.ClaudeHome); err != nil {
					return fmt.Errorf("install projects: %w", err)
				}
				continue
			}

			if err := fsutil.MoveEntry(src, dst); err != nil {
				return fmt.Errorf("install %s: %w", entry.Name(), err)
			}
		}
	}

	// Install claude.json
	jsonSrc := filepath.Join(stagingDir, "claude.json")
	if _, err := os.Stat(jsonSrc); err == nil {
		if err := fsutil.CopyFile(jsonSrc, paths.ClaudeJSON); err != nil {
			return fmt.Errorf("install claude.json: %w", err)
		}
	}

	return nil
}

// installProjects merges project memory directories from staging into ~/.claude/projects/.
func installProjects(stagedProjectsDir, claudeHome string) error {
	projectsDir := filepath.Join(claudeHome, config.ProjectsDirName)
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(stagedProjectsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		memSrc := filepath.Join(stagedProjectsDir, entry.Name(), config.ProjectSubdirInclude)
		if _, err := os.Stat(memSrc); os.IsNotExist(err) {
			continue
		}
		memDst := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
		if err := os.MkdirAll(filepath.Dir(memDst), 0755); err != nil {
			return err
		}
		if err := fsutil.MoveEntry(memSrc, memDst); err != nil {
			return fmt.Errorf("install project memory %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// restoreFromRollback restores config from the rollback directory.
func restoreFromRollback(rollbackDir string, paths config.Paths) error {
	claudeRollback := filepath.Join(rollbackDir, "claude")
	if _, err := os.Stat(claudeRollback); err == nil {
		entries, err := os.ReadDir(claudeRollback)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			src := filepath.Join(claudeRollback, entry.Name())
			dst := filepath.Join(paths.ClaudeHome, entry.Name())
			// Remove whatever was partially installed
			os.RemoveAll(dst)
			if err := fsutil.MoveEntry(src, dst); err != nil {
				return fmt.Errorf("restore %s: %w", entry.Name(), err)
			}
		}
	}

	// Restore claude.json
	jsonBackup := filepath.Join(rollbackDir, "claude.json")
	if _, err := os.Stat(jsonBackup); err == nil {
		if err := fsutil.CopyFile(jsonBackup, paths.ClaudeJSON); err != nil {
			return fmt.Errorf("restore claude.json: %w", err)
		}
	}

	return nil
}
