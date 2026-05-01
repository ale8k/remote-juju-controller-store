package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear local RCS session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := clearSession(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "logged out")
		return nil
	},
}
