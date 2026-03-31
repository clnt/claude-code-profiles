package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current config to the active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close() //nolint:errcheck

		active, err := database.GetActiveProfile()
		if err != nil {
			return fmt.Errorf("get active profile: %w", err)
		}
		if active == "" {
			return fmt.Errorf("no active profile. Create one with: ccp create <name>")
		}

		if err := profile.Save(database, paths, active); err != nil {
			return err
		}

		ui.Success("Saved current config to profile %q.", active)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
