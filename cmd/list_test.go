package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MSmaili/hetki/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerSanitizesExternalArgumentsBeforeStyling(t *testing.T) {
	output := captureLoggerOutput(t, func() {
		logger.Plain("error: %v", assert.AnError)
		logger.Plain("name: %s", "dev\x1b[31m-red\x1b[0m\nnext")
	})
	assert.Equal(t, "error: assert.AnError general error for testing\nname: dev-red\\nnext\n", output)
}

func TestHumanListOutputIsTerminalSafeButJSONIsStructural(t *testing.T) {
	resetCommandGlobals()
	name := "dev\x1b[31m-red\x1b[0m\nnext"

	human := captureStdout(t, func() { require.NoError(t, outputNames([]string{name})) })
	assert.Equal(t, "dev-red\\nnext\n", human)

	listFormat = "json"
	structured := captureStdout(t, func() { require.NoError(t, outputNames([]string{name})) })
	assert.Contains(t, structured, `dev\u001b[31m-red\u001b[0m\nnext`)
}

func TestRunListWorkspaceFormats(t *testing.T) {
	workspaceYAML := `sessions:
  - name: dev
    windows:
      - name: editor
        path: /tmp/editor
        panes:
          - path: /tmp/editor
          - path: /tmp/api
`

	t.Run("flat names are sorted", func(t *testing.T) {
		resetCommandGlobals()
		home := t.TempDir()
		workspacesDir := filepath.Join(home, ".config", "hetki", "workspaces")
		require.NoError(t, os.MkdirAll(workspacesDir, 0755))
		writeWorkspaceFile(t, workspacesDir, "zeta.yaml", workspaceYAML)
		writeWorkspaceFile(t, workspacesDir, "alpha.yaml", workspaceYAML)
		t.Setenv("HOME", home)

		output := captureStdout(t, func() {
			require.NoError(t, runList(nil, []string{"workspaces"}))
		})

		assert.Equal(t, "alpha\nzeta\n", output)
	})

	t.Run("tree format includes stable session and pane output", func(t *testing.T) {
		resetCommandGlobals()
		home := t.TempDir()
		workspacesDir := filepath.Join(home, ".config", "hetki", "workspaces")
		require.NoError(t, os.MkdirAll(workspacesDir, 0755))
		writeWorkspaceFile(t, workspacesDir, "alpha.yaml", workspaceYAML)
		t.Setenv("HOME", home)

		listSessions = true
		listWindows = true
		listPanes = true
		listFormat = "tree"

		output := captureStdout(t, func() {
			require.NoError(t, runList(nil, []string{"workspaces"}))
		})

		assert.Equal(t, "alpha:dev\n└── editor\n    ├── 0\n    └── 1\n", output)
	})

	t.Run("malformed files report errors without hiding valid results", func(t *testing.T) {
		resetCommandGlobals()
		home := t.TempDir()
		workspacesDir := filepath.Join(home, ".config", "hetki", "workspaces")
		require.NoError(t, os.MkdirAll(workspacesDir, 0755))
		writeWorkspaceFile(t, workspacesDir, "alpha.yaml", workspaceYAML)
		brokenPath := writeWorkspaceFile(t, workspacesDir, "broken.yaml", "sessions:\n  - unknown: value\n")
		t.Setenv("HOME", home)

		listSessions = true
		var runErr error
		output := captureStdout(t, func() { runErr = runList(nil, []string{"workspaces"}) })

		assert.Equal(t, "alpha:dev\n", output)
		require.ErrorContains(t, runErr, `workspace "broken" (`+brokenPath+`): parse yaml config:`)
		require.ErrorContains(t, runErr, `field unknown not found`)
	})

	t.Run("json format matches listed workspaces", func(t *testing.T) {
		resetCommandGlobals()
		home := t.TempDir()
		workspacesDir := filepath.Join(home, ".config", "hetki", "workspaces")
		require.NoError(t, os.MkdirAll(workspacesDir, 0755))
		writeWorkspaceFile(t, workspacesDir, "alpha.yaml", workspaceYAML)
		t.Setenv("HOME", home)

		listSessions = true
		listWindows = true
		listFormat = "json"

		output := captureStdout(t, func() {
			require.NoError(t, runList(nil, []string{"workspaces"}))
		})

		assert.Equal(t, "[\n  {\n    \"name\": \"alpha:dev\",\n    \"windows\": [\n      {\n        \"name\": \"editor\"\n      }\n    ]\n  }\n]\n", output)
	})
}
