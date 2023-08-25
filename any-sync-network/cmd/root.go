package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var defaultsFlag bool

var rootCmd = &cobra.Command{
	Use:   "anyconf",
	Short: "Configuration builder for Any-Sync nodes.",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	create.Flags().BoolVar(&defaultsFlag, "defaults", false, "generate configuration files using default parameters")
	rootCmd.AddCommand(create)
}
