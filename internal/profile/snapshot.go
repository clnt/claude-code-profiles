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
func Snapshot(paths config.Paths, profileDir string) error {
	claudeDir := filepath.Join(profileDir, "claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude dir: %w", err)
	}

	// Copy whitelisted files and directories from ~/.claude/
	for _, item := range config.ConfigIncludeList {
		src := filepath.Join(paths.ClaudeHome, item)
		dst := filepath.Join(claudeDir, item)

		info, err := os.Lstat(src)
		if os.IsNotExist(err) {
			continue // Skip items that don't exist
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", item, err)
		}

		if info.IsDir() {
			if err := fsutil.CopyDir(src, dst); err != nil {
				return fmt.Errorf("copy dir %s: %w", item, err)
			}
		} else {
			if err := fsutil.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy file %s: %w", item, err)
			}
		}
	}

	// Copy projects/*/memory/ directories
	if err := snapshotProjects(paths.ClaudeHome, claudeDir); err != nil {
		return err
	}

	// Copy ~/.claude.json
	if err := snapshotClaudeJSON(paths.ClaudeJSON, profileDir); err != nil {
		return err
	}

	return nil
}

// snapshotProjects copies only the memory/ subdirectory from each project
// in ~/.claude/projects/.
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
