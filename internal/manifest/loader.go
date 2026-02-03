package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "muxie"), nil
}

type Loader interface {
	Load() (*Workspace, error)
}

type FileLoader struct {
	Path string
}

func NewFileLoader(path string) *FileLoader {
	return &FileLoader{Path: path}
}

func (l *FileLoader) Load() (*Workspace, error) {
	extendedPath := expandPath(l.Path)
	data, err := os.ReadFile(extendedPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw Workspace
	ext := filepath.Ext(extendedPath)

	switch ext {
	case ".yaml", ".yml":
		if err = yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse yaml config: %w", err)
		}
	case ".json":
		if err = json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse json config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s (use .yaml, .yml, or .json)", ext)
	}

	if err = validate(&raw); err != nil {
		return nil, err
	}

	return normalize(&raw)
}

func validate(cfg *Workspace) error {
	if len(cfg.Sessions) == 0 {
		return fmt.Errorf("sessions block missing or empty")
	}

	for _, sess := range cfg.Sessions {
		if sess.Name == "" {
			return fmt.Errorf("session name cannot be empty")
		}
		if len(sess.Windows) == 0 {
			return fmt.Errorf("session '%s' has no windows", sess.Name)
		}
		for _, w := range sess.Windows {
			if w.Path == "" && sess.Root == "" {
				return fmt.Errorf("window in session '%s' missing path (no session root defined)", sess.Name)
			}
		}
	}

	return nil
}

func normalize(cfg *Workspace) (*Workspace, error) {
	out := &Workspace{Sessions: make([]Session, len(cfg.Sessions))}

	for i, sess := range cfg.Sessions {
		sess.Root = expandPath(sess.Root)
		normalized := make([]Window, len(sess.Windows))

		for j, w := range sess.Windows {
			if w.Path == "" {
				w.Path = sess.Root
			}
			w.Path = expandPath(w.Path)
			if w.Name == "" {
				w.Name = inferNameFromPath(w.Path)
			}
			normalized[j] = w
		}
		sess.Windows = normalized
		out.Sessions[i] = sess
	}

	return out, nil
}

func inferNameFromPath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

func expandPath(p string) string {
	if p == "" {
		return ""
	}
	p = os.ExpandEnv(p)

	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func ScanWorkspaces(dir string) (map[string]string, error) {
	expandedDir := expandPath(dir)
	entries, err := os.ReadDir(expandedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	paths := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			paths[strings.TrimSuffix(name, ext)] = filepath.Join(expandedDir, name)
		}
	}
	return paths, nil
}

func loadFromMemory(data []byte) (*Workspace, error) {
	var raw Workspace

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := validate(&raw); err != nil {
		return nil, err
	}

	return normalize(&raw)
}
