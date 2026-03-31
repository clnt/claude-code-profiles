package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestProfile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create profile structure
	claudeDir := filepath.Join(dir, "claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{"model":"opus"}`), 0644)
	os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# Guidelines"), 0644)
	os.WriteFile(filepath.Join(claudeDir, "keybindings.json"), []byte(`{}`), 0644)

	// Plugins
	pluginsDir := filepath.Join(claudeDir, "plugins")
	os.MkdirAll(pluginsDir, 0755)
	os.WriteFile(filepath.Join(pluginsDir, "config.json"), []byte(`{"plugins":true}`), 0644)

	// Skills with symlink
	skillsDir := filepath.Join(claudeDir, "skills")
	os.MkdirAll(skillsDir, 0755)
	os.WriteFile(filepath.Join(skillsDir, "my-skill.md"), []byte("skill content"), 0644)

	// Projects with path-encoded names
	projDir := filepath.Join(claudeDir, "projects", "-Users-matt-Projects-secret-app", "memory")
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, "note.md"), []byte("project memory"), 0644)

	// claude.json
	os.WriteFile(filepath.Join(dir, "claude.json"), []byte(`{"projects":{"/Users/matt/Projects/secret-app":{}}}`), 0644)

	return dir
}

func TestExportImport_Roundtrip(t *testing.T) {
	profileDir := createTestProfile(t)
	archivePath := filepath.Join(t.TempDir(), "test.tar.gz")
	importDir := filepath.Join(t.TempDir(), "imported")

	// Export with all components
	err := Export(ExportOptions{
		ProfileDir:         profileDir,
		OutputPath:         archivePath,
		IncludeSettings:    true,
		IncludePlugins:     true,
		IncludeSkills:      true,
		IncludeAgents:      true,
		IncludeProjects:    true,
		IncludeClaudeJSON:  true,
		IncludeKeybindings: true,
		IncludeClaudeMD:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive not created")
	}

	// Import
	result, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify imported files
	data, err := os.ReadFile(filepath.Join(importDir, "claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"model":"opus"}` {
		t.Errorf("settings.json = %q", string(data))
	}

	data, _ = os.ReadFile(filepath.Join(importDir, "claude", "CLAUDE.md"))
	if string(data) != "# Guidelines" {
		t.Errorf("CLAUDE.md = %q", string(data))
	}

	data, _ = os.ReadFile(filepath.Join(importDir, "claude.json"))
	if !strings.Contains(string(data), "secret-app") {
		t.Error("claude.json should contain project data")
	}

	_ = result
}

func TestExport_ComponentSelection(t *testing.T) {
	profileDir := createTestProfile(t)
	archivePath := filepath.Join(t.TempDir(), "partial.tar.gz")

	// Export only settings, exclude projects and claude.json
	err := Export(ExportOptions{
		ProfileDir:         profileDir,
		OutputPath:         archivePath,
		IncludeSettings:    true,
		IncludePlugins:     false,
		IncludeSkills:      false,
		IncludeProjects:    false,
		IncludeClaudeJSON:  false,
		IncludeKeybindings: true,
		IncludeClaudeMD:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Import and verify only selected components are present
	importDir := filepath.Join(t.TempDir(), "imported")
	_, err = Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Settings should exist
	if _, err := os.Stat(filepath.Join(importDir, "claude", "settings.json")); os.IsNotExist(err) {
		t.Error("settings.json should be included")
	}

	// Plugins should NOT exist
	if _, err := os.Stat(filepath.Join(importDir, "claude", "plugins")); !os.IsNotExist(err) {
		t.Error("plugins/ should be excluded")
	}

	// Projects should NOT exist
	if _, err := os.Stat(filepath.Join(importDir, "claude", "projects")); !os.IsNotExist(err) {
		t.Error("projects/ should be excluded")
	}

	// claude.json should NOT exist
	if _, err := os.Stat(filepath.Join(importDir, "claude.json")); !os.IsNotExist(err) {
		t.Error("claude.json should be excluded")
	}
}

func TestExport_StripPaths(t *testing.T) {
	profileDir := createTestProfile(t)
	archivePath := filepath.Join(t.TempDir(), "stripped.tar.gz")

	err := Export(ExportOptions{
		ProfileDir:        profileDir,
		OutputPath:        archivePath,
		StripPaths:        true,
		IncludeSettings:   true,
		IncludeProjects:   true,
		IncludeClaudeJSON: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Read archive and check for path mappings
	importDir := filepath.Join(t.TempDir(), "imported")
	result, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.PathMappings) == 0 {
		t.Error("expected path mappings for anonymized paths")
	}

	// Verify project directory names are anonymized
	projectsDir := filepath.Join(importDir, "claude", "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, entry := range entries {
			if strings.Contains(entry.Name(), "matt") || strings.Contains(entry.Name(), "secret") {
				t.Errorf("project dir %q should be anonymized", entry.Name())
			}
		}
	}
}

func TestImport_PathRemapping(t *testing.T) {
	profileDir := createTestProfile(t)
	archivePath := filepath.Join(t.TempDir(), "stripped.tar.gz")

	// Export with path stripping
	Export(ExportOptions{
		ProfileDir:        profileDir,
		OutputPath:        archivePath,
		StripPaths:        true,
		IncludeSettings:   true,
		IncludeProjects:   true,
		IncludeClaudeJSON: true,
	})

	// Import with path remapping
	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
		PathMap: map[string]string{
			"<project-1>": "-Users-alice-Projects-my-app",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify remapped project directory exists
	remappedDir := filepath.Join(importDir, "claude", "projects", "-Users-alice-Projects-my-app")
	if _, err := os.Stat(remappedDir); os.IsNotExist(err) {
		// List what actually exists
		entries, _ := os.ReadDir(filepath.Join(importDir, "claude", "projects"))
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected remapped project dir, got: %v", names)
	}
}

func TestImport_PathTraversal(t *testing.T) {
	// Create a malicious archive with path traversal
	archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")

	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	// Add format version
	tw.WriteHeader(&tar.Header{Name: "ccp-profile/format_version", Size: 2, Mode: 0644})
	tw.Write([]byte("1\n"))

	// Add malicious path traversal entry
	tw.WriteHeader(&tar.Header{Name: "ccp-profile/../../../etc/passwd", Size: 4, Mode: 0644})
	tw.Write([]byte("evil"))

	tw.Close()
	gzw.Close()
	f.Close()

	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err == nil {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("error should mention path traversal, got: %v", err)
	}
}

func TestImport_SymlinkAbsoluteTarget(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")

	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	tw.WriteHeader(&tar.Header{Name: "ccp-profile/format_version", Size: 2, Typeflag: tar.TypeReg, Mode: 0644})
	tw.Write([]byte("1\n"))

	// Symlink with absolute target
	tw.WriteHeader(&tar.Header{
		Name:     "ccp-profile/claude/plugins/evil",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	})

	tw.Close()
	gzw.Close()
	f.Close()

	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err == nil {
		t.Error("expected error for symlink with absolute target")
	}
	if !strings.Contains(err.Error(), "absolute target") {
		t.Errorf("error should mention absolute target, got: %v", err)
	}
}

func TestImport_SymlinkEscapesProfile(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")

	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	tw.WriteHeader(&tar.Header{Name: "ccp-profile/format_version", Size: 2, Typeflag: tar.TypeReg, Mode: 0644})
	tw.Write([]byte("1\n"))

	// Symlink with relative target that escapes the profile
	tw.WriteHeader(&tar.Header{
		Name:     "ccp-profile/claude/plugins/evil",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../../../etc/passwd",
	})

	tw.Close()
	gzw.Close()
	f.Close()

	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err == nil {
		t.Error("expected error for symlink escaping profile directory")
	}
	if !strings.Contains(err.Error(), "escaping profile directory") {
		t.Errorf("error should mention escaping, got: %v", err)
	}
}

func TestImport_SymlinkRelativeValid(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "valid.tar.gz")

	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	tw.WriteHeader(&tar.Header{Name: "ccp-profile/format_version", Size: 2, Typeflag: tar.TypeReg, Mode: 0644})
	tw.Write([]byte("1\n"))

	// Create a directory and file that the symlink will reference
	tw.WriteHeader(&tar.Header{Name: "ccp-profile/claude/plugins/", Typeflag: tar.TypeDir, Mode: 0755})
	tw.WriteHeader(&tar.Header{Name: "ccp-profile/claude/plugins/config.json", Size: 2, Typeflag: tar.TypeReg, Mode: 0644})
	tw.Write([]byte("{}"))

	// Valid relative symlink within the profile
	tw.WriteHeader(&tar.Header{
		Name:     "ccp-profile/claude/skills/link",
		Typeflag: tar.TypeSymlink,
		Linkname: "../plugins/config.json",
	})

	tw.Close()
	gzw.Close()
	f.Close()

	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err != nil {
		t.Fatalf("valid relative symlink should succeed, got: %v", err)
	}

	// Verify symlink was created
	target, err := os.Readlink(filepath.Join(importDir, "claude", "skills", "link"))
	if err != nil {
		t.Fatal("symlink should exist")
	}
	if target != "../plugins/config.json" {
		t.Errorf("symlink target = %q, want ../plugins/config.json", target)
	}
}

func TestImport_InvalidFormat(t *testing.T) {
	// Create archive without format_version
	archivePath := filepath.Join(t.TempDir(), "bad.tar.gz")

	f, _ := os.Create(archivePath)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	tw.WriteHeader(&tar.Header{Name: "ccp-profile/settings.json", Size: 2, Mode: 0644})
	tw.Write([]byte("{}"))

	tw.Close()
	gzw.Close()
	f.Close()

	importDir := filepath.Join(t.TempDir(), "imported")
	_, err := Import(ImportOptions{
		ArchivePath: archivePath,
		ProfileDir:  importDir,
	})
	if err == nil {
		t.Error("expected error for missing format_version")
	}
}

func TestDeriveProfileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"work.tar.gz", "work"},
		{"/tmp/work-profile.tar.gz", "work-profile"},
		{"../foo/bar.tar", "bar"},
		{"plain", "plain"},
		{"my-profile.tar", "my-profile"},
		{"dots.in.name.tar.gz", "dots.in.name"},
	}
	for _, tt := range tests {
		got := DeriveProfileName(tt.input)
		if got != tt.want {
			t.Errorf("DeriveProfileName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPathMapping_JSON(t *testing.T) {
	mappings := []PathMapping{
		{Placeholder: "<project-1>", Description: "project: myapp"},
		{Placeholder: "<project-2>", Description: "project: api"},
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	var decoded []PathMapping
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded) != 2 {
		t.Fatalf("got %d mappings, want 2", len(decoded))
	}
	if decoded[0].Placeholder != "<project-1>" {
		t.Errorf("placeholder = %q", decoded[0].Placeholder)
	}
}
