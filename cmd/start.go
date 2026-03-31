package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [profile] [-- claude-args...]",
	Short: "Switch profile and launch Claude Code",
	Long: `Switch to the given profile (or default) and launch Claude Code.

Any arguments after -- are passed directly to claude.

Examples:
  ccp start work
  ccp start personal -- --model sonnet
  ccp start                          # uses default profile`,
	Args:                  cobra.ArbitraryArgs,
	DisableFlagParsing:    false,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close() //nolint:errcheck

		// Determine profile name
		var profileName string
		var claudeArgs []string

		if len(args) > 0 {
			profileName = args[0]
			claudeArgs = args[1:]
		}

		if profileName == "" {
			// Use default profile
			def, err := database.GetDefault()
			if err != nil {
				return err
			}
			if def == nil {
				return fmt.Errorf("no profile specified and no default set. Set one with: ccp default <name>")
			}
			profileName = def.Name
		}

		// Switch profile if not already active
		active, _ := database.GetActiveProfile()
		if active != profileName {
			if err := profile.Activate(database, paths, profileName, active != ""); err != nil {
				return err
			}
			ui.Success("Switched to profile %q.", profileName)
		} else {
			ui.Info("Profile %q already active.", profileName)
		}

		// Find claude binary
		claudeBin, err := findClaude()
		if err != nil {
			return err
		}

		// Exec into claude (replaces this process)
		env := os.Environ()
		return execClaude(claudeBin, claudeArgs, env)
	},
}

// findClaude locates the claude binary on PATH.
func findClaude() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude not found on PATH. Install it from https://claude.ai/download")
	}
	return path, nil
}

// execClaude replaces the current process with claude.
func execClaude(bin string, args []string, env []string) error {
	argv := append([]string{"claude"}, args...)
	return execvp(bin, argv, env)
}

func init() {
	rootCmd.AddCommand(startCmd)
}
