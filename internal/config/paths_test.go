package config

import (
	"path/filepath"
	"testing"
)

func TestResolvePaths_Defaults(t *testing.T) {
	// Clear env overrides
	t.Setenv("CCP_HOME", "")
	t.Setenv("CLAUDE_HOME", "")
	t.Setenv("CLAUDE_JSON", "")

	p := ResolvePaths()

	if filepath.Base(p.CCPHome) != ".ccp" {
		t.Errorf("expected CCPHome to end with .ccp, got %s", p.CCPHome)
	}
	if filepath.Base(p.ClaudeHome) != ".claude" {
		t.Errorf("expected ClaudeHome to end with .claude, got %s", p.ClaudeHome)
	}
	if filepath.Base(p.ClaudeJSON) != ".claude.json" {
		t.Errorf("expected ClaudeJSON to end with .claude.json, got %s", p.ClaudeJSON)
	}
}

func TestResolvePaths_EnvOverrides(t *testing.T) {
	t.Setenv("CCP_HOME", "/tmp/test-ccp")
	t.Setenv("CLAUDE_HOME", "/tmp/test-claude")
	t.Setenv("CLAUDE_JSON", "/tmp/test-claude.json")

	p := ResolvePaths()

	if p.CCPHome != "/tmp/test-ccp" {
		t.Errorf("CCPHome = %s, want /tmp/test-ccp", p.CCPHome)
	}
	if p.ClaudeHome != "/tmp/test-claude" {
		t.Errorf("ClaudeHome = %s, want /tmp/test-claude", p.ClaudeHome)
	}
	if p.ClaudeJSON != "/tmp/test-claude.json" {
		t.Errorf("ClaudeJSON = %s, want /tmp/test-claude.json", p.ClaudeJSON)
	}
}

func TestPaths_ProfileDir(t *testing.T) {
	p := Paths{CCPHome: "/tmp/ccp"}

	got := p.ProfileDir("work")
	want := "/tmp/ccp/profiles/work"
	if got != want {
		t.Errorf("ProfileDir = %s, want %s", got, want)
	}
}

func TestPaths_DatabasePath(t *testing.T) {
	p := Paths{CCPHome: "/tmp/ccp"}

	got := p.DatabasePath()
	want := "/tmp/ccp/ccp.db"
	if got != want {
		t.Errorf("DatabasePath = %s, want %s", got, want)
	}
}
