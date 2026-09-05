package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/MSmaili/hetki/internal/logger"
)

const maxSourceWorkspaceBytes = 1 << 30

type GoUpdater struct {
	exePath string
	Verbose bool
}

func (g *GoUpdater) Name() string { return "go install" }

func (g *GoUpdater) DryRun(target Target) {
	logger.Info("Would run: go install %s@%s and verify commit %s", modulePath, target.Tag, target.Commit)
}

func (g *GoUpdater) Update(ctx context.Context, target Target) error {
	if err := g.validateUpdate(target); err != nil {
		return err
	}
	version := target.Tag
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

func (g *GoUpdater) validateUpdate(target Target) error {
	if target.Tag == "" {
		return errors.New("no exact release tag resolved; refusing mutable module ref")
	}
	if _, err := ParseVersion(target.Tag); err != nil {
		return fmt.Errorf("resolved version is not a release tag: %w", err)
	}
	if !validCommit(target.Commit) {
		return errors.New("resolved target has no valid commit")
	}
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("go binary not found in PATH")
	}
	return rejectSymlinkedInstall(g.exePath)
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
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
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
