package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/clnt/claude-code-profiles/internal/archive"
	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a profile as a tar.gz archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		output, _ := cmd.Flags().GetString("output")
		stripPaths, _ := cmd.Flags().GetBool("strip-paths")
		allComponents, _ := cmd.Flags().GetBool("all")

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

		if output == "" {
			output = name + ".tar.gz"
		}

		profileDir := paths.ProfileDir(name)

		// Build export options
		opts := archive.ExportOptions{
			ProfileDir: profileDir,
			OutputPath: output,
			StripPaths: stripPaths,
		}

		if allComponents {
			opts.IncludeSettings = true
			opts.IncludePlugins = true
			opts.IncludeSkills = true
			opts.IncludeAgents = true
			opts.IncludeProjects = true
			opts.IncludeClaudeJSON = true
			opts.IncludeKeybindings = true
			opts.IncludeClaudeMD = true
		} else {
			// Interactive component selection
			opts = selectComponents(opts)
		}

		// Write metadata.json for the archive
		p, _ := database.GetProfile(name)
		if p != nil {
			meta := map[string]interface{}{
				"name":        p.Name,
				"description": p.Description,
				"created_at":  p.CreatedAt,
				"updated_at":  p.UpdatedAt,
			}
			data, _ := json.MarshalIndent(meta, "", "  ")
			metaPath := fmt.Sprintf("%s/metadata.json", profileDir)
			os.WriteFile(metaPath, data, 0644)
			defer os.Remove(metaPath)
		}

		if err := archive.Export(opts); err != nil {
			return fmt.Errorf("export: %w", err)
		}

		// Get file size
		info, _ := os.Stat(output)
		size := ""
		if info != nil {
			size = fmt.Sprintf(" (%.1f KB)", float64(info.Size())/1024)
		}

		ui.Success("Exported profile %q to %s%s", name, output, size)
		return nil
	},
}

func selectComponents(opts archive.ExportOptions) archive.ExportOptions {
	type component struct {
		name     string
		field    *bool
		warning  string
		default_ bool
	}

	components := []component{
		{"settings.json", &opts.IncludeSettings, "", true},
		{"plugins/", &opts.IncludePlugins, "", true},
		{"skills/", &opts.IncludeSkills, "", true},
		{"agents/", &opts.IncludeAgents, "", true},
		{"CLAUDE.md", &opts.IncludeClaudeMD, "", true},
		{"keybindings.json", &opts.IncludeKeybindings, "", true},
		{"projects/", &opts.IncludeProjects, "(contains project names)", false},
		{"claude.json", &opts.IncludeClaudeJSON, "(contains project paths)", false},
	}

	fmt.Println(ui.Bold("Select components to include in export:"))
	reader := bufio.NewReader(os.Stdin)

	for _, c := range components {
		defaultStr := "Y/n"
		if !c.default_ {
			defaultStr = "y/N"
		}

		warning := ""
		if c.warning != "" {
			warning = " " + ui.Yellow(c.warning)
		}

		fmt.Printf("  Include %s?%s [%s] ", c.name, warning, defaultStr)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" {
			*c.field = c.default_
		} else {
			*c.field = response == "y" || response == "yes"
		}
	}
	fmt.Println()

	return opts
}

func init() {
	exportCmd.Flags().StringP("output", "o", "", "output file path (default: <name>.tar.gz)")
	exportCmd.Flags().Bool("strip-paths", false, "anonymize filesystem paths with placeholders")
	exportCmd.Flags().Bool("all", false, "include all components without prompting")
	rootCmd.AddCommand(exportCmd)
}
