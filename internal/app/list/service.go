package list

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/MSmaili/hetki/internal/backend"
	"github.com/MSmaili/hetki/internal/manifest"
	"golang.org/x/sync/errgroup"
)

const (
	ModeWorkspaces     = "workspaces"
	ModeSessions       = "sessions"
	workspaceLoadLimit = 8
)

type Options struct {
	Mode            string
	IncludeSessions bool
	IncludeWindows  bool
	IncludePanes    bool
	CurrentOnly     bool
	Marker          string
}

type Result struct {
	NamesOnly bool
	Names     []string
	Items     []Item
}

type Item struct {
	Name    string
	Windows []Window
}

type Window struct {
	Name       string
	Panes      []int
	ActivePane int
}

type Service struct {
	DetectBackend  func(...string) (backend.Backend, error)
	GetConfigDir   func() (string, error)
	ScanWorkspaces func(string) (map[string]string, error)
	LoadWorkspace  func(string) (*manifest.Workspace, error)
}

func NewService(detectBackend func(...string) (backend.Backend, error)) Service {
	return Service{DetectBackend: detectBackend}
}

func (s Service) Run(opts Options) (Result, error) {
	mode := opts.Mode
	if mode == "" {
		mode = ModeWorkspaces
	}

	if mode == ModeSessions {
		items, err := s.listActiveSessions(opts)
		return Result{Items: items}, err
	}

	return s.listWorkspaceFiles(opts)
}

func (s Service) listWorkspaceFiles(opts Options) (Result, error) {
	configDir, err := s.getConfigDir()
	if err != nil {
		return Result{}, fmt.Errorf("failed to get config directory: %w", err)
	}

	paths, err := s.scanWorkspaces(filepath.Join(configDir, "workspaces"))
	if err != nil {
		return Result{}, fmt.Errorf("failed to scan workspaces: %w", err)
	}

	names := sortedKeys(paths)
	if !opts.IncludeSessions {
		return Result{NamesOnly: true, Names: names}, nil
	}

	var (
		g          errgroup.Group
		mu         sync.Mutex
		results    = make(map[string]*manifest.Workspace)
		loadErrors = make(map[string]error)
	)
	g.SetLimit(workspaceLoadLimit)

	for _, wname := range names {
		name, path := wname, paths[wname]
		g.Go(func() error {
			ws, err := s.loadWorkspace(path)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				loadErrors[name] = fmt.Errorf("workspace %q (%s): %w", name, path, err)
				return nil
			}
			results[name] = ws
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return Result{}, err
	}

	var (
		items    []Item
		loadErrs []error
	)
	for _, name := range names {
		if err := loadErrors[name]; err != nil {
			loadErrs = append(loadErrs, err)
			continue
		}
		items = append(items, workspaceToItems(name, results[name], opts)...)
	}

	return Result{Items: items}, errors.Join(loadErrs...)
}

func (s Service) listActiveSessions(opts Options) ([]Item, error) {
	b, err := s.detectBackend()
	if err != nil {
		return nil, fmt.Errorf("failed to detect backend: %w\nHint: Make sure a supported multiplexer is running", err)
	}

	result, err := b.QueryState()
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}

	sessions := result.Sessions
	if opts.CurrentOnly {
		if result.Active.Session == "" {
			return nil, fmt.Errorf("not in a session")
		}
		for _, sess := range sessions {
			if sess.Name == result.Active.Session {
				sessions = []backend.Session{sess}
				break
			}
		}
	}

	items := make([]Item, len(sessions))
	for i, sess := range sessions {
		items[i] = sessionToItem(sess, result.Active, opts)
	}
	return items, nil
}

func (s Service) detectBackend() (backend.Backend, error) {
	if s.DetectBackend != nil {
		return s.DetectBackend()
	}
	return backend.Detect()
}

func (s Service) getConfigDir() (string, error) {
	if s.GetConfigDir != nil {
		return s.GetConfigDir()
	}
	return manifest.GetConfigDir()
}

func (s Service) scanWorkspaces(dir string) (map[string]string, error) {
	if s.ScanWorkspaces != nil {
		return s.ScanWorkspaces(dir)
	}
	return manifest.ScanWorkspaces(dir)
}

func (s Service) loadWorkspace(path string) (*manifest.Workspace, error) {
	if s.LoadWorkspace != nil {
		return s.LoadWorkspace(path)
	}
	return manifest.NewFileLoader(path).Load()
}

func workspaceToItems(name string, ws *manifest.Workspace, opts Options) []Item {
	items := make([]Item, 0, len(ws.Sessions))

	for _, sess := range ws.Sessions {
		item := Item{Name: name + ":" + sess.Name}
		if opts.IncludeWindows {
			for _, win := range sess.Windows {
				lw := Window{Name: win.Name}
				if opts.IncludePanes {
					paneCount := max(1, len(win.Panes))
					for p := range paneCount {
						lw.Panes = append(lw.Panes, p)
					}
				}
				item.Windows = append(item.Windows, lw)
			}
		}
		items = append(items, item)
	}

	return items
}

func sessionToItem(sess backend.Session, active backend.ActiveContext, opts Options) Item {
	item := Item{Name: applyMarker(sess.Name, sess.Name == active.Session && !opts.IncludeWindows, opts.Marker)}

	if opts.IncludeWindows {
		for _, win := range sess.Windows {
			isActiveWindow := sess.Name == active.Session && win.Name == active.Window
			lw := Window{
				Name:       applyMarker(win.Name, isActiveWindow && !opts.IncludePanes, opts.Marker),
				ActivePane: -1,
			}

			if opts.IncludePanes {
				if isActiveWindow {
					lw.ActivePane = active.Pane
				}
				for _, p := range win.Panes {
					lw.Panes = append(lw.Panes, p.Index)
				}
			}
			item.Windows = append(item.Windows, lw)
		}
	}
	return item
}

func applyMarker(name string, isActive bool, marker string) string {
	if marker != "" && isActive {
		return marker + name
	}
	return name
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
