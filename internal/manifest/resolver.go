package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const DefaultExt = ".yaml"

type Resolver struct {
	configDir func() (string, error)
}

func NewResolver() *Resolver {
	return &Resolver{
		configDir: GetConfigDir,
	}
}

func (r *Resolver) Resolve(nameOrPath string) (string, error) {
	if nameOrPath == "" {
		return r.findLocalWorkspace()
	}

	if r.isPath(nameOrPath) {
		return r.resolveAsPath(nameOrPath)
	}

	return r.findNamedWorkspace(nameOrPath)
}

func (r *Resolver) NamedPath(name string) (string, error) {
	filename, err := workspaceFilename(name)
	if err != nil {
		return "", err
	}

	configDir, err := r.configDir()
	if err != nil {
		return "", fmt.Errorf("getting config dir: %w", err)
	}
	return filepath.Join(configDir, "workspaces", filename), nil
}

func workspaceFilename(name string) (string, error) {
	if name == "" || name != strings.TrimSpace(name) || name == "." || name == ".." {
		return "", fmt.Errorf("invalid workspace name %q", name)
	}
	if strings.ContainsAny(name, `/\\`) || filepath.IsAbs(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("workspace name must be one filename: %q", name)
	}

	ext := filepath.Ext(name)
	if ext == "" {
		return name + DefaultExt, nil
	}
	if !hasValidExt(name) || strings.TrimSuffix(name, ext) == "" {
		return "", fmt.Errorf("invalid workspace extension %q (use .yaml or .yml)", ext)
	}
	return name, nil
}

func (r *Resolver) LocalPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return filepath.Join(cwd, ".hetki"+DefaultExt), nil
}

func hasValidExt(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

func (r *Resolver) isPath(s string) bool {
	return strings.ContainsAny(s, "/\\") || filepath.IsAbs(s)
}

func (r *Resolver) resolveAsPath(path string) (string, error) {
	expanded := expandPath(path)
	if _, err := os.Stat(expanded); err != nil {
		return "", fmt.Errorf("workspace file not found: %s\nHint: Check the path or use a workspace name instead", expanded)
	}
	return expanded, nil
}

func (r *Resolver) findNamedWorkspace(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return filepath.Abs(name)
	}

	configDir, err := r.configDir()
	if err != nil {
		return "", err
	}

	workspacesDir := filepath.Join(configDir, "workspaces")
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(workspacesDir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("named workspace not found: %s\nHint: Check the workspace name or create it with 'hetki save -n %s'", name, name)
}

func (r *Resolver) findLocalWorkspace() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(cwd, ".hetki"+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no local workspace found (.hetki.{yaml,yml})\nHint: Create one with 'hetki save .' or specify a workspace name")
}
