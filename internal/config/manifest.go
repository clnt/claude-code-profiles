package config

// ConfigIncludeList defines the file/directory basenames to copy from ~/.claude/
// into a profile. This is a whitelist: anything not listed is excluded.
// New directories that Claude Code adds in future versions are excluded by default.
var ConfigIncludeList = []string{
	"settings.json",
	"settings.local.json",
	"keybindings.json",
	"CLAUDE.md",
	"plugins",
	"skills",
	"agents",
}

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
