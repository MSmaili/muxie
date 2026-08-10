package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStartDryRunOutputsPlan(t *testing.T) {
	resetCommandGlobals()

	tmpDir := t.TempDir()
	workspace := "sessions:\n  - name: dev\n    windows:\n      - name: editor\n        path: " + tmpDir + "\n"
	writeWorkspaceFile(t, tmpDir, ".hetki.yaml", workspace)

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousWD))
	})

	stub := &stubBackend{dryRunLines: []string{"tmux new-session -d -s dev -n editor -c " + tmpDir}}
	withStubBackend(t, stub)
	dryRun = true

	output := captureLoggerOutput(t, func() {
		require.NoError(t, runStart(startCmd, nil))
	})

	assert.Contains(t, output, "Dry run - actions to execute:")
	assert.Contains(t, output, stub.dryRunLines[0])
	assert.Equal(t, 1, stub.dryRunCalls)
	assert.Zero(t, stub.applyCalls)
	assert.Zero(t, stub.attachCalls)
	if assert.Len(t, stub.lastActions, 1) {
		assert.IsType(t, backend.CreateSessionAction{}, stub.lastActions[0])
	}
}

func TestRunStartFailsWhenBackendStateQueryFails(t *testing.T) {
	resetCommandGlobals()

	tmpDir := t.TempDir()
	workspace := "sessions:\n  - name: dev\n    windows:\n      - name: editor\n        path: " + filepath.ToSlash(tmpDir) + "\n"
	writeWorkspaceFile(t, tmpDir, ".hetki.yaml", workspace)

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previousWD))
	})

	stub := &stubBackend{queryErr: errors.New("query failed")}
	withStubBackend(t, stub)

	err = runStart(startCmd, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to query backend state: query failed")
	assert.ErrorContains(t, err, "hetki list sessions")
	assert.Zero(t, stub.dryRunCalls)
	assert.Zero(t, stub.applyCalls)
}
