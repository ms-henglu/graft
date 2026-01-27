package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ms-henglu/graft/internal/log"
	"github.com/ms-henglu/graft/internal/vendors"
	"github.com/spf13/cobra"
)

func NewCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Cleans up graft artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			log.Section("Removing build artifacts...")
			// 1. Delete .graft folder
			graftDir := filepath.Join(cwd, ".graft")
			if _, err := os.Stat(graftDir); !os.IsNotExist(err) {
				if err := os.RemoveAll(graftDir); err != nil {
					return fmt.Errorf("failed to remove .graft directory: %w", err)
				}
				log.Item(".graft directory")
			}

			// 2. Delete _graft_override.tf and _graft_add.tf files
			for _, fileName := range []string{"_graft_add.tf", "_graft_override.tf"} {
				fileToRemove := filepath.Join(cwd, fileName)
				if _, err := os.Stat(fileToRemove); !os.IsNotExist(err) {
					if err := os.Remove(fileToRemove); err != nil {
						return fmt.Errorf("failed to remove %s: %w", fileName, err)
					}
					log.Item(fileName)
				}
			}

			log.Section("Resetting module links...")
			// 3. Delete entries in modules.json redirecting to .graft
			modulesJSONPath := filepath.Join(cwd, ".terraform", "modules", "modules.json")
			if _, err := os.Stat(modulesJSONPath); err == nil {
				modulesJSON, err := vendors.LoadModulesJSON(cwd)
				if err != nil {
					return err
				}

				var keptModules []vendors.Module
				for _, mod := range modulesJSON.Modules {
					// Check if Dir points to .graft
					// We check if the path contains .graft/build or starts with .graft
					if !strings.Contains(mod.Dir, ".graft") {
						keptModules = append(keptModules, mod)
					}
				}

				modulesJSON.Modules = keptModules
				updatedData, err := json.MarshalIndent(modulesJSON, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal modules.json: %w", err)
				}

				if err := os.WriteFile(modulesJSONPath, updatedData, 0644); err != nil {
					return fmt.Errorf("failed to write modules.json: %w", err)
				}
				log.Item("modules.json updated")

				// Only hint to restore if we actually changed modules.json (removed links)
				log.Hint("Next Step: Run 'terraform init' to restore original paths.")
			}

			log.Success("Clean complete!")

			return nil
		},
	}

	return cmd
}
