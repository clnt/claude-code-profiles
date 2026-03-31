package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ImportOptions controls how the archive is imported.
type ImportOptions struct {
	ArchivePath string
	ProfileDir  string
	PathMap     map[string]string // placeholder -> local path
}

// ImportResult contains information about the imported profile.
type ImportResult struct {
	Name         string
	Description  string
	PathMappings []PathMapping // Mappings found in the archive (for prompting user)
}

// Import extracts a tar.gz archive into a profile directory.
// Returns metadata and path mappings for the caller to handle.
func Import(opts ImportOptions) (*ImportResult, error) {
	f, err := os.Open(opts.ArchivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	tr := tar.NewReader(gzReader)

	result := &ImportResult{}
	var totalSize int64

	// First pass: validate and collect metadata
	entries := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}

		// Security: reject path traversal
		if strings.Contains(header.Name, "..") {
			return nil, fmt.Errorf("archive contains path traversal: %s", header.Name)
		}

		// Security: reject absolute paths
		if filepath.IsAbs(header.Name) {
			return nil, fmt.Errorf("archive contains absolute path: %s", header.Name)
		}

		// Security: size cap
		totalSize += header.Size
		if totalSize > maxArchiveSize {
			return nil, fmt.Errorf("archive exceeds maximum size of %d MB", maxArchiveSize/1024/1024)
		}

		// Verify root directory
		if !strings.HasPrefix(header.Name, archiveRoot+"/") && header.Name != archiveRoot {
			return nil, fmt.Errorf("unexpected archive entry: %s (expected root %s/)", header.Name, archiveRoot)
		}

		if header.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(io.LimitReader(tr, header.Size+1))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", header.Name, err)
			}
			entries[header.Name] = data
		}
	}

	// Validate format version
	versionData, ok := entries[filepath.Join(archiveRoot, "format_version")]
	if !ok {
		return nil, fmt.Errorf("archive missing format_version file")
	}
	version := strings.TrimSpace(string(versionData))
	if version != formatVersion {
		return nil, fmt.Errorf("unsupported archive format version: %s (expected %s)", version, formatVersion)
	}

	// Read metadata
	if metaData, ok := entries[filepath.Join(archiveRoot, "metadata.json")]; ok {
		var meta struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(metaData, &meta); err == nil {
			result.Name = meta.Name
			result.Description = meta.Description
		}
	}

	// Read path mappings
	if mappingData, ok := entries[filepath.Join(archiveRoot, "path_mappings.json")]; ok {
		if err := json.Unmarshal(mappingData, &result.PathMappings); err != nil {
			return nil, fmt.Errorf("parse path_mappings.json: %w", err)
		}
	}

	// Second pass: extract files
	f.Seek(0, io.SeekStart)
	gzReader2, _ := gzip.NewReader(f)
	defer gzReader2.Close()
	tr2 := tar.NewReader(gzReader2)

	if err := os.MkdirAll(opts.ProfileDir, 0755); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}

	for {
		header, err := tr2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}

		// Strip the archive root prefix
		relPath := strings.TrimPrefix(header.Name, archiveRoot+"/")
		if relPath == "" || relPath == archiveRoot {
			continue
		}

		// Skip metadata files (already processed)
		if relPath == "format_version" || relPath == "path_mappings.json" {
			continue
		}

		// Apply path remapping
		if len(opts.PathMap) > 0 {
			for placeholder, localPath := range opts.PathMap {
				relPath = strings.ReplaceAll(relPath, placeholder, localPath)
			}
		}

		targetPath := filepath.Join(opts.ProfileDir, relPath)

		// Security: ensure target is within profile dir
		absTarget, _ := filepath.Abs(targetPath)
		absProfile, _ := filepath.Abs(opts.ProfileDir)
		if !strings.HasPrefix(absTarget, absProfile) {
			return nil, fmt.Errorf("path escapes profile directory: %s", relPath)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return nil, fmt.Errorf("create dir %s: %w", relPath, err)
			}

		case tar.TypeSymlink:
			// Security: reject absolute symlink targets
			if filepath.IsAbs(header.Linkname) {
				return nil, fmt.Errorf("archive contains symlink with absolute target: %s -> %s", relPath, header.Linkname)
			}
			// Security: reject symlinks that escape the profile directory
			resolvedTarget := filepath.Join(filepath.Dir(targetPath), header.Linkname)
			absResolved, _ := filepath.Abs(resolvedTarget)
			if !strings.HasPrefix(absResolved+string(filepath.Separator), absProfile+string(filepath.Separator)) {
				return nil, fmt.Errorf("archive contains symlink escaping profile directory: %s -> %s", relPath, header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return nil, fmt.Errorf("mkdir for symlink: %w", err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return nil, fmt.Errorf("create symlink %s: %w", relPath, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return nil, fmt.Errorf("mkdir for file: %w", err)
			}

			data, err := io.ReadAll(io.LimitReader(tr2, header.Size+1))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", relPath, err)
			}

			// Apply path remapping to file contents
			if len(opts.PathMap) > 0 {
				content := string(data)
				for placeholder, localPath := range opts.PathMap {
					content = strings.ReplaceAll(content, placeholder, localPath)
				}
				data = []byte(content)
			}

			if err := os.WriteFile(targetPath, data, os.FileMode(header.Mode)); err != nil {
				return nil, fmt.Errorf("write %s: %w", relPath, err)
			}
		}
	}

	return result, nil
}

// DeriveProfileName extracts a profile name from an archive file path by
// taking the basename and stripping .tar.gz / .tar extensions.
func DeriveProfileName(archivePath string) string {
	name := filepath.Base(archivePath)
	name = strings.TrimSuffix(name, ".gz")
	name = strings.TrimSuffix(name, ".tar")
	return name
}
