package update

import (
	"bufio"
	"context"
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
	"syscall"
	"time"

	"github.com/MSmaili/hetki/internal/logger"
)

const (
	modulePath       = "github.com/MSmaili/hetki@latest"
	modulePathSource = "github.com/MSmaili/hetki@main"
	githubRepo       = "MSmaili/hetki"
	githubAPIURL     = "https://api.github.com/repos/"
	githubReleaseURL = "https://github.com/"
)

type Options struct {
	CurrentVersion string
	FromSource     bool
	DryRun         bool
	Verbose        bool
}

type Updater interface {
	Name() string
	Update(context.Context, string) error
	DryRun()
}

type Service struct {
	SetVerbose       func(bool)
	Executable       func() (string, error)
	DetermineUpdater func(string) (Updater, error)
	GetLatestVersion func(context.Context) (string, error)
}

func NewService() Service {
	return Service{}
}

func (s Service) Run(ctx context.Context, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.setVerbose(opts.Verbose)

	exePath, err := s.executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	logger.Debug("Executable path: %s", exePath)

	updater, err := s.determineUpdater(exePath, opts)
	if err != nil {
		return err
	}

	logger.Verbose("Detected installation method: %s", updater.Name())

	if opts.DryRun {
		updater.DryRun()
		return nil
	}

	latestVersion, err := s.getLatestVersion(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		logger.Debug("Could not check latest version: %v", err)
		logger.Info("Could not check latest version, proceeding with update")
	} else if opts.CurrentVersion != "dev" && latestVersion == opts.CurrentVersion {
		logger.Success("Already on the latest version: %s", opts.CurrentVersion)
		return nil
	} else if opts.CurrentVersion == "dev" {
		logger.Info("Development build detected, will update to: %s", latestVersion)
	} else {
		logger.Info("Current version: %s", opts.CurrentVersion)
		logger.Info("Latest version: %s", latestVersion)
	}

	if err := updater.Update(ctx, latestVersion); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	logger.Success("Update completed successfully")
	return nil
}

func (s Service) setVerbose(verbose bool) {
	if s.SetVerbose != nil {
		s.SetVerbose(verbose)
		return
	}
	logger.SetVerbose(verbose)
}

func (s Service) executable() (string, error) {
	if s.Executable != nil {
		return s.Executable()
	}
	return os.Executable()
}

func (s Service) determineUpdater(exePath string, opts Options) (Updater, error) {
	if s.DetermineUpdater != nil {
		return s.DetermineUpdater(exePath)
	}
	return DetermineUpdater(exePath, opts)
}

func (s Service) getLatestVersion(ctx context.Context) (string, error) {
	if s.GetLatestVersion != nil {
		return s.GetLatestVersion(ctx)
	}
	return GetLatestVersion(ctx)
}

func GetLatestVersion(ctx context.Context) (string, error) {
	url := githubAPIURL + githubRepo + "/releases/latest"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

func DetermineUpdater(exePath string, opts Options) (Updater, error) {
	if installedViaGo(exePath) {
		return &GoUpdater{FromSource: opts.FromSource, Verbose: opts.Verbose}, nil
	}

	if isUserLocalInstall(exePath) {
		return &BinaryUpdater{exePath: exePath, FromSource: opts.FromSource}, nil
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

type GoUpdater struct {
	FromSource bool
	Verbose    bool
}

func (g *GoUpdater) Name() string { return "go install" }

func (g *GoUpdater) DryRun() {
	module := modulePath
	if g.FromSource {
		module = modulePathSource
	}

	logger.Info("Would run: go install %s", module)
}

func (g *GoUpdater) Update(ctx context.Context, _ string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go binary not found in PATH")
	}

	module := modulePath
	if g.FromSource {
		module = modulePathSource
		logger.Debug("Building from source: %s", module)
	} else {
		logger.Debug("Installing release: %s", module)
	}

	logger.Info("Updating hetki...")

	args := []string{"install"}
	if g.Verbose {
		args = append(args, "-v")
	}
	args = append(args, module)

	logger.Debug("Running command: go %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(ctxErr, err)
	}
	return err
}

type BinaryUpdater struct {
	exePath    string
	FromSource bool
}

func (b *BinaryUpdater) Name() string { return "binary release" }

func (b *BinaryUpdater) DryRun() {
	if b.FromSource {
		logger.Info("Would build from source: go install %s", modulePathSource)
		logger.Info("Note: --source with binary install falls back to go install")
	} else {
		binaryName := fmt.Sprintf("hetki-%s-%s", runtime.GOOS, runtime.GOARCH)
		logger.Info("Would download: %s%s/releases/latest/download/%s", githubReleaseURL, githubRepo, binaryName)
		logger.Info("Would verify checksum and replace: %s", b.exePath)
	}
}

func (b *BinaryUpdater) Update(ctx context.Context, latestVersion string) error {
	if b.FromSource {
		logger.Info("--source flag set, falling back to go install...")
		return (&GoUpdater{FromSource: true}).Update(ctx, latestVersion)
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

	if err := downloadToFile(ctx, downloadURL, tempFile); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	info, err := os.Stat(tempPath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	if info.Size() < 1<<20 {
		return fmt.Errorf("downloaded file is too small (%d bytes), expected a Go binary (>1MB)", info.Size())
	}

	if err := verifyBinaryChecksum(ctx, checksumsURL, tempPath, binaryName); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tempPath, b.exePath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

func downloadToFile(ctx context.Context, url string, f *os.File) (err error) {
	defer func() { err = errors.Join(err, f.Close()) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyBinaryChecksum(ctx context.Context, checksumsURL, filePath, binaryName string) error {
	logger.Info("Verifying checksum...")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
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
	if _, err := io.Copy(h, contextReader{ctx: ctx, reader: f}); err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	logger.Info("Checksum verified: %s", actualHash)
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, err
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
