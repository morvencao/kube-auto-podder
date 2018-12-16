package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:	"version",
	Short:	"Print the version number of auto-podder",
	Long:	`All software has version, this is auto-podder's`,
	Run:	func(cmd *cobra.Command, args []string) {
		fmt.Println("Auto-podder v0.1.0 -- HEAD")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
