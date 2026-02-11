package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ms-henglu/graft/internal/absorb"
	"github.com/ms-henglu/graft/internal/log"
	"github.com/spf13/cobra"
)

func NewAbsorbCmd() *cobra.Command {
	var outputFile string
	var providersSchemaFile string

	cmd := &cobra.Command{
		Use:   "absorb <plan.json>",
		Short: "Absorb drift from Terraform plan into graft manifest",
		Long: `Absorb drift from a Terraform plan JSON file into an absorb.graft.hcl.

This command analyzes the plan JSON to identify resources with "update" actions
(drift) and generates override blocks to match the current remote state.

When a providers schema file is provided (via --providers-schema or -p), the
command uses schema information to improve the accuracy of the generated manifest.
If no providers schema file is provided, the command will attempt to run 
'terraform providers schema -json' in the current directory.

Workflow:
  1. terraform plan -out=tfplan
  2. terraform show -json tfplan > plan.json
  3. graft absorb plan.json
  4. graft build
  5. terraform plan (should show zero changes)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planFile := args[0]

			// Validate the plan file exists
			if _, err := os.Stat(planFile); os.IsNotExist(err) {
				return fmt.Errorf("plan file not found: %s", planFile)
			}

			// Validate the providers schema file exists if specified
			if providersSchemaFile != "" {
				if _, err := os.Stat(providersSchemaFile); os.IsNotExist(err) {
					return fmt.Errorf("providers schema file not found: %s", providersSchemaFile)
				}
			} else {
				// If no providers schema file specified, try to generate one
				log.Section("Fetching providers schema...")
				tmpFile, err := fetchProvidersSchema()
				if err != nil {
					log.Warn(fmt.Sprintf("Could not fetch providers schema: %s", err))
					log.Hint("Continuing without schema. For better results, run with: -p providers.json")
				} else {
					providersSchemaFile = tmpFile
					defer func() { _ = os.Remove(tmpFile) }()
				}
			}

			log.Section("Reading Terraform plan JSON...")
			driftChanges, err := absorb.ParsePlanFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to parse plan file: %w", err)
			}

			if len(driftChanges) == 0 {
				log.Hint("No drift detected in the plan. Nothing to absorb.")
				return nil
			}

			log.Section(fmt.Sprintf("Found %d resource(s) with drift...", len(driftChanges)))
			for _, change := range driftChanges {
				log.Item(change.Address)
			}

			log.Section("Generating manifest...")
			manifestContent, err := absorb.GenerateManifest(driftChanges, providersSchemaFile)
			if err != nil {
				return fmt.Errorf("failed to generate manifest: %w", err)
			}

			// Determine output file path
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			filename := outputFile
			if filename == "" {
				filename = "absorb.graft.hcl"
			}
			savePath := filename
			if !filepath.IsAbs(savePath) {
				savePath = filepath.Join(cwd, savePath)
			}

			if err := os.WriteFile(savePath, manifestContent, 0644); err != nil {
				return fmt.Errorf("failed to write manifest: %w", err)
			}

			log.Success(fmt.Sprintf("Manifest saved to %s", savePath))
			log.Hint("Next steps:\n  1. Review the generated manifest\n  2. Run 'graft build' to apply overrides\n  3. Run 'terraform plan' to verify zero changes")
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: absorb.graft.hcl)")
	cmd.Flags().StringVarP(&providersSchemaFile, "providers-schema", "p", "", "Path to providers schema JSON file (from 'terraform providers schema -json')")

	return cmd
}

// fetchProvidersSchema runs 'terraform providers schema -json' and writes the
// output to a temporary file. Returns the path to the temp file or an error.
func fetchProvidersSchema() (string, error) {
	cmd := exec.Command("terraform", "providers", "schema", "-json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("terraform providers schema failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("terraform providers schema failed: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "graft-providers-schema-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	if err := os.WriteFile(tmpPath, output, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write schema: %w", err)
	}

	return tmpPath, nil
}
