package tmux

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MSmaili/hetki/internal/backend"
)

type Query[T any] interface {
	Args() []string
	Parse(output string) (T, error)
}

func RunQuery[T any](c Client, q Query[T]) (T, error) {
	output, err := c.Run(q.Args()...)
	result, parseErr := q.Parse(output)
	if parseErr != nil {
		var zero T
		return zero, parseErr
	}
	return result, err
}

type Session struct {
	ID            string
	Name          string
	WorkspacePath string
	Windows       []Window
}

type LoadStateResult struct {
	Sessions        []Session
	Active          ActiveContext
	PaneBaseIndex   int
	WindowBaseIndex int
}

type ActiveContext struct {
	SessionID   string
	Session     string
	WindowID    string
	Window      string
	WindowIndex int
	PaneID      string
	Pane        int
	Path        string
}

type LoadStateQuery struct{}

func (q LoadStateQuery) Args() []string {
	return []string{
		"start-server",
		";", "show-options", "-gv", "base-index",
		";", "show-options", "-gv", "pane-base-index",
		";", "list-panes", "-a",
		"-F", "#{session_id}|#{session_name}|#{window_id}|#{window_name}|#{window_index}|#{window_layout}|#{window_zoomed_flag}|#{window_active}|#{pane_id}|#{pane_index}|#{pane_active}|#{pane_current_path}|#{pane_current_command}|#{" + backend.WorkspacePathOption + "}",
	}
}

func (q LoadStateQuery) Parse(output string) (LoadStateResult, error) {
	if output == "" {
		return LoadStateResult{}, nil
	}

	currentID := getCurrentSessionID()
	builder := newStateBuilder()

	lines := strings.Split(output, "\n")

	var result LoadStateResult
	if len(lines) >= 2 {
		fmt.Sscanf(strings.TrimSpace(lines[0]), "%d", &result.WindowBaseIndex)
		fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &result.PaneBaseIndex)
	}

	for _, line := range lines[2:] {
		if p, ok := parsePaneLine(line); ok {
			builder.addPane(p, currentID)
		}
	}

	built := builder.result()
	result.Sessions = built.Sessions
	result.Active = built.Active

	return result, nil
}

type paneLine struct {
	sessionID, sessionName, windowID, windowName string
	windowIndex                                  int
	windowLayout                                 string
	windowZoomed                                 bool
	windowActive                                 bool
	paneID                                       string
	paneIndex                                    int
	paneActive                                   bool
	panePath, paneCmd                            string
	workspacePath                                string
}

func parsePaneLine(line string) (paneLine, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return paneLine{}, false
	}

	var p paneLine
	var ok bool
	var windowIndexStr, windowZoomedStr, windowActiveStr, paneIndexStr, paneActiveStr string

	if p.sessionID, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if p.sessionName, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if p.windowID, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if p.windowName, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if windowIndexStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	fmt.Sscanf(windowIndexStr, "%d", &p.windowIndex)
	if p.windowLayout, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if windowZoomedStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.windowZoomed = windowZoomedStr == "1"
	if windowActiveStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.windowActive = windowActiveStr == "1"

	if p.paneID, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if paneIndexStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	fmt.Sscanf(paneIndexStr, "%d", &p.paneIndex)

	if paneActiveStr, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	p.paneActive = paneActiveStr == "1"

	if p.panePath, line, ok = strings.Cut(line, "|"); !ok {
		return paneLine{}, false
	}
	if idx := strings.LastIndex(line, "|"); idx >= 0 {
		p.paneCmd = line[:idx]
		p.workspacePath = line[idx+1:]
	} else {
		p.paneCmd = line
	}
	return p, true
}

type stateBuilder struct {
	sessions map[string]*Session
	active   ActiveContext
}

func newStateBuilder() *stateBuilder {
	return &stateBuilder{sessions: make(map[string]*Session)}
}

func (b *stateBuilder) addPane(p paneLine, currentID string) {
	if p.sessionID == currentID {
		b.active.SessionID = p.sessionID
		b.active.Session = p.sessionName
		if p.windowActive {
			b.active.WindowID = p.windowID
			b.active.Window = p.windowName
			b.active.WindowIndex = p.windowIndex
		}
		if p.paneActive {
			b.active.PaneID = p.paneID
			b.active.Pane = p.paneIndex
			b.active.Path = p.panePath
		}
	}

	sess := b.getOrCreateSession(p.sessionID, p.sessionName)
	if sess.WorkspacePath == "" && strings.TrimSpace(p.workspacePath) != "" {
		sess.WorkspacePath = strings.TrimSpace(p.workspacePath)
	}
	win := b.getOrCreateWindow(sess, p.windowID, p.windowName, p.windowIndex, p.windowLayout, p.panePath)
	win.Panes = append(win.Panes, Pane{ID: p.paneID, Index: p.paneIndex, Path: p.panePath, Command: p.paneCmd, Zoom: p.windowZoomed && p.paneActive})
}

func (b *stateBuilder) getOrCreateSession(id, name string) *Session {
	if sess, ok := b.sessions[id]; ok {
		return sess
	}
	sess := &Session{ID: id, Name: name}
	b.sessions[id] = sess
	return sess
}

func (b *stateBuilder) getOrCreateWindow(sess *Session, id, name string, index int, layout, path string) *Window {
	for i := range sess.Windows {
		if sess.Windows[i].ID == id {
			if layout != "" {
				sess.Windows[i].Layout = layout
			}
			return &sess.Windows[i]
		}
	}
	sess.Windows = append(sess.Windows, Window{ID: id, Name: name, Index: index, Path: path, Layout: layout})
	return &sess.Windows[len(sess.Windows)-1]
}

func (b *stateBuilder) result() LoadStateResult {
	sessions := make([]Session, 0, len(b.sessions))
	for _, session := range b.sessions {
		copy := *session
		sort.Slice(copy.Windows, func(i, j int) bool {
			return copy.Windows[i].Index < copy.Windows[j].Index
		})
		sessions = append(sessions, copy)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Name == sessions[j].Name {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].Name < sessions[j].Name
	})
	return LoadStateResult{Sessions: sessions, Active: b.active}
}

func getCurrentSessionID() string {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return ""
	}
	parts := strings.Split(tmuxEnv, ",")
	if len(parts) < 3 {
		return ""
	}
	return "$" + parts[2]
}
