package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	configDir, err := r.configDir()
	if err != nil {
		return "", fmt.Errorf("getting config dir: %w", err)
	}

	if !hasValidExt(name) {
		name = name + DefaultExt
	}

	return filepath.Join(configDir, "workspaces", name), nil
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
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
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
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		path := filepath.Join(workspacesDir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("named workspace not found: %s\nHint: List available workspaces with 'muxie list' or create one with 'muxie save -n %s'", name, name)
}

func (r *Resolver) findLocalWorkspace() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for _, ext := range []string{".yaml", ".yml", ".json"} {
		path := filepath.Join(cwd, ".hetki"+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no local workspace found (.hetki.{yaml,yml,json})\nHint: Create one with 'hetki save .' or specify a workspace name")
}
