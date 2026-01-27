package cmd

import (
	"os"

	"github.com/ms-henglu/graft/internal/log"
	"github.com/ms-henglu/graft/internal/manifest"
	"github.com/ms-henglu/graft/internal/patch"
	"github.com/ms-henglu/graft/internal/vendors"
	"github.com/spf13/cobra"
)

func NewBuildCmd() *cobra.Command {
	var manifestFile string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Vendors modules and applies patches",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			log.Section("Reading " + manifestFile + "...")
			m, err := manifest.Parse(manifestFile)
			if err != nil {
				return err
			}

			log.Section("Vendoring modules...")
			vendorMap, err := vendors.VendorModules(cwd, m)
			if err != nil {
				return err
			}

			log.Section("Applying patches...")
			if err := patch.ApplyPatches(vendorMap, m); err != nil {
				return err
			}

			// Link Stage: Redirect modules
			if len(vendorMap) > 0 {
				log.Section("Linking modules...")
				if err := vendors.RedirectModules(cwd); err != nil {
					return err
				}
			}

			log.Success("Build complete!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&manifestFile, "manifest", "m", "manifest.graft.hcl", "Path to manifest file")
	return cmd
}
