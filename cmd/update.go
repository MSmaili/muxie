package cmd

import (
	appupdate "github.com/MSmaili/hetki/internal/app/update"
	"github.com/spf13/cobra"
)

var (
	updateFromSource bool
	updateDryRun     bool
	updateVerbose    bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hetki to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateFromSource, "source", false, "Build from source instead of using release")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be done without updating")
	updateCmd.Flags().BoolVarP(&updateVerbose, "verbose", "v", false, "Show verbose output")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	return appupdate.NewService().Run(cmd.Context(), appupdate.Options{
		CurrentVersion: Version,
		FromSource:     updateFromSource,
		DryRun:         updateDryRun,
		Verbose:        updateVerbose,
	})
}
