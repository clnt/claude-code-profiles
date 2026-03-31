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

	// Create plugins
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

	// Modify current config and create beta
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"sonnet"}`), 0644)
	os.WriteFile(paths.ClaudeJSON, []byte(`{"version":"modified"}`), 0644)
	if err := Create(database, paths, "beta", "", false, false); err != nil {
		t.Fatal(err)
	}

	// Activate beta first (no active profile yet, so no auto-save)
	if err := Activate(database, paths, "beta", false); err != nil {
		t.Fatal(err)
	}

	// Now switch to alpha
	if err := Activate(database, paths, "alpha", false); err != nil {
		t.Fatal(err)
	}

	// Verify settings restored to alpha's version
	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("settings.json = %q, want opus config", string(data))
	}

	data, _ = os.ReadFile(paths.ClaudeJSON)
	if string(data) != `{"version":"original"}` {
		t.Errorf("claude.json = %q, want original", string(data))
	}

	// Verify active profile updated in DB
	active, _ := database.GetActiveProfile()
	if active != "alpha" {
		t.Errorf("active = %q, want alpha", active)
	}
}

func TestActivate_AutoSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	// Create and activate profile "alpha"
	if err := Create(database, paths, "alpha", "", false, false); err != nil {
		t.Fatal(err)
	}
	database.SetActiveProfile("alpha")

	// Create profile "beta"
	if err := Create(database, paths, "beta", "", false, false); err != nil {
		t.Fatal(err)
	}

	// Modify config while alpha is active
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Switch to beta (should auto-save alpha)
	if err := Activate(database, paths, "beta", true); err != nil {
		t.Fatal(err)
	}

	// Verify alpha's saved config has the modified settings
	alphaSettings, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(alphaSettings) != `{"model":"haiku"}` {
		t.Errorf("alpha settings = %q, want haiku (auto-saved)", string(alphaSettings))
	}
}

func TestActivate_NoSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	if err := Create(database, paths, "alpha", "", false, false); err != nil {
		t.Fatal(err)
	}
	database.SetActiveProfile("alpha")

	if err := Create(database, paths, "beta", "", false, false); err != nil {
		t.Fatal(err)
	}

	// Modify config
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Switch WITHOUT auto-save
	if err := Activate(database, paths, "beta", false); err != nil {
		t.Fatal(err)
	}

	// Alpha should still have the original settings (not haiku)
	alphaSettings, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(alphaSettings) != `{"model":"opus"}` {
		t.Errorf("alpha settings = %q, want opus (should not have been auto-saved)", string(alphaSettings))
	}
}

func TestActivate_PreservesEphemeralData(t *testing.T) {
	database, paths := setupActivateTest(t)

	if err := Create(database, paths, "alpha", "", false, false); err != nil {
		t.Fatal(err)
	}
	database.SetActiveProfile("alpha")

	if err := Create(database, paths, "beta", "", false, false); err != nil {
		t.Fatal(err)
	}

	if err := Activate(database, paths, "beta", true); err != nil {
		t.Fatal(err)
	}

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

	// Project session log should be preserved
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

	// Create alpha from current config (opus)
	Create(database, paths, "alpha", "", false, false)

	// Modify and create beta (sonnet)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"sonnet"}`), 0644)
	Create(database, paths, "beta", "", false, false)

	// Activate alpha (no-save since no active profile yet)
	Activate(database, paths, "alpha", false)

	// Verify alpha config
	data, _ := os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("after switch to alpha: %q, want opus", string(data))
	}

	// Switch to beta (auto-save alpha with opus config)
	Activate(database, paths, "beta", true)

	// Verify beta config
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"sonnet"}` {
		t.Errorf("after switch to beta: %q, want sonnet", string(data))
	}

	// Switch back to alpha (auto-save beta with sonnet config)
	Activate(database, paths, "alpha", true)

	// Alpha should still have opus (saved before switching to beta)
	data, _ = os.ReadFile(filepath.Join(paths.ClaudeHome, "settings.json"))
	if string(data) != `{"model":"opus"}` {
		t.Errorf("after switch back to alpha: %q, want opus", string(data))
	}
}

func TestSave(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	database.SetActiveProfile("alpha")

	// Modify config
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"haiku"}`), 0644)

	// Save
	if err := Save(database, paths, "alpha"); err != nil {
		t.Fatal(err)
	}

	// Verify profile was updated
	data, _ := os.ReadFile(filepath.Join(paths.ProfileDir("alpha"), "claude", "settings.json"))
	if string(data) != `{"model":"haiku"}` {
		t.Errorf("saved settings = %q, want haiku", string(data))
	}
}

func TestActivate_CleanupOnSuccess(t *testing.T) {
	database, paths := setupActivateTest(t)

	Create(database, paths, "alpha", "", false, false)
	Create(database, paths, "beta", "", false, false)
	database.SetActiveProfile("alpha")

	Activate(database, paths, "beta", true)

	// Verify no staging/rollback directories remain
	entries, _ := os.ReadDir(paths.CCPHome)
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 8 && (name[:8] == "staging-" || name[:9] == "rollback-") {
			t.Errorf("leftover directory: %s", name)
		}
	}
}
