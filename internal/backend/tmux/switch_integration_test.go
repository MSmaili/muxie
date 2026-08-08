//go:build integration

package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newNavigableTmuxBackend runs on a throwaway socket and logs attach/switch
// navigation instead of touching a real terminal.
func newNavigableTmuxBackend(t *testing.T) (*TmuxBackend, string) {
	t.Helper()
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required for integration tests")
	}
	socketDir, err := os.MkdirTemp("/tmp", "hetki-tmux-switch-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "tmux.sock")
	t.Cleanup(func() { _ = exec.Command(realTmux, "-S", socket, "kill-server").Run() })

	navLog := filepath.Join(socketDir, "nav.log")
	wrapperDir := t.TempDir()
	wrapper := fmt.Sprintf("#!/bin/sh\ncase \"$1\" in attach-session|switch-client) printf '%%s\\n' \"$*\" >> %q; exit 0;; esac\nexec %q -S %q \"$@\"\n", navLog, realTmux, socket)
	require.NoError(t, os.WriteFile(filepath.Join(wrapperDir, "tmux"), []byte(wrapper), 0755))
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "")

	b, err := NewBackend()
	require.NoError(t, err)
	return b, navLog
}

func TestSwitchTargetsStableIDsOnColonNamedSessions(t *testing.T) {
	b, navLog := newNavigableTmuxBackend(t)

	require.NoError(t, b.client.Execute(CreateSession{Name: "a:b", WindowName: "w"}))
	state, err := RunQuery(b.client, LoadStateQuery{})
	require.NoError(t, err)
	require.Len(t, state.Sessions, 1)
	sessionID := state.Sessions[0].ID
	windowID := state.Sessions[0].Windows[0].ID
	windowIndex := state.Sessions[0].Windows[0].Index
	paneBase := state.PaneBaseIndex

	require.NoError(t, b.Switch(sessionID))
	require.NoError(t, b.Switch(fmt.Sprintf("%s:%s", sessionID, windowID)))
	require.NoError(t, b.Switch(fmt.Sprintf("%s:%s.0", sessionID, windowID)))

	assert.Error(t, b.Switch("a:b"), "ambiguous name form must fail closed")
	assert.Error(t, b.Switch(fmt.Sprintf("%s:@999", sessionID)), "unknown window ID must fail closed")

	logged, err := os.ReadFile(navLog)
	require.NoError(t, err)
	assert.Equal(t, []string{
		fmt.Sprintf("attach-session -t %s", sessionID),
		fmt.Sprintf("attach-session -t %s:%d", sessionID, windowIndex),
		fmt.Sprintf("attach-session -t %s:%d.%d", sessionID, windowIndex, paneBase),
	}, strings.Split(strings.TrimSpace(string(logged)), "\n"))
}
