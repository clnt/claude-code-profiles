package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

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
//
// The switch is atomic: current state is backed up before any mutations, and
// restored on failure.
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

	if err := os.MkdirAll(paths.ClaudeHome, 0755); err != nil {
		return fmt.Errorf("create claude home: %w", err)
	}

	// Create rollback backup of current managed state
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	rollbackDir := paths.RollbackDir(ts)

	if err := backupManagedState(paths, rollbackDir); err != nil {
		os.RemoveAll(rollbackDir)
		return fmt.Errorf("backup current state: %w", err)
	}

	// Apply new profile state — rollback on failure
	if err := applyProfileState(paths, targetName); err != nil {
		restoreManagedState(paths, rollbackDir) // best-effort
		os.RemoveAll(rollbackDir)
		return fmt.Errorf("activate profile %q: %w", targetName, err)
	}

	// Success — clean up rollback dir
	os.RemoveAll(rollbackDir)

	// Update database
	if err := database.SetActiveProfile(targetName); err != nil {
		return fmt.Errorf("update active profile: %w", err)
	}
	_ = database.UpdateProfileTimestamp(targetName)

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

// symlinkBackup records the state of symlink-managed directories for rollback.
type symlinkBackup struct {
	// Symlinks maps dir name to symlink target (empty string = was not a symlink)
	Symlinks map[string]string `json:"symlinks"`
}

// backupManagedState saves the current managed state to rollbackDir for recovery.
func backupManagedState(paths config.Paths, rollbackDir string) error {
	if err := os.MkdirAll(rollbackDir, 0755); err != nil {
		return err
	}

	backupClaudeDir := filepath.Join(rollbackDir, "claude")
	if err := os.MkdirAll(backupClaudeDir, 0755); err != nil {
		return err
	}

	// Back up symlink dirs: record targets (or copy real dirs)
	backup := symlinkBackup{Symlinks: make(map[string]string)}
	for _, dir := range config.ConfigSymlinkDirs {
		livePath := filepath.Join(paths.ClaudeHome, dir)
		info, err := os.Lstat(livePath)
		if err != nil {
			continue // doesn't exist, nothing to back up
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(livePath)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", dir, err)
			}
			backup.Symlinks[dir] = target
		} else if info.IsDir() {
			// Real dir — copy it to rollback
			dst := filepath.Join(backupClaudeDir, dir)
			if err := fsutil.CopyDir(livePath, dst); err != nil {
				return fmt.Errorf("backup dir %s: %w", dir, err)
			}
			backup.Symlinks[dir] = "" // empty means "was a real dir"
		}
	}

	data, err := json.Marshal(backup)
	if err != nil {
		return fmt.Errorf("marshal symlink backup: %w", err)
	}
	if err := os.WriteFile(filepath.Join(rollbackDir, "symlinks.json"), data, 0644); err != nil {
		return err
	}

	// Back up copy-files
	for _, file := range config.ConfigCopyFiles {
		src := filepath.Join(paths.ClaudeHome, file)
		if _, err := os.Lstat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(backupClaudeDir, file)
		if err := fsutil.CopyFile(src, dst); err != nil {
			return fmt.Errorf("backup file %s: %w", file, err)
		}
	}

	// Back up claude.json
	if _, err := os.Stat(paths.ClaudeJSON); err == nil {
		dst := filepath.Join(rollbackDir, "claude.json")
		if err := fsutil.CopyFile(paths.ClaudeJSON, dst); err != nil {
			return fmt.Errorf("backup claude.json: %w", err)
		}
	}

	// Back up project memory
	projectsDir := filepath.Join(paths.ClaudeHome, config.ProjectsDirName)
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			memSrc := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
			if _, err := os.Stat(memSrc); os.IsNotExist(err) {
				continue
			}
			memDst := filepath.Join(backupClaudeDir, config.ProjectsDirName, entry.Name(), config.ProjectSubdirInclude)
			if err := os.MkdirAll(filepath.Dir(memDst), 0755); err != nil {
				return fmt.Errorf("mkdir for project memory backup: %w", err)
			}
			if err := fsutil.CopyDir(memSrc, memDst); err != nil {
				return fmt.Errorf("backup project memory %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// restoreManagedState restores live config from a rollback backup. Best-effort;
// errors are intentionally ignored since this is a recovery path.
//
//nolint:errcheck
func restoreManagedState(paths config.Paths, rollbackDir string) {
	backupClaudeDir := filepath.Join(rollbackDir, "claude")

	// Restore symlink dirs
	data, err := os.ReadFile(filepath.Join(rollbackDir, "symlinks.json"))
	if err == nil {
		var backup symlinkBackup
		if json.Unmarshal(data, &backup) == nil {
			for dir, target := range backup.Symlinks {
				livePath := filepath.Join(paths.ClaudeHome, dir)
				os.Remove(livePath)
				os.RemoveAll(livePath)
				if target != "" {
					// Was a symlink — restore it
					os.Symlink(target, livePath)
				} else {
					// Was a real dir — copy it back
					src := filepath.Join(backupClaudeDir, dir)
					if _, err := os.Stat(src); err == nil {
						fsutil.CopyDir(src, livePath)
					}
				}
			}
		}
	}

	// Restore copy-files
	for _, file := range config.ConfigCopyFiles {
		dst := filepath.Join(paths.ClaudeHome, file)
		src := filepath.Join(backupClaudeDir, file)
		os.Remove(dst)
		if _, err := os.Stat(src); err == nil {
			fsutil.CopyFile(src, dst)
		}
	}

	// Restore claude.json
	os.Remove(paths.ClaudeJSON)
	src := filepath.Join(rollbackDir, "claude.json")
	if _, err := os.Stat(src); err == nil {
		fsutil.CopyFile(src, paths.ClaudeJSON)
	}

	// Restore project memory
	projectsDir := filepath.Join(paths.ClaudeHome, config.ProjectsDirName)
	backupProjectsDir := filepath.Join(backupClaudeDir, config.ProjectsDirName)
	if entries, err := os.ReadDir(backupProjectsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			memSrc := filepath.Join(backupProjectsDir, entry.Name(), config.ProjectSubdirInclude)
			if _, err := os.Stat(memSrc); os.IsNotExist(err) {
				continue
			}
			memDst := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
			os.RemoveAll(memDst)
			os.MkdirAll(filepath.Dir(memDst), 0755)
			fsutil.CopyDir(memSrc, memDst)
		}
	}
}

// applyProfileState performs the actual filesystem mutations to switch profiles.
func applyProfileState(paths config.Paths, targetName string) error {
	targetDir := paths.ProfileDir(targetName)
	claudeDir := filepath.Join(targetDir, "claude")

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
					_ = os.MkdirAll(claudeDir, 0755)
					if err := os.Rename(livePath, profilePath); err != nil {
						// Cross-device fallback
						_ = fsutil.CopyDir(livePath, profilePath)
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

	// Copy claude.json (or remove if target doesn't have one)
	os.Remove(paths.ClaudeJSON)
	claudeJSONSrc := filepath.Join(targetDir, "claude.json")
	if _, err := os.Stat(claudeJSONSrc); err == nil {
		if err := fsutil.CopyFile(claudeJSONSrc, paths.ClaudeJSON); err != nil {
			return fmt.Errorf("copy claude.json: %w", err)
		}
	}

	return nil
}

// installProjectMemory copies project memory dirs from the profile into ~/.claude/projects/.
// It also removes live project memory dirs that are absent from the target profile.
func installProjectMemory(claudeDir, claudeHome string) error {
	projectsSrc := filepath.Join(claudeDir, config.ProjectsDirName)
	projectsDst := filepath.Join(claudeHome, config.ProjectsDirName)

	// Collect target profile's project names
	targetProjects := make(map[string]bool)
	if entries, err := os.ReadDir(projectsSrc); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				targetProjects[entry.Name()] = true
			}
		}
	}

	// Remove live project memory dirs absent from target
	if entries, err := os.ReadDir(projectsDst); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || targetProjects[entry.Name()] {
				continue
			}
			memDst := filepath.Join(projectsDst, entry.Name(), config.ProjectSubdirInclude)
			os.RemoveAll(memDst)
		}
	}

	// Copy target's project memory to live
	if _, err := os.Stat(projectsSrc); os.IsNotExist(err) {
		return nil
	}

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
