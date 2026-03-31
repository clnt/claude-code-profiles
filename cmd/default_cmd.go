package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var defaultCmd = &cobra.Command{
	Use:   "default [name]",
	Short: "Get or set the default profile",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close() //nolint:errcheck

		if len(args) == 0 {
			// Show current default
			p, err := database.GetDefault()
			if err != nil {
				return err
			}
			if p == nil {
				fmt.Println("No default profile set.")
				return nil
			}
			fmt.Println(p.Name)
			return nil
		}

		// Set default
		name := args[0]
		if err := database.SetDefault(name); err != nil {
			return err
		}
		ui.Success("Set %q as the default profile.", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(defaultCmd)
}
