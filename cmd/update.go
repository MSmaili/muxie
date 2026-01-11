package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MSmaili/tms/internal/logger"
	"github.com/spf13/cobra"
)

const (
	modulePath       = "github.com/MSmaili/tms@latest"
	installScriptURL = "https://raw.githubusercontent.com/MSmaili/tms/main/install.sh"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update tms to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	logger.Info("Updating tms...")

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	if isGoInstall(exePath) {
		logger.Debug("Detected Go-based installation", "path", exePath)
		return updateViaGo()
	}

	logger.Debug("Detected script-based installation", "path", exePath)
	return updateViaScript()
}

func isGoInstall(exePath string) bool {
	exePath = filepath.Clean(exePath)

	var binDirs []string

	if gobin := os.Getenv("GOBIN"); gobin != "" {
		binDirs = append(binDirs, gobin)
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, p := range filepath.SplitList(gopath) {
			binDirs = append(binDirs, filepath.Join(p, "bin"))
		}
	}

	for _, dir := range binDirs {
		dir = filepath.Clean(dir) + string(os.PathSeparator)
		if strings.HasPrefix(exePath, dir) {
			return true
		}
	}

	return false
}

func updateViaGo() error {
	logger.Info("Updating via go install...")

	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}

	if err := runCommand("go", "install", modulePath); err != nil {
		return fmt.Errorf("go install failed: %w", err)
	}

	logger.Success("Updated successfully via go install")
	return nil
}

func updateViaScript() error {
	logger.Info("Updating via install script...")

	if err := runCommand(
		"bash",
		"-c",
		fmt.Sprintf("curl -fsSL %s | bash", installScriptURL),
	); err != nil {
		return fmt.Errorf("script update failed: %w", err)
	}

	logger.Success("Updated successfully via install script")
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
