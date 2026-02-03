package cmd

import (
	"fmt"
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

			var m *manifest.Manifest

			// Check if -m flag was explicitly set
			if cmd.Flags().Changed("manifest") {
				// Use the specified manifest file
				log.Section("Reading " + manifestFile + "...")
				m, err = manifest.Parse(manifestFile)
				if err != nil {
					return err
				}
			} else {
				// Discover all *.graft.hcl files in current directory
				manifests, err := manifest.DiscoverManifests(cwd)
				if err != nil {
					return err
				}

				if len(manifests) == 0 {
					// no graft manifests found, recommend the scaffold command
					log.Hint("No graft manifests found in the current directory.\nYou can create one by running 'graft scaffold' command.")
					return nil
				}

				// Parse and merge all discovered graft manifests
				log.Section(fmt.Sprintf("Reading %d graft manifests...", len(manifests)))

				m, err = manifest.ParseMultiple(manifests)
				if err != nil {
					return err
				}
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

	cmd.Flags().StringVarP(&manifestFile, "manifest", "m", "manifest.graft.hcl", "Path to graft manifest (if not specified, all *.graft.hcl files in current directory will be used)")
	return cmd
}
