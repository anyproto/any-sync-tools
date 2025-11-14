package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var autoFlag bool
var templatePath string
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
	rootCmd.AddCommand(create)
	create.Flags().BoolVar(&autoFlag, "auto", false, "auto generation in non-interactive mode")
	create.Flags().StringVar(&templatePath, "c", "./defaultTemplate.yml", "path to the template file")
}
