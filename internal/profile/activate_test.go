package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
)

func setupActivateTest(t *testing.T) (*db.DB, config.Paths) {
	t.Helper()
	tmp := t.TempDir()

	paths := config.Paths{
		CCPHome:    filepath.Join(tmp, ".ccp"),
		ClaudeHome: filepath.Join(tmp, ".claude"),
		ClaudeJSON: filepath.Join(tmp, ".claude.json"),
	}

	// Create Claude config with identifiable content
	os.MkdirAll(paths.ClaudeHome, 0755)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"opus"}`), 0644)
	os.WriteFile(paths.ClaudeJSON, []byte(`{"version":"original"}`), 0644)

	// Create plugins (a symlink-managed directory)
	os.MkdirAll(filepath.Join(paths.ClaudeHome, "plugins"), 0755)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"), []byte(`{"plugins":"original"}`), 0644)

	// Create project memory
	memDir := filepath.Join(paths.ClaudeHome, "projects", "myproject", "memory")
	os.MkdirAll(memDir, 0755)
	os.WriteFile(filepath.Join(memDir, "note.md"), []byte("original memory"), 0644)

	// Create ephemeral data that should be preserved across switches
	os.WriteFile(filepath.Join(paths.ClaudeHome, "history.jsonl"), []byte("history"), 0644)
	os.MkdirAll(filepath.Join(paths.ClaudeHome, "sessions"), 0755)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "sessions", "sess.json"), []byte("session"), 0644)

	// Also create a session log in the project dir
	os.WriteFile(filepath.Join(paths.ClaudeHome, "projects", "myproject", "session.jsonl"), []byte("session log"), 0644)

	database, err := db.Open(paths.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	return database, paths
}

func TestActivate_BasicSwitch(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create profile "alpha" from current config (opus)
	if err := Create(database, paths, "alpha", "", false, false); err != nil {
		t.Fatal(err)
	}

	// After create, plugins should still be a real directory (no symlinks yet)
	info, err := os.Lstat(filepath.Join(paths.ClaudeHome, "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("plugins should still be a real directory after create, not a symlink")
	}

	// Modify settings and create beta
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"sonnet"}`), 0644)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"), []byte(`{"plugins":"modified"}`), 0644)
	if err := Create(database, paths, "beta", "", false, false); err != nil {
		t.Fatal(err)
	}

	// Activate alpha — first activation replaces real dirs with symlinks
	if err := Activate(database, paths, "alpha", false); err != nil {
		t.Fatal(err)
	}

	// plugins should now be a symlink
	_, err = os.Readlink(filepath.Join(paths.ClaudeHome, "plugins"))
	if err != nil {
		t.Fatal("plugins should be a symlink after activate")
	}

	// Settings should be alpha's (opus)
	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("settings.json = %q, want opus", string(data))
	}

	// claude.json should be alpha's
	data, _ = os.ReadFile(paths.ClaudeJSON)
	if string(data) != `{"version":"original"}` {
		t.Errorf("claude.json = %q, want original", string(data))
	}

	active, _ := database.GetActiveProfile()
	if active != "alpha" {
		t.Errorf("active = %q, want alpha", active)
	}
}

func TestActivate_SymlinkSwap(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create alpha — moves plugins into alpha's profile dir, symlinks back
	Create(database, paths, "alpha", "", false, false)

	// Create beta as blank and set up its own plugins directly
	Create(database, paths, "beta", "", true, false)
	betaPlugins := filepath.Join(paths.ProfileDir("beta"), "claude", "plugins")
	os.MkdirAll(betaPlugins, 0755)
	os.WriteFile(filepath.Join(betaPlugins, "config.json"), []byte(`{"plugins":"beta-version"}`), 0644)

	// Switch to beta
	Activate(database, paths, "beta", false)

	// plugins should be symlinked to beta's directory
	linkTarget, _ := os.Readlink(filepath.Join(paths.ClaudeHome, "plugins"))
	if linkTarget != betaPlugins {
		t.Errorf("plugins symlink = %s, want %s", linkTarget, betaPlugins)
	}
	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"beta-version"}` {
		t.Errorf("plugins config = %q, want beta-version", string(data))
	}

	// Switch to alpha
	Activate(database, paths, "alpha", false)

	alphaPlugins := filepath.Join(paths.ProfileDir("alpha"), "claude", "plugins")
	linkTarget, _ = os.Readlink(filepath.Join(paths.ClaudeHome, "plugins"))
	if linkTarget != alphaPlugins {
		t.Errorf("plugins symlink = %s, want %s", linkTarget, alphaPlugins)
	}
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"original"}` {
		t.Errorf("plugins config = %q, want original", string(data))
	}
}

func TestActivate_AutoSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	Create(database, paths, "beta", "", false, false)

	// Modify a copy-file while alpha is active
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Switch to beta with auto-save
	if err := Activate(database, paths, "beta", true); err != nil {
		t.Fatal(err)
	}

	// Alpha's saved settings should have the modified value
	alphaSettings, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(alphaSettings) != `{"model":"haiku"}` {
		t.Errorf("alpha settings = %q, want haiku (auto-saved)", string(alphaSettings))
	}
}

func TestActivate_NoSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	Create(database, paths, "beta", "", false, false)

	// Modify settings
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Switch WITHOUT auto-save
	if err := Activate(database, paths, "beta", false); err != nil {
		t.Fatal(err)
	}

	// Alpha should still have opus (not auto-saved)
	alphaSettings, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(alphaSettings) != `{"model":"opus"}` {
		t.Errorf("alpha settings = %q, want opus", string(alphaSettings))
	}
}

func TestActivate_PreservesEphemeralData(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Create(database, paths, "beta", "", false, false)

	Activate(database, paths, "alpha", false)

	// Ephemeral files should still exist
	data, err := os.ReadFile(filepath.Join(paths.ClaudeHome, "history.jsonl"))
	if err != nil {
		t.Fatal("history.jsonl should be preserved")
	}
	if string(data) != "history" {
		t.Error("history.jsonl content changed")
	}

	data, err = os.ReadFile(filepath.Join(paths.ClaudeHome, "sessions", "sess.json"))
	if err != nil {
		t.Fatal("sessions should be preserved")
	}
	if string(data) != "session" {
		t.Error("session content changed")
	}

	data, err = os.ReadFile(filepath.Join(paths.ClaudeHome, "projects", "myproject", "session.jsonl"))
	if err != nil {
		t.Fatal("project session.jsonl should be preserved")
	}
	if string(data) != "session log" {
		t.Error("project session content changed")
	}
}

func TestActivate_AlreadyActive(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	err := Activate(database, paths, "alpha", true)
	if err == nil {
		t.Error("expected error when activating already-active profile")
	}
}

func TestActivate_NonExistent(t *testing.T) {
	database, paths := setupActivateTest(t)

	err := Activate(database, paths, "nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestActivate_RoundTrip(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create alpha (opus settings, original plugins)
	Create(database, paths, "alpha", "", false, false)

	// Create beta as blank with its own settings and plugins
	Create(database, paths, "beta", "", true, false)
	betaClaude := filepath.Join(paths.ProfileDir("beta"), "claude")
	os.MkdirAll(filepath.Join(betaClaude, "plugins"), 0755)
	os.WriteFile(filepath.Join(betaClaude, "settings.json"), []byte(`{"model":"sonnet"}`), 0644)
	os.WriteFile(filepath.Join(betaClaude, "plugins", "config.json"), []byte(`{"plugins":"beta"}`), 0644)

	// Activate alpha
	Activate(database, paths, "alpha", false)

	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("after switch to alpha: settings = %q, want opus", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"original"}` {
		t.Errorf("after switch to alpha: plugins = %q, want original", string(data))
	}

	// Switch to beta
	Activate(database, paths, "beta", true)

	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"sonnet"}` {
		t.Errorf("after switch to beta: settings = %q, want sonnet", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"beta"}` {
		t.Errorf("after switch to beta: plugins = %q, want beta", string(data))
	}

	// Switch back to alpha
	Activate(database, paths, "alpha", true)

	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("after switch back to alpha: settings = %q, want opus", string(data))
	}
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"original"}` {
		t.Errorf("after switch back to alpha: plugins = %q, want original", string(data))
	}
}

func TestActivate_FirstActivation_SymlinksToBlankProfile(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create a blank profile — has scaffolded empty dirs
	Create(database, paths, "fresh", "", true, false)

	// Activate it — real dirs get replaced with symlinks to empty profile dirs
	Activate(database, paths, "fresh", false)

	// plugins should now be a symlink to the profile's empty plugins dir
	linkTarget, err := os.Readlink(filepath.Join(paths.ClaudeHome, "plugins"))
	if err != nil {
		t.Fatal("plugins should be a symlink after first activation")
	}

	expectedTarget := filepath.Join(paths.ProfileDir("fresh"), "claude", "plugins")
	if linkTarget != expectedTarget {
		t.Errorf("symlink target = %s, want %s", linkTarget, expectedTarget)
	}

	// The symlinked dir should be empty (blank profile)
	entries, _ := os.ReadDir(filepath.Join(paths.ClaudeHome, "plugins"))
	if len(entries) != 0 {
		t.Errorf("blank profile plugins should be empty, got %d entries", len(entries))
	}
}

func TestActivate_CreateDoesNotSymlink(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)

	// ~/.claude/plugins should still be a real directory, not a symlink
	info, _ := os.Lstat(filepath.Join(paths.ClaudeHome, "plugins"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("create should not create symlinks — only activate should")
	}

	// Original data should still be readable from ~/.claude/
	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "plugins", "config.json"))
	if string(data) != `{"plugins":"original"}` {
		t.Errorf("live plugins config = %q, want original (untouched)", string(data))
	}
}

func TestActivate_MinimalProfileCleansClaudeJSON(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Alpha has claude.json (from setupActivateTest)
	Create(database, paths, "alpha", "", false, false)

	// Beta is blank — no claude.json
	Create(database, paths, "beta", "", true, false)

	// Activate alpha so claude.json is installed
	Activate(database, paths, "alpha", false)
	if _, err := os.Stat(paths.ClaudeJSON); os.IsNotExist(err) {
		t.Fatal("claude.json should exist after activating alpha")
	}

	// Switch to beta — claude.json should be removed
	Activate(database, paths, "beta", false)
	if _, err := os.Stat(paths.ClaudeJSON); !os.IsNotExist(err) {
		t.Error("claude.json should be removed after switching to blank profile")
	}
}

func TestActivate_MinimalProfileCleansProjectMemory(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Alpha has project memory (from setupActivateTest)
	Create(database, paths, "alpha", "", false, false)

	// Beta is blank
	Create(database, paths, "beta", "", true, false)

	Activate(database, paths, "alpha", false)

	// Verify memory exists
	memDir := filepath.Join(paths.ClaudeHome, "projects", "myproject", "memory")
	if _, err := os.Stat(memDir); os.IsNotExist(err) {
		t.Fatal("project memory should exist after activating alpha")
	}

	// Switch to beta
	Activate(database, paths, "beta", false)

	// Memory should be removed
	if _, err := os.Stat(memDir); !os.IsNotExist(err) {
		t.Error("project memory should be removed after switching to blank profile")
	}

	// But the project dir and session log should be preserved
	sessionLog := filepath.Join(paths.ClaudeHome, "projects", "myproject", "session.jsonl")
	if _, err := os.Stat(sessionLog); os.IsNotExist(err) {
		t.Error("project session.jsonl should be preserved (not managed)")
	}
}

func TestActivate_ForcePreservesAutoSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Activate(database, paths, "alpha", false)

	// Modify a copy-file while alpha is active
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Simulate the --force flow: save first, then clear active, then reactivate
	// This mirrors what cmd/use.go does with the fix
	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}
	database.ClearActiveProfile()
	if err := Activate(database, paths, "alpha", true); err != nil {
		t.Fatal(err)
	}

	// Alpha's profile should have the modified settings (not lost)
	data, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(data) != `{"model":"haiku"}` {
		t.Errorf("alpha settings = %q, want haiku (should be preserved by force auto-save)", string(data))
	}
}

func TestSave_RemovesDeletedCopyFiles(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	// Verify settings.json exists in profile
	profileSettings := filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json")
	if _, err := os.Stat(profileSettings); os.IsNotExist(err) {
		t.Fatal("settings.json should exist in profile after create")
	}

	// Delete settings.json from live config
	os.Remove(filepath.Join(paths.ClaudeHome, "settings.json"))

	// Save — should remove stale file from profile
	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(profileSettings); !os.IsNotExist(err) {
		t.Error("settings.json should be removed from profile after save (user deleted it)")
	}
}

func TestSave_RemovesDeletedClaudeJSON(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	// Verify claude.json exists in profile
	profileClaudeJSON := filepath.Join(paths.ProfileDir("alpha"), "claude.json")
	if _, err := os.Stat(profileClaudeJSON); os.IsNotExist(err) {
		t.Fatal("claude.json should exist in profile after create")
	}

	// Delete ~/.claude.json
	os.Remove(paths.ClaudeJSON)

	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(profileClaudeJSON); !os.IsNotExist(err) {
		t.Error("claude.json should be removed from profile after save (user deleted it)")
	}
}

func TestSave_RemovesStaleProjectMemory(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	// Verify project memory exists in profile
	profileMemDir := filepath.Join(paths.ProfileDir("alpha"), "claude", "projects", "myproject", "memory")
	if _, err := os.Stat(profileMemDir); os.IsNotExist(err) {
		t.Fatal("project memory should exist in profile after create")
	}

	// Delete project memory from live config
	os.RemoveAll(filepath.Join(paths.ClaudeHome, "projects", "myproject", "memory"))

	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	// Profile-side project dir should be removed (no memory left)
	profileProjectDir := filepath.Join(paths.ProfileDir("alpha"), "claude", "projects", "myproject")
	if _, err := os.Stat(profileProjectDir); !os.IsNotExist(err) {
		t.Error("stale project dir should be removed from profile after save")
	}
}

func TestSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	// Modify a copy-file
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(data) != `{"model":"haiku"}` {
		t.Errorf("saved settings = %q, want haiku", string(data))
	}
}
