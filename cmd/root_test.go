package cmd

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
