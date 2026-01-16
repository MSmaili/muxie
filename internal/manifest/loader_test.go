package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `sessions:
  myapp:
    - name: editor
      path: /home/user/code
      command: vim
    - name: server
      path: /home/user/code
      command: npm run dev
`

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	loader := NewFileLoader(configPath)
	workspace, err := loader.Load()
	require.NoError(t, err)

	assert.NotNil(t, workspace)
	assert.Len(t, workspace.Sessions, 1)
	assert.Len(t, workspace.Sessions["myapp"], 2)
	assert.Equal(t, "editor", workspace.Sessions["myapp"][0].Name)
	assert.Equal(t, "/home/user/code", workspace.Sessions["myapp"][0].Path)
}

func TestLoadJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.json")

	jsonContent := `{
  "sessions": {
    "myapp": [
      {
        "name": "editor",
        "path": "/home/user/code",
        "command": "vim"
      }
    ]
  }
}`

	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	loader := NewFileLoader(configPath)
	workspace, err := loader.Load()
	require.NoError(t, err)

	assert.NotNil(t, workspace)
	assert.Len(t, workspace.Sessions, 1)
	assert.Len(t, workspace.Sessions["myapp"], 1)
	assert.Equal(t, "editor", workspace.Sessions["myapp"][0].Name)
}

func TestLoadUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(configPath, []byte("invalid"), 0644)
	require.NoError(t, err)

	loader := NewFileLoader(configPath)
	_, err = loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config format")
}

func TestLoadFileNotFound(t *testing.T) {
	loader := NewFileLoader("/nonexistent/config.yaml")
	_, err := loader.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestLoadYAMLWithPanes(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `sessions:
  myapp:
    - name: editor
      path: /home/user/code
      panes:
        - command: vim
          split: vertical
          size: 50
        - command: npm run dev
          split: horizontal
          size: 30
`

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	loader := NewFileLoader(configPath)
	workspace, err := loader.Load()
	require.NoError(t, err)

	assert.NotNil(t, workspace)
	assert.Len(t, workspace.Sessions["myapp"], 1)
	assert.Len(t, workspace.Sessions["myapp"][0].Panes, 2)
	assert.Equal(t, "vim", workspace.Sessions["myapp"][0].Panes[0].Command)
	assert.Equal(t, "vertical", workspace.Sessions["myapp"][0].Panes[0].Split)
}
