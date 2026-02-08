package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/MSmaili/hetki/internal/logger"
	"github.com/spf13/cobra"
)

const (
	modulePath       = "github.com/MSmaili/hetki@latest"
	modulePathSource = "github.com/MSmaili/hetki@main"
	githubRepo       = "MSmaili/hetki"
	githubAPIURL     = "https://api.github.com/repos/"
	githubReleaseURL = "https://github.com/"
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
	logger.SetVerbose(updateVerbose)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	logger.Debug("Executable path: %s", exePath)

	updater, err := determineUpdater(exePath)
	if err != nil {
		return err
	}

	logger.Verbose("Detected installation method: %s", updater.Name())

	if updateDryRun {
		updater.DryRun()
		return nil
	}

	latestVersion, err := getLatestVersion()
	if err != nil {
		logger.Debug("Could not check latest version: %v", err)
		logger.Info("Could not check latest version, proceeding with update")
	} else if Version != "dev" && latestVersion == Version {
		logger.Success("Already on the latest version: %s", Version)
		return nil
	} else if Version == "dev" {
		logger.Info("Development build detected, will update to: %s", latestVersion)
	} else {
		logger.Info("Current version: %s", Version)
		logger.Info("Latest version: %s", latestVersion)
	}

	if err := updater.Update(latestVersion); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	logger.Success("Update completed successfully")
	return nil
}

func getLatestVersion() (string, error) {
	url := githubAPIURL + githubRepo + "/releases/latest"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

type Updater interface {
	Name() string
	Update(latestVersion string) error
	DryRun()
}

func determineUpdater(exePath string) (Updater, error) {
	if installedViaGo(exePath) {
		return &GoUpdater{}, nil
	}

	if isUserLocalInstall(exePath) {
		return &BinaryUpdater{exePath: exePath}, nil
	}

	return nil, errors.New(
		"hetki was not installed via `go install` or to ~/.local/bin or ~/bin; manual update required",
	)
}

func isUserLocalInstall(exePath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return false
	}

	dir := filepath.Dir(resolved)
	return dir == filepath.Join(home, ".local", "bin") || dir == filepath.Join(home, "bin")
}

type GoUpdater struct{}

func (g *GoUpdater) Name() string { return "go install" }

func (g *GoUpdater) DryRun() {
	module := modulePath
	if updateFromSource {
		module = modulePathSource
	}

	logger.Info("Would run: go install %s", module)
}

func (g *GoUpdater) Update(_ string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go binary not found in PATH")
	}

	module := modulePath
	if updateFromSource {
		module = modulePathSource
		logger.Debug("Building from source: %s", module)
	} else {
		logger.Debug("Installing release: %s", module)
	}

	logger.Info("Updating hetki...")

	args := []string{"install"}
	if updateVerbose {
		args = append(args, "-v")
	}
	args = append(args, module)

	logger.Debug("Running command: go %s", strings.Join(args, " "))

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type BinaryUpdater struct {
	exePath string
}

func (b *BinaryUpdater) Name() string { return "binary release" }

func (b *BinaryUpdater) DryRun() {
	if updateFromSource {
		logger.Info("Would build from source: go install %s", modulePathSource)
		logger.Info("Note: --source with binary install falls back to go install")
	} else {
		binaryName := fmt.Sprintf("hetki-%s-%s", runtime.GOOS, runtime.GOARCH)
		logger.Info("Would download: %s%s/releases/latest/download/%s", githubReleaseURL, githubRepo, binaryName)
		logger.Info("Would verify checksum and replace: %s", b.exePath)
	}
}

func (b *BinaryUpdater) Update(latestVersion string) error {
	if updateFromSource {
		logger.Info("--source flag set, falling back to go install...")
		return (&GoUpdater{}).Update(latestVersion)
	}

	if latestVersion == "" {
		return errors.New("could not determine latest version")
	}

	binaryName := fmt.Sprintf("hetki-%s-%s", runtime.GOOS, runtime.GOARCH)
	downloadURL := fmt.Sprintf("%s%s/releases/download/%s/%s", githubReleaseURL, githubRepo, latestVersion, binaryName)
	checksumsURL := fmt.Sprintf("%s%s/releases/download/%s/checksums.txt", githubReleaseURL, githubRepo, latestVersion)

	logger.Info("Downloading hetki %s for %s/%s...", latestVersion, runtime.GOOS, runtime.GOARCH)

	tempFile, err := os.CreateTemp(filepath.Dir(b.exePath), "hetki-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := downloadToFile(downloadURL, tempFile); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	info, err := os.Stat(tempPath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	if info.Size() < 1<<20 {
		return fmt.Errorf("downloaded file is too small (%d bytes), expected a Go binary (>1MB)", info.Size())
	}

	if err := verifyBinaryChecksum(checksumsURL, tempPath, binaryName); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tempPath, b.exePath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

func downloadToFile(url string, f *os.File) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}

	return f.Close()
}

func verifyBinaryChecksum(checksumsURL, filePath, binaryName string) error {
	logger.Info("Verifying checksum...")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums: HTTP %d", resp.StatusCode)
	}

	var expectedHash string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == binaryName {
			expectedHash = parts[0]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read checksums: %w", err)
	}

	if expectedHash == "" {
		return fmt.Errorf("binary %s not found in checksums.txt", binaryName)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	logger.Info("Checksum verified: %s", actualHash)
	return nil
}

func installedViaGo(exePath string) bool {
	exeReal, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		logger.Debug("Failed to resolve symlinks for %s: %v", exePath, err)
		return false
	}
	logger.Debug("Resolved executable path: %s", exeReal)

	for _, dir := range goBinDirs() {
		dirReal, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}

		logger.Debug("Checking Go bin directory: %s", dirReal)
		if isWithinDir(exeReal, dirReal) {
			logger.Debug("Executable is within Go bin directory")
			return true
		}
	}

	return false
}

func goBinDirs() []string {
	var dirs []string

	if gobin := os.Getenv("GOBIN"); gobin != "" {
		dirs = append(dirs, gobin)
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, p := range filepath.SplitList(gopath) {
			dirs = append(dirs, filepath.Join(p, "bin"))
		}
	}

	if len(dirs) == 0 {
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "go", "bin"))
		}
	}

	return dirs
}

func isWithinDir(file, dir string) bool {
	rel, err := filepath.Rel(dir, file)
	return err == nil && !strings.HasPrefix(rel, "..")
}
