package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "hetki"), nil
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
	file, err := os.Open(extendedPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	defer file.Close()
	// Bounded read: the size cap must apply before the file is fully read.
	data, err := io.ReadAll(io.LimitReader(file, MaxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return Parse(data, extendedPath)
}

// Parse is the single manifest pipeline: bounded read → strict decode →
// validate with field paths → normalize (first-pane compilation, root/path
// inheritance). Every consumer (start, list, save verification, TUI) goes
// through here.
func Parse(data []byte, path string) (*Workspace, error) {
	if len(data) > MaxManifestBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", MaxManifestBytes)
	}

	raw, err := decodeStrict(data, path)
	if err != nil {
		return nil, err
	}
	return prepare(raw)
}

func prepare(raw *Workspace) (*Workspace, error) {
	if errs := Validate(raw); len(errs) > 0 {
		return nil, ToError(errs)
	}
	normalized := normalize(raw)
	if errs := validateNormalized(normalized); len(errs) > 0 {
		return nil, ToError(errs)
	}
	return normalized, nil
}

// normalize applies root/path inheritance, resolves paths, infers window
// names, and compiles every window into an explicit first pane carrying the
// window command. A window's start path always equals its first pane's path;
// conflicting explicit values are rejected by validateNormalized.
func normalize(cfg *Workspace) *Workspace {
	out := &Workspace{Sessions: make([]Session, len(cfg.Sessions))}

	for i, sess := range cfg.Sessions {
		sess.Root = resolvePath(sess.Root)
		normalized := make([]Window, len(sess.Windows))

		for j, w := range sess.Windows {
			w.Path = resolvePath(w.Path)
			if w.Path == "" && len(w.Panes) > 0 {
				// An explicit first-pane path overrides the inherited session root.
				w.Path = resolvePath(w.Panes[0].Path)
			}
			if w.Path == "" {
				w.Path = sess.Root
			}
			w.Panes = compilePanes(w)
			w.Command = "" // compiled into the first pane; keep one source of truth
			if w.Name == "" {
				w.Name = inferNameFromPath(w.Path)
			}
			normalized[j] = w
		}
		sess.Windows = normalized
		out.Sessions[i] = sess
	}

	return out
}

func compilePanes(w Window) []Pane {
	if len(w.Panes) == 0 {
		return []Pane{{Path: w.Path, Command: w.Command}}
	}
	panes := make([]Pane, len(w.Panes))
	for k, p := range w.Panes {
		p.Path = resolvePath(p.Path)
		if p.Path == "" {
			p.Path = w.Path
		}
		if k == 0 && p.Command == "" {
			p.Command = w.Command
		}
		panes[k] = p
	}
	return panes
}

// validateNormalized re-checks values produced by normalization: inferred
// names and expanded/resolved paths must satisfy the same limits raw input
// does, and every window's start path must equal its first pane's path so
// creation and comparison always agree.
func validateNormalized(ws *Workspace) []ValidationError {
	var errs []ValidationError
	for i, sess := range ws.Sessions {
		sessionAt := fmt.Sprintf("sessions[%d]", i)
		if len(sess.Root) > MaxPathLength {
			errs = append(errs, ValidationError{Field: sessionAt + ".root", Message: "resolved session root path is too long"})
		}
		if err := checkPath(sess.Root); err != "" {
			errs = append(errs, ValidationError{Field: sessionAt + ".root", Message: err})
		}
		for j, w := range sess.Windows {
			at := fmt.Sprintf("%s.windows[%d]", sessionAt, j)
			if w.Name == "" {
				errs = append(errs, ValidationError{Field: at + ".name", Message: "inferred window name cannot be empty"})
			} else if len(w.Name) > MaxNameLength {
				errs = append(errs, ValidationError{Field: at + ".name", Message: nameTooLong("inferred window")})
			}
			if err := checkName(w.Name); err != "" {
				errs = append(errs, ValidationError{Field: at + ".name", Message: err + " (inferred from path)"})
			}
			if len(w.Path) > MaxPathLength {
				errs = append(errs, ValidationError{Field: at + ".path", Message: "resolved window path is too long"})
			}
			if err := checkPath(w.Path); err != "" {
				errs = append(errs, ValidationError{Field: at + ".path", Message: err})
			}
			if len(w.Panes) > 0 && w.Panes[0].Path != w.Path {
				errs = append(errs, ValidationError{Field: at + ".panes[0].path", Message: fmt.Sprintf("first pane path %q conflicts with window path %q; a window starts wherever its first pane starts", w.Panes[0].Path, w.Path)})
			}
			for k, pane := range w.Panes {
				paneAt := fmt.Sprintf("%s.panes[%d].path", at, k)
				if len(pane.Path) > MaxPathLength {
					errs = append(errs, ValidationError{Field: paneAt, Message: "resolved pane path is too long"})
				}
				if err := checkPath(pane.Path); err != "" {
					errs = append(errs, ValidationError{Field: paneAt, Message: err})
				}
			}
		}
	}
	return errs
}

func inferNameFromPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Base(filepath.Clean(p))
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

// resolvePath expands and canonicalizes to the physical path so desired
// paths compare equal to what tmux reports (macOS /var → /private/var etc.).
func resolvePath(p string) string {
	p = expandPath(p)
	if p == "" {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// ScanWorkspaces lists YAML workspaces in dir. For duplicate basename
// variants, .yaml wins over .yml.
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
		if !hasValidExt(name) {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		path := filepath.Join(expandedDir, name)
		if existing, ok := paths[base]; !ok || extPrecedence(name) < extPrecedence(existing) {
			paths[base] = path
		}
	}
	return paths, nil
}
