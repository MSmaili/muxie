package cmd

import (
	"fmt"
	"os"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/logger"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"

	detectBackend = backend.Detect
)

var rootCmd = &cobra.Command{
	Use:           "hetki",
	Short:         "hetki - Terminal Multiplexer Session Manager",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `hetki is a terminal multiplexer session manager that helps you manage complex tmux sessions with less manual setup.

It supports:
- Multiple sessions and windows with panes
- Strict YAML configuration files
- Named and local workspaces
- Templates for reusable configurations`,
	Version: Version,
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("hetki version %s\ncommit: %s\nbuilt: %s\n", Version, GitCommit, BuildDate))
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
}
