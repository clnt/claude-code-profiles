package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new profile from current Claude Code config",
	Args:  requireArgs("<name>"),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		description, _ := cmd.Flags().GetString("description")
		blank, _ := cmd.Flags().GetBool("blank")
		setDefault, _ := cmd.Flags().GetBool("default")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		if err := profile.Create(database, paths, name, description, blank, setDefault); err != nil {
			return err
		}

		if blank {
			ui.Success("Created blank profile %q.", name)
		} else {
			ui.Success("Created profile %q from current config.", name)
		}
		return nil
	},
}

func init() {
	createCmd.Flags().StringP("description", "d", "", "profile description")
	createCmd.Flags().Bool("blank", false, "create an empty profile")
	createCmd.Flags().Bool("default", false, "set as the default profile")
	rootCmd.AddCommand(createCmd)
}
