//go:build integration

package start

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	backendtmux "github.com/MSmaili/hetki/internal/backend/tmux"
	"github.com/MSmaili/hetki/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForceKeepsUnrelatedSessionOnIsolatedTmux(t *testing.T) {
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is required for integration tests")
	}

	socketDir, err := os.MkdirTemp("/tmp", "hetki-start-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "tmux.sock")
	navigationLog := filepath.Join(socketDir, "navigation.log")
	wrapperDir := t.TempDir()
	wrapperPath := filepath.Join(wrapperDir, "tmux")
	wrapper := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"attach-session\" ]; then\n  printf '%%s\\n' \"$*\" > %q\n  exit 0\nfi\nexec %q -S %q \"$@\"\n", navigationLog, realTmux, socket)
	require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0755))
	t.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "")
	t.Cleanup(func() { _ = exec.Command(realTmux, "-S", socket, "kill-server").Run() })

	b, err := backendtmux.NewBackend()
	require.NoError(t, err)
	keepPath, removePath := t.TempDir(), t.TempDir()
	require.NoError(t, b.Apply([]backend.Action{
		backend.CreateSessionAction{Name: "dev", WindowName: "editor", Path: t.TempDir()},
		backend.CreateWindowAction{Session: "dev", Name: "scratch", Path: keepPath},
		backend.CreateWindowAction{Session: "dev", Name: "scratch", Path: removePath},
		backend.CreateSessionAction{Name: "personal", WindowName: "shell", Path: t.TempDir()},
	}))

	live, err := b.QueryState()
	require.NoError(t, err)
	dev := findBackendSession(t, live.Sessions, "dev")
	editor := findBackendWindow(t, dev.Windows, "editor")
	scratch := findBackendWindow(t, dev.Windows, "scratch")
	workspace := &manifest.Workspace{Sessions: []manifest.Session{{
		Name: "dev",
		Windows: []manifest.Window{
			{Name: editor.Name, Path: editor.Path, Layout: editor.Layout, Panes: manifestPanes(editor.Panes)},
			{Name: scratch.Name, Path: scratch.Path, Layout: scratch.Layout, Panes: manifestPanes(scratch.Panes)},
		},
	}}}
	service := NewService(func(...string) (backend.Backend, error) { return b, nil })
	service.LoadWorkspace = func(string) (*manifest.Workspace, string, error) { return workspace, "", nil }

	require.NoError(t, service.Run(Options{Force: true}))
	after, err := b.QueryState()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dev", "personal"}, backendSessionNames(after.Sessions))
	devAfter := findBackendSession(t, after.Sessions, "dev")
	assert.Equal(t, []string{"editor", "scratch"}, backendWindowNames(devAfter.Windows))
	assert.Equal(t, scratch.ID, findBackendWindow(t, devAfter.Windows, "scratch").ID, "force must preserve the matched duplicate by stable ID")
	_, err = os.Stat(navigationLog)
	assert.NoError(t, err)
}

func findBackendSession(t *testing.T, sessions []backend.Session, name string) backend.Session {
	t.Helper()
	for _, session := range sessions {
		if session.Name == name {
			return session
		}
	}
	t.Fatalf("session %q not found", name)
	return backend.Session{}
}

func findBackendWindow(t *testing.T, windows []backend.Window, name string) backend.Window {
	t.Helper()
	for _, window := range windows {
		if window.Name == name {
			return window
		}
	}
	t.Fatalf("window %q not found", name)
	return backend.Window{}
}

func manifestPanes(panes []backend.Pane) []manifest.Pane {
	result := make([]manifest.Pane, len(panes))
	for i, pane := range panes {
		result[i] = manifest.Pane{Path: pane.Path, Command: pane.Command, Zoom: pane.Zoom}
	}
	return result
}

func backendSessionNames(sessions []backend.Session) []string {
	result := make([]string, len(sessions))
	for i, session := range sessions {
		result[i] = session.Name
	}
	return result
}

func backendWindowNames(windows []backend.Window) []string {
	result := make([]string, len(windows))
	for i, window := range windows {
		result[i] = window.Name
	}
	return result
}
