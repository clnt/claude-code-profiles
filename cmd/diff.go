package cmd

import (
	"fmt"
	"os"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/fsutil"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <profile-a> <profile-b>",
	Short: "Compare two profiles",
	Long:  "Compare two profiles file-by-file. Use @current to reference live config.",
	Args:  requireArgs("<profile-a>", "<profile-b>"),
	RunE: func(cmd *cobra.Command, args []string) error {
		nameA, nameB := args[0], args[1]

		if nameA == nameB {
			return fmt.Errorf("cannot diff a profile with itself")
		}

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close() //nolint:errcheck

		dirA, cleanupA, err := resolveProfileDir(database, paths, nameA)
		if err != nil {
			return err
		}
		if cleanupA != nil {
			defer cleanupA()
		}

		dirB, cleanupB, err := resolveProfileDir(database, paths, nameB)
		if err != nil {
			return err
		}
		if cleanupB != nil {
			defer cleanupB()
		}

		result, err := fsutil.DiffDirs(dirA, dirB)
		if err != nil {
			return fmt.Errorf("diff: %w", err)
		}

		fmt.Printf("Comparing profiles: %s vs %s\n\n", ui.Bold(nameA), ui.Bold(nameB))

		if len(result.OnlyInA) > 0 {
			fmt.Printf("%s\n", ui.Cyan("Files only in "+nameA+":"))
			for _, f := range result.OnlyInA {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
		}

		if len(result.OnlyInB) > 0 {
			fmt.Printf("%s\n", ui.Cyan("Files only in "+nameB+":"))
			for _, f := range result.OnlyInB {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
		}

		if len(result.Different) > 0 {
			fmt.Printf("%s\n", ui.Yellow("Files that differ:"))
			for _, f := range result.Different {
				fmt.Printf("  %s\n", f)
			}
			fmt.Println()
		}

		if len(result.Identical) > 0 {
			fmt.Printf("%s\n", ui.Dim("Files identical: "+fmt.Sprintf("%d", len(result.Identical))))
		}

		if len(result.OnlyInA) == 0 && len(result.OnlyInB) == 0 && len(result.Different) == 0 {
			fmt.Println(ui.Green("Profiles are identical."))
		}

		return nil
	},
}

// resolveProfileDir returns the directory to compare for a profile name.
// @current creates a temporary snapshot of the live config.
func resolveProfileDir(database *db.DB, paths config.Paths, name string) (string, func(), error) {
	if name == "@current" {
		if _, err := os.Stat(paths.ClaudeHome); os.IsNotExist(err) {
			return "", nil, fmt.Errorf("claude code config not found at %s", paths.ClaudeHome)
		}
		tmpDir, err := os.MkdirTemp("", "ccp-diff-current-*")
		if err != nil {
			return "", nil, err
		}
		if err := profile.Snapshot(paths, tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", nil, fmt.Errorf("snapshot current config: %w", err)
		}
		return tmpDir, func() { os.RemoveAll(tmpDir) }, nil
	}

	exists, err := database.ProfileExists(name)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		return "", nil, fmt.Errorf("profile %q not found", name)
	}

	return paths.ProfileDir(name), nil, nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
