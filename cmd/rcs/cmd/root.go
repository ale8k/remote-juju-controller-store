package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "rcs",
	Short:         "Remote Juju controller store CLI",
	Long:          `rcs is a CLI for managing a remote Juju controller store.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrln("error:", err)
		os.Exit(1)
	}
}
