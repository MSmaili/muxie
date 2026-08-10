package cmd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseTargetPreservesIDsAndLeadingPunctuation(t *testing.T) {
	for raw, want := range map[string]string{
		"$1:@2":            "$1:@2",
		"!production":      "!production",
		"● core:editor:0": "core:editor.0",
	} {
		require.Equal(t, want, parseTarget(raw))
	}
}

func TestRunSwitchPreservesPipedCanonicalID(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	_, err = writer.WriteString("$1:@2\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	previous := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		_ = reader.Close()
	})
	stub := &stubBackend{}
	withStubBackend(t, stub)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	require.NoError(t, runSwitch(cmd, nil))
	require.Equal(t, "$1:@2", stub.lastSwitch)
}

func TestReadStdinLineCancelsBlockedPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	previous := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		_ = reader.Close()
		_ = writer.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := readStdinLine(ctx)
		done <- err
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(2 * time.Second):
		t.Fatal("blocked stdin read ignored cancellation")
	}
}

func TestRunSwitchPreservesStdinCancellation(t *testing.T) {
	cmd := &cobra.Command{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)

	err := runSwitch(cmd, nil)

	require.ErrorIs(t, err, context.Canceled)
}
