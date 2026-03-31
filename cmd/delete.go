package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		exists, err := database.ProfileExists(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", name)
		}

		if !force {
			if !ui.Confirm(fmt.Sprintf("Delete profile %q? This cannot be undone.", name)) {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := profile.Delete(database, paths, name); err != nil {
			return err
		}

		ui.Success("Deleted profile %q.", name)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "skip confirmation")
	rootCmd.AddCommand(deleteCmd)
}
