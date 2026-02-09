package main

import (
	"os"

	"github.com/ms-henglu/graft/cmd"
	"github.com/ms-henglu/graft/internal/log"
	"github.com/spf13/cobra"
)

var version = "v0.1.0"

func main() {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:           "graft",
		Short:         "The Overlay Engine for Terraform " + version,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			log.Init(verbose)
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(cmd.NewBuildCmd())
	rootCmd.AddCommand(cmd.NewCleanCmd())
	rootCmd.AddCommand(cmd.NewScaffoldCmd())
	rootCmd.AddCommand(cmd.NewAbsorbCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
