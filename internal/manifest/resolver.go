package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func (r *Resolver) isPath(s string) bool {
	return strings.ContainsAny(s, "/\\") || filepath.IsAbs(s)
}

func (r *Resolver) resolveAsPath(path string) (string, error) {
	expanded := expandPath(path)
	if _, err := os.Stat(expanded); err != nil {
		return "", fmt.Errorf("workspace file not found: %s", expanded)
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

	return "", fmt.Errorf("named workspace not found: %s", name)
}

func (r *Resolver) findLocalWorkspace() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for _, ext := range []string{".yaml", ".yml", ".json"} {
		path := filepath.Join(cwd, ".tms"+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no local workspace found (.tms.{yaml,yml,json})")
}
