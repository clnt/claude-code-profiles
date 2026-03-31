package cmd

import (
	"fmt"
	"os"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/spf13/cobra"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		active, err := database.GetActiveProfile()
		if err != nil {
			return fmt.Errorf("get active profile: %w", err)
		}

		if active == "" {
			if quiet {
				os.Exit(1)
			}
			fmt.Println("No active profile.")
			return nil
		}

		if !quiet {
			fmt.Println(active)
		}
		return nil
	},
}

func init() {
	currentCmd.Flags().BoolP("quiet", "q", false, "exit code only (0=active, 1=none)")
	rootCmd.AddCommand(currentCmd)
}
