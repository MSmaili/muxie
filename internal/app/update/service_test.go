package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awaitUpdateChannel[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for test channel")
		var zero T
		return zero
	}
}

type stubUpdater struct {
	dryRunCalls int
	updateCalls int
	lastTarget  Target
}

func (s *stubUpdater) Name() string    { return "stub" }
func (s *stubUpdater) DryRun(_ Target) { s.dryRunCalls++ }
func (s *stubUpdater) Update(_ context.Context, target Target) error {
	s.updateCalls++
	s.lastTarget = target
	return nil
}

type cancelingReader struct{ cancel context.CancelFunc }

func (r cancelingReader) Read(p []byte) (int, error) {
	p[0] = 'x'
	r.cancel()
	return 1, nil
}

func TestServiceRunCanceledBeforeDispatch(t *testing.T) {
	called := false
	service := Service{Executable: func() (string, error) {
		called = true
		return "", nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.Run(ctx, Options{})

	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestDownloadToFileCancelsHTTP(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	file, err := os.CreateTemp(t.TempDir(), "download")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- downloadToFile(ctx, server.URL, file, 1<<20) }()

	awaitUpdateChannel(t, started)
	cancel()

	require.ErrorIs(t, awaitUpdateChannel(t, done), context.Canceled)
	_, err = file.Write(nil)
	require.ErrorIs(t, err, os.ErrClosed)
}

func TestContextReaderStopsHashingWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	_, err := io.Copy(io.Discard, contextReader{ctx: ctx, reader: cancelingReader{cancel: cancel}})

	require.ErrorIs(t, err, context.Canceled)
}

func TestGoUpdaterCancelsProcessTree(t *testing.T) {
	bin := t.TempDir()
	goPath := filepath.Join(bin, "go")
	pidPath := filepath.Join(bin, "child.pid")
	script := "#!/bin/sh\nsleep 10 &\necho $! > \"$HETKI_CHILD_PID\"\nwait\n"
	require.NoError(t, os.WriteFile(goPath, []byte(script), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HETKI_CHILD_PID", pidPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exePath := filepath.Join(bin, "hetki")
	fakeBinary(t, exePath, "v1.0.0")
	done := make(chan error, 1)
	go func() { done <- (&GoUpdater{exePath: exePath}).Update(ctx, testTarget("v1.2.3")) }()
	var earlyErr error
	require.Eventually(t, func() bool {
		select {
		case earlyErr = <-done:
			return true
		default:
		}
		_, err := os.Stat(pidPath)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, earlyErr, "updater exited before child started")
	require.FileExists(t, pidPath)
	data, err := os.ReadFile(pidPath)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)

	cancel()

	updateErr := awaitUpdateChannel(t, done)
	require.ErrorIs(t, updateErr, context.Canceled)
	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 5*time.Second, 10*time.Millisecond, "child process %d outlived updater", pid)
}

func TestDirectorySizeExceedsBound(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "artifact"), make([]byte, 11), 0600))

	exceeded, err := directorySizeExceeds(dir, 10)
	require.NoError(t, err)
	assert.True(t, exceeded)
	exceeded, err = directorySizeExceeds(dir, 11)
	require.NoError(t, err)
	assert.False(t, exceeded)
}

func TestCommandOutputKillsInheritedPipeDescendant(t *testing.T) {
	dir := t.TempDir()
	script, pidPath := filepath.Join(dir, "leak-output"), filepath.Join(dir, "child.pid")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 10 &\necho $! > \"$1\"\necho done\n"), 0755))
	started := time.Now()

	_, err := commandOutput(context.Background(), time.Second, 1024, script, pidPath)

	require.Error(t, err)
	assert.Less(t, time.Since(started), 4*time.Second)
	pidBytes, readErr := os.ReadFile(pidPath)
	require.NoError(t, readErr)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, parseErr)
	require.Eventually(t, func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }, time.Second, 10*time.Millisecond)
}

func TestServiceRunDryRunUsesUpdaterDryRun(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		ResolveTarget:    func(context.Context, Options) (string, error) { return "v1.2.3", nil },
		ResolveCommit:    func(context.Context, string) (string, error) { return testCommit, nil },
	}

	err := service.Run(context.Background(), Options{CurrentVersion: "v1.0.0", DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, updater.dryRunCalls)
	assert.Zero(t, updater.updateCalls)
}

func TestServiceRunSkipsUpdateWhenAlreadyLatest(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		ResolveTarget:    func(context.Context, Options) (string, error) { return "v1.2.3", nil },
	}

	err := service.Run(context.Background(), Options{CurrentVersion: "v1.2.3"})
	require.NoError(t, err)
	assert.Zero(t, updater.dryRunCalls)
	assert.Zero(t, updater.updateCalls)
	assert.Empty(t, updater.lastTarget.Tag)
}

func TestServiceRunFailsClosedWhenResolutionFails(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		ResolveTarget: func(context.Context, Options) (string, error) {
			return "", errors.New("network unreachable")
		},
	}

	err := service.Run(context.Background(), Options{CurrentVersion: "v1.0.0"})
	require.ErrorContains(t, err, "could not resolve a release")
	assert.Zero(t, updater.updateCalls)
}

func TestServiceRunExactVersionReinstallsAndDowngrades(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		ResolveTarget: func(_ context.Context, opts Options) (string, error) {
			assert.Equal(t, "v1.0.0", opts.TargetVersion)
			return "v1.0.0", nil
		},
		ResolveCommit: func(context.Context, string) (string, error) { return testCommit, nil },
	}

	// Downgrade and reinstall are the same explicit path (D4).
	err := service.Run(context.Background(), Options{CurrentVersion: "v1.2.3", TargetVersion: "v1.0.0"})
	require.NoError(t, err)
	assert.Equal(t, 1, updater.updateCalls)
	assert.Equal(t, testTarget("v1.0.0"), updater.lastTarget)
}

func TestServiceRunRefusesPrereleaseWithoutOptIn(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		ResolveTarget:    func(context.Context, Options) (string, error) { return "v1.3.0-rc.1", nil },
	}

	err := service.Run(context.Background(), Options{CurrentVersion: "v1.2.3"})
	require.ErrorContains(t, err, "prerelease")
	assert.Zero(t, updater.updateCalls)
}

func TestGoUpdaterBuildsToTemporaryGOBINAndReplaces(t *testing.T) {
	bin := t.TempDir()
	goPath := filepath.Join(bin, "go")
	script := fmt.Sprintf(`#!/bin/sh
[ "$GOPROXY" = direct ] && [ "$GOSUMDB" = sum.golang.org ] && [ -n "$GOMODCACHE" ] && [ "$GOCACHE" = "${GOMODCACHE%%/modcache}/gocache" ] && [ "$GOTMPDIR" = "${GOMODCACHE%%/modcache}/tmp" ] && [ -z "$GONOSUMDB" ] && [ -z "$GOPRIVATE" ] && [ -z "$GONOPROXY" ] || exit 7
if [ "$1 $2" = "mod download" ]; then
  printf '{"Path":"github.com/MSmaili/hetki","Version":"v1.2.3","Sum":"h1:test","Origin":{"URL":"https://github.com/MSmaili/hetki","Hash":"%s","Ref":"refs/tags/v1.2.3"}}\n'
  exit 0
fi
case "$*" in *github.com/MSmaili/hetki@v1.2.3*) ;; *) exit 8 ;; esac
case "$*" in *GitCommit=%s*) ;; *) exit 9 ;; esac
mkdir -p "$GOBIN"
printf '#!/bin/sh\nprintf '\''hetki version v1.2.3\\ncommit: %s\\n'\''\n' > "$GOBIN/hetki"
chmod +x "$GOBIN/hetki"
`, testCommit, testCommit, testCommit)
	require.NoError(t, os.WriteFile(goPath, []byte(script), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	exePath := filepath.Join(bin, "hetki")
	fakeBinary(t, exePath, "v1.0.0")

	require.NoError(t, (&GoUpdater{exePath: exePath}).Update(context.Background(), testTarget("v1.2.3")))
	assert.Equal(t, "v1.2.3", readScriptVersion(t, exePath))
	assert.NoFileExists(t, exePath+".hetki-backup")
}

func TestGoUpdaterBoundsInheritedOutputDescendant(t *testing.T) {
	bin := t.TempDir()
	pidPath := filepath.Join(bin, "child.pid")
	goPath := filepath.Join(bin, "go")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1 $2" = "mod download" ]; then
  printf '{"Path":"github.com/MSmaili/hetki","Version":"v1.2.3","Sum":"h1:test","Origin":{"URL":"https://github.com/MSmaili/hetki","Hash":"%s","Ref":"refs/tags/v1.2.3"}}\n'
  exit 0
fi
mkdir -p "$GOBIN"
printf '#!/bin/sh\nprintf '\''hetki version v1.2.3\\ncommit: %s\\n'\''\n' > "$GOBIN/hetki"
chmod +x "$GOBIN/hetki"
sleep 10 & echo $! > %q
`, testCommit, testCommit, pidPath)
	require.NoError(t, os.WriteFile(goPath, []byte(script), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	exePath := filepath.Join(bin, "hetki")
	fakeBinary(t, exePath, "v1.0.0")
	started := time.Now()

	err := (&GoUpdater{exePath: exePath}).Update(context.Background(), testTarget("v1.2.3"))

	require.Error(t, err)
	assert.Less(t, time.Since(started), 4*time.Second)
	pidBytes, readErr := os.ReadFile(pidPath)
	require.NoError(t, readErr)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	require.NoError(t, parseErr)
	require.Eventually(t, func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "v1.0.0", readScriptVersion(t, exePath))
}

func TestGoUpdaterRequiresExactVersion(t *testing.T) {
	bin := t.TempDir()
	goPath := filepath.Join(bin, "go")
	require.NoError(t, os.WriteFile(goPath, []byte("#!/bin/sh\nexit 0\n"), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	require.ErrorContains(t, (&GoUpdater{}).Update(context.Background(), Target{}), "exact release tag")
	require.ErrorContains(t, (&GoUpdater{}).Update(context.Background(), Target{Tag: "main", Commit: testCommit}), "release tag")
}
