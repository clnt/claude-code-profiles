package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/clnt/claude-code-profiles/internal/archive"
	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/profile"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a profile from a tar.gz archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		archivePath := args[0]
		nameOverride, _ := cmd.Flags().GetString("name")
		mapPathFlags, _ := cmd.Flags().GetStringSlice("map-path")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		// Parse --map-path flags
		pathMap := make(map[string]string)
		for _, mp := range mapPathFlags {
			parts := strings.SplitN(mp, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid --map-path format: %q (expected '<placeholder>=<path>')", mp)
			}
			pathMap[parts[0]] = parts[1]
		}

		// Import to a temporary location first to read metadata
		tmpDir, err := os.MkdirTemp("", "ccp-import-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		result, err := archive.Import(archive.ImportOptions{
			ArchivePath: archivePath,
			ProfileDir:  tmpDir,
			PathMap:     nil, // Don't apply yet - need to prompt first
		})
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}

		// Determine profile name
		name := result.Name
		if nameOverride != "" {
			name = nameOverride
		}
		if name == "" {
			name = strings.TrimSuffix(strings.TrimSuffix(archivePath, ".gz"), ".tar")
			name = strings.TrimSuffix(name, ".tar")
		}

		if err := profile.ValidateName(name); err != nil {
			return fmt.Errorf("derived name %q is invalid: %w. Use --name to specify a valid name", name, err)
		}

		exists, err := database.ProfileExists(name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("profile %q already exists. Use --name <new-name> to import with a different name", name)
		}

		// Handle path mappings - prompt user if there are unmapped placeholders
		if len(result.PathMappings) > 0 {
			for _, pm := range result.PathMappings {
				if _, mapped := pathMap[pm.Placeholder]; !mapped {
					// Prompt user
					fmt.Printf("  %s (%s): ", ui.Cyan(pm.Placeholder), ui.Dim(pm.Description))
					reader := bufio.NewReader(os.Stdin)
					response, _ := reader.ReadString('\n')
					response = strings.TrimSpace(response)
					if response != "" {
						pathMap[pm.Placeholder] = response
					}
				}
			}
			if len(pathMap) > 0 {
				fmt.Println()
			}
		}

		// Now do the real import with path mappings applied
		profileDir := paths.ProfileDir(name)
		_, err = archive.Import(archive.ImportOptions{
			ArchivePath: archivePath,
			ProfileDir:  profileDir,
			PathMap:     pathMap,
		})
		if err != nil {
			os.RemoveAll(profileDir)
			return fmt.Errorf("import: %w", err)
		}

		// Register in database
		description := result.Description
		if err := database.CreateProfile(name, description, false); err != nil {
			os.RemoveAll(profileDir)
			return fmt.Errorf("register profile: %w", err)
		}

		ui.Success("Imported profile %q. Activate with: ccp use %s", name, name)
		return nil
	},
}

func init() {
	importCmd.Flags().String("name", "", "override the profile name")
	importCmd.Flags().StringSlice("map-path", nil, "map anonymized path placeholder to local path (e.g., '<project-1>=/Users/you/Projects/app')")
	rootCmd.AddCommand(importCmd)
}
