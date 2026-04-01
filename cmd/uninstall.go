package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ccp and restore Claude Code config to standalone state",
	Long: `Uninstall resolves all symlinks back to real directories, preserving
the active profile's configuration in ~/.claude/, then removes all
stored profiles and the .ccp data directory.

After uninstalling, Claude Code's configuration is fully standalone
with no dependency on ccp.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		paths := config.ResolvePaths()

		// Check if ccp is even installed
		if _, err := db.Open(paths.DatabasePath()); err != nil {
			return fmt.Errorf("no ccp installation found at %s", paths.CCPHome)
		}

		// Auto-save the active profile before uninstalling
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}

		active, err := database.GetActiveProfile()
		if err != nil {
			database.Close() //nolint:errcheck
			return fmt.Errorf("get active profile: %w", err)
		}

		if active != "" {
			if err := profile.Save(database, paths, active); err != nil {
				database.Close() //nolint:errcheck
				return fmt.Errorf("save active profile %q: %w", active, err)
			}
		}

		database.Close() //nolint:errcheck

		if !force {
			msg := "Uninstall ccp? This will remove all stored profiles and the .ccp data directory."
			if active != "" {
				msg = fmt.Sprintf("Uninstall ccp? Configuration from profile %q will be kept in ~/.claude/. All stored profiles and the .ccp data directory will be removed.", active)
			}
			if !ui.Confirm(msg) {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := profile.Uninstall(paths); err != nil {
			return err
		}

		ui.Success("Uninstalled ccp. Claude Code config is now standalone.")
		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "skip confirmation")
	rootCmd.AddCommand(uninstallCmd)
}
