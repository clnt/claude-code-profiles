# Code Review Findings

Reviewed on 2026-03-31 against the current working tree.

## Findings

### 1. High - `Activate` is destructive and non-atomic, so a mid-switch failure can leave `~/.claude` in a mixed state

Files: `internal/profile/activate.go:55-121`, `README.md:74-80`

`ccp use` is documented as an atomic switch with staging and rollback, but `Activate` mutates the live Claude config in place: it removes or renames live directories, recreates symlinks, deletes copy-managed files, copies new files, copies project memory, then finally updates the database. If any step in that sequence fails, there is no rollback path and the user can be left with a partially switched config. The presence of unused `StagingDir` and `RollbackDir` helpers in `internal/config/paths.go` makes this look like planned behavior that never landed.

### 2. High - Switching profiles does not fully remove included state that is absent from the target profile

Files: `internal/profile/activate.go:104-115`, `internal/profile/activate.go:139-169`

The switch logic only copies `claude.json` when the target profile has one, and `installProjectMemory` only overwrites memory directories that exist in the target profile. Missing target state is never removed. Switching from a populated profile to a blank or minimal profile therefore leaks prior profile state forward, especially `~/.claude.json` contents and stale `projects/*/memory` trees. That breaks the expected isolation boundary between profiles.

### 3. High - Importing an archive can plant arbitrary symlinks into managed profiles

Files: `internal/archive/import.go:170-175`, `internal/fsutil/copy.go:17-23`, `internal/fsutil/copy.go:79-87`

`Import` recreates tar symlinks without validating `header.Linkname`, and the later copy/activation paths preserve symlinks rather than resolving or rejecting them. A crafted shared archive can therefore create profile entries that point outside the profile store and then project those symlinks back into `~/.claude` on activation. The current import tests cover path traversal in archive entry names, but not malicious symlink targets.

### 4. Medium - `save` and auto-save do not record deletions, so removed files and memories reappear later

Files: `internal/profile/snapshot.go:80-104`, `internal/profile/snapshot.go:109-151`

`SnapshotFilesOnly` copies files and project memory that currently exist, but it never removes profile-side copies when the live config deleted them. If a user removes `settings.local.json`, `CLAUDE.md`, `~/.claude.json`, or a `projects/*/memory` tree and then runs `ccp save` or switches with auto-save enabled, the profile keeps the stale copy. Reactivating that profile later resurrects data the user explicitly deleted.

### 5. Medium - `ccp use --force` silently disables the normal auto-save behavior

Files: `cmd/use.go:44-49`, `internal/profile/activate.go:39-46`

`--force` clears `active_profile` before calling `Activate` so the already-active guard does not fire. `Activate` only auto-saves when it sees a non-empty active profile, so forcing a reactivation of the current profile skips the save path entirely. That means `ccp use <name> --force` can discard unsaved copy-file or project-memory changes even though the flag is described as a repair mechanism, not a "discard local changes" mode.

### 6. Medium - `ccp import` derives the default profile name from the full archive path instead of its basename

Files: `cmd/import_cmd.go:64-70`

When archive metadata does not provide a name, the code strips `.tar.gz` from `archivePath` directly. Importing `/tmp/work-profile.tar.gz` therefore derives `/tmp/work-profile`, which immediately fails `ValidateName` because of the slash. The basic import flow only works reliably when the archive is already in the current directory or the caller remembers to pass `--name`.

### 7. Low - Non-TTY color auto-detection is undone before every command

Files: `internal/ui/printer.go:24-33`, `cmd/root.go:21-23`

The UI package correctly disables color when stdout is not a TTY, but `PersistentPreRun` overwrites that state with the `--no-color` flag value on every command. Because the flag defaults to `false`, piped and redirected output gets ANSI escape codes unless the user explicitly passes `--no-color`. That makes scripting and machine-readable output less reliable than intended.

## Verification

Commands run during review:

- `go test ./...`
- `go vet ./...`

Both completed successfully.

## Coverage Gaps

The current tests are good at happy-path profile creation, switching, and archive traversal checks, but they do not cover the failure and deletion cases behind findings 1 through 6. The most valuable follow-up tests would be:

- activation rollback or failure-recovery coverage
- switching to a profile that lacks `claude.json` or project memory
- save/auto-save after deleting managed files
- import rejection of archive symlinks with external targets
- `use --force` preserving normal save semantics
- import name derivation from archive paths with directory components
