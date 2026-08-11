package cmd

import (
	"context"
	"os"
	"runtime/debug"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplyBuildInfoUsesModuleVersionAndRevision(t *testing.T) {
	oldVersion, oldCommit := Version, GitCommit
	t.Cleanup(func() { Version, GitCommit = oldVersion, oldCommit })
	Version, GitCommit = "dev", "unknown"

	applyBuildInfo(&debug.BuildInfo{
		Main:     debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
	})

	require.Equal(t, "v1.2.3", Version)
	require.Equal(t, "abc123", GitCommit)
}

func TestCommandSignalContextCancelsOnTerminationSignals(t *testing.T) {
	for _, terminationSignal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(terminationSignal.String(), func(t *testing.T) {
			ctx, stop := commandSignalContext()
			defer stop()
			process, err := os.FindProcess(os.Getpid())
			require.NoError(t, err)
			require.NoError(t, process.Signal(terminationSignal))

			select {
			case <-ctx.Done():
				require.ErrorIs(t, ctx.Err(), context.Canceled)
			case <-time.After(time.Second):
				t.Fatalf("command context did not observe %s", terminationSignal)
			}
		})
	}
}
