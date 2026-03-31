package cmd

import (
	"fmt"
	"os"

	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
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
		if noColor {
			ui.SetNoColor(true)
		}
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

// requireArgs returns a cobra.PositionalArgs validator that produces
// a helpful error message naming each missing argument.
//
//	requireArgs("<name>")           → 'missing required argument: <name>'
//	requireArgs("<profile-a>", "<profile-b>") with 1 arg → 'missing required argument: <profile-b>'
func requireArgs(names ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= len(names) {
			return nil
		}
		missing := names[len(args)]
		return fmt.Errorf("missing required argument: %s\n\nUsage:\n  %s", missing, cmd.UseLine())
	}
}
