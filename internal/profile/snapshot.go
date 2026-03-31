package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
)

// Snapshot captures the config-relevant files from ~/.claude/ and ~/.claude.json
// into the given profile directory.
//
// For symlink dirs (plugins, skills, agents): if the source is a real directory
// it is moved into the profile and a symlink is created back. If it's already a
// symlink (pointing to another profile), the target is copied.
//
// For copy files (settings.json, etc.): always copied (they're small).
func Snapshot(paths config.Paths, profileDir string) error {
	claudeDir := filepath.Join(profileDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}

	// Handle symlink directories
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
			// Already a symlink (from another profile) — resolve and copy
			resolved, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("resolve symlink %s: %w", dir, err)
			}
			if err := fsutil.CopyDir(resolved, dst); err != nil {
				return fmt.Errorf("copy symlinked dir %s: %w", dir, err)
			}
		} else {
			// Real directory — move it into the profile (fast, same filesystem)
			if err := os.Rename(src, dst); err != nil {
				// Fall back to copy if rename fails (cross-device)
				if err := fsutil.CopyDir(src, dst); err != nil {
					return fmt.Errorf("copy dir %s: %w", dir, err)
				}
				os.RemoveAll(src)
			}
			// Create symlink back so Claude Code still works
			if err := os.Symlink(dst, src); err != nil {
				return fmt.Errorf("symlink %s: %w", dir, err)
			}
		}
	}

	// Handle copy files
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
// don't need saving.
func SnapshotFilesOnly(paths config.Paths, profileDir string) error {
	claudeDir := filepath.Join(profileDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}

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

	if err := snapshotProjects(paths.ClaudeHome, claudeDir); err != nil {
		return err
	}

	if err := snapshotClaudeJSON(paths.ClaudeJSON, profileDir); err != nil {
		return err
	}

	return nil
}

// snapshotProjects copies only the memory/ subdirectory from each project.
func snapshotProjects(claudeHome, claudeDir string) error {
	projectsDir := filepath.Join(claudeHome, config.ProjectsDirName)
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil
	}

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

	return nil
}

// snapshotClaudeJSON copies ~/.claude.json into the profile directory.
func snapshotClaudeJSON(claudeJSONPath, profileDir string) error {
	if _, err := os.Stat(claudeJSONPath); os.IsNotExist(err) {
		return nil
	}

	dst := filepath.Join(profileDir, "claude.json")
	if err := fsutil.CopyFile(claudeJSONPath, dst); err != nil {
		return fmt.Errorf("copy claude.json: %w", err)
	}

	return nil
}
