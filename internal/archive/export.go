package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	archiveRoot    = "ccp-profile"
	formatVersion  = "1"
	maxArchiveSize = 500 * 1024 * 1024 // 500MB
)

// ExportOptions controls what gets included in the export archive.
type ExportOptions struct {
	ProfileDir string
	OutputPath string
	StripPaths bool

	// Component selection - true means include
	IncludeSettings    bool
	IncludePlugins     bool
	IncludeSkills      bool
	IncludeAgents      bool
	IncludeProjects    bool
	IncludeClaudeJSON  bool
	IncludeKeybindings bool
	IncludeClaudeMD    bool
}

// PathMapping describes an anonymized path placeholder.
type PathMapping struct {
	Placeholder string `json:"placeholder"`
	Description string `json:"description"`
}

// Export creates a tar.gz archive from a profile directory.
func Export(opts ExportOptions) error {
	outFile, err := os.Create(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tw := tar.NewWriter(gzWriter)
	defer tw.Close()

	// Write format version
	if err := writeArchiveFile(tw, filepath.Join(archiveRoot, "format_version"), []byte(formatVersion+"\n")); err != nil {
		return err
	}

	// Write metadata.json if it exists
	metadataPath := filepath.Join(opts.ProfileDir, "metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		data, _ := os.ReadFile(metadataPath)
		if err := writeArchiveFile(tw, filepath.Join(archiveRoot, "metadata.json"), data); err != nil {
			return err
		}
	}

	// Collect path mappings for anonymization
	var pathMappings []PathMapping
	pathCounter := 0
	pathReplacements := make(map[string]string) // original -> placeholder

	if opts.StripPaths {
		pathMappings, pathReplacements, pathCounter = collectPathMappings(opts)
		_ = pathCounter
	}

	// Write selected components from claude/ directory
	claudeDir := filepath.Join(opts.ProfileDir, "claude")
	if _, err := os.Stat(claudeDir); err == nil {
		componentMap := map[string]bool{
			"settings.json":       opts.IncludeSettings,
			"settings.local.json": opts.IncludeSettings,
			"plugins":             opts.IncludePlugins,
			"skills":              opts.IncludeSkills,
			"agents":              opts.IncludeAgents,
			"projects":            opts.IncludeProjects,
			"keybindings.json":    opts.IncludeKeybindings,
			"CLAUDE.md":           opts.IncludeClaudeMD,
		}

		entries, _ := os.ReadDir(claudeDir)
		for _, entry := range entries {
			include, known := componentMap[entry.Name()]
			if known && !include {
				continue
			}
			if !known {
				continue // Skip unknown items
			}

			srcPath := filepath.Join(claudeDir, entry.Name())
			archivePath := filepath.Join(archiveRoot, "claude", entry.Name())

			if entry.IsDir() {
				if err := addDirToArchive(tw, srcPath, archivePath, opts.StripPaths, pathReplacements); err != nil {
					return fmt.Errorf("add %s: %w", entry.Name(), err)
				}
			} else {
				if err := addFileToArchive(tw, srcPath, archivePath, opts.StripPaths, pathReplacements); err != nil {
					return fmt.Errorf("add %s: %w", entry.Name(), err)
				}
			}
		}
	}

	// Write claude.json if selected
	if opts.IncludeClaudeJSON {
		claudeJSONPath := filepath.Join(opts.ProfileDir, "claude.json")
		if _, err := os.Stat(claudeJSONPath); err == nil {
			if err := addFileToArchive(tw, claudeJSONPath, filepath.Join(archiveRoot, "claude.json"), opts.StripPaths, pathReplacements); err != nil {
				return fmt.Errorf("add claude.json: %w", err)
			}
		}
	}

	// Write path mappings if stripping paths
	if opts.StripPaths && len(pathMappings) > 0 {
		data, _ := json.MarshalIndent(pathMappings, "", "  ")
		if err := writeArchiveFile(tw, filepath.Join(archiveRoot, "path_mappings.json"), data); err != nil {
			return err
		}
	}

	return nil
}

func collectPathMappings(opts ExportOptions) ([]PathMapping, map[string]string, int) {
	var mappings []PathMapping
	replacements := make(map[string]string)
	counter := 0

	// Scan project directories for path-encoded names
	if opts.IncludeProjects {
		projectsDir := filepath.Join(opts.ProfileDir, "claude", "projects")
		if entries, err := os.ReadDir(projectsDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				counter++
				placeholder := fmt.Sprintf("<project-%d>", counter)
				originalName := entry.Name()

				// Convert path-encoded dir name to human description
				desc := describeProjectPath(originalName)

				mappings = append(mappings, PathMapping{
					Placeholder: placeholder,
					Description: desc,
				})
				replacements[originalName] = placeholder
			}
		}
	}

	// Scan claude.json for project paths
	if opts.IncludeClaudeJSON {
		claudeJSONPath := filepath.Join(opts.ProfileDir, "claude.json")
		if data, err := os.ReadFile(claudeJSONPath); err == nil {
			// Find path-like keys in the JSON
			pathPattern := regexp.MustCompile(`/[A-Za-z][A-Za-z0-9/._-]+`)
			matches := pathPattern.FindAllString(string(data), -1)
			for _, match := range matches {
				if _, exists := replacements[match]; exists {
					continue
				}
				// Only anonymize paths that look like user directories
				if strings.Contains(match, "/Users/") || strings.Contains(match, "/home/") {
					counter++
					placeholder := fmt.Sprintf("<path-%d>", counter)
					replacements[match] = placeholder
					mappings = append(mappings, PathMapping{
						Placeholder: placeholder,
						Description: "filesystem path",
					})
				}
			}
		}
	}

	return mappings, replacements, counter
}

// describeProjectPath converts a path-encoded directory name to a human description.
// e.g., "-Users-matt-Projects-myapp" -> "project: myapp"
func describeProjectPath(encodedName string) string {
	parts := strings.Split(encodedName, "-")
	if len(parts) > 0 {
		return "project: " + parts[len(parts)-1]
	}
	return "project"
}

func writeArchiveFile(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 0644,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func addFileToArchive(tw *tar.Writer, srcPath, archivePath string, stripPaths bool, replacements map[string]string) error {
	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(srcPath)
		if err != nil {
			return err
		}
		header := &tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     archivePath,
			Linkname: target,
			Mode:     0755,
		}
		return tw.WriteHeader(header)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Apply path stripping to file contents
	if stripPaths && len(replacements) > 0 {
		content := string(data)
		for original, placeholder := range replacements {
			content = strings.ReplaceAll(content, original, placeholder)
		}
		data = []byte(content)
	}

	header := &tar.Header{
		Name: archivePath,
		Size: int64(len(data)),
		Mode: int64(info.Mode().Perm()),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err = tw.Write(data)
	return err
}

func addDirToArchive(tw *tar.Writer, srcDir, archiveDir string, stripPaths bool, replacements map[string]string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(srcDir, path)
		archivePath := filepath.Join(archiveDir, rel)

		// Apply path stripping to directory names
		if stripPaths {
			for original, placeholder := range replacements {
				archivePath = strings.ReplaceAll(archivePath, original, placeholder)
			}
		}

		if info.IsDir() {
			header := &tar.Header{
				Typeflag: tar.TypeDir,
				Name:     archivePath + "/",
				Mode:     int64(info.Mode().Perm()),
			}
			return tw.WriteHeader(header)
		}

		return addFileToArchive(tw, path, archivePath, stripPaths, replacements)
	})
}
