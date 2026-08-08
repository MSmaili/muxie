//go:build integration

package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIsolatedTmuxBackend runs the client against a throwaway socket; the
// user's default server is never touched.
func newIsolatedTmuxBackend(t *testing.T) *TmuxBackend {
	t.Helper()
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required for integration tests")
	}
	socketDir, err := os.MkdirTemp("/tmp", "hetki-tmux-parse-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "tmux.sock")
	t.Cleanup(func() { _ = exec.Command(realTmux, "-S", socket, "kill-server").Run() })

	wrapperDir := t.TempDir()
	wrapper := fmt.Sprintf("#!/bin/sh\nexec %q -S %q \"$@\"\n", realTmux, socket)
	require.NoError(t, os.WriteFile(filepath.Join(wrapperDir, "tmux"), []byte(wrapper), 0755))
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "")

	b, err := NewBackend()
	require.NoError(t, err)
	return b
}

func TestQueryStateRoundTripsDelimiterAndControlValues(t *testing.T) {
	b := newIsolatedTmuxBackend(t)

	sessionName := "dev|ops"
	// '|' previously corrupted the row; backslash names get extra tmux
	// escaping so they stay in unit tests and the option value below.
	windowName := `edit|"main" 'aux' 日本語`
	require.NoError(t, b.Apply([]backend.Action{backend.CreateSessionAction{Name: sessionName, WindowName: windowName}}))
	require.NoError(t, b.client.Execute(SetSessionOption{Session: sessionName, Key: backend.WorkspacePathOption, Value: ` /tmp/ws|path\\1.yaml `}))

	live, err := b.QueryState()
	require.NoError(t, err)

	if assert.Len(t, live.Sessions, 1) && assert.Len(t, live.Sessions[0].Windows, 1) {
		assert.Equal(t, sessionName, live.Sessions[0].Name)
		assert.Equal(t, windowName, live.Sessions[0].Windows[0].Name)
		assert.Equal(t, ` /tmp/ws|path\\1.yaml `, live.Sessions[0].WorkspacePath, "workspace path must round-trip byte-exact")
	}
}

func TestQueryStateFailsClosedOnNewlineValues(t *testing.T) {
	for _, value := range []string{"x\n", "a\nb", "\n", "a\n\nb"} {
		b := newIsolatedTmuxBackend(t)
		require.NoError(t, b.Apply([]backend.Action{backend.CreateSessionAction{Name: "dev", WindowName: "ed"}}))
		require.NoError(t, b.client.Execute(SetSessionOption{Session: "dev", Key: backend.WorkspacePathOption, Value: value}))

		_, err := b.QueryState()
		assert.Error(t, err, "workspace option %q must fail closed", value)
	}
}
