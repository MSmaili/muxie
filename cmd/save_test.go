package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSaveWritesCurrentSessionWorkspace(t *testing.T) {
	resetCommandGlobals()

	home := t.TempDir()
	t.Setenv("HOME", home)

	outputPath := filepath.Join(t.TempDir(), "workspace.yaml")
	stub := &stubBackend{queryResult: backend.StateResult{
		Sessions: []backend.Session{{
			Name: "dev",
			Windows: []backend.Window{{
				Name: "editor",
				Path: filepath.Join(home, "code", "hetki"),
			}},
		}},
		Active: backend.ActiveContext{Session: "dev"},
	}}
	withStubBackend(t, stub)
	savePath = outputPath

	logOutput := captureLoggerOutput(t, func() {
		require.NoError(t, runSave(saveCmd, nil))
	})

	assert.Contains(t, logOutput, "Saved to "+outputPath)
	assert.Zero(t, stub.applyCalls)

	loader := manifest.NewFileLoader(outputPath)
	workspace, err := loader.Load()
	require.NoError(t, err)
	require.Len(t, workspace.Sessions, 1)
	assert.Equal(t, "dev", workspace.Sessions[0].Name)
	require.Len(t, workspace.Sessions[0].Windows, 1)
	assert.Equal(t, filepath.Join(home, "code", "hetki"), workspace.Sessions[0].Windows[0].Path)

	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "path: ~/code/hetki")
}
