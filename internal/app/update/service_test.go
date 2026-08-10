package update

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
	lastVersion string
}

func (s *stubUpdater) Name() string { return "stub" }
func (s *stubUpdater) DryRun()      { s.dryRunCalls++ }
func (s *stubUpdater) Update(_ context.Context, version string) error {
	s.updateCalls++
	s.lastVersion = version
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
	go func() { done <- downloadToFile(ctx, server.URL, file) }()

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
	done := make(chan error, 1)
	go func() { done <- (&GoUpdater{}).Update(ctx, "") }()
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
	var exitErr *exec.ExitError
	require.ErrorAs(t, updateErr, &exitErr)
	require.Eventually(t, func() bool {
		return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
	}, 5*time.Second, 10*time.Millisecond, "child process %d outlived updater", pid)
}

func TestServiceRunDryRunUsesUpdaterDryRun(t *testing.T) {
	updater := &stubUpdater{}
	service := Service{
		SetVerbose:       func(bool) {},
		Executable:       func() (string, error) { return "/tmp/hetki", nil },
		DetermineUpdater: func(string) (Updater, error) { return updater, nil },
		GetLatestVersion: func(context.Context) (string, error) {
			t.Fatal("GetLatestVersion should not be called in dry-run mode")
			return "", nil
		},
	}

	err := service.Run(context.Background(), Options{DryRun: true})
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
		GetLatestVersion: func(context.Context) (string, error) { return "v1.2.3", nil },
	}

	err := service.Run(context.Background(), Options{CurrentVersion: "v1.2.3"})
	require.NoError(t, err)
	assert.Zero(t, updater.dryRunCalls)
	assert.Zero(t, updater.updateCalls)
	assert.Empty(t, updater.lastVersion)
}
