# ccp - Claude Code Profiles

A CLI tool for creating, switching, and sharing Claude Code configuration profiles.

Claude Code stores settings, plugins, skills, agents, and per-project memory across `~/.claude/` and `~/.claude.json`. `ccp` lets you snapshot these into named profiles and swap between them instantly.

## Install

### From source

```bash
go install github.com/clnt/claude-code-profiles@latest
```

### Build locally

```bash
git clone https://github.com/clnt/claude-code-profiles.git
cd claude-code-profiles
make install
```

## Quick start

```bash
# Create a profile from your current Claude Code config
ccp create work -d "Work setup with MCP servers"

# Make changes to your Claude Code config, then create another
ccp create personal -d "Personal projects"

# Switch between profiles
ccp use work       # auto-saves current config, then swaps in "work"
ccp use personal   # auto-saves "work", swaps in "personal"

# Use the default profile (first created is auto-default)
ccp use
```

## Commands

| Command | Description |
|---|---|
| `ccp create <name>` | Create a profile from current config |
| `ccp use [name]` | Switch to a profile (or default if no name) |
| `ccp save` | Save current config to the active profile |
| `ccp list` | List all profiles |
| `ccp current` | Show the active profile |
| `ccp default [name]` | Get or set the default profile |
| `ccp show <name>` | Show profile details and file tree |
| `ccp delete <name>` | Delete a profile |
| `ccp diff <a> <b>` | Compare two profiles |
| `ccp export <name>` | Export a profile as a tar.gz archive |
| `ccp import <file>` | Import a profile from a tar.gz archive |

## What's captured in a profile

Profiles snapshot config-relevant files only. Ephemeral data (sessions, telemetry, cache, history) is left untouched during switches.

**Included:**
- `settings.json` / `settings.local.json` - model, effort level, plugins
- `plugins/` - installed plugins and marketplace config
- `skills/` - custom skill definitions
- `agents/` - agent definitions
- `CLAUDE.md` - global instructions
- `keybindings.json` - key bindings
- `projects/*/memory/` - per-project memory
- `~/.claude.json` - per-project tool permissions, MCP servers

**Excluded:** sessions, telemetry, cache, history, plans, tasks, IDE state, and all other runtime data.

## Profile switching

`ccp use` performs an atomic switch with rollback protection:

1. Auto-saves current config to the active profile
2. Stages the target profile's files in a temp directory
3. Backs up current config to a rollback directory
4. Installs the staged files into `~/.claude/`
5. On failure, restores from the rollback automatically

Use `--no-save` to skip the auto-save step if you want to discard changes.

## Sharing profiles

### Export

```bash
# Interactive - choose which components to include
ccp export work

# Include everything without prompting
ccp export work --all -o work-profile.tar.gz

# Anonymize filesystem paths for sharing
ccp export work --all --strip-paths -o work-profile.tar.gz
```

The `--strip-paths` flag replaces real paths (like `/Users/you/Projects/secret-app`) with placeholders (`<project-1>`), so project names and directory structures aren't leaked.

### Import

```bash
# Basic import
ccp import work-profile.tar.gz

# Import with a different name
ccp import work-profile.tar.gz --name colleague-setup

# Remap anonymized paths to your local directories
ccp import work-profile.tar.gz --map-path '<project-1>=/Users/you/Projects/my-app'
```

If the archive was exported with `--strip-paths`, you'll be prompted to map each placeholder to a local path.

## Storage

`ccp` stores its data in `~/.ccp/`:

```
~/.ccp/
├── ccp.db              # SQLite database (profile metadata, active/default state)
└── profiles/
    └── <name>/
        ├── claude/     # Snapshot of ~/.claude/ config files
        └── claude.json # Snapshot of ~/.claude.json
```

## Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `CCP_HOME` | `~/.ccp` | Override ccp data directory |
| `CLAUDE_HOME` | `~/.claude` | Override Claude Code config directory |
| `CLAUDE_JSON` | `~/.claude.json` | Override Claude Code JSON config path |
