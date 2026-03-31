package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
)

func testSetup(t *testing.T) (*db.DB, config.Paths) {
	t.Helper()
	tmp := t.TempDir()

	paths := config.Paths{
		CCPHome:    filepath.Join(tmp, ".ccp"),
		ClaudeHome: filepath.Join(tmp, ".claude"),
		ClaudeJSON: filepath.Join(tmp, ".claude.json"),
	}

	// Create a minimal Claude Code config
	os.MkdirAll(paths.ClaudeHome, 0755)
	os.WriteFile(filepath.Join(paths.ClaudeHome, "settings.json"), []byte(`{"model":"opus"}`), 0644)
	os.WriteFile(paths.ClaudeJSON, []byte(`{"projects":{}}`), 0644)

	// Create plugins dir with a file
	pluginsDir := filepath.Join(paths.ClaudeHome, "plugins")
	os.MkdirAll(pluginsDir, 0755)
	os.WriteFile(filepath.Join(pluginsDir, "config.json"), []byte(`{}`), 0644)

	// Create a project with memory
	memDir := filepath.Join(paths.ClaudeHome, "projects", "test-project", "memory")
	os.MkdirAll(memDir, 0755)
	os.WriteFile(filepath.Join(memDir, "note.md"), []byte("remember this"), 0644)

	// Create an ephemeral file that should NOT be captured
	os.WriteFile(filepath.Join(paths.ClaudeHome, "history.jsonl"), []byte("{}"), 0644)
	os.MkdirAll(filepath.Join(paths.ClaudeHome, "sessions"), 0755)

	database, err := db.Open(paths.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	return database, paths
}

func TestValidateName(t *testing.T) {
	valid := []string{"work", "personal", "my-profile", "test_123", "A"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) should be valid, got: %v", name, err)
		}
	}

	invalid := []string{"", "-start", "_start", "123start", "has space", "has.dot", "a/b", "a\\b",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} // 67 chars
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) should be invalid", name)
		}
	}
}

func TestCreate(t *testing.T) {
	database, paths := testSetup(t)

	if err := Create(database, paths, "work", "Work config", false, false); err != nil {
		t.Fatal(err)
	}

	// Verify profile directory exists
	profileDir := paths.ProfileDir("work")
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		t.Error("profile directory should exist")
	}

	// Verify settings.json was copied
	data, err := os.ReadFile(filepath.Join(profileDir, "claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"model":"opus"}` {
		t.Errorf("settings.json = %q", string(data))
	}

	// Verify claude.json was copied
	data, err = os.ReadFile(filepath.Join(profileDir, "claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"projects":{}}` {
		t.Errorf("claude.json = %q", string(data))
	}

	// Verify project memory was copied
	data, err = os.ReadFile(filepath.Join(profileDir, "claude", "projects", "test-project", "memory", "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "remember this" {
		t.Errorf("memory note = %q", string(data))
	}

	// Verify ephemeral files were NOT copied
	if _, err := os.Stat(filepath.Join(profileDir, "claude", "history.jsonl")); !os.IsNotExist(err) {
		t.Error("history.jsonl should not be copied")
	}
	if _, err := os.Stat(filepath.Join(profileDir, "claude", "sessions")); !os.IsNotExist(err) {
		t.Error("sessions/ should not be copied")
	}

	// Verify database entry
	p, err := database.GetProfile("work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "work" || p.Description != "Work config" {
		t.Errorf("DB profile = %+v", p)
	}

	// First profile should be auto-default
	if !p.IsDefault {
		t.Error("first profile should be auto-default")
	}
}

func TestCreate_Duplicate(t *testing.T) {
	database, paths := testSetup(t)

	Create(database, paths, "work", "", false, false)
	err := Create(database, paths, "work", "", false, false)
	if err == nil {
		t.Error("expected error for duplicate profile")
	}
}

func TestCreate_Blank(t *testing.T) {
	database, paths := testSetup(t)

	if err := Create(database, paths, "blank", "", true, false); err != nil {
		t.Fatal(err)
	}

	// Profile dir should have scaffolded empty dirs for symlink targets
	profileDir := paths.ProfileDir("blank")
	for _, dir := range config.ConfigSymlinkDirs {
		dirPath := filepath.Join(profileDir, "claude", dir)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("blank profile should have empty %s dir", dir)
		} else if !info.IsDir() {
			t.Errorf("%s should be a directory", dir)
		}
	}

	// Should NOT have any config files
	if _, err := os.Stat(filepath.Join(profileDir, "claude", "settings.json")); !os.IsNotExist(err) {
		t.Error("blank profile should not have settings.json")
	}
}

func TestCreate_InvalidName(t *testing.T) {
	database, paths := testSetup(t)

	err := Create(database, paths, "123bad", "", false, false)
	if err == nil {
		t.Error("expected error for invalid name")
	}
}

func TestDelete(t *testing.T) {
	database, paths := testSetup(t)

	Create(database, paths, "work", "", false, false)
	Create(database, paths, "personal", "", false, false)

	if err := Delete(database, paths, "personal"); err != nil {
		t.Fatal(err)
	}

	// Should not exist in DB or filesystem
	exists, _ := database.ProfileExists("personal")
	if exists {
		t.Error("profile should be deleted from DB")
	}
	if _, err := os.Stat(paths.ProfileDir("personal")); !os.IsNotExist(err) {
		t.Error("profile directory should be deleted")
	}
}

func TestDelete_ActiveProfile(t *testing.T) {
	database, paths := testSetup(t)

	Create(database, paths, "work", "", false, false)
	database.SetActiveProfile("work")

	err := Delete(database, paths, "work")
	if err == nil {
		t.Error("should not be able to delete active profile")
	}
}
