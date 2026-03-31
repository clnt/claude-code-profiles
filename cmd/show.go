package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clnt/claude-code-profiles/internal/config"
	"github.com/clnt/claude-code-profiles/internal/db"
	"github.com/clnt/claude-code-profiles/internal/ui"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show profile details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		asJSON, _ := cmd.Flags().GetBool("json")

		paths := config.ResolvePaths()
		database, err := db.Open(paths.DatabasePath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close()

		p, err := database.GetProfile(name)
		if err != nil {
			return fmt.Errorf("profile %q not found", name)
		}

		active, _ := database.GetActiveProfile()
		profileDir := paths.ProfileDir(name)

		if asJSON {
			return showJSON(p, active, profileDir)
		}

		fmt.Printf("%s %s\n", ui.Bold("Profile:"), p.Name)
		if p.Description != "" {
			fmt.Printf("%s %s\n", ui.Bold("Description:"), p.Description)
		}
		fmt.Printf("%s %s\n", ui.Bold("Created:"), p.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("%s %s\n", ui.Bold("Updated:"), p.UpdatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Printf("%s %v\n", ui.Bold("Active:"), active == p.Name)
		fmt.Printf("%s %v\n", ui.Bold("Default:"), p.IsDefault)
		fmt.Println()

		// List files
		fmt.Println(ui.Bold("Files:"))
		if err := walkProfileFiles(profileDir, ""); err != nil {
			fmt.Println(ui.Dim("  (empty)"))
		}

		return nil
	},
}

func walkProfileFiles(root, prefix string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 && prefix == "" {
		return fmt.Errorf("empty")
	}

	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		rel := prefix + entry.Name()

		if entry.IsDir() {
			// Count items in directory
			subEntries, _ := os.ReadDir(path)
			fmt.Printf("  %s %s\n", rel+"/", ui.Dim(fmt.Sprintf("(%d items)", len(subEntries))))
			walkProfileFiles(path, rel+"/")
		} else {
			info, _ := entry.Info()
			size := formatSize(info.Size())
			fmt.Printf("  %s %s\n", rel, ui.Dim(size))
		}
	}
	return nil
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

type showOutput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Active      bool     `json:"active"`
	IsDefault   bool     `json:"is_default"`
	Files       []string `json:"files"`
}

func showJSON(p *db.Profile, active, profileDir string) error {
	out := showOutput{
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		Active:      active == p.Name,
		IsDefault:   p.IsDefault,
	}

	filepath.Walk(profileDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == profileDir {
			return nil
		}
		rel, _ := filepath.Rel(profileDir, path)
		if info.IsDir() {
			rel += "/"
		}
		rel = strings.ReplaceAll(rel, "\\", "/")
		out.Files = append(out.Files, rel)
		return nil
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	showCmd.Flags().Bool("json", false, "output as JSON")
	rootCmd.AddCommand(showCmd)
}
