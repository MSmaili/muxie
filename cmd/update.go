package cmd

import (
	appupdate "github.com/MSmaili/hetki/internal/app/update"
	"github.com/spf13/cobra"
)

var (
	updateFromSource bool
	updateDryRun     bool
	updateVerbose    bool
	updateTargetVer  string
	updateAllowPre   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hetki to the latest version",
	Long: `Update hetki to a verified release.

By default installs the newest stable release; prereleases are ignored unless
--pre is passed. Use --version to pin or downgrade to an exact release. Every
downloaded binaries are verified against release attestations before replacement.
Source installs use the Go checksum database. Emergency-only bypass:
HETKI_UNSAFE_SKIP_VERIFY=1.`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&updateFromSource, "source", false, "Build from source via go install instead of downloading a release binary")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Show what would be done without updating")
	updateCmd.Flags().BoolVarP(&updateVerbose, "verbose", "v", false, "Show verbose output")
	updateCmd.Flags().StringVar(&updateTargetVer, "version", "", "Install this exact release tag (e.g. v1.2.3); allows downgrade or reinstall")
	updateCmd.Flags().BoolVar(&updateAllowPre, "pre", false, "Consider prereleases; required for any prerelease target")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	return appupdate.NewService().Run(cmd.Context(), appupdate.Options{
		CurrentVersion:  Version,
		TargetVersion:   updateTargetVer,
		AllowPrerelease: updateAllowPre,
		FromSource:      updateFromSource,
		DryRun:          updateDryRun,
		Verbose:         updateVerbose,
	})
}
