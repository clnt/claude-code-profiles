package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
)

// Snapshot captures the config-relevant files from ~/.claude/ and ~/.claude.json
// into the given profile directory. This is a pure copy — it never modifies the
// live ~/.claude/ directory. Symlinks are only created later by Activate.
func Snapshot(paths config.Paths, profileDir string) error {
	claudeDir := filepath.Join(profileDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}

	// Copy directories (plugins, skills, agents)
	for _, dir := range config.ConfigSymlinkDirs {
		src := filepath.Join(paths.ClaudeHome, dir)
		dst := filepath.Join(claudeDir, dir)

		info, err := os.Lstat(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", dir, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Already a symlink (from an active profile) — resolve and copy
			resolved, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("resolve symlink %s: %w", dir, err)
			}
			if err := fsutil.CopyDir(resolved, dst); err != nil {
				return fmt.Errorf("copy symlinked dir %s: %w", dir, err)
			}
		} else {
			// Real directory — copy it
			if err := fsutil.CopyDir(src, dst); err != nil {
				return fmt.Errorf("copy dir %s: %w", dir, err)
			}
		}
	}

	// Copy files (settings.json, keybindings.json, etc.)
	for _, file := range config.ConfigCopyFiles {
		src := filepath.Join(paths.ClaudeHome, file)
		dst := filepath.Join(claudeDir, file)

		if _, err := os.Lstat(src); os.IsNotExist(err) {
			continue
		}
		if err := fsutil.CopyFile(src, dst); err != nil {
			return fmt.Errorf("copy file %s: %w", file, err)
		}
	}

	// Copy projects/*/memory/ directories (small, keep as copies)
	if err := snapshotProjects(paths.ClaudeHome, claudeDir); err != nil {
		return err
	}

	// Copy ~/.claude.json
	if err := snapshotClaudeJSON(paths.ClaudeJSON, profileDir); err != nil {
		return err
	}

	return nil
}

// SnapshotFilesOnly saves only the small copy-files and project memory from
// the live config into the profile. Symlink dirs are already in-place and
// don't need saving. Also removes stale profile-side files that the user deleted.
func SnapshotFilesOnly(paths config.Paths, profileDir string) error {
	claudeDir := filepath.Join(profileDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}

	for _, file := range config.ConfigCopyFiles {
		src := filepath.Join(paths.ClaudeHome, file)
		dst := filepath.Join(claudeDir, file)

		if _, err := os.Lstat(src); os.IsNotExist(err) {
			os.Remove(dst) // Remove stale profile-side file
			continue
		}
		if err := fsutil.CopyFile(src, dst); err != nil {
			return fmt.Errorf("copy file %s: %w", file, err)
		}
	}

	if err := snapshotProjects(paths.ClaudeHome, claudeDir); err != nil {
		return err
	}

	if err := snapshotClaudeJSON(paths.ClaudeJSON, profileDir); err != nil {
		return err
	}

	return nil
}

// snapshotProjects copies only the memory/ subdirectory from each project.
// It also removes profile-side project memory dirs that no longer exist in live config.
func snapshotProjects(claudeHome, claudeDir string) error {
	projectsDir := filepath.Join(claudeHome, config.ProjectsDirName)

	// Copy live project memory dirs to profile
	if _, err := os.Stat(projectsDir); err == nil {
		entries, err := os.ReadDir(projectsDir)
		if err != nil {
			return fmt.Errorf("read projects dir: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			memoryDir := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
			if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
				continue
			}

			dstMemoryDir := filepath.Join(claudeDir, config.ProjectsDirName, entry.Name(), config.ProjectSubdirInclude)
			if err := fsutil.CopyDir(memoryDir, dstMemoryDir); err != nil {
				return fmt.Errorf("copy project memory %s: %w", entry.Name(), err)
			}
		}
	}

	// Remove stale profile-side project memory dirs
	profileProjectsDir := filepath.Join(claudeDir, config.ProjectsDirName)
	if profileEntries, err := os.ReadDir(profileProjectsDir); err == nil {
		for _, entry := range profileEntries {
			if !entry.IsDir() {
				continue
			}
			liveMemDir := filepath.Join(projectsDir, entry.Name(), config.ProjectSubdirInclude)
			if _, err := os.Stat(liveMemDir); os.IsNotExist(err) {
				os.RemoveAll(filepath.Join(profileProjectsDir, entry.Name()))
			}
		}
	}

	return nil
}

// snapshotClaudeJSON copies ~/.claude.json into the profile directory.
// If ~/.claude.json has been deleted, removes the stale profile-side copy.
func snapshotClaudeJSON(claudeJSONPath, profileDir string) error {
	dst := filepath.Join(profileDir, "claude.json")

	if _, err := os.Stat(claudeJSONPath); os.IsNotExist(err) {
		os.Remove(dst) // Remove stale profile-side file
		return nil
	}

	if err := fsutil.CopyFile(claudeJSONPath, dst); err != nil {
		return fmt.Errorf("copy claude.json: %w", err)
	}

	return nil
}
