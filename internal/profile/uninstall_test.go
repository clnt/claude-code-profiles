package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clnt/claude-code-profiles/internal/config"
)

func TestUninstall_ResolvesSymlinks(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create and activate a profile so symlinks are in place
	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)

	// Verify plugins is a symlink
	if _, err := os.Readlink(filepath.Join(paths.ClaudeHome, "plugins")); err != nil {
		t.Fatal("plugins should be a symlink before uninstall")
	}

	database.Close() //nolint:errcheck

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	// plugins should now be a real directory with the original content
	info, err := os.Lstat(filepath.Join(paths.ClaudeHome, "plugins"))
	if err != nil {
		t.Fatal("plugins should exist after uninstall")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("plugins should be a real directory after uninstall, not a symlink")
	}

	data, err := os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if err != nil {
		t.Fatal("plugins/config.json should exist after uninstall")
	}
	if string(data) != `{"plugins":"original"}` {
		t.Errorf("plugins config = %q, want original", string(data))
	}
}

func TestUninstall_PreservesCopyFiles(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)
	database.Close() //nolint:errcheck

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	// Copy files should still be present
	data, err := os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if err != nil {
		t.Fatal("settings.json should exist after uninstall")
	}
	if string(data) != `{"model":"opus"}` {
		t.Errorf("settings = %q, want opus", string(data))
	}
}

func TestUninstall_PreservesClaudeJSON(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)
	database.Close() //nolint:errcheck

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(paths.ClaudeJSON)
	if err != nil {
		t.Fatal("claude.json should exist after uninstall")
	}
	if string(data) != `{"version":"original"}` {
		t.Errorf("claude.json = %q, want original", string(data))
	}
}

func TestUninstall_RemovesCCPHome(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)
	database.Close() //nolint:errcheck

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(paths.CCPHome); !os.IsNotExist(err) {
		t.Error(".ccp directory should be removed after uninstall")
	}
}

func TestUninstall_NoActiveProfile(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create profiles but don't activate any
	Create(database, paths, "alpha", "", false, false)
	database.Close() //nolint:errcheck

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	// No symlinks should exist, dirs should remain real
	info, err := os.Lstat(filepath.Join(paths.ClaudeHome, "plugins"))
	if err != nil {
		t.Fatal("plugins should exist after uninstall")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("plugins should be a real directory")
	}

	if _, err := os.Stat(paths.CCPHome); !os.IsNotExist(err) {
		t.Error(".ccp directory should be removed")
	}
}

func TestUninstall_MultipleSymlinkDirs(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create real skills and agents dirs too
	for _, dir := range config.ConfigSymlinkDirs {
		dirPath := filepath.Join(paths.ClaudeHome, dir)
		os.MkdirAll(dirPath, 0755)
		os.WriteFile(filepath.Join(dirPath, "marker.txt"), []byte(dir+"-data"), 0644)
	}

	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)
	database.Close() //nolint:errcheck

	// Verify all are symlinks
	for _, dir := range config.ConfigSymlinkDirs {
		if _, err := os.Readlink(filepath.Join(paths.ClaudeHome, dir)); err != nil {
			t.Fatalf("%s should be a symlink before uninstall", dir)
		}
	}

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	// Verify all are real directories with content
	for _, dir := range config.ConfigSymlinkDirs {
		info, err := os.Lstat(filepath.Join(paths.ClaudeHome, dir))
		if err != nil {
			t.Fatalf("%s should exist after uninstall", dir)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a real directory after uninstall", dir)
		}
		data, err := os.ReadFile(filepath.Join(paths.ClaudeHome, dir, "marker.txt"))
		if err != nil {
			t.Fatalf("%s/marker.txt should exist after uninstall", dir)
		}
		if string(data) != dir+"-data" {
			t.Errorf("%s marker = %q, want %s-data", dir, string(data), dir)
		}
	}
}

func TestUninstall_MissingSymlinkDir(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", true, false)
	Activate(database, paths, "alpha", false)
	database.Close() //nolint:errcheck

	// Remove one of the symlink dirs entirely
	os.Remove(filepath.Join(paths.ClaudeHome, "skills"))

	// Uninstall should handle missing dirs gracefully
	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(paths.CCPHome); !os.IsNotExist(err) {
		t.Error(".ccp directory should be removed")
	}
}
