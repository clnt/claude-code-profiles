package config

// ConfigSymlinkDirs are directories that get symlinked from ~/.claude/ to the
// profile directory. These are typically large (plugins can be 45MB+) so we
// avoid copying them on every switch. The profile dir holds the real data and
// ~/.claude/<dir> becomes a symlink.
var ConfigSymlinkDirs = []string{
	"plugins",
	"skills",
	"agents",
}

// ConfigCopyFiles are individual files that get copied between ~/.claude/ and
// the profile directory. These are small (< 1MB) so copying is fine.
var ConfigCopyFiles = []string{
	"settings.json",
	"settings.local.json",
	"keybindings.json",
	"CLAUDE.md",
}

// ConfigIncludeList is the combined list of all config items (for export/diff).
var ConfigIncludeList = append(append([]string{}, ConfigCopyFiles...), ConfigSymlinkDirs...)

// ProjectSubdirInclude is the only subdirectory copied from each
// ~/.claude/projects/<name>/ directory. Session logs and UUID dirs are excluded.
const ProjectSubdirInclude = "memory"

// ProjectsDirName is the name of the projects directory inside ~/.claude/.
const ProjectsDirName = "projects"

// IsIncluded returns true if the given basename (file or dir) should be
// captured from ~/.claude/ into a profile.
func IsIncluded(name string) bool {
	for _, item := range ConfigIncludeList {
		if item == name {
			return true
		}
	}
	return name == ProjectsDirName
}

// IsSymlinkDir returns true if the given name is a directory managed via symlinks.
func IsSymlinkDir(name string) bool {
	for _, item := range ConfigSymlinkDirs {
		if item == name {
			return true
		}
	}
	return false
}
