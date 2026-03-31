package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/clnt/claude-code-profiles/internal/ui"
)

var (
	verbose bool
	noColor bool
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "ccp",
	Short: "Claude Code Profiles - manage configuration profiles for Claude Code",
	Long:  "ccp lets you create, switch, and share Claude Code configuration profiles.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ui.SetNoColor(noColor)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
}

// SetVersion sets the version string (injected at build time).
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.Error("%s", err)
		os.Exit(1)
	}
}
