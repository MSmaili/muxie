package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	modulePath = "github.com/MSmaili/hetki"
	githubRepo = "MSmaili/hetki"

	// maxBinaryBytes bounds release binary downloads; maxChecksumsBytes
	// bounds checksums.txt.
	maxBinaryBytes          = 128 << 20
	maxChecksumsBytes       = 64 << 10
	maxSourceWorkspaceBytes = 1 << 30
)

// githubReleaseURL points at release downloads; a var so tests can redirect
// it to a stub server.
var githubReleaseURL = "https://github.com/"

type Options struct {
	CurrentVersion  string
	TargetVersion   string // exact tag from --version; empty means latest
	AllowPrerelease bool   // --pre; prereleases are otherwise invisible
	FromSource      bool
	DryRun          bool
	Verbose         bool
}

type Target struct {
	Tag    string
	Commit string
}

type Updater interface {
	Name() string
	Update(context.Context, Target) error
	DryRun(Target)
}

type Service struct {
	SetVerbose       func(bool)
	Executable       func() (string, error)
	DetermineUpdater func(string) (Updater, error)
	ResolveTarget    func(context.Context, Options) (string, error)
	ResolveCommit    func(context.Context, string) (string, error)
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

	// D4: fail closed — no update proceeds without an exact resolved tag.
	targetTag, err := s.resolveTarget(ctx, opts)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("could not resolve a release to install: %w", err)
	}
	exact := opts.TargetVersion != ""
	install, reason, err := decideUpdate(opts.CurrentVersion, targetTag, exact, opts.AllowPrerelease)
	if err != nil {
		return err
	}
	if !install {
		logger.Success("%s (%s)", reason, opts.CurrentVersion)
		return nil
	}
	commit, err := s.resolveCommit(ctx, targetTag)
	if err != nil {
		return fmt.Errorf("could not resolve commit for %s: %w", targetTag, err)
	}
	target := Target{Tag: targetTag, Commit: commit}
	if opts.DryRun {
		updater.DryRun(target)
		return nil
	}
	if opts.CurrentVersion == "dev" {
		logger.Info("Development build detected, updating to: %s", targetTag)
	} else {
		logger.Info("Current version: %s", opts.CurrentVersion)
		logger.Info("Updating to: %s (%s)", targetTag, reason)
	}

	if err := updater.Update(ctx, target); err != nil {
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

func (s Service) resolveTarget(ctx context.Context, opts Options) (string, error) {
	if s.ResolveTarget != nil {
		return s.ResolveTarget(ctx, opts)
	}
	return ResolveTarget(ctx, opts)
}

func (s Service) resolveCommit(ctx context.Context, tag string) (string, error) {
	if s.ResolveCommit != nil {
		return s.ResolveCommit(ctx, tag)
	}
	return resolveTagCommit(ctx, tag)
}

func DetermineUpdater(exePath string, opts Options) (Updater, error) {
	if installedViaGo(exePath) {
		return &GoUpdater{exePath: exePath, Verbose: opts.Verbose}, nil
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
	exePath string
	Verbose bool
}

func (g *GoUpdater) Name() string { return "go install" }

func (g *GoUpdater) DryRun(target Target) {
	logger.Info("Would run: go install %s@%s and verify commit %s", modulePath, target.Tag, target.Commit)
}

func (g *GoUpdater) Update(ctx context.Context, target Target) error {
	version := target.Tag
	if version == "" {
		return errors.New("no exact release tag resolved; refusing mutable module ref")
	}
	if _, err := ParseVersion(version); err != nil {
		return fmt.Errorf("resolved version is not a release tag: %w", err)
	}
	if !validCommit(target.Commit) {
		return errors.New("resolved target has no valid commit")
	}
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go binary not found in PATH")
	}
	if err := rejectSymlinkedInstall(g.exePath); err != nil {
		return err
	}

	buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	tempDir, err := os.MkdirTemp(filepath.Dir(g.exePath), ".hetki-source-*")
	if err != nil {
		return fmt.Errorf("creating source build directory: %w", err)
	}
	defer removeSourceDir(tempDir)
	sourceCtx, cancelSource := context.WithCancelCause(buildCtx)
	monitorDone := watchDirectorySize(sourceCtx, tempDir, maxSourceWorkspaceBytes, cancelSource)
	defer func() {
		cancelSource(nil)
		<-monitorDone
	}()

	module := modulePath + "@" + version
	modCache := filepath.Join(tempDir, "modcache")
	if err := os.MkdirAll(filepath.Join(tempDir, "tmp"), 0700); err != nil {
		return fmt.Errorf("creating source temporary directory: %w", err)
	}
	if err := verifySourceOrigin(sourceCtx, module, target, modCache); err != nil {
		return err
	}
	ldflags := fmt.Sprintf("-X %s/cmd.Version=%s -X %s/cmd.GitCommit=%s", modulePath, version, modulePath, target.Commit)
	args := []string{"install", "-ldflags", ldflags}
	if g.Verbose {
		args = append(args, "-v")
	}
	args = append(args, module)
	logger.Info("Installing from source: %s", module)
	logger.Debug("Running command: go %s", strings.Join(args, " "))

	cmdArgs := append([]string{"-c", `ulimit -f 131072; exec "$@"`, "_", "go"}, args...)
	cmd := exec.CommandContext(sourceCtx, "sh", cmdArgs...)
	cmd.Env = sourceInstallEnv(tempDir, modCache)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	stdout, stderr := &boundedBuffer{max: maxProcessOutputBytes}, &boundedBuffer{max: maxProcessOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	defer killCommandGroup(cmd)
	if err := cmd.Run(); err != nil {
		if cause := context.Cause(sourceCtx); cause != nil {
			return errors.Join(cause, err)
		}
		return err
	}
	if cause := context.Cause(sourceCtx); cause != nil {
		return cause
	}
	if exceeded, err := directorySizeExceeds(tempDir, maxSourceWorkspaceBytes); err != nil {
		return fmt.Errorf("checking source workspace size: %w", err)
	} else if exceeded {
		return fmt.Errorf("source workspace exceeds %d bytes", maxSourceWorkspaceBytes)
	}
	return replaceExecutable(ctx, filepath.Join(tempDir, "hetki"), g.exePath, target)
}

func watchDirectorySize(ctx context.Context, path string, maxBytes int64, cancel context.CancelCauseFunc) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				exceeded, err := directorySizeExceeds(path, maxBytes)
				if err != nil {
					cancel(fmt.Errorf("checking source workspace size: %w", err))
					return
				}
				if exceeded {
					cancel(fmt.Errorf("source workspace exceeds %d bytes", maxBytes))
					return
				}
			}
		}
	}()
	return done
}

func directorySizeExceeds(root string, maxBytes int64) (bool, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			if total > maxBytes {
				return fs.SkipAll
			}
		}
		return nil
	})
	return total > maxBytes, err
}

func killCommandGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

func removeSourceDir(path string) {
	_ = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = os.Chmod(path, info.Mode().Perm()|0700)
		}
		return nil
	})
	if err := os.RemoveAll(path); err != nil {
		logger.Warning("Could not remove source build directory %s: %v", path, err)
	}
}

func verifySourceOrigin(ctx context.Context, module string, target Target, modCache string) error {
	out, err := commandOutputEnv(ctx, 60*time.Second, 1<<20, sourceInstallEnv("", modCache), "go", "mod", "download", "-json", module)
	if err != nil {
		return fmt.Errorf("verifying source origin: %w", err)
	}
	var download struct {
		Path, Version, Sum string
		Origin             struct{ URL, Hash, Ref string }
	}
	if err := json.Unmarshal(out, &download); err != nil {
		return fmt.Errorf("decoding source origin: %w", err)
	}
	if download.Path != modulePath || download.Version != target.Tag || download.Sum == "" ||
		download.Origin.URL != "https://github.com/MSmaili/hetki" || download.Origin.Hash != target.Commit ||
		download.Origin.Ref != "refs/tags/"+target.Tag {
		return fmt.Errorf("source origin does not match %s at %s", target.Tag, target.Commit)
	}
	return nil
}

func sourceInstallEnv(gobin, modCache string) []string {
	env := os.Environ()
	for _, name := range []string{"GOBIN", "GOMODCACHE", "GOCACHE", "GOTMPDIR", "GOPROXY", "GOSUMDB", "GONOSUMDB", "GOPRIVATE", "GONOPROXY"} {
		prefix := name + "="
		for i := len(env) - 1; i >= 0; i-- {
			if strings.HasPrefix(env[i], prefix) {
				env = append(env[:i], env[i+1:]...)
			}
		}
	}
	root := filepath.Dir(modCache)
	return append(env, "GOBIN="+gobin, "GOMODCACHE="+modCache, "GOCACHE="+filepath.Join(root, "gocache"),
		"GOTMPDIR="+filepath.Join(root, "tmp"), "GOPROXY=direct", "GOSUMDB=sum.golang.org", "GONOSUMDB=", "GOPRIVATE=", "GONOPROXY=")
}

type BinaryUpdater struct {
	exePath    string
	FromSource bool
}

func (b *BinaryUpdater) Name() string { return "binary release" }

func (b *BinaryUpdater) DryRun(target Target) {
	if b.FromSource {
		logger.Info("Would run: go install %s@%s and verify commit %s", modulePath, target.Tag, target.Commit)
		return
	}
	binaryName := fmt.Sprintf("hetki-%s-%s", runtime.GOOS, runtime.GOARCH)
	logger.Info("Would download: %s%s/releases/download/%s/%s", githubReleaseURL, githubRepo, target.Tag, binaryName)
	logger.Info("Would verify its GitHub artifact attestation and replace: %s", b.exePath)
}

func (b *BinaryUpdater) Update(ctx context.Context, target Target) error {
	targetTag := target.Tag
	if b.FromSource {
		logger.Info("--source flag set, falling back to go install...")
		return (&GoUpdater{exePath: b.exePath}).Update(ctx, target)
	}

	if targetTag == "" {
		return errors.New("no exact release tag resolved")
	}
	if _, err := ParseVersion(targetTag); err != nil {
		return fmt.Errorf("resolved version is not a release tag: %w", err)
	}
	if !validCommit(target.Commit) {
		return errors.New("resolved target has no valid commit")
	}
	if err := rejectSymlinkedInstall(b.exePath); err != nil {
		return err
	}

	// supportedPlatforms mirrors the release matrix; anything else fails
	// before a download is attempted.
	if !supportedPlatform(runtime.GOOS, runtime.GOARCH) {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	binaryName := fmt.Sprintf("hetki-%s-%s", runtime.GOOS, runtime.GOARCH)
	downloadURL := fmt.Sprintf("%s%s/releases/download/%s/%s", githubReleaseURL, githubRepo, targetTag, binaryName)
	checksumsURL := fmt.Sprintf("%s%s/releases/download/%s/checksums.txt", githubReleaseURL, githubRepo, targetTag)

	logger.Info("Downloading hetki %s for %s/%s...", targetTag, runtime.GOOS, runtime.GOARCH)

	tempFile, err := os.CreateTemp(filepath.Dir(b.exePath), ".hetki-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := downloadToFile(ctx, downloadURL, tempFile, maxBinaryBytes); err != nil {
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

	digest, err := fileDigest(tempPath)
	if err != nil {
		return err
	}
	if _, err := verifyReleaseArtifact(ctx, tempPath, binaryName, digest, target); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return replaceExecutable(ctx, tempPath, b.exePath, target)
}

// rejectSymlinkedInstall refuses to update through a symlink: replacing the
// link target implicitly is ambiguous, so the user updates the target path
// directly (D4: explicit handling, fail closed).
func rejectSymlinkedInstall(exePath string) error {
	info, err := os.Lstat(exePath)
	if err != nil {
		return fmt.Errorf("cannot inspect current installation: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; update the real binary path directly", exePath)
	}
	return nil
}

// replaceExecutable swaps in the verified binary and keeps a hard-link
// backup until the replacement reports the exact tag and commit.
func replaceExecutable(ctx context.Context, newPath, exePath string, target Target) (err error) {
	candidateInfo, err := inspectInstallCandidate(newPath)
	if err != nil {
		return err
	}
	lockPath := exePath + ".hetki-update-lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fmt.Errorf("another update is active or left a lock at %s: %w", lockPath, err)
	}
	lockInfo, err := lock.Stat()
	if err != nil {
		lock.Close()
		return errors.Join(err, os.Remove(lockPath))
	}
	defer func() {
		if closeErr := lock.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		if removeErr := removeOwnedPath(lockPath, lockInfo); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("removing update lock: %w", removeErr))
		}
	}()

	oldInfo, err := os.Lstat(exePath)
	if err != nil {
		return fmt.Errorf("inspecting current installation: %w", err)
	}
	if !oldInfo.Mode().IsRegular() {
		return errors.New("current installation is not a regular file")
	}
	mode := oldInfo.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	stat, ok := oldInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot inspect current installation ownership")
	}
	if err := os.Chown(newPath, int(stat.Uid), int(stat.Gid)); err != nil {
		return fmt.Errorf("failed to preserve ownership: %w", err)
	}
	if err := os.Chmod(newPath, mode); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	afterChmodInfo, err := os.Lstat(newPath)
	if err != nil {
		return fmt.Errorf("inspecting install candidate after chmod: %w", err)
	}
	if !afterChmodInfo.Mode().IsRegular() || !os.SameFile(candidateInfo, afterChmodInfo) {
		return errors.New("install candidate identity changed during chmod")
	}
	candidateInfo = afterChmodInfo

	backupPath := exePath + ".hetki-backup"
	if err := os.Link(exePath, backupPath); err != nil {
		return fmt.Errorf("failed to create exclusive backup (remove %s only after recovery): %w", backupPath, err)
	}
	backupInfo, err := os.Lstat(backupPath)
	if err != nil {
		return errors.Join(fmt.Errorf("inspecting recovery backup: %w", err), os.Remove(backupPath))
	}
	if !os.SameFile(oldInfo, backupInfo) {
		_ = removeOwnedPath(backupPath, backupInfo)
		return errors.New("installation changed while creating recovery backup")
	}
	replaced := false
	defer func() {
		if err == nil {
			return
		}
		if !replaced {
			if removeErr := removeOwnedPath(backupPath, backupInfo); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("removing unused recovery backup: %w", removeErr))
			}
			return
		}
		if restoreErr := restoreOwnedBackup(backupPath, backupInfo, exePath, candidateInfo); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("ROLLBACK FAILED; previous binary remains at %s: %w", backupPath, restoreErr))
		}
	}()

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requirePathIdentity(exePath, oldInfo); err != nil {
		return fmt.Errorf("installation changed before replacement: %w", err)
	}
	if err := requirePathIdentity(newPath, candidateInfo); err != nil {
		return fmt.Errorf("candidate changed before replacement: %w", err)
	}
	if err := requirePathIdentity(backupPath, backupInfo); err != nil {
		return fmt.Errorf("recovery backup changed before replacement: %w", err)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	replaced = true

	identity, err := installedIdentity(ctx, exePath)
	if err != nil {
		return fmt.Errorf("verifying installed binary: %w", err)
	}
	if err := requirePathIdentity(exePath, candidateInfo); err != nil {
		return fmt.Errorf("installed binary changed during verification: %w", err)
	}
	if identity != target {
		return fmt.Errorf("installed binary reports %s at %s, expected %s at %s; rolling back",
			identity.Tag, identity.Commit, target.Tag, target.Commit)
	}
	if err = removeOwnedPath(backupPath, backupInfo); err != nil {
		return fmt.Errorf("removing recovery backup after verification: %w", err)
	}
	logger.Info("Installed binary verified at %s (%s)", identity.Tag, identity.Commit)
	return nil
}

func inspectInstallCandidate(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect install candidate: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("install candidate must be a regular, non-symlink file")
	}
	if info.Size() <= 0 || info.Size() > maxBinaryBytes {
		return nil, fmt.Errorf("install candidate size %d is outside the 1-%d byte bound", info.Size(), maxBinaryBytes)
	}
	return info, nil
}

func requirePathIdentity(path string, want os.FileInfo) error {
	got, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(got, want) {
		return errors.New("file identity does not match")
	}
	return nil
}

func removeOwnedPath(path string, want os.FileInfo) error {
	if err := requirePathIdentity(path, want); err != nil {
		return err
	}
	return os.Remove(path)
}

func restoreOwnedBackup(backupPath string, backupInfo os.FileInfo, exePath string, candidateInfo os.FileInfo) error {
	if err := requirePathIdentity(backupPath, backupInfo); err != nil {
		return fmt.Errorf("recovery backup identity changed: %w", err)
	}
	if err := requirePathIdentity(exePath, candidateInfo); err != nil {
		return fmt.Errorf("destination identity changed: %w", err)
	}
	return os.Rename(backupPath, exePath)
}

type boundedBuffer struct {
	bytes.Buffer
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, fmt.Errorf("command output exceeds %d bytes", b.max)
	}
	return b.Buffer.Write(p)
}

func commandOutput(ctx context.Context, timeout time.Duration, maxBytes int, name string, args ...string) ([]byte, error) {
	return commandOutputEnv(ctx, timeout, maxBytes, nil, name, args...)
}

func commandOutputEnv(ctx context.Context, timeout time.Duration, maxBytes int, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	defer killCommandGroup(cmd)
	if env != nil {
		cmd.Env = env
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	output := &boundedBuffer{max: maxBytes}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return output.Bytes(), nil
}

func installedIdentity(ctx context.Context, exePath string) (Target, error) {
	out, err := commandOutput(ctx, 30*time.Second, 4<<10, exePath, "--version")
	if err != nil {
		return Target{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return Target{}, fmt.Errorf("unexpected --version output %q", out)
	}
	version, versionOK := strings.CutPrefix(strings.TrimSpace(lines[0]), "hetki version ")
	commit, commitOK := strings.CutPrefix(strings.TrimSpace(lines[1]), "commit: ")
	if !versionOK || !commitOK {
		return Target{}, fmt.Errorf("unexpected --version output %q", out)
	}
	return Target{Tag: version, Commit: commit}, nil
}

func installedVersion(exePath string) (string, error) {
	identity, err := installedIdentity(context.Background(), exePath)
	return identity.Tag, err
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadToFile(ctx context.Context, url string, f *os.File, maxBytes int64) (err error) {
	defer func() { err = errors.Join(err, f.Close()) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := githubHTTPClient(60 * time.Second).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	written, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if written > maxBytes {
		return fmt.Errorf("download exceeds %d bytes from %s", maxBytes, url)
	}
	return nil
}

func verifyBinaryChecksum(ctx context.Context, checksumsURL, filePath, binaryName string) error {
	logger.Info("Verifying checksum...")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumsURL, nil)
	if err != nil {
		return err
	}
	resp, err := githubHTTPClient(15 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read checksums: %w", err)
	}
	if len(data) > maxChecksumsBytes {
		return fmt.Errorf("checksums.txt exceeds %d bytes", maxChecksumsBytes)
	}
	var expectedHash string
	scanner := bufio.NewScanner(bytes.NewReader(data))
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

// supportedPlatform mirrors the release artifact matrix.
func supportedPlatform(goos, goarch string) bool {
	switch {
	case goos == "darwin" && (goarch == "amd64" || goarch == "arm64"):
		return true
	case goos == "linux" && (goarch == "amd64" || goarch == "arm64"):
		return true
	default:
		return false
	}
}
