package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolverNamedPathAcceptsOneWorkspaceFilename(t *testing.T) {
	resolver := &Resolver{configDir: func() (string, error) { return "/config/hetki", nil }}

	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "dev", want: "dev.yaml"},
		{name: "dev.yaml", want: "dev.yaml"},
		{name: "dev.yml", want: "dev.yml"},
		{name: "dev.json", want: "dev.json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path, err := resolver.NamedPath(tt.name)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join("/config/hetki", "workspaces", tt.want), path)
		})
	}
}

func TestResolverNamedPathRejectsEscapeReservedAndInvalidNames(t *testing.T) {
	configCalled := false
	resolver := &Resolver{configDir: func() (string, error) {
		configCalled = true
		return t.TempDir(), nil
	}}

	for _, name := range []string{"", ".", "..", "../dev", "dir/dev", `dir\\dev`, "/tmp/dev", "dev.toml", ".yaml", " dev", "dev ", "dev\nname"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			_, err := resolver.NamedPath(name)
			assert.Error(t, err)
		})
	}
	assert.False(t, configCalled, "invalid names must fail before resolving or touching the workspace directory")
}

func TestResolverExplicitPath(t *testing.T) {
	tmpDir := t.TempDir()
	workspacePath := filepath.Join(tmpDir, "myworkspace.yaml")

	err := os.WriteFile(workspacePath, []byte("sessions: {}"), 0644)
	require.NoError(t, err)

	resolver := NewResolver()
	resolved, err := resolver.Resolve(workspacePath)
	require.NoError(t, err)
	assert.Equal(t, workspacePath, resolved)
}

func TestResolverExplicitPathNotFound(t *testing.T) {
	resolver := NewResolver()
	_, err := resolver.Resolve("/nonexistent/workspace.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace file not found")
}

func TestResolverNamedWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	err := os.MkdirAll(workspacesDir, 0755)
	require.NoError(t, err)

	workspacePath := filepath.Join(workspacesDir, "myproject.yaml")
	err = os.WriteFile(workspacePath, []byte("sessions: {}"), 0644)
	require.NoError(t, err)

	resolver := &Resolver{
		configDir: func() (string, error) {
			return tmpDir, nil
		},
	}

	resolved, err := resolver.Resolve("myproject")
	require.NoError(t, err)
	assert.Equal(t, workspacePath, resolved)
}

func TestResolverLocalWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	localPath := filepath.Join(tmpDir, ".hetki.yaml")
	err = os.WriteFile(localPath, []byte("sessions: {}"), 0644)
	require.NoError(t, err)

	resolver := NewResolver()
	resolved, err := resolver.Resolve("")
	require.NoError(t, err)

	expectedPath, _ := filepath.EvalSymlinks(localPath)
	actualPath, _ := filepath.EvalSymlinks(resolved)
	assert.Equal(t, expectedPath, actualPath)
}

func TestResolverNotFound(t *testing.T) {
	resolver := NewResolver()
	_, err := resolver.Resolve("nonexistent-workspace")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace not found")
}

func TestResolverPrefersNamedOverLocal(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesDir := filepath.Join(tmpDir, "workspaces")
	err := os.MkdirAll(workspacesDir, 0755)
	require.NoError(t, err)

	namedPath := filepath.Join(workspacesDir, "test.yaml")
	err = os.WriteFile(namedPath, []byte("sessions: {}"), 0644)
	require.NoError(t, err)

	localDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalWd)) })
	err = os.Chdir(localDir)
	require.NoError(t, err)

	localPath := filepath.Join(localDir, ".hetki.yaml")
	err = os.WriteFile(localPath, []byte("sessions: {}"), 0644)
	require.NoError(t, err)

	resolver := &Resolver{
		configDir: func() (string, error) {
			return tmpDir, nil
		},
	}

	resolved, err := resolver.Resolve("test")
	require.NoError(t, err)
	assert.Equal(t, namedPath, resolved)
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "tilde expansion",
			input:    "~/test",
			contains: "test",
		},
		{
			name:     "just tilde",
			input:    "~",
			contains: "",
		},
		{
			name:     "no expansion needed",
			input:    "/absolute/path",
			contains: "/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}

			if tt.input == "~" {
				home, _ := os.UserHomeDir()
				assert.Equal(t, home, result)
			}
		})
	}
}
