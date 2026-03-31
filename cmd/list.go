package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all profiles",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		profiles, err := database.ListProfiles()
		if err != nil {
			return fmt.Errorf("list profiles: %w", err)
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found. Create one with: ccp create <name>")
			return nil
		}

		active, _ := database.GetActiveProfile()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
			ui.Bold("NAME"), ui.Bold("DESCRIPTION"), ui.Bold("UPDATED"), ui.Bold("DEFAULT"))

		for _, p := range profiles {
			marker := " "
			if p.Name == active {
				marker = ui.Green("*")
			}

			defaultMark := ""
			if p.IsDefault {
				defaultMark = ui.Dim("(default)")
			}

			fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n",
				marker,
				p.Name,
				truncate(p.Description, 30),
				p.UpdatedAt.Local().Format("2006-01-02 15:04"),
				defaultMark,
			)
		}
		w.Flush()
		return nil
	},
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	rootCmd.AddCommand(listCmd)
}
