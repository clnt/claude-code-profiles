package cmd

import (
	"fmt"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Switch to a profile",
	Long:  "Switch to the named profile, or the default profile if no name is given.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		noSave, _ := cmd.Flags().GetBool("no-save")
		force, _ := cmd.Flags().GetBool("force")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Use default profile
			def, err := database.GetDefault()
			if err != nil {
				return err
			}
			if def == nil {
				return fmt.Errorf("no profile name given and no default profile set. Set one with: ccp default <name>")
			}
			name = def.Name
		}

		// Clear active state if --force so Activate doesn't reject it
		if force {
			database.ClearActiveProfile()
		}

		if err := profile.Activate(database, paths, name, !noSave); err != nil {
			return err
		}

		ui.Success("Switched to profile %q. Run 'claude' or 'ccp start' to launch.", name)
		return nil
	},
}

func init() {
	useCmd.Flags().Bool("no-save", false, "don't auto-save current config before switching")
	useCmd.Flags().BoolP("force", "f", false, "re-activate even if already active (repairs symlinks)")
	rootCmd.AddCommand(useCmd)
}
