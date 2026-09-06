//go:build integration

package tmux

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLastSessionUsesTheInvokingClientOnAnIsolatedServer(t *testing.T) {
	b := newIsolatedTmuxBackend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	run := func(args ...string) string {
		t.Helper()
		output, err := b.client.Run(ctx, args...)
		require.NoError(t, err)
		return strings.TrimSpace(output)
	}
	for _, session := range []string{"current", "previous", "other"} {
		run("new-session", "-d", "-s", session, "sleep 60")
	}
	previousID := run("display-message", "-p", "-t", "previous", "#{session_id}")
	attach := func(session string, count int) {
		t.Helper()
		cmd := exec.CommandContext(ctx, b.client.(*client).bin, "-C", "attach-session", "-t", session)
		input, err := cmd.StdinPipe()
		require.NoError(t, err)
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		require.NoError(t, cmd.Start())
		t.Cleanup(func() {
			_ = input.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
		require.Eventually(t, func() bool {
			return len(strings.Fields(run("list-clients", "-F", "#{client_pid}"))) == count
		}, 3*time.Second, 10*time.Millisecond)
	}
	attach("current", 1)
	owner := run("list-clients", "-F", "#{client_name}")
	run("switch-client", "-c", owner, "-t", "previous")
	run("switch-client", "-c", owner, "-t", "current")
	attach("other", 2) // More recent does not mean it is our client.
	t.Setenv("TMUX", ",,0")
	t.Setenv("TMUX_PANE", "") // A popup inherits the session, not a pane ID.

	markedSession := func() string {
		t.Helper()
		state, err := b.QueryState(ctx)
		require.NoError(t, err)
		id := ""
		for _, session := range state.Sessions {
			if session.Last {
				require.Empty(t, id, "only one session may be marked")
				id = session.ID
			}
		}
		return id
	}
	require.Equal(t, previousID, markedSession())
	run("rename-session", "-t", previousID, "renamed")
	require.Equal(t, previousID, markedSession())
	run("kill-session", "-t", previousID)
	require.Empty(t, markedSession())
}
