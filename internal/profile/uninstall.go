package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
)

// Uninstall resolves any symlinks in ~/.claude/ back to real directories,
// then removes the entire .ccp data directory. After this, Claude Code's
// config is fully standalone with no dependency on ccp.
func Uninstall(paths config.Paths) error {
	// Resolve symlink directories back to real copies
	for _, dir := range config.ConfigSymlinkDirs {
		livePath := filepath.Join(paths.ClaudeHome, dir)

		info, err := os.Lstat(livePath)
		if err != nil {
			continue // doesn't exist, nothing to do
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue // already a real dir, nothing to do
		}

		target, err := os.Readlink(livePath)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", dir, err)
		}

		// Remove the symlink
		if err := os.Remove(livePath); err != nil {
			return fmt.Errorf("remove symlink %s: %w", dir, err)
		}

		// Copy the target directory contents back as a real directory
		if _, err := os.Stat(target); err == nil {
			if err := fsutil.CopyDir(target, livePath); err != nil {
				return fmt.Errorf("copy %s from profile: %w", dir, err)
			}
		}
	}

	// Remove the entire .ccp data directory
	if err := os.RemoveAll(paths.CCPHome); err != nil {
		return fmt.Errorf("remove ccp data directory: %w", err)
	}

	return nil
}
