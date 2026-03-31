package config

import "testing"

func TestIsIncluded(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"settings.json", true},
		{"settings.local.json", true},
		{"keybindings.json", true},
		{"CLAUDE.md", true},
		{"plugins", true},
		{"skills", true},
		{"agents", true},
		{"projects", true},
		// Excluded items
		{"sessions", false},
		{"telemetry", false},
		{"cache", false},
		{"paste-cache", false},
		{"history.jsonl", false},
		{"backups", false},
		{"statsig", false},
		{"unknown-new-dir", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsIncluded(tt.name)
			if got != tt.want {
				t.Errorf("IsIncluded(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
